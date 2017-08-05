// Основные типа для пакета uniset
package uniset

import (
	"time"
)

type SensorID int32
type ObjectID int32

const DefaultObjectID int32 = -1

type SensorMessage struct {
	Id        SensorID
	Value     int32
	Timestamp time.Time
}

type TimerMessage struct {
	Id        uint32
	//Timestamp time.Time
}
