package uniset_test

import (
	"sync"
	"testing"
	"time"
	"uniset"
)

// -----------------------------------------------------------------------------
// тестовая реализация интерфейса UObjecter
type TestObject struct {
	id       uniset.ObjectID
	rchannel chan uniset.UMessage
	wchannel chan uniset.UMessage

	SensorEventCounter   uint64
	AskResultCounter     uint64
	ActivateEventCounter uint64
}

func (c *TestObject) ID() uniset.ObjectID {
	return c.id
}

func (c *TestObject) UEvent() chan<- uniset.UMessage {
	return c.rchannel
}

func (c *TestObject) UCommand() <-chan uniset.UMessage {
	return c.wchannel
}

// -----------------------------------------------------------------------------
func (c *TestObject) AskSensor(sid uniset.SensorID) {
	var umsg uniset.UMessage
	umsg.Push(uniset.AskCommand{sid, false})
	c.wchannel <- umsg
}

// -----------------------------------------------------------------------------
func (c *TestObject) ReadEvent(timeout_msec time.Duration) {

	for {

		timeout := time.After(timeout_msec * time.Millisecond)
		select {
		case umsg := <-c.rchannel:

			_, ok := umsg.PopAsAskCommand()
			if ok {
				c.AskResultCounter++
				continue
			}

			_, ok = umsg.PopAsSensorEvent()
			if ok {
				c.SensorEventCounter++
				continue
			}

			_, ok = umsg.PopAsActivateEvent()
			if ok {
				c.ActivateEventCounter++
				continue
			}

		case <-timeout:
			return
		}
	}
}

// -----------------------------------------------------------------------------
func makeUObjects(beginID uniset.ObjectID, count int) []*TestObject {

	conslist := make([]*TestObject, count, count)

	var id uniset.ObjectID = beginID
	for i := 0; i < len(conslist); i++ {
		conslist[i] = &TestObject{id, make(chan uniset.UMessage, 10), make(chan uniset.UMessage, 10), 0, 0, 0}
		id++
	}

	return conslist
}

// -----------------------------------------------------------------------------
// Тесты
// -----------------------------------------------------------------------------

// ----------------------------------------------------------------
// Преобразование сообщений UMessage <--> SensorEvent
// ----------------------------------------------------------------
func TestUMessage2SensorMessage(t *testing.T) {

	sm := uniset.SensorEvent{30, 10500, time.Now()}
	u := uniset.UMessage{}
	u.Push(sm)

	_, ok := u.PopAsSetValueCommand()

	if ok {
		t.Errorf("SM --> UM --> CommandSetValue?!!")
	}

	sm2, ok := u.PopAsSensorEvent()

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
// Преобразование сообщений UMessage <--> AskCommand
// ----------------------------------------------------------------
func TestUMessage2CmdAskSensor(t *testing.T) {

	am := uniset.AskCommand{1, false}
	u := uniset.UMessage{}
	u.Push(am)

	_, ok := u.PopAsSetValueCommand()

	if ok {
		t.Errorf("Ask --> UM --> CommandSetValue?!")
	}

	am2, ok := u.PopAsAskCommand()

	if am2.Id != am.Id {
		t.Errorf("Ask --> UM --> Ask: Incorrect ID")
	}
}

// ----------------------------------------------------------------
// Преобразование сообщений UMessage <--> SetValueCommand
// ----------------------------------------------------------------
func TestUMessage2CmdSetValue(t *testing.T) {

	m := uniset.SetValueCommand{1, 4000, false}
	u := uniset.UMessage{}
	u.Push(m)

	_, ok := u.PopAsAskCommand()

	if ok {
		t.Errorf("Set --> UM --> CommandAskSensor!")
	}

	m2, ok := u.PopAsSetValueCommand()

	if m2.Id != m.Id {
		t.Errorf("Set --> UM --> Set: Incorrect ID")
	}

	if m2.Value != m.Value {
		t.Errorf("Set --> UM --> Set: Incorrect Value")
	}
}

// ----------------------------------------------------------------
func (c *TestObject) read(t *testing.T, sid uniset.SensorID, timeout_msec int, wg *sync.WaitGroup) int {

	defer wg.Done()

	timeout := time.After(time.Duration(timeout_msec) * time.Millisecond)
	var num int

	for {
		select {
		case msg := <-c.rchannel:

			sm, ok := msg.PopAsSensorEvent()
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

// ----------------------------------------------------------------
// Тест получения значения датчика
// ----------------------------------------------------------------
func TestGetValue(t *testing.T) {

	ui, err := uniset.NewUInterface("configure.xml", 53817)
	if err != nil {
		t.Error("UInterface: error: %s", err)
	}

	uproxy := uniset.NewUProxy("UProxy1", "configure.xml", 53817)

	defer uproxy.Terminate()
	uproxy.Run()

	if !uproxy.IsActive() {
		t.Error("UProxy: Not ACTIVE after run")
	}

	ui.SetValue(20, 20, -1)

	val, err := uproxy.GetValue(20)

	if err != nil {
		t.Error("UProxy: GetValue error: %s", err)
	}

	if err == nil && val != 20 {
		t.Errorf("UProxy: GetValue error: value=%d != %d", val, 20)
	}

}

// ----------------------------------------------------------------
// Тест заказа датчика (многопоточный заказ)
// ----------------------------------------------------------------
func doAskSensors(t *testing.T, sid uniset.SensorID, list []*TestObject, wg *sync.WaitGroup) {

	defer wg.Done()

	for _, obj := range list {
		obj.AskSensor(sid)
	}
}

func doReadSensorEvents(t *testing.T, timeout_msec time.Duration, list []*TestObject, wg *sync.WaitGroup) {

	defer wg.Done()

	for _, obj := range list {
		obj.ReadEvent(timeout_msec)
	}
}

// ----------------------------------------------------------------
// Штатная работы UProxy-а
// ----------------------------------------------------------------
func TestUWorking(t *testing.T) {

	uproxy := uniset.NewUProxy("UProxy2", "configure.xml", 53817)
	ui, err := uniset.NewUInterface("configure.xml", 53817)

	if err != nil {
		t.Error("UInterface: error: %s", err)
	}

	maxNum := 4

	clist := makeUObjects(100, maxNum)

	defer uproxy.Terminate()

	err = uproxy.Run()
	if err != nil {
		t.Errorf("UProxy: Run error: %s", err.Error())
	}

	if !uproxy.IsActive() {
		t.Error("UProxy: active failed!")
	}

	// активируем объекты
	for _, c := range clist {
		uproxy.Add(c)
	}

	time.Sleep(2 * time.Second)

	var wg sync.WaitGroup
	wg.Add(2)

	var sid uniset.SensorID = 20

	go doAskSensors(t, sid, clist[0:maxNum], &wg)

	msgCount := 3
	for i := 0; i < msgCount; i++ {
		err = ui.SetValue(sid, 10+int64(i), -2)
		if err != nil {
			t.Errorf("SetValue error: %s", err)
		}
	}

	go doReadSensorEvents(t, 300, clist[0:maxNum], &wg)

	wg.Wait()

	for _, c := range clist[0:maxNum] {
		if c.SensorEventCounter < uint64(msgCount) {
			t.Errorf("UObjecter %d: SensorEventCounter = %d < %d", c.ID(), c.SensorEventCounter, msgCount)
		}

		if c.ActivateEventCounter < 1 {
			t.Errorf("UObjecter %d: ActivateEventCounter = %d < 1", c.ID(), c.ActivateEventCounter)
		}
	}
}
