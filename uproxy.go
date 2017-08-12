// UProxy.
// Объект реализующий взаимодействие с c++-ой частью,
// и рассылающий уведомления подписавшимся.
// В текущей реализации сделана попытка уйти от использования mutex-ов
// и сделать всё взаимодействие через каналы (сообщениями)
// Т.е. вся работа с внутренними структурами ведётся только из одной го-рутины (см. mainLoop)
// А все public-функции работают через посылку сообщений, которые обрабатываются в mainLoop().
// ---------
// Для получения сообщений желающие должны реализовать интерфейс UObjecter и добавить себя в uproxy
// при помощи функции Add(). А дальше уже при помощи канала команд можно заказывать датчики,
// а при помоищи канала "событий" получать уведомления об их изменении
// ---------
package uniset

import (
	"container/list"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"uniset_internal_api"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// внутренний список объектов
type consumersList struct {
	list list.List
}

// ----------------------------------------------------------------------------------
// Объект через который идёт всё взаимодействие с uniset-системой
// При своём запуске Run() создаётся c++-ный объект который реально работает
// с uniset-системой, а UProxy проксирует запросы к нему и обработку сообщений
// преобразуя их в события в go-каналах.
// Следует иметь ввиду, что c++-ый Proxy сам создаётся ещё потоки в системе необходимые ему для работы
type UProxy struct {
	askmap      map[SensorID]*consumersList
	active      bool
	actmutex    sync.RWMutex
	term        sync.WaitGroup
	name        string
	id          ObjectID
	confile     string
	uniset_port int
	uproxy      uniset_internal_api.UProxy
	initOK      bool
	omap        map[ObjectID]UObjecter // список зарегистрированных объектов
	add         chan UObjecter
	msg         chan *SensorEvent
}

// ----------------------------------------------------------------------------------
// Создание UProxy
// в качестве аргумента передаётся идентификатор
// используемый для создания c++-объекта
// название uniset conf-файла и порт на котором работать
func NewUProxy(name string, confile string, uniset_port int) *UProxy {
	ui := UProxy{}
	ui.askmap = make(map[SensorID]*consumersList)
	ui.active = false
	ui.name = name
	ui.id = ObjectID(DefaultObjectID)
	ui.confile = confile
	ui.uniset_port = uniset_port
	ui.initOK = false
	ui.omap = make(map[ObjectID]UObjecter)
	ui.add = make(chan UObjecter, 10)
	ui.msg = make(chan *SensorEvent, 100)
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
// Зарегистрировать UObjecter
func (ui *UProxy) Add(obj UObjecter) {
	ui.add <- obj
}
// ----------------------------------------------------------------------------------
// Завершить работу
func (ui *UProxy) Terminate() error {

	if ui.IsActive() {
		ui.uproxy.Terminate()
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

	if !ui.initOK {

		err := ui.uniset_init()
		if err != nil {
			return err
		}

		ui.uproxy = uniset_internal_api.NewUProxy(ui.name)
		ui.uproxy.Run(200)
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

	ui.term.Add(1)

	go ui.mainLoop()
	go ui.doReadMessages()

	return nil
}

// ----------------------------------------------------------------------------------
// получить значение (напрямую из proxy)
func (ui *UProxy) GetValue(sid SensorID) (int64, error) {

	ret := ui.uproxy.SafeGetValue(int64(sid))

	if !ret.GetOk() {
		return 0, errors.New(ret.GetErr())
	}

	return ret.GetValue(), nil
}

// ----------------------------------------------------------------------------------
// инициализация
func (ui *UProxy) uniset_init() error {

	params := uniset_internal_api.ParamsInst()
	params.Add_str("--confile")
	params.Add_str(ui.confile)

	params.Add_str("--ulog-add-levels")
	params.Add_str("system,crit,warn")

	//params.Add_str("--UProxy1-log-add-levels")
	//params.Add_str("any")

	if ui.uniset_port > 0 {
		uport := strconv.Itoa(ui.uniset_port)
		params.Add_str("--uniset-port")
		params.Add_str(uport)
	}

	err := uniset_internal_api.Uniset_init_params(params, ui.confile)
	if !err.GetOk() {
		return errors.New(err.GetErr())
	}

	ui.id = ObjectID(uniset_internal_api.GetObjectID(ui.name))
	if ui.id == ObjectID(DefaultObjectID) {
		return errors.New(fmt.Sprintf("UProxy::Uniset_init: Unknown ObjectID for %s", ui.name))
	}

	return nil
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

			if ok {
				ui.doSensorEvent(msg)
			}

			if !ui.IsActive() {
				break
			}

		default:
			if !ui.IsActive() {
				break
			}
			if !ui.doCommands() {
				time.Sleep(100*time.Millisecond)
			}
		}
	}
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
		// пересылаем сообщение в основную го-рутину
		ui.msg <- msg
	}
}
// ----------------------------------------------------------------------------------
// Добавление нового объекта
func (ui *UProxy) doAdd(obj UObjecter) {

	if ui.doAddObject(obj) {
		var ret ActivateEvent
		ret.Id = obj.ID()
		umsg := UMessage{ret}
		ui.send(obj, umsg)
	}
}
// ----------------------------------------------------------------------------------
// обработка команды "установить значение"
func (ui *UProxy) doSetValue(sid SensorID, value int64, supplier ObjectID) error {

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
		return
	}

	// формируем сообщение
	u := UMessage{}
	u.Push(m)

	// рассылаем всем заказчикам
	ui.sendMessage(&u, lst)
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
func (ui *UProxy) doCommandFromObject(obj UObjecter) bool {

	select {
	case umsg, ok := <-obj.UCommand():

		if !ok {
			return false
		}

		msg, ok := umsg.PopAsAskCommand()
		if ok {
			err := ui.doAskSensor(msg.Id, obj)
			msg.Result = (err != nil)
			ret := UMessage{msg}
			ui.send(obj, ret)
			return true
		}

		ask, ok := umsg.PopAsSetValueCommand()
		if ok {
			err := ui.doSetValue(ask.Id, int64(ask.Value), obj.ID())
			ask.Result = (err == nil)
			ret := UMessage{ask}
			ui.send(obj, ret)
			return true
		}

		return true

	default:
	}

	return false;
}

// ----------------------------------------------------------------------------------
// обработка команды добавления объекта
func (ui *UProxy) doAddObject(obj UObjecter) bool {

	_, found := ui.omap[obj.ID()]
	if found {
		return true
	}

	ui.omap[obj.ID()] = obj
	return true
}

// ----------------------------------------------------------------------------------
// обработка команды "заказ датчика"
func (ui *UProxy) doAskSensor(sid SensorID, cons UObjecter) (err error) {

	// сперва делаем реальный заказ
	ret := ui.uproxy.SafeAskSensor(int64(sid))

	if !ret.GetOk() {
		return errors.New(fmt.Sprintf("%s (doAskSensor): sid=%d error: %s", ui.name, sid, ret.GetErr()))
	}

	// потом уже вносим в список заказчиков
	lst, found := ui.askmap[sid]
	if found {
		lst.add(cons)
		return nil
	}

	lst = newConsumersList()

	if lst == nil {
		return err
	}

	lst.add(cons)
	ui.askmap[sid] = lst

	return nil
}

// ----------------------------------------------------------------------------------
// рассылка сообщений по списку
func (ui *UProxy) sendMessage(msg *UMessage, l *consumersList) {

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObjecter)
		ui.send(c,*msg)
	}
}
// ----------------------------------------------------------------------------------
// посылка сообщения объекту
func (ui *UProxy) send(obj UObjecter, msg UMessage) {

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
		c := e.Value.(UObjecter)
		str = fmt.Sprintf("%s %d", str, c.ID())
	}
	str += " ]"
	return str
}

// ----------------------------------------------------------------------------------
func (l *consumersList) add(cons UObjecter) {

	for e := l.list.Front(); e != nil; e = e.Next() {
		c := e.Value.(UObjecter)
		if c.ID() == cons.ID() {
			return
		}
	}

	l.list.PushBack(cons)
}

// ----------------------------------------------------------------------------------
func newConsumersList() *consumersList {
	lst := consumersList{}
	return &lst
}