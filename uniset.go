// Интерфейс для работы с uniset-датчиками
// Основная идея:  подписка на уведомления об изменении датиков
// Пользователь реализует интерфейс Consumer, предоставляющий канал
// для посылки в него SensorMessage
// Чтобы получать уведомления пользователь при помощи функции Subscribe
// подписывается на события, указывая идентификатор желаемого датчика
package uniset

import (
	"time"
	"fmt"
	"errors"
)

type SensorID int32
type ObjectID int32

type SensorMessage struct {
	Id        SensorID
	Value     int32
	Timestamp time.Time
}

func (m *SensorMessage) String() string {
	return fmt.Sprintf("id: %d value: %d", m.Id, m.Value)
}

type Consumer interface {
	EventChannel() chan SensorMessage
	ID() ObjectID
}

type consumerList struct {
	list []Consumer
}

func newConsumerList(sz int, cap int) (consumerList, error) {
	lst := consumerList{}
	return lst, nil
}

type askMap map[SensorID]consumerList

type UInterface struct {
	// map[sensorID] = consumer list
	askmap askMap
}

func NewUInterface() (UInterface, error) {
	ui := UInterface{}
	ui.askmap = make(askMap)
	return ui, nil
}

func (ui *UInterface) Size() int {
	return len(ui.askmap)
}

func (ui *UInterface) NumberOfCunsumers(sid SensorID) int {
	v, found := ui.askmap[sid]
	if found {
		return len(v.list)
	}

	return 0
}

func (ui *UInterface) addConsumer(l *consumerList, cons Consumer) (err error) {

	for _, c := range l.list {
		if c.ID() == cons.ID() {
			return nil
		}
	}

	l.list = append(l.list, cons)
	return nil
}

func (ui *UInterface) Subscribe(sid SensorID, cons Consumer) (err error) {
	v, found := ui.askmap[sid]
	if found {
		ui.addConsumer(&v, cons)
		return nil
	}

	lst, err := newConsumerList(1, 1)

	if err != nil {
		return err
	}

	ui.addConsumer(&lst, cons)
	ui.askmap[sid] = lst
	return nil
}

func (ui *UInterface) SendSensorMessage(sm *SensorMessage) error {

	v, found := ui.askmap[sm.Id]
	if !found {
		err := fmt.Sprintf("Not found: sensorID '%d'", sm.Id)
		return errors.New(err)
	}

	for _, c := range v.list {
		c.EventChannel() <- (*sm)
	}

	return nil
}
