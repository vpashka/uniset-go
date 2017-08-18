// Интерфейс для работы с uniset-системами
// ( см. https://habrahabr.ru/post/278535/ )
// Основная идея: "PUB-SUB" подписка на уведомления об изменении датиков
// Пользователь реализует интерфейс UObject, предоставляющий канал
// для посылки в него SensorEvent
// Чтобы получать уведомления пользователь при помощи функции doAskSensor
// подписывается на события, указывая идентификатор желаемого датчика
// Сама работа с заказом и выставление датчиков осуществляется через канал для команд,
// предоставляемый каждым UObject.
// Итого: UObject должен иметь два канала (для получения сообщений и для взаимодействия с UProxy),
// а также уникальный ID.
//
// UProxy является фасадом для c++ объекта, который запускает в системе поток, для чтения дачиков.
// После чего UProxy забирает у него сообщения об изменении датчиков (в отдельной go-рутине)
// и через go-каналы рассылает заказчикам.
// Для взаимодействия с c++ объектом используется uniset_internal_api входящий в состав uniset
//
// В общем случае, достаточно одного UProxy объекта на программу. Но тем не менее он не сделан singleton-ом
// чтобы не ограничивать возможности
//
// До начала работы необходимо обязательно вызвать функцию uniset.Init() которая проинициализирует c++-часть
// -------------------
// Для того, чтобы просто читать или писать датчики (без подписки на события) достаточно использовать
// uniset.UInterace() и UObject создавать не нужно.
// -------------------
// Т.к. основная работа UObject-ов это работа с датчиками, то для того, чтобы проще было писать объекты
// реализующие нужную логику, создан генератор кода uniset2-codegen-go.
// Его суть заключается в том, что в специальном xml-файле описываются входы и выходы объекта, по которым
// генерируется базовая go-структура, которую достаточно просто встроить к себе в объект (как анонимное поле).
// При этом объявленные входы и выходы объекта становяться доступны для использования как поля структуры.
// про формат xml-файла можно (будет) почтитать здесь http://wiki.etersoft.ru/UniSet2/docs/page__codegen_go.html
// Пример: https://github.com/....uniset-example-go
// -------------------
package uniset

import (
	"errors"
	"strconv"
	"uniset_internal_api"
	"fmt"
	"encoding/json"
	"os"
)

type UConfig struct {
	Name string // имя объекта для которого получены настройки
	Config []UProp `json: "config"`
}

type UProp struct {
	Prop string  `json: "prop"`
	Value string `json: "value"`
}

// Простой интерфейс для работы с uniset-датчиками (get/set)
type UInterface struct {
}

func NewUInterface(confile string, uniset_port int) (*UInterface, error) {

	if !uniset_internal_api.IsUniSetInitOK() {
		return nil, errors.New("Not uniset init...")
	}

	ui := UInterface{}

	return &ui, nil
}
// ----------------------------------------------------------------------------------
func (ui *UInterface) SetValue(sid SensorID, value int64, supplier ObjectID) error {

	err := uniset_internal_api.SetValue(int64(sid), value, int64(supplier))

	if !err.GetOk() {
		return errors.New(err.GetErr())
	}
	return nil
}
// ----------------------------------------------------------------------------------
func (ui *UInterface) GetValue(sid SensorID) (int64, error) {
	err := uniset_internal_api.GetValue(int64(sid))
	if !err.GetOk() {
		return 0, errors.New(err.GetErr())
	}

	return err.GetValue(), nil
}

// ----------------------------------------------------------------------------------
// глобальная инициализация
func Init(confile string) {

	cmdline := uniset_internal_api.ParamsInst()

	for _, p := range os.Args {
		cmdline.Add_str(p)
	}

	err := uniset_internal_api.Uniset_init_params(cmdline, confile)
	if !err.GetOk() {
		panic(err.GetErr())
	}
}
// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция - обёртка для заказа датчиков
func AskSensor(ch chan<- UMessage, sid SensorID) {

	ch <- UMessage{&AskCommand{sid, false}}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция - обёртка для выставления значения
func SetValue(ch chan<- UMessage, sid SensorID, value int64) {

	ch <- UMessage{&SetValueCommand{sid, value, false}}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// Обновление значений по SensorEvent
func DoUpdateInputs(inputs *[]*Int64Value, sm *SensorEvent) {

	for _, s := range *inputs {
		if *s.Sid == sm.Id {
			*s.Val = sm.Value
			s.prev = sm.Value
		}
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// обновление аналоговых-значений в SM
// Проходим по списку и если значение поменялось, относительно предыдущего
// обновляем в SM (SetValue)
func DoUpdateOutputs(outs *[]*Int64Value, cmdchannel chan<- UMessage) {

	for _, s := range *outs {
		if s.prev != *s.Val {

			SetValue(cmdchannel, *s.Sid, *s.Val)
			// возможно обновлять prev, стоит после подтверждения от UProxy
			// но пока для простосты обновляем сразу
			s.prev = *s.Val
		}
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// заказ аналоговых датчиков
func DoAskSensors(inputs *[]*Int64Value, cmdchannel chan<- UMessage) {

	for _, s := range *inputs {
		AskSensor(cmdchannel, *s.Sid)
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// обновление аналоговых значений из SM
func DoReadInputs(inputs *[]*Int64Value) {

	for _, s := range *inputs {

		ret := uniset_internal_api.GetValue(int64(*s.Sid))

		if ret.GetOk() {
			*s.Val = ret.GetValue()
			s.prev = *s.Val
		}
	}
}

// ----------------------------------------------------------------------------------
// получить аргумент из командной строки
func GetArgParam( param string, defval string ) string {

	argc := len(os.Args)

	for i:=0; i<argc; i++ {

		if os.Args[i] == param {
			if (i+1) < argc {
				return os.Args[i+1]
			}
			panic(fmt.Sprintf("(uniset.GetArgParam): error: required argument for %s",param))
		}
	}

	return defval
}
// ----------------------------------------------------------------------------------
// функция получения значения для указанного свойства.
// Выбор делается по следующему приоритету:
// если задан аргумент в командной строке, то выбирается он
// если нет, смотрим config, если там тоже нет, то возвращаем defval
// При этом в командной строке ищется значение --name-propname
//
func PropValueByName( cfg *UConfig, propname string, defval string ) string {

	if len(propname) == 0 {
		return defval
	}

	p := GetArgParam(fmt.Sprintf("--%s-%s",cfg.Name,propname),"")

	if( len(p)!=0 ){
		return p;
	}

	for _, v := range cfg.Config {

		if v.Prop == propname {
			return v.Value
		}
	}

	return defval
}
// ----------------------------------------------------------------------------------
func InitInt32( cfg *UConfig, propname string, defval string ) int32 {

	sval := PropValueByName(cfg,propname,defval)
	if len(sval) == 0 {
		return 0
	}

	i, err := strconv.ParseInt(sval, 10, 32)
	if err != nil{
		panic(fmt.Sprintf("(Init_Imitator_SK): convert type '%s' error: %s", propname, err))
	}

	return int32(i)
}
// ----------------------------------------------------------------------------------
func InitInt64( cfg *UConfig, propname string, defval string ) int64 {

	sval := PropValueByName(cfg,propname,defval)
	if len(sval) == 0 {
		return 0
	}

	i, err := strconv.ParseInt(sval, 10, 64)
	if err != nil{
		panic(fmt.Sprintf("(Init_Imitator_SK): convert type '%s' error: %s", propname, err))
	}

	return i
}
// ----------------------------------------------------------------------------------
func InitFloat32( cfg *UConfig, propname string, defval string ) float32 {

	sval := PropValueByName(cfg,propname,defval)
	if len(sval) == 0 {
		return 0.0
	}
	i, err := strconv.ParseFloat(sval, 32)
	if err != nil{
		panic(fmt.Sprintf("(Init_Imitator_SK): convert type '%s' error: %s", propname, err))
	}

	return float32(i)
}
// ----------------------------------------------------------------------------------
func InitFloat64( cfg *UConfig, propname string, defval string ) float64 {

	sval := PropValueByName(cfg,propname,defval)
	if len(sval) == 0 {
		return 0.0
	}

	i, err := strconv.ParseFloat(sval, 64)
	if err != nil{
		panic(fmt.Sprintf("(Init_Imitator_SK): convert type '%s' error: %s", propname, err))
	}

	return i
}
// ----------------------------------------------------------------------------------
func InitBool( cfg *UConfig, propname string, defval string ) bool {

	sval := PropValueByName(cfg,propname,defval)
	if len(sval) == 0 {
		return false
	}

	i, err := strconv.ParseBool(sval)
	if err != nil{
		panic(fmt.Sprintf("(Init_Imitator_SK): convert type '%s' error: %s", propname, err))
	}

	return i
}
// ----------------------------------------------------------------------------------
func InitString( cfg *UConfig, propname string, defval string ) string {

	return PropValueByName(cfg,propname,defval)
}
// ----------------------------------------------------------------------------------
func InitSensorID( cfg *UConfig, propname string, defval string ) SensorID {

	//fmt.Printf("init sensorID %s ret=%d\n",propname,uniset_internal_api.GetSensorID(PropValueByName(cfg,propname)))
	return SensorID(uniset_internal_api.GetSensorID(PropValueByName(cfg,propname,defval)))
}
// ----------------------------------------------------------------------------------
func InitObjectID( cfg *UConfig, propname string, defval string ) ObjectID {

	return ObjectID(uniset_internal_api.GetObjectID(PropValueByName(cfg,propname,defval)))
}
// ----------------------------------------------------------------------------------
func GetConfigParamsByName(name string, section string) (*UConfig, error) {

	jstr := uniset_internal_api.GetConfigParamsByName(name, section)

	if len(jstr) == 0 {
		return nil, errors.New(fmt.Sprintf("(GetConfigParamsByName): Not found config section <%s name='%s'..> read error", section, name))
	}

	cfg := UConfig{}
	cfg.Name = name;
	bytes := []byte(jstr)

	err := json.Unmarshal(bytes, &cfg)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("(GetConfigParamsByName): error: %s", err))
	}

	return &cfg, nil;
}
// ----------------------------------------------------------------------------------
