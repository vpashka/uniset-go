// UProxy.
// Объект в реализующий взаимодействие с c++-ой частью,
// и рассылающий уведомления подписавшимся
package uniset

import (
	"container/list"
	"fmt"
	"strconv"
	"sync"
	"time"
	"uniset_internal_api"
	"errors"
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

// ----------------------------------------------------------------------------------
// Объект через который идёт всё взаимодействие с uniset-системой
// При своём запуске Run() создаётся c++-ный объект который реально работает
// с uniset-системой, а UProxy проксирует запросы к нему и обработку сообщений
// преобразуя их в события в go-каналах.
type UProxy struct {
	// map: sensorID => consumer list
	askmap      *askMap
	active      bool
	actmutex    sync.RWMutex
	term        sync.WaitGroup
	id          string
	confile     string
	uniset_port int
	uproxy 		uniset_internal_api.UProxy
	initOK	bool
}

// ----------------------------------------------------------------------------------
// Создание UProxy
// в качестве аргумента передаётся идентификатор
// используемый для создания c++-объекта
func NewUProxy(id string, confile string, uniset_port int) *UProxy {
	ui := UProxy{}
	ui.askmap = newAskMap()
	ui.active = false
	ui.id = id
	ui.confile = confile
	ui.uniset_port = uniset_port
	ui.initOK = false
	//ui.term.add(1)
	return &ui
}

// ----------------------------------------------------------------------------------
// Узнать активен ли UProxy
func (ui *UProxy) IsActive() bool {
	ui.actmutex.RLock()
	defer ui.actmutex.RUnlock()
	return ui.active
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) setActive(set bool) {
	ui.actmutex.Lock()
	defer ui.actmutex.Unlock()
	ui.active = set
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) Size() int {
	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()
	return len(ui.askmap.cmap)
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) NumberOfConsumers(sid SensorID) int {

	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	v, found := ui.askmap.cmap[sid]
	if found {
		return v.list.Len()
	}

	return 0
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) AskSensor(sid SensorID, cons UObject) (err error) {

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
// Завершитьь работу UProxy
func (ui *UProxy) Terminate() error {

	if ui.IsActive() {
		ui.setActive(false)
		ui.term.Wait()
	}

	return nil
}

// ----------------------------------------------------------------------------------
// инициализация
func (ui *UProxy) uniset_init() error {

	params := uniset_internal_api.ParamsInst()
	params.Add_str("--confile")
	params.Add_str(ui.confile)

	//params.Add_str("--ulog-add-levels")
	//params.Add_str("any")

	if ui.uniset_port > 0 {
		uport := strconv.Itoa(ui.uniset_port)
		params.Add_str("--uniset-port")
		params.Add_str(uport)
	}

	uniset_internal_api.Uniset_init_params(params, ui.confile)

	myid := uniset_internal_api.GetObjectID(ui.id)
	if myid == DefaultObjectID {
		return errors.New( fmt.Sprintf("UProxy::Uniset_init: Unknown ObjectID for %s",ui.id) )
	}

	return nil
}

// ----------------------------------------------------------------------------------
// запустить UProxy в работу
func (ui *UProxy) Run() error {

	if ui.IsActive() {
		return nil
	}

	if( !ui.initOK ) {

		err := ui.uniset_init()
		if err != nil {
			return err
		}

		ui.uproxy = uniset_internal_api.NewUProxy(ui.id)
		uniset_internal_api.Uniset_activate_objects()
		ui.initOK = true
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
func (ui *UProxy) SetValue(sid SensorID, value int64, supplier ObjectID) bool {

	sm := SensorMessage{sid, value, time.Now()}
	ui.askmap.mut.RLock()
	defer ui.askmap.mut.RUnlock()

	lst, found := ui.askmap.cmap[sm.Id]
	if !found {
		return false
	}

	u := UMessage{}
	u.Push(&sm)
	ui.sendMessage(&u, lst)
	return true
}

// ----------------------------------------------------------------------------------
// рассылка сообщений объектам
func (ui *UProxy) sendMessage(msg *UMessage, l *consumersList) {

	l.mut.RLock()
	defer l.mut.RUnlock()

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)

		// делаем по две попытки на подписчика..
		for i := 0; i < 2; i++ {
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
