// Activator
// Главный объект в приложении реализующий взаимодействие с c++-ой частью,
// и рассылающий уведомления подписавшимся
package uniset

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type consumersList struct {
	list list.List
	mut  sync.RWMutex
}

func newConsumersList() *consumersList {
	lst := consumersList{}
	return &lst
}

func (l *consumersList) add(cons UObject) {

	l.mut.Lock()
	defer l.mut.Unlock()
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)
		if c.ID() == cons.ID() {
			return
		}
	}

	l.list.PushBack(cons)
}

type askMap struct {
	cmap map[SensorID]*consumersList
	mut  sync.RWMutex
}

func newAskMap() *askMap {
	m := askMap{}
	m.cmap = make(map[SensorID]*consumersList)
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
	//ui.term.add(1)
	return &ui
}
// ----------------------------------------------------------------------------------
func (ui *UActivator) IsActive() bool {
	ui.actmutex.RLock()
	defer ui.actmutex.RUnlock()
	return ui.active
}
// ----------------------------------------------------------------------------------
func (ui *UActivator) setActive(set bool) {
	ui.actmutex.Lock()
	defer ui.actmutex.Unlock()
	ui.active = set
}

// ----------------------------------------------------------------------------------
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

// ----------------------------------------------------------------------------------
func (ui *UActivator) Size() int {
	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()
	return len(ui.askmap.cmap)
}
// ----------------------------------------------------------------------------------
func (ui *UActivator) NumberOfConsumers(sid SensorID) int {

	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	v, found := ui.askmap.cmap[sid]
	if found {
		return v.list.Len()
	}

	return 0
}
// ----------------------------------------------------------------------------------
func (ui *UActivator) AskSensor(sid SensorID, cons UObject) (err error) {

	ui.askmap.mut.Lock()
	defer ui.askmap.mut.Unlock()

	lst, found := ui.askmap.cmap[sid]
	if found {
		lst.add(cons)
		return nil
	}

	lst = newConsumersList()

	if lst == nil {
		return err
	}

	lst.add(cons)
	ui.askmap.cmap[sid] = lst
	return nil
}
// ----------------------------------------------------------------------------------
func (ui *UActivator) Terminate() error {

	if ui.IsActive() {
		ui.setActive(false)
		ui.term.Wait()
	}

	return nil
}
// ----------------------------------------------------------------------------------
// запустить активатор в работу..
func (ui *UActivator) Run() error {

	if ui.IsActive() {
		return nil
	}

	ui.term.Add(1)
	ui.setActive(true)
	go func() {
		defer ui.term.Done()

		for ui.IsActive() {
			// пока-что деятельность имитируем
			time.Sleep(1 * time.Second)
		}
	}()

	return nil
}

// ----------------------------------------------------------------------------------
// сохранить значение
func (ui *UActivator) SetValue( sid SensorID, value int32, supplier ObjectID ) bool {

	sm := SensorMessage{sid, value, time.Now()}
	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	lst, found := ui.askmap.cmap[sm.Id]
	if !found {
		return false
	}

	u := UMessage{}
	u.Push(&sm)
	ui.sendMessage(&u,lst)
	return true
}

// ----------------------------------------------------------------------------------
// рассылка сообщений объектам
func (ui *UActivator) sendMessage( msg *UMessage, l *consumersList) {

	l.mut.RLock()
	defer l.mut.RUnlock()

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)

		// делаем по две попытки на подписчика..
		for i:=0; i<2; i++ {
			//finish := time.After(time.Duration(20) * time.Millisecond)
			select {
			case c.UEvent() <- *msg:

			//case <-finish:
			//	if i == 2 {
			//		break
			//	}
			//	continue

			default:
				if i == 1 {
					break
				}
				//time.Sleep(10 * time.Millisecond)
				continue
			}
		}
	}
}

// ----------------------------------------------------------------------------------
func (l *consumersList) String() string {
	l.mut.RLock()
	defer l.mut.RUnlock()

	var str string
	str = "["
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)
		str = fmt.Sprintf("%s %d", str, c.ID())
	}
	str += " ]"
	return str
}
