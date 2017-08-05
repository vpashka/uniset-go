// Основные типа для пакета uniset
package uniset

import (
	"time"
)

type SensorID int32
type ObjectID int32

const DefaultObjectID int32 = -1

const (
	SensorMessageType = iota
	TimerMessageType
)

type SensorMessage struct {
	Id        SensorID
	Value     int32
	Timestamp time.Time
}

type TimerMessage struct {
	Id uint32
	//Timestamp time.Time
}

type UMessage struct {
	mtype uint32
	msg   interface{}
	//Timestamp time.Time
}

func (u *UMessage) Push(id uint32, m interface{}) {
	u.mtype = id
	u.msg = m
}

func (u *UMessage) Pop() interface{} {
	return u.msg
}

func (u *UMessage) PopSensorMessage() (SensorMessage, bool) {
	//if u.mtype == SensorMessageType {
	//	return u.msg.(SensorMessage), true
	//}

	switch u.msg.(type) {

	case SensorMessage:
		return u.msg.(SensorMessage), true
	}

	return SensorMessage{}, false
}

func (u *UMessage) PopTimerMessage() (TimerMessage, bool) {
	if u.mtype == TimerMessageType {
		return u.msg.(TimerMessage), true
	}

	return TimerMessage{}, false
}

type UObject interface {
	EventChannel() chan SensorMessage
	UEvent() chan UMessage
	ID() ObjectID
}
