package uniset_test

import (
	"testing"
	"uniset"
	"time"
)

// тестовый consumer
type TestConsumer struct {
	eventChannel chan uniset.SensorMessage
	id           uniset.ObjectID
}

func (c *TestConsumer) EventChannel() chan uniset.SensorMessage {
	return c.eventChannel
}

func (c *TestConsumer) ID() uniset.ObjectID {
	return c.id
}

func (c *TestConsumer) ReadMessage(t *testing.T, sid uniset.SensorID, count int) int {

	var num int
	for i := 0; i < count; i++ {
		msg := <-c.eventChannel
		if msg.Id != sid {
			t.Errorf("ReadMessage: sid=%d != %d", msg.Id, sid)
		}
		num++
		//t.Log(msg.String())
	}

	return num
}

func sendMessages(t *testing.T, ui *uniset.UInterface, msg *uniset.SensorMessage, count int) {
	for i := 0; i < count; i++ {
		err := ui.SendSensorMessage(msg)
		if err != nil {
			t.Error("SendMessages error: " + err.Error())
		}
	}
}

func TestSubscribe(t *testing.T) {

	ui, err := uniset.NewUInterface()
	if err != nil {
		t.Error(err.Error())
	}

	consumer := TestConsumer{make(chan uniset.SensorMessage, 10), 100}

	ui.Subscribe(10, &consumer)

	if ui.Size() != 1 {
		t.Errorf("Subscribe: size %d != %d", ui.Size(), 1)
	}

	num := ui.NumberOfCunsumers(10)
	if num != 1 {
		t.Errorf("Subscribe: NumberOfCunsumers=%d != %d", num, 1)
	}

	sm1 := uniset.SensorMessage{10, 10500, time.Now()}
	sm2 := uniset.SensorMessage{11, 100500, time.Now()}

	msgCount := 3

	go sendMessages(t, &ui, &sm1, msgCount)

	err = ui.SendSensorMessage(&sm2)
	if err == nil {
		t.Error("SendSensorMessage found Unknown sensor?!" + err.Error())
	}

	rnum := consumer.ReadMessage(t, sm1.Id, msgCount)

	if rnum != msgCount {
		t.Errorf("Count of received messages %d != 2", rnum)
	}

}
