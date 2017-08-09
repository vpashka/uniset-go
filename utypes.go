// Основные типа для пакета uniset
package uniset

import (
	"fmt"
	"time"
)

type SensorID int64
type ObjectID int64

const DefaultObjectID int64 = -1

type SensorMessage struct {
	Id        SensorID
	Value     int64
	Timestamp time.Time
}

type TimerMessage struct {
	Id uint32
	//Timestamp time.Time
}

type UMessage struct {
	msg interface{}
	//Timestamp time.Time
}

// Интерфейс который должны реализовать объекты
// желающие подписаться на uniset-события
type UObject interface {
	UEvent() chan UMessage
	ID() ObjectID
}

func (u *UMessage) Push(m interface{}) {
	u.msg = m
}

func (u *UMessage) Pop() interface{} {
	return u.msg
}

func (u *UMessage) PopAsSensorMessage() (*SensorMessage, bool) {

	switch u.msg.(type) {

	case SensorMessage:
		sm := u.msg.(SensorMessage)
		return &sm, true

	case *SensorMessage:
		sm := u.msg.(*SensorMessage)
		return sm, true
	}

	return nil, false
}

func (u *UMessage) PopAsTimerMessage() (*TimerMessage, bool) {
	switch u.msg.(type) {

	case TimerMessage:
		tm := u.msg.(TimerMessage)
		return &tm, true

	case *TimerMessage:
		tm := u.msg.(*TimerMessage)
		return tm, true
	}

	return nil, false
}

func (m *SensorMessage) String() string {
	return fmt.Sprintf("id: %d value: %d", m.Id, m.Value)
}
