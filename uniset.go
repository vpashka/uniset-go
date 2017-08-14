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
// \todo Механизм чтения привязок из configue.xml (идея: возвращать json и парсить его на уровне go)
// \todo Продумать возможно ли генерирование на основе формата uniset-codegen (uniset-codegen-go)
package uniset

import (
	"errors"
	"strconv"
	"uniset_internal_api"
)

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
func DoUpdateBoolInputs(inputs *[]*BoolValue, sm *SensorEvent) {

	for _, s := range *inputs {
		if *s.Sid == sm.Id {
			if sm.Value == 0 {
				*s.Val = false
			} else {
				*s.Val = true
			}

			s.prev = *s.Val
		}
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// Обновление значений по SensorEvent
func DoUpdateAnalogInputs(inputs *[]*Int64Value, sm *SensorEvent) {

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
func DoUpdateAnalogOutputs(outs *[]*Int64Value, cmdchannel chan<- UMessage) {

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
// обновление bool-значений в SM
// Проходим по списку и если значение поменялось, относительно предыдущего
// обновляем в SM (SetValue)
func DoUpdateBoolOutputs(outs *[]*BoolValue, cmdchannel chan<- UMessage) {

	for _, s := range *outs {
		if s.prev != *s.Val {
			var val int64
			if *s.Val {
				val = 1
			}
			SetValue(cmdchannel, *s.Sid, val)
			// возможно обновлять prev, стоит после подтверждения от UProxy
			// но пока для простосты обновляем сразу
			s.prev = *s.Val
		}
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// заказ bool датчиков
func DoAskSensorsBool(inputs *[]*BoolValue, cmdchannel chan<- UMessage) {

	for _, s := range *inputs {
		AskSensor(cmdchannel, *s.Sid)
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// заказ аналоговых датчиков
func DoAskSensorsAnalog(inputs *[]*Int64Value, cmdchannel chan<- UMessage) {

	for _, s := range *inputs {
		AskSensor(cmdchannel, *s.Sid)
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// обновление аналоговых значений из SM
func DoReadAnalogInputs(inputs *[]*Int64Value) {

	for _, s := range *inputs {

		ret := uniset_internal_api.GetValue(int64(*s.Sid))

		if ret.GetOk() {
			*s.Val = ret.GetValue()
			s.prev = *s.Val
		}
	}
}

// ----------------------------------------------------------------------------------
// обобщённая вспомогательная функция
// обновление bool-значений из SM
func DoReadBoolInputs(inputs *[]*BoolValue) {

	for _, s := range *inputs {

		ret := uniset_internal_api.GetValue(int64(*s.Sid))

		if ret.GetOk() {
			if ret.GetValue() == 0 {
				*s.Val = false
			} else {
				*s.Val = true
			}
			s.prev = *s.Val
		}
	}
}
// ----------------------------------------------------------------------------------
