// UProxy.
// Объект в реализующий взаимодействие с c++-ой частью,
// и рассылающий уведомления подписавшимся
package uniset

import (
	"container/list"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"uniset_internal_api"
)

type consumersList struct {
	list list.List
	mut  sync.RWMutex
}

func newConsumersList() *consumersList {
	lst := consumersList{}
	return &lst
}

func (l *consumersList) add(cons UObjecter) {

	l.mut.Lock()
	defer l.mut.Unlock()
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObjecter)
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
	name          string
	id		ObjectID
	confile     string
	uniset_port int
	uproxy      uniset_internal_api.UProxy
	initOK      bool
}

// ----------------------------------------------------------------------------------
// Создание UProxy
// в качестве аргумента передаётся идентификатор
// используемый для создания c++-объекта
func NewUProxy(name string, confile string, uniset_port int) *UProxy {
	ui := UProxy{}
	ui.askmap = newAskMap()
	ui.active = false
	ui.name = name
	ui.id = ObjectID(DefaultObjectID)
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
func (ui *UProxy) AskSensor(sid SensorID, cons UObjecter) (err error) {

	if !ui.IsActive(){
		return errors.New(fmt.Sprintf("%s (AskSensor): error: UProxy no activate..",ui.name))
	}

	ui.askmap.mut.Lock()
	defer ui.askmap.mut.Unlock()

	// сперва делаем реальный заказ
	ret := ui.uproxy.SafeAskSensor(int64(sid))

	if !ret {
		return errors.New(fmt.Sprintf("%s (AskSensor): ask SensorID=%d failed..",ui.name,sid))
	}

	// потом уже вносим в список заказчиков
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
	//
	//params.Add_str("--UProxy1-log-add-levels")
	//params.Add_str("any")

	if ui.uniset_port > 0 {
		uport := strconv.Itoa(ui.uniset_port)
		params.Add_str("--uniset-port")
		params.Add_str(uport)
	}

	uniset_internal_api.Uniset_init_params(params, ui.confile)

	ui.id = ObjectID(uniset_internal_api.GetObjectID(ui.name))
	if ui.id == ObjectID(DefaultObjectID) {
		return errors.New(fmt.Sprintf("UProxy::Uniset_init: Unknown ObjectID for %s", ui.name))
	}

	return nil
}

// ----------------------------------------------------------------------------------
// запустить UProxy в работу
func (ui *UProxy) Run() error {

	if ui.IsActive() {
		return nil
	}

	if !ui.initOK {

		err := ui.uniset_init()
		if err != nil {
			return err
		}

		ui.uproxy = uniset_internal_api.NewUProxy(ui.name)
		uniset_internal_api.Uniset_activate_objects()
		ui.initOK = true
	}

	ui.term.Add(1)
	ui.setActive(true)
	go ui.readProc()

	return nil
}

// ----------------------------------------------------------------------------------
// получить значение
func (ui *UProxy) GetValue(sid SensorID) (int64, bool) {
	ret := ui.uproxy.SafeGetValue(int64(sid))
	return ret.GetValue(), ret.GetOk()
}

// ----------------------------------------------------------------------------------
// сохранить значение
func (ui *UProxy) SetValue(sid SensorID, value int64, supplier ObjectID) bool {

	return ui.uproxy.SafeSetValue(int64(sid), value)
}

// ----------------------------------------------------------------------------------
// рассылка сообщений объектам
func (ui *UProxy) sendMessage(msg *UMessage, l *consumersList) {

	l.mut.RLock()
	defer l.mut.RUnlock()

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObjecter)

		// делаем по две попытки на подписчика..
		for i := 0; i < 2; i++ {
			//finish := time.After(time.Duration(20) * time.Millisecond)
			select {
			case c.URead() <- *msg:

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
// Главная go-рутина читающая сообщения от c++ объекта
func (ui *UProxy) readProc() {

	defer ui.term.Done()

	for {
		m := ui.uproxy.SafeWaitMessage(5000)

		if !ui.IsActive() {
			break
		}

		if !m.GetOk() {
			continue
		}

		ui.askmap.mut.RLock()
		defer ui.askmap.mut.RUnlock()

		lst, found := ui.askmap.cmap[SensorID(m.GetSinfo().GetId())]
		if !found {
			continue
		}

		// формируем сообщение
		msg := makeSensorMessage(m.GetSinfo())
		u := UMessage{}
		u.Push(msg)

		// рассылаем всем заказчикам
		ui.sendMessage(&u, lst)
	}
}

// ----------------------------------------------------------------------------------

func (l *consumersList) String() string {
	l.mut.RLock()
	defer l.mut.RUnlock()

	var str string
	str = "["
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObjecter)
		str = fmt.Sprintf("%s %d", str, c.ID())
	}
	str += " ]"
	return str
}
