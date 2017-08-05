package uniset_test

import (
	"sync"
	"testing"
	"time"
	"uniset"
)

// -----------------------------------------------------------------------------
// тестовая реализация интерфейса UObject
type TestObject struct {
	id            uniset.ObjectID
	ueventChannel chan uniset.UMessage
}

func (c *TestObject) ID() uniset.ObjectID {
	return c.id
}

func (c *TestObject) UEvent() chan uniset.UMessage {
	return c.ueventChannel
}

func makeUObjects(beginID uniset.ObjectID) []*TestObject {

	conslist := make([]*TestObject, 10, 10)

	var id uniset.ObjectID = beginID
	for i := 0; i < len(conslist); i++ {
		conslist[i] = &TestObject{id, make(chan uniset.UMessage, 10)}
		id++
	}

	return conslist
}

// ----------------------------------------------------------------
// Преобразование сообщений UMessage <--> SensorMessage
// ----------------------------------------------------------------
func TestUMessage2SensorMessage(t *testing.T) {

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

// ----------------------------------------------------------------
// Преобразование сообщений UMessage <--> TimerMessage
// ----------------------------------------------------------------
func TestUMessage2TimerMessage(t *testing.T) {

	tm := uniset.TimerMessage{1}
	u := uniset.UMessage{}
	u.Push(tm)

	_, ok := u.PopAsSensorMessage()

	if ok {
		t.Errorf("TM --> UM --> SM: ?!!")
	}

	tm2, ok := u.PopAsTimerMessage()

	if tm2.Id != tm.Id {
		t.Errorf("TM --> UM --> TM: Incorrect ID")
	}
}

// ----------------------------------------------------------------
func (c *TestObject) read(t *testing.T, sid uniset.SensorID, timeout_msec int, wg *sync.WaitGroup) int {

	defer wg.Done()

	timeout := time.After(time.Duration(timeout_msec) * time.Millisecond)
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
		case <-timeout:
			//t.Logf("timed out (%d msec)", timeout_msec)
			return num
		}
	}

	return num
}

func sendMessages(t *testing.T, msg *uniset.SensorMessage, count int, wg *sync.WaitGroup) {

	defer wg.Done()

	ui := uniset.GetActivator()

	for i := 0; i < count; i++ {
		ok := ui.SetValue(msg.Id, msg.Value, 1)
		if !ok {
			t.Error("SendMessages FAILED")
		}
	}
}

// ----------------------------------------------------------------
// Тест заказа датчика (простой заказ)
// ----------------------------------------------------------------
func TestAskSensor(t *testing.T) {

	ui := uniset.GetActivator()

	consumer := TestObject{100, make(chan uniset.UMessage, 10)}

	ui.AskSensor(10, &consumer)

	if ui.Size() != 1 {
		t.Errorf("AskSensor: size %d != %d", ui.Size(), 1)
	}

	num := ui.NumberOfConsumers(10)
	if num != 1 {
		t.Errorf("AskSensor: NumberOfCunsumers=%d != %d", num, 1)
	}

	sm1 := uniset.SensorMessage{10, 10500, time.Now()}

	msgCount := 3
	timeout := 800
	var wg sync.WaitGroup

	wg.Add(2)

	go sendMessages(t, &sm1, msgCount, &wg)

	rnum := consumer.read(t, sm1.Id, timeout, &wg)

	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}
}

func askSensor(clist *[]*TestObject, sid uniset.SensorID, wg *sync.WaitGroup) {

	defer wg.Done()
	ui := uniset.GetActivator()
	for _, c := range *clist {
		ui.AskSensor(sid, c)
	}
}

// ----------------------------------------------------------------
// Тест заказа датчика (многопоточный заказ)
// ----------------------------------------------------------------
func TestMultithreadAskSensors(t *testing.T) {

	ui := uniset.GetActivator()

	conslist := makeUObjects(100)

	var wg sync.WaitGroup
	wg.Add(2)

	l1 := conslist[0:5]
	l2 := conslist[5:len(conslist)]

	go askSensor(&l1, 30, &wg)
	go askSensor(&l2, 30, &wg)

	wg.Wait()

	if ui.NumberOfConsumers(30) != len(conslist) {
		t.Errorf("AskSensor: size %d != %d", ui.NumberOfConsumers(30), len(conslist))
	}

	sm := uniset.SensorMessage{30, 10500, time.Now()}

	msgCount := 3
	timeout := 800

	var wg2 sync.WaitGroup
	wg2.Add(12)

	go sendMessages(t, &sm, msgCount, &wg2)
	go sendMessages(t, &sm, msgCount, &wg2)

	rnum := (*conslist[0]).read(t, 30, timeout, &wg2)
	if rnum < msgCount {
		t.Errorf("TObject1: Count of received messages %d < %d", rnum, msgCount)
	}

	rnum = (*conslist[1]).read(t, 30, timeout, &wg2)
	if rnum < msgCount {
		t.Errorf("TObject2: Count of received messages %d < %d", rnum, msgCount)
	}

	wg2.Done()
}

// ----------------------------------------------------------------
// Штатная работы UActivator-а
// ----------------------------------------------------------------
func TestUWorking(t *testing.T) {

	ui := uniset.GetActivator()

	clist := makeUObjects(100)

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

	var sid uniset.SensorID = 21
	msgCount := 3

	err = ui.AskSensor(sid, clist[0])
	if err != nil {
		t.Errorf("AskSensor error: %s", err)
	}

	for i:=0; i<msgCount; i++ {
		ui.SetValue(sid, 10+int32(i), 2)
	}

	rnum := (*clist[0]).read(t, sid, 2000, &wg)
	if rnum < msgCount {
		t.Errorf("Count of received messages %d < %d", rnum, msgCount)
	}

	//wg.Wait()
}
