// Интерфейс для работы с uniset-датчиками
// Основная идея:  подписка на уведомления об изменении датиков
// Пользователь реализует интерфейс UObject, предоставляющий канал
// для посылки в него SensorMessage
// Чтобы получать уведомления пользователь при помощи функции Subscribe
// подписывается на события, указывая идентификатор желаемого датчика
//
// Планируется, что UActivator будет Singleton-ом, т.к. будет в системе (c++) запускать один общий
// UniSetObject, который и будет (в отдельном системном потоке!) заказывать и получать
// датчики, а UActivator уже будет забирать у него сообщения и через go-каналы рассылать заказчикам.
// Запуск будет осуществляться через go-обёртку (uniset-api) входящую в состав uniset
package uniset

type UInterface struct {

}
