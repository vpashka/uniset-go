package uniset_test

import (
	"sync"
	"testing"
	"time"
	"uniset"
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

func (c *TestConsumer) read(t *testing.T, sid uniset.SensorID, timeout int, wg *sync.WaitGroup) int {

	defer wg.Done()

	finish := time.After(time.Duration(timeout)*time.Millisecond)
	var num int

	for {
		select {
		case msg := <-c.eventChannel:
			if msg.Id != sid {
				t.Errorf("ReadMessage: sid=%d != %d", msg.Id, sid)
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
	//	if msg.Id != sid {
	//		t.Errorf("ReadMessage: sid=%d != %d", msg.Id, sid)
	//	}
	//	num++
	//	//t.Log(msg.String())
	//}
}

func sendMessages(t *testing.T, ui *uniset.UInterface, msg *uniset.SensorMessage, count int, wg *sync.WaitGroup) {

	defer wg.Done()

	for i := 0; i < count; i++ {
		_,err := ui.SendSensorMessage(msg)
		if err != nil {
			t.Error("SendMessages error: " + err.Error())
		}
	}
}

func TestSubscribe(t *testing.T) {

	ui := uniset.NewUInterface()

	consumer := TestConsumer{make(chan uniset.SensorMessage, 10), 100}

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

func subscribe(ui *uniset.UInterface, clist *[]*TestConsumer, sid uniset.SensorID, wg *sync.WaitGroup) {

	defer wg.Done()
	for _, c := range *clist {
		ui.Subscribe(sid, c)
	}
}

func TestMultithreadSubscribe(t *testing.T) {

	ui := uniset.NewUInterface()

	conslist := make([]*TestConsumer, 10, 10)

	var id uniset.ObjectID = 100
	for i := 0; i < len(conslist); i++ {
		conslist[i] = &TestConsumer{make(chan uniset.SensorMessage, 10), id}
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

	ui := uniset.NewUInterface()

	clist := make([]*TestConsumer, 3, 3)

	var id uniset.ObjectID = 100
	for i := 0; i < len(clist); i++ {
		clist[i] = &TestConsumer{make(chan uniset.SensorMessage, 10), id}
		id++
	}

	err := ui.Run()
	if err != nil {
		t.Errorf("UInterface: Run error: %s", err.Error())
	}

	ui.Terminate()

	if ui.IsActive(){
		t.Error("UInterface: is active after terminate!")
	}

	ui.Run()
	if !ui.IsActive(){
		t.Error("UInterface: NOT active after run!")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var sid uniset.SensorID = 10
	msgCount := 3

	err = ui.Subscribe(sid,clist[0])
	if err != nil {
		t.Errorf("Subscribe error: %s", err)
	}

	rnum := (*clist[0]).read(t, sid, 6000, &wg)
	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}

	//wg.Wait()
}
