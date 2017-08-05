package uniset_test

import (
	"sync"
	"testing"
	"time"
	"uniset"
)

func TestUMessage(t *testing.T) {
	sm := uniset.SensorMessage{30, 10500, time.Now()}
	u := uniset.UMessage{}
	u.Push(sm)

	_, ok := u.PopAsTimerMessage()

	if ok {
		t.Errorf("SM --> UM --> TM: ?!!")
	}

	sm2, ok := u.PopAsSensorMessage()

	if sm2.Id != sm.Id {
		t.Errorf("SM --> UM --> SM: Incorrect ID")
	}

	if sm2.Value != sm.Value {
		t.Errorf("SM --> UM --> SM: Incorrect Value")
	}

	if sm2.Timestamp != sm.Timestamp {
		t.Errorf("SM --> UM --> SM: Incorrect Timestamp")
	}

}

// тестовая реализация интерфейса UObject
type TestConsumer struct {
	id            uniset.ObjectID
	ueventChannel chan uniset.UMessage
}

func (c *TestConsumer) ID() uniset.ObjectID {
	return c.id
}

func (c *TestConsumer) UEvent() chan uniset.UMessage {
	return c.ueventChannel
}

func (c *TestConsumer) read(t *testing.T, sid uniset.SensorID, timeout int, wg *sync.WaitGroup) int {

	defer wg.Done()

	finish := time.After(time.Duration(timeout) * time.Millisecond)
	var num int

	for {
		select {
		case msg := <-c.ueventChannel:

			sm, ok := msg.PopAsSensorMessage()
			if !ok {
				t.Errorf("ReadMessage: unknown message")
				continue
			}

			if sm.Id != sid {
				t.Errorf("ReadMessage: Unknown sensorID=%d != %d", sm.Id, sid)
			} else {
				num++
			}
		case <-finish:
			//t.Log("timed out")
			return num

		default:
			time.Sleep(200 * time.Millisecond)
			continue
		}
	}

	return num

	//var num int
	//for i := 0; i < count; i++ {
	//	msg := <-c.eventChannel
	//	if msg.mtype != sid {
	//		t.Errorf("ReadMessage: sid=%d != %d", msg.mtype, sid)
	//	}
	//	num++
	//	//t.Log(msg.String())
	//}
}

func sendMessages(t *testing.T, ui *uniset.UActivator, msg *uniset.SensorMessage, count int, wg *sync.WaitGroup) {

	defer wg.Done()

	for i := 0; i < count; i++ {
		_, err := ui.SendSensorMessage(msg)
		if err != nil {
			t.Error("SendMessages error: " + err.Error())
		}
	}
}

func TestSubscribe(t *testing.T) {

	ui := uniset.GetActivator()

	consumer := TestConsumer{100, make(chan uniset.UMessage, 10)}

	ui.Subscribe(10, &consumer)

	if ui.Size() != 1 {
		t.Errorf("Subscribe: size %d != %d", ui.Size(), 1)
	}

	num := ui.NumberOfConsumers(10)
	if num != 1 {
		t.Errorf("Subscribe: NumberOfCunsumers=%d != %d", num, 1)
	}

	sm1 := uniset.SensorMessage{10, 10500, time.Now()}
	sm2 := uniset.SensorMessage{11, 100500, time.Now()}

	msgCount := 3
	timeout := 800
	var wg sync.WaitGroup

	wg.Add(2)

	go sendMessages(t, ui, &sm1, msgCount, &wg)

	_, err := ui.SendSensorMessage(&sm2)
	if err == nil {
		t.Error("SendSensorMessage found Unknown sensor?!" + err.Error())
	}

	rnum := consumer.read(t, sm1.Id, timeout, &wg)

	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}
}

func subscribe(ui *uniset.UActivator, clist *[]*TestConsumer, sid uniset.SensorID, wg *sync.WaitGroup) {

	defer wg.Done()
	for _, c := range *clist {
		ui.Subscribe(sid, c)
	}
}

func TestMultithreadSubscribe(t *testing.T) {

	ui := uniset.GetActivator()

	conslist := make([]*TestConsumer, 10, 10)

	var id uniset.ObjectID = 100
	for i := 0; i < len(conslist); i++ {
		conslist[i] = &TestConsumer{id, make(chan uniset.UMessage, 10)}
		id++
	}

	var wg sync.WaitGroup
	wg.Add(2)

	l1 := conslist[0:5]
	l2 := conslist[5:len(conslist)]

	go subscribe(ui, &l1, 30, &wg)
	go subscribe(ui, &l2, 30, &wg)

	wg.Wait()

	if ui.NumberOfConsumers(30) != len(conslist) {
		t.Errorf("Subscribe: size %d != %d", ui.NumberOfConsumers(30), len(conslist))
	}

	sm := uniset.SensorMessage{30, 10500, time.Now()}

	msgCount := 3
	timeout := 800

	var wg2 sync.WaitGroup
	wg2.Add(12)

	go sendMessages(t, ui, &sm, msgCount, &wg2)
	go sendMessages(t, ui, &sm, msgCount, &wg2)

	rnum := (*conslist[0]).read(t, 30, timeout, &wg2)
	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}

	rnum = (*conslist[1]).read(t, 30, timeout, &wg2)
	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}

	wg2.Done()
}

func TestUWorking(t *testing.T) {

	ui := uniset.GetActivator()

	clist := make([]*TestConsumer, 3, 3)

	var id uniset.ObjectID = 100
	for i := 0; i < len(clist); i++ {
		clist[i] = &TestConsumer{id, make(chan uniset.UMessage, 10)}
		id++
	}

	err := ui.Run()
	if err != nil {
		t.Errorf("UActivator: Run error: %s", err.Error())
	}

	ui.Terminate()

	if ui.IsActive() {
		t.Error("UActivator: is active after terminate!")
	}

	ui.Run()
	if !ui.IsActive() {
		t.Error("UActivator: NOT active after run!")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var sid uniset.SensorID = 10
	msgCount := 3

	err = ui.Subscribe(sid, clist[0])
	if err != nil {
		t.Errorf("Subscribe error: %s", err)
	}

	rnum := (*clist[0]).read(t, sid, 6000, &wg)
	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}

	//wg.Wait()
}
