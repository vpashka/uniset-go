package uniset

import (
	"time"
)

type SensorMessage struct {
	Value int32
	Timestamp time.Time
}

type Consumer struct {
	Name  string
	MsgChannel <-chan SensorMessage
}

type UInterface struct {
	askmap map[string]Consumer
}

func (ui *UInterface) ask(consumerID string, eventChannel <-chan SensorMessage) {
	// \todo добавить проверку на существование заказчика
	var c = Consumer{consumerID,eventChannel }
	ui.askmap[consumerID] = c
}


func MyFirstFunc() int {
	return 42
}
