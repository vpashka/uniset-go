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
// имеет две вспомогательные функции Push(msg) и Pop(msg)
type UMessage struct {
	msg interface{}
	//Timestamp time.Time
}

// ----------------------------------------------------------------------------------
// сообщение о том, что объект успешно активирован
type ActivateEvent struct {
	Id ObjectID
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
	Value  int32
	Result bool
}

// ----------------------------------------------------------------------------------
func (u *UMessage) Push(m interface{}) {
	u.msg = m
}

func (u *UMessage) Pop() interface{} {
	return u.msg
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsSensorEvent() (*SensorEvent, bool) {

	switch u.msg.(type) {

	case SensorEvent:
		sm := u.msg.(SensorEvent)
		return &sm, true

	case *SensorEvent:
		sm := u.msg.(*SensorEvent)
		return sm, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsAskCommand() (*AskCommand, bool) {
	switch u.msg.(type) {

	case AskCommand:
		c := u.msg.(AskCommand)
		return &c, true

	case *AskCommand:
		c := u.msg.(*AskCommand)
		return c, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsSetValueCommand() (*SetValueCommand, bool) {
	switch u.msg.(type) {

	case SetValueCommand:
		c := u.msg.(SetValueCommand)
		return &c, true

	case *SetValueCommand:
		c := u.msg.(*SetValueCommand)
		return c, true
	}

	return nil, false
}

// ----------------------------------------------------------------------------------
func (u *UMessage) PopAsActivateEvent() (*ActivateEvent, bool) {
	switch u.msg.(type) {

	case ActivateEvent:
		c := u.msg.(ActivateEvent)
		return &c, true

	case *ActivateEvent:
		c := u.msg.(*ActivateEvent)
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
