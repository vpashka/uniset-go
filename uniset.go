// Интерфейс для работы с uniset-датчиками
// Основная идея:  подписка на уведомления об изменении датиков
// Пользователь реализует интерфейс UObjecter, предоставляющий канал
// для посылки в него SensorEvent
// Чтобы получать уведомления пользователь при помощи функции doAskSensor
// подписывается на события, указывая идентификатор желаемого датчика
//
// Планируется, что UProxy будет Singleton-ом, т.к. будет в системе (c++) запускать один общий
// UniSetObject, который и будет (в отдельном системном потоке!) заказывать и получать
// датчики, а UProxy уже будет забирать у него сообщения и через go-каналы рассылать заказчикам.
// Запуск будет осуществляться через go-обёртку (uniset-api) входящую в состав uniset
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

func (ui *UInterface) SetValue(sid SensorID, value int64, supplier ObjectID) error {

	err := uniset_internal_api.SetValue(int64(sid), value, int64(supplier))

	if !err.GetOk() {
		return errors.New(err.GetErr())
	}
	return nil
}

func (ui *UInterface) GetValue(sid SensorID) (int64, error) {
	err := uniset_internal_api.GetValue(int64(sid))
	if !err.GetOk() {
		return 0, errors.New(err.GetErr())
	}

	return err.GetValue(), nil
}
