// Интерфейс для работы с uniset-датчиками
// Основная идея:  подписка на уведомления об изменении датиков
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
// После чего UProxy уже забирает у него сообщения об изменении датчиков и через go-каналы рассылает заказчикам.
// Запуск и взаимодейтсвие осуществляться через go-обёртку (uniset_internal_api) входящую в состав uniset
//
// В общем случае, достаточно одного UProxy объекта на программу. Но тем не менее он не сделан singleton-ом
// чтобы не ограничивать возможности
//
// \todo Обработка аргументов командной строки
package uniset

import (
	"errors"
	"strconv"
	"uniset_internal_api"
	"fmt"
	"encoding/json"
)

type UConfig struct {
	Config []UProp `json: "config"`
}

type UProp struct {
	Prop string  `json: "prop"`
	Value string `json: "value"`
}

type UInterface struct {
}

func NewUInterface(confile string, uniset_port int) (*UInterface, error) {

	ui := UInterface{}

	params := uniset_internal_api.ParamsInst()
	params.Add_str("--confile")
	params.Add_str(confile)

	//params.Add_str("--ulog-add-levels")
	//params.Add_str("any")
	//
	//params.Add_str("--UProxy1-log-add-levels")
	//params.Add_str("any")

	if uniset_port > 0 {
		uport := strconv.Itoa(uniset_port)
		params.Add_str("--uniset-port")
		params.Add_str(uport)
	}

	err := uniset_internal_api.Uniset_init_params(params, confile)
	if !err.GetOk() {
		return nil, errors.New(err.GetErr())
	}

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
// \todo Сделать обработку аргументов командной строки (flag)
func Init(confile string, uniset_port int) {

	params := uniset_internal_api.ParamsInst()
	params.Add_str("--confile")
	params.Add_str(confile)

	if uniset_port > 0 {
		uport := strconv.Itoa(uniset_port)
		params.Add_str("--uniset-port")
		params.Add_str(uport)
	}

	err := uniset_internal_api.Uniset_init_params(params, confile)
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
func PropValueByName( cfg *UConfig, propname string, defval string ) string {

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
	bytes := []byte(jstr)

	err := json.Unmarshal(bytes, &cfg)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("(GetConfigParamsByName): error: %s", err))
	}

	return &cfg, nil;
}
// ----------------------------------------------------------------------------------
