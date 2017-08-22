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
type UObject interface {
	UEvent() chan<- UMessage
	UCommand() <-chan UMessage
	ID() ObjectID
}

// ----------------------------------------------------------------------------------
// Интерфейс для сообщений "обёртка"
type UMessage struct {
	Msg interface{}
	//Timestamp time.Time
}

// ----------------------------------------------------------------------------------
// сообщение о том, что объект успешно активирован и может начать работу
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
// связывание sensor id и bool-поля структуры
// для формирования списков входов и выходов
type BoolValue struct {
	Sid  *SensorID
	Val  *bool
	prev bool
}

func NewBoolValue(sid *SensorID, val *bool) *BoolValue {
	return &BoolValue{sid, val, *val}
}

// ----------------------------------------------------------------------------------
// связывание sensor id и int64-поля структуры
// для формирования списков входов и выходов
type Int64Value struct {
	Sid  *SensorID
	Val  *int64
	prev int64
}

func NewInt64Value(sid *SensorID, val *int64) *Int64Value {
	return &Int64Value{sid, val, *val}
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
