// Activator
// Главный объект в приложении реализующий взаимодействие с c++-ой частью,
// и рассылающий уведомления подписавшимся
package uniset

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"time"
)

type UConsumer interface {
	EventChannel() chan SensorMessage
	ID() ObjectID
}

type consumerList struct {
	list list.List
	mut  sync.RWMutex
}

func newConsumerList() *consumerList {
	lst := consumerList{}
	return &lst
}

func (l *consumerList) Add(cons UConsumer) {

	l.mut.Lock()
	defer l.mut.Unlock()
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UConsumer)
		if c.ID() == cons.ID() {
			return
		}
	}

	l.list.PushBack(cons)
}

type askMap struct {
	cmap map[SensorID]*consumerList
	mut  sync.RWMutex
}

func newAskMap() *askMap {
	m := askMap{}
	m.cmap = make(map[SensorID]*consumerList)
	return &m
}

type UActivator struct {
	// map: sensorID => consumer list
	askmap   *askMap
	active   bool
	actmutex sync.RWMutex
	term     sync.WaitGroup
}

func newUActivator() *UActivator {
	ui := UActivator{}
	ui.askmap = newAskMap()
	ui.active = false
	//ui.term.Add(1)
	return &ui
}

func (ui *UActivator) IsActive() bool {
	ui.actmutex.RLock()
	defer ui.actmutex.RUnlock()
	return ui.active
}

func (ui *UActivator) setActive(set bool) {
	ui.actmutex.Lock()
	defer ui.actmutex.Unlock()
	ui.active = set
}


var instance *UActivator
var once sync.Once

func GetActivator() *UActivator {
	once.Do(func() {
		instance = newUActivator()
		if instance == nil {
			panic("UActivator::GetInstance error..")
		}
	})
	return instance
}


func (ui *UActivator) Size() int {
	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()
	return len(ui.askmap.cmap)
}

func (ui *UActivator) NumberOfConsumers(sid SensorID) int {

	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	v, found := ui.askmap.cmap[sid]
	if found {
		return v.list.Len()
	}

	return 0
}

func (ui *UActivator) Subscribe(sid SensorID, cons UConsumer) (err error) {

	ui.askmap.mut.Lock()
	defer ui.askmap.mut.Unlock()

	lst, found := ui.askmap.cmap[sid]
	if found {
		lst.Add(cons)
		return nil
	}

	lst = newConsumerList()

	if lst == nil {
		return err
	}

	lst.Add(cons)
	ui.askmap.cmap[sid] = lst
	return nil
}

func (ui *UActivator) Terminate() error {

	if ui.IsActive() {
		ui.setActive(false)
		ui.term.Wait()
	}

	return nil
}

func (ui *UActivator) Run() error {

	if ui.IsActive() {
		return nil
	}

	ui.term.Add(1)
	ui.setActive(true)
	go func() {
		defer ui.term.Done()
		sm1 := SensorMessage{10, 10500, time.Now()}
		sm2 := SensorMessage{11, 10501, time.Now()}

		for ui.IsActive() {
			// пока-что деятельность имитируем
			time.Sleep(1 * time.Second)
			ui.SendSensorMessage(&sm1)
			time.Sleep(1 * time.Second)
			ui.SendSensorMessage(&sm2)
		}
	}()

	return nil
}

func (ui *UActivator) SendSensorMessage(sm *SensorMessage) (int, error) {

	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	v, found := ui.askmap.cmap[sm.Id]
	if !found {
		err := fmt.Sprintf("Not found: sensorID '%d'", sm.Id)
		return 0, errors.New(err)
	}

	num := 0
	for e := v.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UConsumer)
		finish := time.After(time.Duration(200) * time.Millisecond)
		select {
		case c.EventChannel() <- *sm:
			num++

		case <-finish:
			continue

			//default:
			//	continue
		}
	}

	return num, nil
}

func (l *consumerList) String() string {
	l.mut.RLock()
	defer l.mut.RUnlock()

	var str string
	str = "["
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UConsumer)
		str = fmt.Sprintf("%s %d", str, c.ID())
	}
	str += " ]"
	return str
}

func (m *SensorMessage) String() string {
	return fmt.Sprintf("id: %d value: %d", m.Id, m.Value)
}
