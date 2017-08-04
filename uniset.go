// Интерфейс для работы с uniset-датчиками
// Основная идея:  подписка на уведомления об изменении датиков
// Пользователь реализует интерфейс Consumer, предоставляющий канал
// для посылки в него SensorMessage
// Чтобы получать уведомления пользователь при помощи функции Subscribe
// подписывается на события, указывая идентификатор желаемого датчика
//
// Планируется, что UInterface будет Singleton-ом, т.к. будет в системе (c++) запускать один общий
// UniSetObject, который и будет (в отдельном системном потоке!) заказывать и получать
// датчики, а UInterface уже будет забирать у него сообщения и через go-каналы рассылать заказчикам.
// Запуск будет осуществляться через go-обёртку (uniset-api) входящую в состав uniset
package uniset

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
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

type Consumer interface {
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

func (l *consumerList) Add(cons Consumer) {

	l.mut.Lock()
	defer l.mut.Unlock()
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(Consumer)
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

type UInterface struct {
	// map: sensorID => consumer list
	askmap   *askMap
	active   bool
	actmutex sync.RWMutex
	term     sync.WaitGroup
}

func NewUInterface() *UInterface {
	ui := UInterface{}
	ui.askmap = newAskMap()
	ui.active = false
	//ui.term.Add(1)
	return &ui
}

func (ui *UInterface) IsActive() bool {
	ui.actmutex.RLock()
	defer ui.actmutex.RUnlock()
	return ui.active
}

func (ui *UInterface) setActive(set bool) {
	ui.actmutex.Lock()
	defer ui.actmutex.Unlock()
	ui.active = set
}

/*
var instance *UInterface
var once sync.Once

func GetSingleton() *UInterface {
	once.Do(func() {
		instance = NewUInterface()
		if instance == nil {
			panic("GetInstance error..")
		}
	})
	return instance
}
*/

func (ui *UInterface) Size() int {
	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()
	return len(ui.askmap.cmap)
}

func (ui *UInterface) NumberOfConsumers(sid SensorID) int {

	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	v, found := ui.askmap.cmap[sid]
	if found {
		return v.list.Len()
	}

	return 0
}

func (ui *UInterface) Subscribe(sid SensorID, cons Consumer) (err error) {

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

func (ui *UInterface) Terminate() error {

	if ui.IsActive() {
		ui.setActive(false)
		ui.term.Wait()
	}

	return nil
}

func (ui *UInterface) Run() error {

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

func (ui *UInterface) SendSensorMessage(sm *SensorMessage) (int, error) {

	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	v, found := ui.askmap.cmap[sm.Id]
	if !found {
		err := fmt.Sprintf("Not found: sensorID '%d'", sm.Id)
		return 0, errors.New(err)
	}

	num := 0
	for e := v.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(Consumer)
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
		c := e.Value.(Consumer)
		str = fmt.Sprintf("%s %d", str, c.ID())
	}
	str += " ]"
	return str
}

func (m *SensorMessage) String() string {
	return fmt.Sprintf("id: %d value: %d", m.Id, m.Value)
}
