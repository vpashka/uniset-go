// UProxy.
// Объект реализующий взаимодействие с c++-ой частью,
// и рассылающий уведомления подписавшимся.
// В текущей реализации сделана попытка уйти от использования mutex-ов
// и сделать всё взаимодействие через каналы (сообщениями)
// Т.е. вся работа с внутренними структурами ведётся только из одной го-рутины (см. mainLoop)
// А все public-функции работают через посылку сообщений, которые обрабатываются в mainLoop().
// ---------
// Для получения сообщений желающие должны реализовать интерфейс UObject и добавить себя в uproxy
// при помощи функции Add(). А дальше уже при помощи канала команд можно заказывать датчики,
// а при помоищи канала "событий" получать уведомления об их изменении
// ---------
package uniset

import (
	"container/list"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"uniset_internal_api"
)

// ----------------------------------------------------------------------------------
// Объект через который идёт всё взаимодействие с uniset-системой
// При своём запуске Run() создаётся c++-ный объект который реально работает
// с uniset-системой, а UProxy проксирует запросы к нему и обработку сообщений
// преобразуя их в события в go-каналах.
// Следует иметь ввиду, что c++-ый Proxy ещё сам создаёт потоки в системе необходимые ему для работы
type UProxy struct {
	askmap       map[ObjectID]*consumersList
	active       bool
	actmutex     sync.RWMutex
	term         sync.WaitGroup
	name         string
	id           ObjectID
	confile      string
	uniset_port  int
	uproxy       uniset_internal_api.UProxy
	initOK       bool
	omap         map[ObjectID]UObject // список зарегистрированных объектов
	add          chan UObject
	msg          chan *SensorEvent
	eventTimeout uint
	pollTimeout  uint
}

// ----------------------------------------------------------------------------------
// Создание UProxy со значениями по умолчанию
// см. NewUProxy(..)
func NewDefaultUProxy(name string) *UProxy {
	return NewUProxy(name, 2000, 20, 5000, 200)
}

// ----------------------------------------------------------------------------------
// Создание UProxy
// в качестве аргумента передаётся идентификатор
// используемый для создания c++-объекта
// mqSize - размер очереди для сообщений об изменении датчиков (приходящих от uniset)
// oqSize - размер очереди для активации объектов
// eventTimeout - timeout (msec) на получение сообщений от uniset-системы
// pollSensorsTime - период обновления информации о состоянии датчиков (используемой в c++-объекте)
func NewUProxy(name string, mqSize uint, oqSize uint, eventTimeout uint, pollSensorsTimeout uint) *UProxy {
	ui := UProxy{}
	ui.askmap = make(map[ObjectID]*consumersList)
	ui.active = false
	ui.name = name
	ui.id = ObjectID(DefaultObjectID)
	ui.initOK = false
	ui.omap = make(map[ObjectID]UObject)
	ui.add = make(chan UObject, oqSize)
	ui.msg = make(chan *SensorEvent, mqSize)
	ui.eventTimeout = eventTimeout
	ui.pollTimeout = pollSensorsTimeout

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
// Зарегистрировать UObject
func (ui *UProxy) Add(obj UObject) {
	ui.add <- obj
}

// ----------------------------------------------------------------------------------
// Завершить работу
func (ui *UProxy) Terminate() error {

	if ui.IsActive() {
		ui.setActive(false)
		ui.term.Wait()
	}

	return nil
}

// ----------------------------------------------------------------------------------
// Ожидание завершения работы
func (ui *UProxy) WaitFinish() {

	if !ui.IsActive() {
		return
	}

	ui.term.Wait()
}

// ----------------------------------------------------------------------------------
// Начать работу
func (ui *UProxy) Run() error {

	if ui.IsActive() {
		return nil
	}

	if !uniset_internal_api.IsUniSetInitOK() {
		panic("Not uniset init...")
	}

	if !ui.initOK {
		ui.uproxy = uniset_internal_api.NewUProxy(ui.name)
		ui.uproxy.Run(int(ui.pollTimeout))
		ui.initOK = true

		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
		go func() {
			sig := <-signalChannel
			switch sig {
			case os.Interrupt:
				ui.Terminate()
			case syscall.SIGTERM:
				ui.Terminate()
			}
		}()
		// ...
	}

	ui.setActive(true)

	ui.term.Add(2)

	go ui.mainLoop()
	go ui.doReadMessages()

	return nil
}

// ----------------------------------------------------------------------------------
// получить значение (напрямую из proxy)
func (ui *UProxy) GetValue(sid ObjectID) (int64, error) {

	ret := ui.uproxy.SafeGetValue(int64(sid))

	if !ret.GetOk() {
		return 0, errors.New(ret.GetErr())
	}

	return ret.GetValue(), nil
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) setActive(set bool) {
	ui.actmutex.Lock()
	defer ui.actmutex.Unlock()
	ui.active = set
}

// ----------------------------------------------------------------------------------
// Главная go-рутина исполняющая команды поступающие от объектов
func (ui *UProxy) mainLoop() {

	defer ui.term.Done()

	for {

		if !ui.IsActive() {
			break
		}

		select {
		case obj, ok := <-ui.add:

			if ok {
				ui.doAdd(obj)
			}

			if !ui.IsActive() {
				break
			}

		case msg, ok := <-ui.msg:

			if !ui.IsActive() {
				break
			}

			if ok {
				ui.doSensorEvent(msg)
			}

		default:
			if !ui.IsActive() {
				break
			}
			if !ui.doCommands() {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	ui.doFinish()
	ui.uproxy.Terminate()
}

// ----------------------------------------------------------------------------------
// Главная go-рутина читающая сообщения от c++ объекта
func (ui *UProxy) doReadMessages() {

	defer ui.term.Done()

	for {

		m := ui.uproxy.SafeWaitMessage(5000)

		if !ui.IsActive() {
			break
		}

		if !m.GetOk() {
			continue
		}

		msg := makeSensorEvent(m.GetSinfo())

		// чтобы вся обработка проходила через одну го-рутину
		// пересылаем сообщение в mainLoop
		ui.msg <- msg
	}
}

// ----------------------------------------------------------------------------------
// Добавление нового объекта
func (ui *UProxy) doAdd(obj UObject) {

	if ui.doAddObject(obj) {
		umsg := UMessage{&ActivateEvent{}}
		ui.send(obj, umsg)
	}
}

// ----------------------------------------------------------------------------------
// Рассылка всем уведомления о завершении работы и закрытие канала
func (ui *UProxy) doFinish() {

	msg := UMessage{&FinishEvent{}}
	for _, obj := range ui.omap {
		ui.send(obj, msg)
	}

	for _, obj := range ui.omap {
		close(obj.UEvent())
	}
}

// ----------------------------------------------------------------------------------
// обработка команды "установить значение"
func (ui *UProxy) doSetValue(sid ObjectID, value int64, supplier ObjectID) error {

	ret := ui.uproxy.SafeSetValue(int64(sid), value)
	if !ret.GetOk() {
		return errors.New(ret.GetErr())
	}
	return nil
}

// ----------------------------------------------------------------------------------
// Рассылка SensorEvent
func (ui *UProxy) doSensorEvent(m *SensorEvent) {

	lst, found := ui.askmap[m.Id]
	if !found {
		//fmt.Printf("sensor %d not found in askmap\n", m.Id)
		return
	}

	// рассылаем всем заказчикам
	ui.sendMessage(&UMessage{m}, lst)
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) doCommands() bool {

	ret := false
	for _, v := range ui.omap {
		if ui.doCommandFromObject(v) {
			ret = true
		}
	}

	return ret
}

// ----------------------------------------------------------------------------------
func (ui *UProxy) doCommandFromObject(obj UObject) bool {

	select {
	case umsg, ok := <-obj.UCommand():

		if !ok {
			return false
		}

		msg, ok := umsg.PopAsAskCommand()
		if ok {
			ret, err := ui.doAskSensor(msg.Id, obj)
			if err != nil {
				msg.Result = false
				ui.send(obj, UMessage{&msg})
			} else {
				ui.send(obj, *ret)
			}

			return true
		}

		ask, ok := umsg.PopAsSetValueCommand()
		if ok {
			err := ui.doSetValue(ask.Id, int64(ask.Value), obj.ID())
			ask.Result = (err == nil)
			ui.send(obj, UMessage{&ask})
			return true
		}

		return true

	default:
	}

	return false
}

// ----------------------------------------------------------------------------------
// обработка команды добавления объекта
func (ui *UProxy) doAddObject(obj UObject) bool {

	_, found := ui.omap[obj.ID()]
	if found {
		return true
	}

	ui.omap[obj.ID()] = obj
	return true
}

// ----------------------------------------------------------------------------------
// обработка команды "заказ датчика"
func (ui *UProxy) doAskSensor(sid ObjectID, cons UObject) (msg *UMessage, err error) {

	//fmt.Printf("ASK SENSOR: %d for uobjecter %d\n",Sid,cons.ID())

	// На текущий момент uniset_internal_api.UProxy
	// не поддерживает заказ датчиков..
	// сперва делаем реальный заказ
	//ret := ui.uproxy.SafeAskSensor(int64(Sid))
	//
	//if !ret.GetOk() {
	//	return errors.New(fmt.Sprintf("%s (doAskSensor): Sid=%d error: %s", ui.name, Sid, ret.GetErr()))
	//}

	// Поэтому сперва получаем текущее значение
	val, err := ui.GetValue(sid)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("%s (doAskSensor): error: %s", ui.name, err))
	}

	msg = &UMessage{&SensorEvent{sid, val, time.Now()}}

	// вносим в список заказчиков
	lst, found := ui.askmap[sid]
	if found {
		lst.add(cons)
		return msg, nil
	}

	lst = newConsumersList()

	if lst == nil {
		return nil, err
	}

	lst.add(cons)
	ui.askmap[sid] = lst

	return msg, nil
}

// ----------------------------------------------------------------------------------
// рассылка сообщений по списку
func (ui *UProxy) sendMessage(msg *UMessage, l *consumersList) {

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)
		ui.send(c, *msg)
	}
}

// ----------------------------------------------------------------------------------
// посылка сообщения объекту
func (ui *UProxy) send(obj UObject, msg UMessage) {

	// делаем две попытки
	for i := 0; i < 2; i++ {
		select {
		case obj.UEvent() <- msg:
			return

		default:
		}
	}
}

// ----------------------------------------------------------------------------------
func (l *consumersList) String() string {

	var str string
	str = "["
	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)
		str = fmt.Sprintf("%s %d", str, c.ID())
	}
	str += " ]"
	return str
}

// ----------------------------------------------------------------------------------
func (l *consumersList) add(cons UObject) {

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObject)
		if c.ID() == cons.ID() {
			return
		}
	}

	l.list.PushBack(cons)
}

// ----------------------------------------------------------------------------------
// внутренний список объектов
type consumersList struct {
	list list.List
}

// ----------------------------------------------------------------------------------
func newConsumersList() *consumersList {
	lst := consumersList{}
	return &lst
}
