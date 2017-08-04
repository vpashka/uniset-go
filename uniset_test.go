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
	count	uint32
}

func (c *TestConsumer) EventChannel() chan uniset.SensorMessage {
	return c.eventChannel
}

func (c *TestConsumer) ID() uniset.ObjectID {
	return c.id
}

func (c *TestConsumer) ReadMessage( t *testing.T, sid uniset.SensorID, count int, done chan<- bool) {

	for i := 0; i < count; i++ {
		msg := <-c.eventChannel
		if msg.Id != sid {
			t.Errorf("ReadMessage: sid=%d != %d", msg.Id, sid)
		}
		c.count++
		//t.Log(msg.String())
	}

	done <- true
}

func TestSubscribe(t *testing.T) {

	ui, err := uniset.NewUInterface()
	if err != nil {
		t.Error(err.Error())
	}

	consumer := TestConsumer{make(chan uniset.SensorMessage, 10), 100, 0}

	ui.Subscribe(10, &consumer)

	if ui.Size() != 1 {
		t.Errorf("Subscribe: size %d != %d",ui.Size(),1)
	}

	num := ui.NumberOfCunsumers(10)
	if num != 1 {
		t.Errorf("Subscribe: NumberOfCunsumers=%d != %d",num,1)
	}

	done := make(chan bool)

	go consumer.ReadMessage(t, 10, 2, done)

	sm1 := uniset.SensorMessage{10,10500, time.Now()}
	sm2 := uniset.SensorMessage{11,100500, time.Now()}

	err = ui.SendSensorMessage(sm1)
	if err != nil {
		t.Error("SendSensorMessage error: " + err.Error())
	}

	err = ui.SendSensorMessage(sm1)
	if err != nil {
		t.Error("SendSensorMessage error: " + err.Error())
	}

	err = ui.SendSensorMessage(sm2)
	if err == nil {
		t.Error("SendSensorMessage found Unknown sensor?!" + err.Error())
	}

	<-done

	if consumer.count != 2 {
		t.Errorf("Count of received messages %d != 2", consumer.count)
	}

}
