// Основные типа для пакета uniset
package uniset

import (
	"fmt"
	"time"
	"uniset_internal_api"
)

type SensorID int64
type ObjectID int64

const DefaultObjectID int64 = -1

// ----------------------------------------------------------------------------------
// Интерфейс который должны реализовать объекты
// желающие подписаться на uniset-события
type UObjecter interface {
	UEvent() chan<- UMessage
	UCommand() <-chan UMessage
	ID() ObjectID
}

// ----------------------------------------------------------------------------------
// Интерфейс для сообщений "обёртка"
// имеет две вспомогательные функции Push(Msg) и Pop(Msg)
type UMessage struct {
	Msg interface{}
	//Timestamp time.Time
}

// ----------------------------------------------------------------------------------
// сообщение о том, что объект успешно активирован
type ActivateEvent struct {
}

// ----------------------------------------------------------------------------------
// сообщение о завершении работы
type FinishEvent struct {
}

// ----------------------------------------------------------------------------------
type SensorEvent struct {
	Id        SensorID
	Value     int64
	Timestamp time.Time
}

// ----------------------------------------------------------------------------------
type AskCommand struct {
	Id     SensorID
	Result bool
}

// ----------------------------------------------------------------------------------
type SetValueCommand struct {
	Id     SensorID
	Value  int64
	Result bool
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsSensorEvent() (*SensorEvent, bool) {

	switch u.Msg.(type) {

	case SensorEvent:
		sm := u.Msg.(SensorEvent)
		return &sm, true

	case *SensorEvent:
		sm := u.Msg.(*SensorEvent)
		return sm, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsAskCommand() (*AskCommand, bool) {
	switch u.Msg.(type) {

	case AskCommand:
		c := u.Msg.(AskCommand)
		return &c, true

	case *AskCommand:
		c := u.Msg.(*AskCommand)
		return c, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsSetValueCommand() (*SetValueCommand, bool) {
	switch u.Msg.(type) {

	case SetValueCommand:
		c := u.Msg.(SetValueCommand)
		return &c, true

	case *SetValueCommand:
		c := u.Msg.(*SetValueCommand)
		return c, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsActivateEvent() (*ActivateEvent, bool) {
	switch u.Msg.(type) {

	case ActivateEvent:
		c := u.Msg.(ActivateEvent)
		return &c, true

	case *ActivateEvent:
		c := u.Msg.(*ActivateEvent)
		return c, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsFinishEvent() (*FinishEvent, bool) {
	switch u.Msg.(type) {

	case FinishEvent:
		c := u.Msg.(FinishEvent)
		return &c, true

	case *FinishEvent:
		c := u.Msg.(*FinishEvent)
		return c, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (m *SensorEvent) String() string {
	return fmt.Sprintf("id: %d value: %d", m.Id, m.Value)
}

// ----------------------------------------------------------------------------------
func makeSensorEvent(m uniset_internal_api.ShortIOInfo) *SensorEvent {
	var msg SensorEvent
	msg.Id = SensorID(m.GetId())
	msg.Value = m.GetValue()
	msg.Timestamp = time.Unix(m.GetTv_sec(), m.GetTv_nsec())
	return &msg
}

// ----------------------------------------------------------------------------------
