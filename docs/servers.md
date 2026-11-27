# Особенности серверов

Где настоящие DAV-серверы и библиотеки расходятся со стандартом или с
ожиданием Carrel, и что Carrel с этим делает. Спека
([docs/spec/](spec/)) говорит, что должно быть; план
([docs/plan/server.md](plan/server.md)) — что сделано; этот файл — что
встретилось по дороге.

## Как вести

- **Особенность — запись здесь в том же коммите**, что и обход. Номер `SV-n` не
  переиспользуется; снятую запись помечают снятой, не удаляют.
- **В коде у обхода — номер** (`docs/servers.md, SV-n`).
- **У записи есть сторож** — тест, который упадёт, если поведение сменится; без
  него запись помечена «сторожа нет».
- **Упал сторож — сначала сюда:** обновить версию и описание, решить, нужен ли
  обход, потом чинить тест.
- «Вживую не проверено» — вывод сделан по коду сервера.

## Линейка

| Сервер | Где | Версия |
|---|---|---|
| Radicale | Linux-машина, `docker-compose.linux.yml` | 3.5.4 (`tomsquest/docker-radicale:3.5.4.0`); SV-4 и SV-5 ещё на 3.8.0 |
| Baikal | `baikal.mixeme.ru/dav.php`, Apache 2.4.65 | sabre/dav 4.7.0, sabre/vobject 4.5.6 |
| SFTPGo | `webdav.mixeme.ru` | не записана |
| Nextcloud | `77-222-37-121.swtest.ru/remote.php/dav/`, nginx, общий хостинг | Nextcloud 28.0.3; на sabre/dav, как Baikal |
| iCloud | `contacts.icloud.com` и `caldav.icloud.com` (календари пускают с 30.09.2026, SV-28) | не записана; заголовок `Server: AppleHttpServer/cff317772116` (30.09.2026) |
| Stalwart | Linux-машина, `docker-compose.linux.yml` | 0.16.23 (`stalwartlabs/stalwart:v0.16`) |
| Xandikos | Linux-машина, `docker-compose.linux.yml` | `ghcr.io/jelmer/xandikos:v0.2.12` |
| Davis | Linux-машина, `docker-compose.linux.yml` | `ghcr.io/tchapi/davis-standalone:latest`, образ от 06.09.2026; sabre/dav 4.7.1 |
| DAViCal | Linux-машина, `docker-compose.linux.yml` | 1.1.12 (`fintechstudios/davical:1.1.12`, PHP 8.2.8, Apache 2.4.57), PostgreSQL 15 |

В воротах на Linux-машине (`scripts/linux-gates.sh`) гоняются стенды, которые
поднимаются там же: Radicale, Stalwart, Xandikos, Davis и DAViCal. Учётную запись
Davis заводит скрипт через форму его же админки
(`testdata/davis/make-account.sh`), учётную запись DAViCal — так же
(`testdata/davical/make-account.sh`). Всё удалённое — Baikal, SFTPGo, OpenCloud,
Nextcloud, SoGo, Yandex, Mail.ru, Google, iCloud — гоняется только вручную, с
машины, где лежит `myenv/carrel/credentials.txt`: на Linux-машину он не копируется, и
шаги SFTPGo и Baikal в воротах всегда пропускаются. Nextcloud берёт весь общий набор, как Baikal; у Google и iCloud
собственные наборы: там нельзя завести вторую книгу, а у Google и календарь,
поэтому пишется в то, что уже есть. Календари iCloud заводятся, но по одному
компоненту (SV-52), и контакты с календарями там на разных хостах, так что
общие наборы вызываются половинами. Учётные данные — `myenv/carrel/credentials.txt`
рядом с копией, не в репозитории.

## Что сервер умеет и что мы у него проверяем

Таблица заведена 23.09.2026 после того, как в разговоре я сказал, что только
у Stalwart есть все три протокола. Это было неверно трижды. Первые две её
редакции тоже оказались неверными, и их ошибки поймал владелец. Сначала
Google и iCloud были помечены «без CalDAV». Потом у Xandikos стояли файлы,
которых он не обещает. **Эта редакция сверена с первоисточниками**: колонки
протоколов — с документацией самих серверов (ссылки ниже таблицы) и с
живыми пробами нашего стенда или аккаунта, колонка «гоняем» — со списком
`grep '^func Test' integration/`. Где факта нет, так и написано. Сверка —
23.09.2026.

**Как читать протоколы.** *да* — сервер **заявляет** протокол в своей
документации, и наш стенд или аккаунт до него доходит. *да, аккаунт не
пускает* — заявлен, но наш аккаунт им воспользоваться не может; это свойство
аккаунта, а не сервера. *нет* — не заявлен. *не проверяли* — не знаем.

Источник колонки — то, что сервер **обещает**, а пробы это только
подтверждают. Если проба показала незаявленное поведение, оно не становится
возможностью сервера: мы на него не опираемся и наборов по нему не гоняем.
Иначе проверки будут держаться на случайности, которая может исчезнуть в
любом выпуске. Правило появилось 23.09.2026, когда владелец заметил, что
Xandikos нигде не обещает файлов.

**Где.** *ворота* — `scripts/linux-gates.sh` на Linux-машине, запускается
само. *вручную* — только с машины, где есть `myenv/carrel/credentials.txt`. На
Linux-машину этот файл не копируется, поэтому шаги SFTPGo и Baikal в воротах
всегда пропускаются, хотя в скрипте они есть.

| Сервер | CalDAV | CardDAV | WebDAV (файлы) | Гоняем | Не гоняем | Где |
|---|---|---|---|---|---|---|
| Radicale | да | да | нет | общий набор; CRUD книг и календарей, потоки `GET`, `If-Match`, слияние при сбое записи, вложения, фото, vCard 4.0, импорт и экспорт, перенос на Baikal | — | ворота |
| Baikal | да | да | нет | общий набор; вложения (свои и в OpenCloud), большая книга, фото, vCard 4.0, импорт и экспорт, разрыв простаивающего соединения, `davloom`, понижение схемы в `.well-known`, заметки из папки SFTPGo | — | вручную |
| Nextcloud | да | да | да (`/remote.php/dav/files/`) | общий набор; импорт и экспорт, фото, vCard 4.0, ссылка-шара, `davloom`, **файлы** | — | вручную |
| SoGo | да | да | нет | общий набор; импорт и экспорт, vCard 4.0, пустые части имени | — | вручную |
| Stalwart | да | да | да (`/dav/file/`) | общий набор, **файлы** | — | ворота |
| Xandikos | да | да | нет: заявлен как «CardDAV/CalDAV server», WebDAV Mount «intentionally not implemented»; простые файлы он принимает, но это побочное свойство хранилища на git, и мы на него не опираемся | общий набор | — | ворота |
| Davis | да | да | да, по флагу `WEBDAV_ENABLED` (по умолчанию выключен; у нас включён 23.09.2026, домашние каталоги в `/dav/home/`) | общий набор, **файлы** | — | ворота |
| DAViCal | да | да (вики проекта: «CalDAV & CardDAV Server»; календари и книги в одном доме `/caldav.php/<логин>/`) | нет: WebDAV у него — доступ к тем же коллекциям, хранилища файлов нет | общий набор | — | ворота |
| SFTPGo | нет | нет | да | файлы; CRUD папок, загрузка папки, большой файл при малой памяти, потоки `GET`, игнор `If-None-Match` | — | вручную |
| OpenCloud | заявлен как дополнение: встроенный Radicale с версии 2.3.0, включает администратор; **у нашего экземпляра не включён** (`PROPFIND /caldav/` — 405) | так же, как CalDAV | да | файлы; вложения, уходящие из Baikal | календарь и контакты: на нашем экземпляре выключены | вручную |
| Yandex | да (`caldav.yandex.ru`) | да; `addressbook-query` нет, книга читается окольным путём (SV-23) | заявлен Яндекс Диск (`webdav.yandex.ru`): отдельный адрес, **только в тарифах Яндекс 360**; не проверяли | жизненный цикл календаря, формы, хронология, отказ второй книги, запись в единственную книгу, создание коллекции по настоящему адресу | общий набор, файлы | вручную |
| Mail.ru | да (`calendar.mail.ru`); нам выдали ссылку на один календарь, а не на дом (SV-22) | заявлен (`carddav.mail.ru`); **не проверяли**: этого адреса в наших учётных данных нет | заявлено Облако (`webdav.cloud.mail.ru`); не проверяли | поля события | общий набор, контакты, файлы | вручную |
| Google | заявлен **только через OAuth 2.0** (`apidata.googleusercontent.com/caldav/v2/`; про Basic документация прямо говорит «401 Unauthorized»). У нас работает по паролю приложения через **незаявленный** `google.com/calendar/dav/` (SV-24) | заявлен **только через OAuth 2.0** (`googleapis.com/.well-known/carddav`). У нас работает по паролю приложения через незаявленный `googleapis.com/carddav/v1/…` | не заявлен | поля контакта, запись в единственную книгу, обнаружение обоих хостов, **событие в существующем календаре**: создание, правка, удаление | общий набор: календарь не создать (`MKCALENDAR` — 404) | вручную |
| iCloud | да: Apple официально пускает сторонние приложения по авторизации или паролю приложения; наш аккаунт пускает с 30.09.2026 (SV-28): три календаря — «Напоминания» (`VTODO`), «Домашний» и «Рабочий» (`VEVENT`), цвета, `schedule-outbox-URL` и два своих адреса | да | не заявлен | поля контакта, жизненный цикл контакта; **календарь**: обнаружение и планирование (да, свои адреса), создание события и задачи в своих календарях и в заведённых, коллекции (создание, имя и цвет, удаление по имени), поиск по двум соединениям, хронология по двум соединениям, правка пароля и отключение; с v0.2.80 — карточки, правка, «Mark done», перенос и удаление через страницы (SV-54), событие без конца (SV-53), уведомление о невзятом роде (SV-52) | заметки — `VJOURNAL` сервер не держит (SV-52); копия и импорт внутри учётной записи — UID занят на всю запись, запись получает свой (SV-55, v0.2.81, v0.2.84) | вручную |

**Первоисточники** (открыты 23.09.2026):

- Radicale — [github.com/Kozea/Radicale](https://github.com/Kozea/Radicale):
  «A small but powerful CalDAV … and CardDAV … server».
- Baikal — [github.com/sabre-io/Baikal](https://github.com/sabre-io/Baikal):
  «the Baïkal CalDAV and CardDAV server».
- Nextcloud — [Accessing Nextcloud files using WebDAV](https://docs.nextcloud.com/server/latest/user_manual/en/files/access_webdav.html):
  «Nextcloud fully supports the WebDAV protocol».
- SOGo — [sogo.nu](https://www.sogo.nu/): CalDAV, CardDAV, GroupDAV,
  ActiveSync; хранилища файлов нет.
- Stalwart — [stalw.art/docs/http/webdav](https://stalw.art/docs/http/webdav/):
  `/dav/cal`, `/dav/card`, `/dav/file` — «general-purpose WebDAV file system».
- Xandikos — `README.rst` в самом образе `v0.2.12`: «CardDAV/CalDAV server»,
  «RFC 4918 … implemented, except for COPY/MOVE/LOCK», «RFC 4709 (WebDAV
  Mount) — intentionally not implemented».
- Davis — [github.com/tchapi/davis](https://github.com/tchapi/davis):
  «CalDAV, CardDAV, WebDAV», флаги `CALDAV_ENABLED`, `CARDDAV_ENABLED`,
  `WEBDAV_ENABLED=false`.
- DAViCal — [wiki.davical.org](https://wiki.davical.org/index.php?title=Main_Page):
  «the DAViCal CalDAV & CardDAV Server»; [davical.org](https://www.davical.org/):
  WebDAV — «backward-compatible access via WebDAV» к календарям, не
  хранилище файлов.
- SFTPGo — [github.com/drakkan/sftpgo](https://github.com/drakkan/sftpgo):
  «SFTP, HTTP/S, FTP/S, WebDAV».
- OpenCloud — [Calendar and Contacts Integration with Radicale](https://docs.opencloud.eu/docs/admin/configuration/radicale-integration/):
  `/caldav`, `/carddav`, `.well-known`; «at least OpenCloud Version 2.3.0».
- Yandex — [календарь на компьютере](https://yandex.ru/support/calendar/common/sync/sync-desktop.html)
  (`caldav.yandex.ru`), [Диск по WebDAV](https://yandex.ru/support/yandex-360/customers/disk/web/ru/webdav)
  (`webdav.yandex.ru`, только в тарифах Яндекс 360).
- Mail.ru — [синхронизация календаря](https://help.mail.ru/calendar-help/synchronization/about/)
  (`calendar.mail.ru`), [контакты](https://help.mail.ru/mail/letters/inbox/contacts/)
  (`carddav.mail.ru`), [Облако по WebDAV](https://help.mail.ru/cloud/desktop/webdav/)
  (`webdav.cloud.mail.ru`).
- Google — [CalDAV API](https://developers.google.com/calendar/caldav/v2/guide):
  «refuses to authenticate a request unless it arrives over HTTPS with OAuth
  2.0»; [CardDAV API](https://developers.google.com/people/carddav): «requires
  OAuth 2.0».
- iCloud — [Access your iCloud Mail, Calendar, and Contacts in third-party apps](https://support.apple.com/en-us/121539):
  авторизация приложения или пароль приложения; адресов серверов и WebDAV
  для iCloud Drive там нет.

«Общий набор» — это семь проверок через слой запроса: записи, формы,
коллекции, перенос, поиск, хронология, удаление коллекции.

**Правило.** Если сервер отдаёт протокол, по нему нужно гонять проверки. Если
не гоняем, пробел записывается в таблицу словом, а не подразумевается. Строка
без пробелов — это утверждение, которое кто-то проверил.

**Что таблица показывает сейчас.**
- Файлы рядом с календарями и контактами проверены у всех трёх серверов,
  которые их так держат: Nextcloud и Stalwart с 23.09.2026, Davis с
  24.09.2026. До того файлы проверялись только у SFTPGo и OpenCloud, а те
  ничего другого не умеют.
- **Google у нас держится на незаявленном поведении.** В документации
  Google CalDAV и CardDAV доступны только через OAuth 2.0, а про Basic
  сказано прямо: 401. Мы ходим по паролю приложения через адреса, которых в
  документации нет. Сегодня это работает, но Google ничего не обещает и
  может закрыть это без предупреждения, и тогда у всех, кто подключил Google
  к Carrel, календарь и контакты перестанут открываться. OAuth 2.0 Carrel не
  умеет. Решать, что с этим делать, должен владелец.
- Календарь Google наш аккаунт видит, но записи там не проверял ни один
  набор. Закрыто 24.09.2026 (`TestGoogleEventInTheExistingCalendar`):
  событие проходит всю жизнь в существующем календаре, без участников,
  чтобы Google никого не пригласил.
- У трёх участников линейки (Yandex, Mail.ru, OpenCloud) заявлено больше,
  чем мы проверяем: файлы Яндекса и Mail.ru, контакты Mail.ru, календари
  OpenCloud. Везде причина одна: у нас нет адреса или служба не включена.
- Удалённые серверы в ворота не входят вообще: их проверяет только тот, кто
  запускает наборы вручную.
- Xandikos в первой редакции был отмечен как сервер без файлов. Живая проба
  показала, что файлы он принимает, и я записал это в возможности. Владелец
  заметил, что Xandikos нигде этого не обещает, и README это подтверждает.
  Строка возвращена к «нет». Ошибку Carrel, найденную при этом (SV-48),
  воспроизводят и отсутствующее событие, и отсутствующая карточка, то есть
  заявленные возможности, так что правка остаётся.

## Чего в линейке не будет

**Exchange, Microsoft 365 и Outlook.com не говорят на CalDAV и CardDAV
вообще** — ни в какой версии, ни с какой настройкой. У них EWS и Microsoft
Graph, протоколы другого устройства. Самая распространённая почтовая система
предприятий недостижима не потому, что руки не дошли, а потому, что общего
языка нет: подключить её — это не «ещё один сервер в прогоне», а вторая
реализация всего доступа к данным. Записано здесь, чтобы вопрос не открывали
заново и чтобы человеку, который принесёт рабочий адрес Exchange, отвечали
правдой сразу.

## Сводка

| № | Сервер | Коротко | Что делает Carrel | Сторож |
|---|---|---|---|---|
| SV-1 | sabre/dav | vCard без версии отдаёт в 3.0 и теряет свойства 4.0 | просит 4.0 | `TestBaikalVCard4Edit` |
| SV-2 | sabre/dav | text-match в CalDAV — регистр только у латиницы | шлёт `i;unicode-casemap`, при отказе — повтор | `TestBaikalSearch` |
| SV-3 | Radicale < 3.7.7 | text-match по составному свойству — 500 | не ищет по N | `TestRadicaleSearch` |
| SV-4 | Radicale | обрезает data: URI фото в vCard 4.0 | пишет по RFC, отчёт о потере, старый формат по выбору | `TestRadicalePhoto` |
| SV-5 | Radicale | PUT без Content-Length — 400 | всегда шлёт длину | `TestRadicaleRecords` |
| SV-6 | Radicale | права `owner_only` и отказы 403 / 405 / 400 | раскладка и операции под них | `TestRadicaleCollections` |
| SV-7 | Radicale | well-known уводит в веб-интерфейс | остаётся на введённом адресе | `TestRadicaleDiscoveryFindsBothKinds` |
| SV-8 | SFTPGo | не соблюдает If-None-Match на PUT | смотрит имя до записи | `TestSFTPGoIgnoresIfNoneMatch` |
| SV-9 | Baikal (хостинг) | Apache закрывает простаивающее соединение через 5 с | не держит соединения между запросами | `TestBaikalIdleConnectionClosed` |
| SV-10 | Baikal (хостинг) | well-known уводит с https на http | не идёт по такому перенаправлению | `TestBaikalWellKnownDowngradesScheme` |
| SV-11 | go-webdav (клиент) | нет полей для collation и версии vCard | свои REPORT в `dav` | `TestQueryCardsNamesTheVersion` |
| SV-12 | go-webdav (сервер, `davtest`) | text-match с регистром; пустой фильтр пуст; нет `MOVE` записи, `DELETE` календаря, `calendar-color` | `davtest` поправляет | тесты `davtest` |
| SV-13 | go-vcard | экранирует запятую в data: URI | `card.Encode` возвращает запятую | тесты `card` |
| SV-31 | go-ical и go-vcard | не сворачивают строки по 75 октетов (RFC 5545 §3.1, RFC 6350 §3.2); наверху отказались дважды | своя свёртка на выходе обоих провайдеров (`model.FoldLines`), не на границе символа | `TestFoldLines`, `TestEncodeFoldsLongLines` (оба провайдера) |
| SV-32 | go-vcard и go-ical | go-vcard теряет группы (`item1.`), второе и дальше значения параметра, запятую-разделитель в `CATEGORIES` и режет кавычки по запятой; go-ical хранит все значения, но наш перевод брал первое | значение, которого никто не трогал, уходит своей строкой как пришло; изменённое пишется заново со всеми значениями параметра; проверка потерь читает `TYPE` множеством | `TestUntouchedLinesGoBackAsTheyCame`, `TestListEditKeepsTheOtherRows`, `TestEventEditKeepsEveryParameterValue` |
| SV-33 | go-ical и go-webdav (клиент) | go-ical паникует на параметре без двоеточия и на тексте после закрывающей кавычки; `caldav.Client` роняет весь REPORT на первой нечитаемой записи | свой `calendar-query`: запись, которая не читается, — одна строка «не прочитана», остальное на месте; разбор нигде не паникует | `TestCalendarQueryKeepsTheReadableRecords`, `TestMalformedRecordIsAnErrorNotAPanic` |
| SV-14 | net/http + любой сервер | закрытое простаивающее соединение: `PROPFIND` и `REPORT` не повторяются, а на общем хостинге так же умирает и запись | повтор один раз в страже: чтение — всегда, запись — если соединение взято из пула простаивающим; поток, который не повторить, идёт своим соединением мимо пула | `TestReadOnlyRequestIsSentAgainAfterAnIdleClose`, `TestWriteOnAnIdleConnectionIsSentAgain`, `TestAStreamedBodyTakesAConnectionOfItsOwn` |
| SV-15 | OpenCloud за Apache | PUT без `Content-Length` — 400 / 502 | длину шлёт, когда знает; иначе отказ со строкой очереди | `TestOpenCloudFiles`, `TestAttachmentNeedsASizeOnSuchAServer` |
| SV-16 | SoGo | text-match в `calendar-query` не соблюдается: отдаёт всю коллекцию | просеивает ответ сам и говорит об этом у выдачи | `TestSoGoSearch` |
| SV-17 | SoGo | `displayname` при создании книги не берётся; карточка отдаётся в 3.0 | дошлёт имя `PROPPATCH`; версию не трогает — свойства целы | `TestSoGoCollections`, `TestSoGoVCard4Edit` |
| SV-18 | SoGo | после `MOVE` в другую книгу меняет и имя ресурса, и `UID` записи | называет смену личности и ведёт в коллекцию, а не редиректом на исчезнувший адрес | `TestMoveWhereTheServerChangesTheUID`, `TestSoGoMove` |
| SV-19 | Yandex | `MKCALENDAR` отвечает 201, календарь кладёт по своему адресу (`events-<id>`), `Location` не шлёт | перечитывает адрес, сверяет детей дома до и после; не угадывает — говорит | `TestCreateWhereTheServerPicksTheAddress`, `TestYandexCalendarLifecycle` |
| SV-20 | Yandex | адресную книгу создать нельзя: `MKCOL` — 405 от nginx, обоими паролями приложения | отказ приходит содержимым листа: метод, адрес, код; каталог не меняется | `TestYandexAddressBookRefused` |
| SV-21 | Yandex, Google | `FN` строит из `N`, а присланный `FN` выбрасывает | обхода не нужно: редактор пишет `N` (U-6.1), и имя приезжает целым | `TestYandexContactInTheOnlyBook`, `TestContactNameFieldsWriteN` |
| SV-22 | Mail.ru | ссылку выдают на один календарь, а не на дом; принципала нет | род ресурса спрашивается до файлового отката: календарь остаётся календарём | `TestEnteredAddressIsACalendar`, `TestLiveDiscoveryReport` |
| SV-23 | Yandex | `addressbook-query` нет вовсе: 400, а с фильтром 403 `supported-report` | книга читается списком и `addressbook-multiget`; поиск просеивается у нас и говорит об этом | `TestSearchFetchesWhatAServerCannotQuery`, `TestYandexContactInTheOnlyBook` |
| SV-24 | Google | контакты и календари на разных хостах; рекомендованный `caldav/v2` требует OAuth | подключение на протокол, как у Yandex; прямой путь обнаружения работает без правок | `TestGoogleContactInTheOnlyBook`, `TestLiveDiscoveryReport` |
| SV-25 | Google | новую карточку называет сам и переписывает её `UID`; `Location` при этом шлёт | созданную запись перечитывает по адресу, куда писал, и берёт личность оттуда; нигде не нашлась — говорит и ведёт в коллекцию | `TestCreateWhereTheServerRewritesTheUID`, `TestGoogleContactInTheOnlyBook` |
| SV-30 | Nextcloud | удалённый календарь остаётся по адресу и в доме, и называет себя календарём — пока не спросишь про `nc:deleted-at` | обход спрашивает об этом в том же запросе и помеченное удалённым не берёт; след говорит, сколько пропущено | `TestDiscoverySkipsWhatTheServerPutInItsTrashBin`, `TestNextcloudCollectionDelete` |
| SV-75 | SFTPGo | `If-Match` на `PUT` не соблюдает, как и `If-None-Match` (SV-8): запись с неверным и с устаревшим ETag проходит, файл переписан | обхода нет: вторая правка файла затирает первую; для файла проекта — сверять ETag перед записью или выключить запись с причиной (server.md, «Проекты») | `TestSFTPGoProjectFile` (27.11.2026) |
| SV-74 | Yandex | задача хранит только свои поля: из записанной с `STATUS:IN-PROCESS`, тремя `RELATED-TO` (одна — `VALUE=URI`) и четырьмя `LINK` (RFC 9253) остаются `UID`, `SUMMARY`, `DUE`; остальное сервер выбрасывает молча, ответ — 201 | обхода нет: чего сервер не сохранил, называет отчёт сохранения (C-9); для `RELATED-TO` и `LINK`, записанных самим Carrel, на Yandex живьём не проверено. Кандидат «Проекты» на Yandex не ложится (server.md) | `TestYandexProjectLinks` (27.11.2026) |
| SV-73 | Google | событию дописывает свои напоминания («10 minutes before · 30 minutes before», у события на весь день — «1 day before, 23:30»), `STATUS:CONFIRMED` и организатора участником; карточке — `TYPE=pref` у телефона и почты, `CATEGORIES` карточки не хранит | дописанное — запись сервера, Carrel показывает её как есть; `pref` и прочие слова формата (`INTERNET`, `VOICE`) в подписи значения не стоят — «Phone · mobile», как в форме (`v0.3.8`) | `web.TestATypedLabelLeavesOutWhatIsNoKind`; живьём — `TestLiveStand` |
| SV-72 | Mail.ru | событие с участниками, чей организатор — не владелец учётной записи, отвергает 403 | запись не пишется, отчёт импорта называет запрос и ответ (SV-66) | живьём — `TestLiveStand` |
| SV-71 | Mail.ru | `calendar-query` с фильтром `VTODO` или `VJOURNAL` — 400 `{"title":"bad request","type":"caldav"}`: календарь держит события | импорт спрашивает коллекцию только о том, что она держит; прежде один отказ делал весь календарь «did not answer», и импорт в Mail.ru был выключен (S-19, `v0.3.8`) | `web.TestImportAsksACalendarOnlyAboutWhatItHolds` (`davtest.EventQueriesOnly`); живьём — `TestLiveStand` |
| SV-70 | Yandex | запись с UID, который на учётной записи уже был и удалён, принимает 201 и не хранит | отчёт импорта: «Not kept by the server · … · not there after it was written»; правки нет | живьём — `TestLiveStand` |
| SV-69 | Yandex | задачу хранит без `DTSTAMP` | запись без `DTSTAMP` читается, отметка берётся из `LAST-MODIFIED`, `CREATED` или времени ответа; прежде все задачи Yandex были «not shown: invalid iCalendar» (S-7.1, `v0.3.8`) | `cal.TestARecordWithoutItsStampIsRead`; живьём — `TestLiveStand` |
| SV-68 | DAViCal | ящики планирования лежат в календарном доме как обычные календари: `.in/` и `.out/`, `resourcetype` — `calendar`, признаков RFC 6638 нет; называют их только `schedule-inbox-URL` и `schedule-outbox-URL` принципала | обход спрашивает у принципала оба адреса и оставляет ящики вне списка, след говорит «2 scheduling boxes (inbox, outbox) left out» (S-6, `v0.3.7`); до того «Carrel Inbox» и «Carrel Outbox» стояли в каждом рейле | `discover.TestTheSchedulingBoxesAreNotCollections`; живьём — `TestLiveStand` |
| SV-67 | Stalwart | вложение в записи, которое читается как текст UTF-8, хранит текстом: `VALUE=BINARY` без `ENCODING=BASE64`, значение с экранированием TEXT; объявляет `max-resource-size` 524288 и запись крупнее отвергает 412 с условием `max-resource-size` | такое вложение читается как текст — размер и данные верные (S-20, `v0.3.7`); импорт сверяет размер записи с `max-resource-size` коллекции до записи: крупная в предпросмотре «Too large: …», на сервер не уходит (S-19) | `cal.TestAnInlineFileKeptAsTextIsItsText`, `web.TestImportLeavesARecordTheCollectionCannotTake`; живьём — `TestLiveStand` |
| SV-66 | Radicale | заметку (`VJOURNAL`) с двумя `DESCRIPTION` отвергает 400 «Bad Request»; RFC 5545 §3.6.3 разрешает их несколько, причина — только в журнале сервера | запись не пишется, отчёт импорта называет запрос и ответ: «… · PUT …/5c81-77aa-0d64.ics → 400 Bad Request» (S-19, `v0.3.7`); `DESCRIPTION` Carrel не склеивает — запись как пришла | `web.TestImportSaysWhatTheServerRefused`; живьём — `TestLiveStand` |
| SV-65 | Nextcloud, ownCloud, Davis | файлы учётной записи лежат по `…/files/<id>/` рядом с принципалом `…/principals/users/<id>/` (Davis — `…/home/<id>/` рядом с `…/principals/<id>/`), но ни одно свойство принципала этого адреса не называет; адрес файлов, введённый сам, `.well-known` уводит к принципалу | принципала такого вида обход спрашивает и об адресе файлов: папка там — хранилище «Files» той же учётной записи (S-6, `v0.3.5`, Davis — `v0.3.7`); до того учётная запись Nextcloud оставалась без файлов при любом адресе | `discover.TestAnAccountOnNextcloudHasItsFiles`, `TestAnAccountOnDavisHasItsFiles` (davtest по их адресам); живьём — стенд приёмки `TestLiveStand` |
| SV-64 | SoGo | календарь, заведённый с одним `VEVENT`, после создания сообщает состав шире: в рейле задач он стоит рядом со списками задач, с нулём | ничего: каталог берёт состав, который сервер назвал после создания (`settleComponents`, S-12.2), и показывает его | сторожа нет; найдено стендом приёмки на тестовых серверах 17.11.2026 |
| SV-63 | Nextcloud | после неверных паролей с адреса отвечает на всякий запрос с него, верный пароль тоже, через 25 с — часами (защита от подбора) | ждёт: предел ответа DAV по умолчанию 30 с (`CARREL_DAV_REQUEST_TIMEOUT`), работа медленная, но идёт; живой набор неверного пароля Nextcloud не шлёт (`holdsAGrudge`) | замер 16.11.2026: три `PROPFIND` с верным паролем — 25,5 с каждый |
| SV-62 | Yandex | `DURATION` хранит как `DTEND`; присланную запись иногда меняет сам, и метка версии сдвигается без правки; на миг после записи отвечает на её `GET` 404 и не отдаёт её в `REPORT` | форма показывает конец, правка пишет `DTEND`; сдвиг под формой — лист конфликта (C-9), как на чужую правку; запись, не найденная при чтении после сохранения, — сохранение стоит, отчёта о потере нет; правки нет | живьём — сверка `TestYandexForms/matrix`; живой набор перечитывает 404 до 10 с (`getSettled`) |
| SV-61 | Google | двоеточие в значении карточки хранит экранированным: `URL:https\://ada.example`, `X-SOCIALPROFILE:https\://…`; при записи разбирает группы и собирает по-своему (`item1.TEL` → `TEL`, `URL` → `item2.URL` с `X-ABLabel`), из двух `TITLE` и двух `ORG` оставляет по одному | `\:` читается как двоеточие (`card.plainColons`), и список читает карточку так же, как запись (`card.ownRead`); до `v0.2.416` карточка показывала адрес с обратной косой и не ссылкой, новый адрес отчитывался потерянным, а сохранить такую карточку не давала проверка перед записью | `card.TestAnEscapedColonIsAColon`; живьём — сверка `TestGoogleContactFields/matrix` |
| SV-60 | Baikal, Nextcloud (sabre), SoGo | планировщик RFC 6638: событию, где владелец позвал участников, пишет `ORGANIZER` владельца; отправленному приглашению — `SCHEDULE-STATUS=1.1`; SoGo, став организатором, сбрасывает ответы участников в `NEEDS-ACTION` и разбирает передачу участия | `SCHEDULE-STATUS` — параметр сервера, не потеря (`model.withoutScheduleStatus`, `v0.2.416`; до того добавленный участник отчитывался потерянным); что SoGo переписал у участников, отчёт о потере называет — это правда | `model.TestAScheduleStatusTheServerWritesIsNotALoss`; живьём — сверка `TestBaikalForms/matrix`, `TestSoGoForms/matrix` |
| SV-59 | все (найдено на Mail.ru) | серии, которой меняют время или пояс, одиннадцать серверов из двенадцати оставляют её отступления как присланы; Mail.ru выбрасывает те, что больше ни на одно повторение не приходятся — и после Carrel, и после прямой записи | **была ошибка Carrel**: он переписывал начало серии, а `EXDATE` и `RECURRENCE-ID` оставлял на прежнем времени — пропуск переставал пропускать, перенос повисал. С `v0.2.186` отступления идут за серией (`cal.RetimeExceptions`, решение владельца 11.10.2026), а отчёт о потере читает их по мгновению и называет `EXDATE` и `RECURRENCE-ID`, когда сервер их не сохранил | `TestSeriesTakesItsExceptionsWithIt`, `cal.TestRetimeExceptions…`, `cal.TestExceptionLoss`, живьём — `liveEventFields` на каждом сервере, проба `CARREL_SERIES_REPORT` |
| SV-58 | iCloud | после одного неверного пароля календарный хост (`caldav.icloud.com`) отвечает 401 и на верный — дольше пяти минут (10.10.2026); адресный хост (`contacts.icloud.com`) при этом работает | ничего: сообщение говорит то, что сказал сервер, — «The server refused the login and password» (`v0.2.185`), и ничего сверх: верен ли пароль, знает только сервер (решение владельца 10.10.2026); живой набор неверный пароль iCloud не шлёт | `integration.TestICloudForms` (половина с неверным паролем пропускается, `holdsAGrudge`) |
| SV-57 | Nextcloud (и все с файлами) | число в папке в `PROPFIND` даёт только Nextcloud (`nc:contained-folder-count`, `nc:contained-file-count`); размеров картинки не даёт никто; `Range` на `GET` соблюдают все проверенные | спрашивает число в том же `PROPFIND`, иначе берёт из прочитанного сессией; размеры выбранной картинки — один `GET` с `Range: bytes=0-65535`, не дальше 64 КиБ и при ответе целиком | `TestListTakesWhatAFolderHoldsFromTheSameAnswer`, `TestDimensionsAskForTheHeadOnly`, `TestNextcloudFolderHeads`, `TestSFTPGoFolderHeads`, `TestOpenCloudFolderHeads`, `TestStalwartFolderHeads`, `TestDavisFolderHeads` |
| SV-28 | iCloud | **снята 30.09.2026**: календарный хост отказывал 401 тем же данным, которые принимает адресный; теперь отвечает | подключение на протокол, как у Google | `TestICloudCalendarDiscovery` |
| SV-29 | iCloud | у единственной книги нет `displayname` вовсе, а `resourcetype` — без `<collection/>`; вторую не завести (403) | род ресурса берётся по адресному признаку; безымянную коллекцию обнаружение называет родом («Address book»), а несколько таких в одном доме различает адресом; живой набор пишет в единственную книгу | `TestACollectionWithoutANameIsNamedByItsKind`, `TestICloudContactLifecycle` |
| SV-52 | iCloud | календарь держит один компонент: из набора `MKCALENDAR` берёт первый и отвечает 201; только `VJOURNAL` — 500 | каталог берёт ответ сервера (S-12.2); списки календаря говорят, каких родов сервер не взял и что им нужен другой календарь (v0.2.80), с кнопкой «New calendar» на лист нового с невзятым родом (v0.2.92, s-agenda-11) | `TestCreateSaysWhatTheServerDidNotTake`, `TestTheNoticeOfWhatWasNotTakenIsTheFrame`; `TestICloudCollections`, `TestICloudCalendarDiscovery` |
| SV-53 | iCloud | `PUT` события без `DTEND` и `DURATION` — 404 с пустым телом | конец пишется всегда: `DTEND` равен `DTSTART`, у события на дату — следующий день (v0.2.80) | `TestAnEventWithoutAnEndIsWrittenWithOne`; `TestICloudEventWithoutAnEnd` |
| SV-54 | iCloud | `calendar-query` с `text-match` по `UID` — 412 с пустым телом; по `SUMMARY` и прочим — 207 без совпадений | поиск по тексту держит путь SV-47; запись по `UID` ищется без фильтра — `GET <UID>.ics`, затем компонент целиком и сверка у себя, отказ сеанс помнит (v0.2.80) | `TestAUIDTheServerWillNotFilterIsFoundWithoutTheFilter`, `TestARecordOpensWhereTheServerRefusesAUIDFilter`; `TestICloudSearch`, `TestICloudRecords` |
| SV-55 | iCloud | UID занят на всю учётную запись: `PUT` записи с UID, лежащим в другом календаре, — 412 с пустым телом, под любым именем и без `If-None-Match` | 412 на имя, под которым ничего нет, — «UID занят»: копия (v0.2.81) и запись импорта (v0.2.84) пишутся со своим UID, экран и отчёт говорят это словами (решение владельца) | `TestACopyGetsAUIDOfItsOwnWhereTheAccountHoldsOneRecordPerUID`, `TestAnImportGetsAUIDOfItsOwnWhereTheAccountHoldsOneRecordPerUID`, `TestICloudMove/copy_back` |
| SV-56 | Google | выбрасывает `DTEND`, равный `DTSTART`; первым `TEL`, `EMAIL`, `ADR` дописывает `PREF`, `TYPE` пишет заглавными; день рождения без года (`--MMDD`) выбрасывает, вид Apple хранит с пометкой внутри `VALUE`; карточку хранит в 3.0 | конец, который подразумевает начало (RFC 5545 §3.6.1), и `PREF`, дописанный сервером, — не потеря (v0.2.91); день рождения без года пишется видом Apple, у новой карточки — второй записью, и один день в другом виде не потеря (решение владельца 04.11.2026, вариант А; v0.2.400) | `TestAnEndTheStartImpliesIsNotLostWhenTheServerLeavesItOut`, `TestAPreferredMarkTheServerAddsIsNotALoss`, `TestADayWithoutAYearInAnotherFormIsNotALoss`, `TestABirthdayWithoutAYearOnGoogle`; `TestGoogleEventInTheExistingCalendar`, `TestGoogleContactFields`, `TestGoogleBirthdayWithoutAYear` |
| SV-27 | SoGo | у составного значения отбрасывает хвостовые пустые части: `N:Фамилия;Имя;;;` → `N:Фамилия;Имя` | сравнивает составные значения по частям, хвостовые пустые не считая: то же имя, записанное короче, — не потеря | `TestCompareIgnoresTrailingEmptyComponents`, `TestSoGoTrimsEmptyNameComponents` |
| SV-26 | Google (и go-webdav) | `addressbook-query` отвечает 207 без ответов; `allprop` внутри `address-data` — 400 | просит карточку пустым `address-data`, своим multiget'ом; пустому отчёту не верит на слово, а сверяет со списком книги | `TestHollowQueryIsCheckedAgainstTheBook`, `TestSearchLooksAgainWhenAQueryAnswersNothing` |
| SV-31 | библиотеки | ни go-vcard, ни go-ical не сворачивают длинные строки | сворачивает сам, по 75 октетов | `TestFoldLines`, `TestEncodeFoldsLongLines` |
| SV-32 | go-vcard | группы, значения параметров и списки не переживают разбор | пишет карточку своим кодом, разбор библиотеки только читает | `TestUntouchedLinesGoBackAsTheyCame`, `TestEventEditKeepsEveryParameterValue` |
| SV-33 | go-ical, go-webdav | go-ical паникует на кривом объекте, go-webdav роняет весь отчёт из-за одного ответа | разбор в карантине, дурная запись показывается строкой состояния, остальные приходят | `TestCalendarQueryKeepsTheReadableRecords`, `TestMalformedRecordIsAnErrorNotAPanic` |
| SV-34 | Yandex | событие переписывается: `COUNT`→`UNTIL`, `INTERVAL=1`, время в свой пояс; `STATUS`, `CATEGORIES`, `X-CUSTOM` не хранятся | каждое расхождение — потеря по S-7.1, человек видит отчёт C-9; обхода нет и он был бы враньём | `TestYandexForms` |
| SV-35 | SoGo | правило хранится переставленным: `FREQ;COUNT;BYDAY;WKST` вместо присланного порядка | `RRULE` и `EXRULE` сравниваются по частям: перестановка — не потеря, пропавшая часть — потеря | `TestARuleReorderedIsTheSameRule`, `TestSoGoForms` |
| SV-36 | Mail.ru | не берёт `X-CUSTOM`, не хранит `CATEGORIES`, переписывает правило и времена, `EXDATE` и `RECURRENCE-ID` хранит в UTC (до 11.10.2026 здесь стояло «не берёт»: проверка искала строки в присланном написании) — и дописывает своё `X-MOZ-LASTACK` | бухгалтерия сервера (`X-MOZ-*`) считается отметкой наравне с `DTSTAMP`; остальное остаётся потерей и сообщается | `TestMailruEventFields`, `TestTheBookkeepingAServerAddsIsNotALoss` |
| SV-37 | SoGo | заметок не держит вовсе: календарь заводится с `VJOURNAL`, а `PUT` заметки — 404 | живой посев записи, которую сервер не берёт, пишет это в лог и снимает своё ожидание | `TestSoGoTimeline` |
| SV-38 | Yandex | `ATTENDEE` события не хранится, вместо него ставится `ORGANIZER` владельца ящика | хронология честно пуста: опрос доходит до конца, источники отвечают «пусто», отчёт не поднимается | `TestYandexTimeline` |
| SV-39 | оборванные соединения (Nextcloud, Apache) | соединение умирает после того, как сервер прочитал запись, и повтор встречает 412 или 405 — «оно уже здесь» | второй ответ помечается как ответ на повтор, и создающее место читает такой отказ как свою же запись, пришедшую дважды | `TestARefusalAfterAResendSaysItWasResent`, `TestACollectionMadeOnAConnectionThatDiedIsStillMade` |

---

## sabre/dav (Baikal, Nextcloud)

### SV-1. vCard отдаётся в 3.0 и теряет свойства 4.0

- **Сервер.** Хранит как прислано, но GET без `Accept` и REPORT без `version`
  отдают 3.0 с конверсией: `KIND` пропадает, появляется `PRODID:-//Sabre//…`.
  RFC 6352 §10.4.2 это разрешает. Незнакомую версию отвергает
  `supported-address-data`.
- **Чем грозило.** Правка записывала 3.0 обратно, и карточка 4.0 теряла `KIND`
  насовсем — отчёт о потере этого не видел.
- **Carrel.** Все чтения карточек просят 4.0: отчёты — `address-data` с
  `content-type="text/vcard" version="4.0"` (запрос и поиск —
  `dav.CardQuerier`, multiget книги без запроса — `dav.CardFetcher`), GET
  одной карточки — `Accept: text/vcard; version=4.0` (`dav.CardGetter`,
  `v0.2.101`; до того GET шёл без `Accept`, и карточка, прочитанная по
  адресу, — запись под чужим UID после создания, переноса или импорта —
  приходила в 3.0). Отказ — повтор без версии; у GET отказ — и 406. Карточка
  3.0 после правки хранится в 4.0: меняется формат, не данные (`v0.0.60`).
- **Сторож.** `TestBaikalVCard4Edit`; без сети — `TestCardReadsAskForVCard4`,
  `TestCardReadsOfAConvertingServerKeepVCard4` (стенд `davtest`
  `ConvertsCards`: чтение без 4.0 — в 3.0 без `KIND`, каждым путём),
  `TestCardGetFallsBackWhenVersionIsRefused`.

### SV-2. text-match в CalDAV: регистр только у латиницы

- **Сервер.** Умолчание CalDAV — `i;ascii-casemap` (RFC 4791 §9.7.5): для
  кириллицы поиск регистрозависим. Знает `i;octet`, `i;ascii-casemap`,
  `i;unicode-casemap`, незнакомую collation отвергает 400.
- **Carrel.** Поиск по календарям шлёт `i;unicode-casemap` (`dav.CalSearcher`);
  отказ 400/403/412/415/422/501 — повтор с умолчанием и заметка «регистр только у
  латиницы» (`cal.ErrLatinCaseOnly`, `v0.0.57`).
- **Сторож.** `TestBaikalSearch`: «ковал» находит «Встреча с Ковалёвой».

---

## Radicale

### SV-3. text-match по составному свойству — 500

- **Сервер.** До 3.7.7 (19.07.2026) `text-match` по `N` и `ADR` — 500
  (`'Name' object has no attribute 'lower'`). Debian stable — 3.5.3.
- **Carrel.** Контакты ищутся по `FN, NICKNAME, EMAIL, TEL, ORG`, без `N`: `FN`
  обязателен и несёт то же имя (`v0.0.59`). Пересмотреть, когда дистрибутивы
  дойдут до 3.7.7.
- **Сторож.** `TestRadicaleSearch`; без сети — `TestSearchSendsNoTextMatchOnN`.

### SV-4. Фото vCard 4.0 обрезается

- **Сервер.** vobject режет `PHOTO:data:image/jpeg;base64,…` (RFC 6350) на
  первой запятой, с `VALUE=URI` так же; хранит `data:image/jpeg\;base64`.
  Целиком держится только форма 3.0 `PHOTO;ENCODING=b;TYPE=JPEG`. Задачи #1795 и
  #1863 закрыты без исправления.
- **Carrel.** Пишет по RFC; после записи — отчёт о потере `PHOTO` и действие
  «сохранить фото в старом формате» по одной карточке (`v0.0.61`, `v0.0.62`).
- **Сторож.** `TestRadicalePhoto` ждёт потерю и упадёт, когда Radicale станет
  хранить фото; без сети — `davtest.CutPhotoDataURI`.

### SV-5. PUT без Content-Length — 400

- **Сервер.** Chunked-тело PUT (HTTP/1.1 разрешает) — 400. Baikal и go-webdav
  принимают.
- **Carrel.** PUT, PROPPATCH и MKCOL всегда с `Content-Length`: `dav.Typed` и
  `dav.Sized` не доходят до `http.NewRequest` (`v0.0.40`, `v0.0.44`).
- **Сторож.** `TestRadicaleRecords`; без сети — `TestSizedBodyIsNotChunked`.

### SV-6. Права `owner_only` и отказы

- **Сервер** (умолчания, `testdata/radicale/config`):
  - календари и книги — только прямо в доме, один дом на оба рода, имена не
    совпадают;
  - нетипизированный `MKCOL` и путь от корня хоста к чужому принципалу — 403;
  - тело не iCalendar / vCard, несколько UID в одном ресурсе — 400;
  - `MOVE` коллекции — 405.
- **Carrel.** Коллекции создаются типизированно прямо в доме; адрес не меняется,
  перенос коллекции — копия записей и `DELETE` (S-12.3).
- **Сторож.** `TestRadicaleCollections`, `TestRadicaleAddressBookCRUD`,
  `TestRadicaleCalendarCRUD`.

### SV-7. well-known уводит в веб-интерфейс

- **Сервер.** `/.well-known/carddav` → 301 `/` → 302 `/.web` → HTML без
  принципала (RFC 6764 ждёт контекст DAV).
- **Carrel.** Цель well-known берётся, только если там есть
  `current-user-principal`; иначе — введённый адрес.
- **Сторож.** `TestRadicaleDiscoveryFindsBothKinds`; без сети —
  `TestWellKnownHTMLWithoutPrincipalStaysOnEnteredURL`.

Не особенность, но полезно: Radicale игнорирует `collation` и `version`, отдаёт
карточки как хранит и сравнивает текст через `lower()`.

---

## SFTPGo (x/net/webdav)

### SV-8. If-None-Match на PUT не соблюдается

- **Сервер.** `If-None-Match: *` игнорирует и перезаписывает, вместо 412
  (RFC 9110 §13.1.2; golang/go#44492). **OpenCloud ведёт себя так же**
  (16.09.2026): второй `PutCreate` того же имени отвечает 201.
- **Carrel.** Перед условным PUT, копией и переносом — `file.Stat` (`v0.0.53`).
  Окно между проверкой и записью остаётся.
- **Сторож.** `TestSFTPGoIgnoresIfNoneMatch`: второй `PutCreate` перезаписывает.
  Ответит 412 — проверку имени можно пересмотреть. То же ожидание держит
  `TestOpenCloudFiles`.

### SV-15. Тело без длины не принимается

- **Сервер.** OpenCloud (oCIS) за `Apache/2.4.65`: PUT, у которого нет
  `Content-Length`, отвергается — 400 по HTTP/1.1 и 502 по HTTP/2. Порог между
  8 КБ и 64 КБ: короткое тело Go успевает померить сам, дальше уходит потоком.
  С объявленной длиной оба протокола отвечают 201, имя с пробелами и кириллицей
  и путь пространства с `$` роли не играют — проверено по одному (16.09.2026).
  `Dav: 1, 2`, протокола возобновляемой загрузки сервер не объявляет.
- **Чем грозило.** Вложение крупнее порога и любая загрузка файла уходили
  потоком без длины: на таком сервере не записывалось ничего, а экран говорил
  только «сервер отказал».
- **Carrel.** Длина объявляется, когда известна: размер, присланный страницей
  (`size.<имя>`), или длина уже прочитанного файла у вложения до порога. Иначе
  запись идёт потоком, а отказ такого сервера становится строкой очереди с
  названной причиной: тело не копится ни в памяти, ни на диске — потолка размера
  файла нет (C-12, S-17.1).
- **Сторож.** `TestOpenCloudFiles` и `TestBaikalOpenCloudAttachments`.

---

## Baikal: окружение

### SV-9. Apache закрывает простаивающее соединение через 5 с

- **Сервер.** `KeepAliveTimeout` 5 с: PUT в закрытое соединение получает `EOF`,
  Go его не повторяет.
- **Carrel.** Повтор читающего запроса в страже (SV-14). Прежняя запись здесь
  говорила, что это не задевает, потому что `web` создаёт `dav.Client` на каждый
  запрос, — **неверно**: пул соединений живёт в `dav.Guard`, один на процесс, и
  новый клиент берёт из него старое соединение (16.09.2026, `v0.0.77`).
- **Сторож.** `TestBaikalIdleConnectionClosed` ждёт, что сервер закроет
  соединение через 3–15 с простоя (сам `EOF` ловится ненадёжно).

### SV-10. well-known уводит с https на http

- **Сервер.** `https://…/.well-known/caldav` и `…/carddav` — 302 на
  `http://baikal.mixeme.ru/dav.php`, оттуда 301 обратно на https (15.09.2026).
  Похоже на обратный прокси хостинга, не на код Baikal; не проверялось.
- **Carrel.** По перенаправлению с https на http не идёт — ни обнаружение, ни
  `dav.Guard`: пароль открытым текстом не уходит. Обнаружение остаётся на
  введённом адресе, шаг well-known в трассировке называет причину (`v0.0.64`).
- **Сторож.** `TestBaikalWellKnownDowngradesScheme`.

---

## Google

### SV-24. Два хоста, и рекомендованный адрес не пускает

- **Сервер.** Протоколы разведены по хостам, и обнаружение с одного не находит
  другой. Контакты: `https://www.googleapis.com/carddav/v1/principals/<почта>/`
  — принципал отдаётся, домашний набор книг находится, в нём одна книга
  «Address Book»; календарный набор там 404. Календари:
  `https://www.google.com/calendar/dav/` — принципал `…/<почта>/user`, дом
  `…/<почта>/`, два календаря, один из них виртуальный («Праздники России»);
  адресный набор там 404. А документированный
  `https://apidata.googleusercontent.com/caldav/v2/` отвечает **401
  `loginRequired`**: ему нужен OAuth2. То есть живым оказался адрес, который
  считается устаревшим, а рекомендованный паролю приложения отказывает.
  `.well-known/carddav` и `.well-known/caldav` на `googleapis.com` — 404 с
  HTML-страницей ошибки. Проба 17.09.2026.
- **Пароль приложения.** Подходит обоим живым адресам — OAuth2 для них не
  нужен. **Но оба живых адреса не заявлены.** Документация Google
  ([CalDAV](https://developers.google.com/calendar/caldav/v2/guide),
  [CardDAV](https://developers.google.com/people/carddav)) даёт только
  `apidata.googleusercontent.com/caldav/v2/` и
  `googleapis.com/.well-known/carddav`, и только через OAuth 2.0, а про Basic
  говорит прямо: «results in an HTTP 401 Unauthorized». Значит, Carrel с Google
  держится на поведении, которого Google не обещает. Сверено по первоисточнику
  23.09.2026. Google печатает его группами по четыре; пробелы там для чтения, и в
  Basic пароль уходит без них (`live_creds.go`).
- **Carrel.** Кода это не потребовало: well-known отваливается, но прямой путь
  — основной (S-6, шаг 2), и обнаружение с введённого адреса находит нужный
  дом. Человек заводит два подключения, по одному на протокол, как у Yandex.
  Оба рабочих адреса записаны в `credentials.txt` (17.09.2026, по просьбе
  владельца — он добавил найденные, не зная, какие отвечают), и там же помечены
  неработающие, чтобы их не пробовали заново. Ссылка на `basic.ics` осталась
  как есть: это файл для чтения, не сервер. «Праздники России» приходят
  `readOnly` — виртуальный календарь.
- **Сторож.** `TestGoogleContactInTheOnlyBook` — живая обёртка по тому, что
  этот аккаунт позволяет: календарь и вторую книгу здесь не создать
  (`MKCALENDAR` — 404, `MKCOL` — 400), поэтому пишется в единственную книгу,
  которая есть; и `TestLiveDiscoveryReport`, где google-carddav и google-caldav
  показывают каждый свой дом.

### SV-25. Новую карточку называет сам и переписывает UID

- **Уточнение 29.10.2026: текст напоминания.** При создании события Google
  заменил `DESCRIPTION:Reminder` у DISPLAY-напоминания за 10 минут на
  `DESCRIPTION:This is an event reminder`, сохранив действие и срок.
  С `v0.2.386` Carrel честно показывает потерю `VALARM`, ответ — форма с
  отчётом, а не перенаправление. Живой тест создания теперь проверяет
  именно этот отчёт и реально возвращённый текст; прочие потери не принимает.

- **Сервер.** `PUT` нового ресурса в книгу — **201** и `Location:
  <книга>/<свой-id>`: имя ресурса Google выбирает сам, а `UID` в карточке
  переписывает своим (16 шестнадцатеричных знаков вместо наших 32). `GET` по
  адресу, куда писали, отвечает **200** и отдаёт карточку — уже с чужим `UID`;
  `DELETE` по тому же адресу — **404**, удалять надо по адресу из `Location`.
  Календарь так себя не ведёт: событие сохраняет и `UID`, и имя ресурса
  (`PUT` 201, `GET` 200, `DELETE` 204), зато Google дописывает ему два своих
  `VALARM`. Пробы 17.09.2026.
- **Чем грозило.** После создания страница уводила на `recordPath(<наш UID>)`
  — адрес, которого на сервере нет: человек нажимал «Save» и получал 404 на
  запись, которая только что записалась целой.
- **Carrel.** Спека первой (S-7.1): созданную запись ищут по личности, которую
  ей дали; не нашлась — читают адрес, по которому её записали, и личность
  оттуда становится личностью записи (`web.finishCreate`). Сам `UID` при этом
  не потеря — его дал Carrel, а не человек, — поэтому сравнение после создания
  его не считает (`model.CompareAfterCreate`), и отчёт о потерях остаётся про
  содержимое. Ни под личностью, ни по адресу — уведомление «Saved, but the
  server does not show it» со ссылкой на коллекцию, как при переносе (SV-18):
  редирект в никуда был бы худшим ответом. `Location` не читается: адрес, по
  которому писали, у Google отвечает, а личность записи — не адрес, а `UID`.
- **Импорт.** То же при импорте файла: после последней записи назначение
  читается целиком один раз, карточка, не найденная под своим `UID`,
  дочитывается по адресу, и строка отчёта называет `UID`, который дал сервер
  (`web.importLanded`, с v0.2.37). До того отчёт писал «Created» под нашим.
  Ссылки между записями файла (`MEMBER`, `RELATED`) после этого
  переписываются на `UID` сервера, записи поправляются по If-Match
  (`web.followTheirs`, с v0.2.39, S-19).
- **Сторож.** `TestCreateWhereTheServerRewritesTheUID`,
  `TestCreateWhereTheServerShowsNothing` и
  `TestImportNamesTheIdentityTheServerGave` на `davtest` (`CreateRewritesUID`,
  `CreateHidesTheRecord`); живьём — `TestGoogleContactInTheOnlyBook`, где
  страница ведёт на карточку под тем `UID`, который оставил сервер.

### SV-26. Отчёт отвечает 207 без ответов, а allprop в address-data — 400

- **Сервер.** `addressbook-query` отвечает **207 без единого ответа**: с
  фильтром и без, с версией в `address-data` и без неё — книга с карточкой
  выглядит пустой. При этом `PROPFIND` глубины 1 её перечисляет, а
  `addressbook-multiget` отдаёт карточку целиком. Отдельно: `allprop` внутри
  `address-data` — **400 `INVALID_ARGUMENT`**, и в query, и в multiget. Пустое
  `address-data`, которым RFC 6352 §10.4 и просит всю карточку, принимается.
  Пробы 17.09.2026.
- **Чем грозило.** Дыра была наполовину наша. `card.cardQueryBody` писал
  `<C:allprop/>` внутрь `address-data`, а go-webdav в своём multiget —
  `<D:allprop/>`; ни того, ни другого RFC 6352 не определяет, и Google отвечал
  400 на оба чтения книги. Вторая половина — серверная: даже правильный запрос
  приходит пустым, поэтому список книги был пуст, запись не находилась по `UID`
  (404 на только что созданную карточку), а поиск молча ничего не находил. Тихая
  ложь, а не отказ.
- **Carrel.** `allprop` из `address-data` убран, а multiget теперь свой
  (`dav.cardMultiGetBody`) — тот же путь, каким в `dav` уже пишутся запросы с
  версией (SV-11). И пустому отчёту не верят на слово: неотфильтрованный
  запрос одних версий (`dav.QueryEtags`) против списка книги
  (`dav.ListCards`) стоит двух ответов без единой карточки и отличает пустую
  книгу от пустого отчёта (`card.confirmEmpty`). Доказано пустым — книга
  читается списком и multiget'ом, а поиск по ней просеивается здесь и говорит
  об этом, как при SV-23. Сервер с рабочим отчётом лишнего не платит: у него
  неотфильтрованный запрос отвечает, и на этом проверка кончается.
- **Сторож.** `TestHollowQueryIsCheckedAgainstTheBook`,
  `TestAnEmptyBookIsTakenAsEmpty` и `TestSearchLooksAgainWhenAQueryAnswersNothing`
  на `davtest` (`HollowCardQuery`, `RefuseAllPropInAddressData`);
  `TestQueryCardsNamesTheVersion` держит тело запроса; живьём —
  `TestGoogleContactInTheOnlyBook`.

### SV-56. Конец, равный началу, и пометка PREF выбрасываются и дописываются

- **Сервер.** Пробы 01.10.2026, `PUT` с `If-None-Match: *`, затем `GET`.
  Событие в календаре «Главный»:

  | в `VEVENT` | хранится |
  |---|---|
  | `DTSTART:20310311T093000Z` + `DTEND` равный | `DTEND` нет |
  | `DTSTART;TZID=Europe/Moscow:…T093000` + `DTEND` равный | `DTEND` нет |
  | `DTSTART` + `DURATION:PT0S` | `DURATION` нет |
  | `DTSTART;VALUE=DATE:20310311` + `DTEND;VALUE=DATE:20310312` | как прислано |
  | `DTSTART:20310311T093000Z`, конца нет | дописан `DTEND:20310311T103000Z` — час |

  Карточка в единственной книге: значения `TYPE` пишутся заглавными, и
  первые `TEL`, `EMAIL`, `ADR` получают `PREF` — `TEL;TYPE=cell` хранится как
  `TEL;TYPE=CELL,PREF`, голый `EMAIL` — как `EMAIL;TYPE=PREF`. День рождения
  без года (`BDAY:--0413` и `BDAY:--04-13`) выбрасывается молча; в форме Apple
  (`BDAY;X-APPLE-OMIT-YEAR=1604:1604-04-13`) хранится как
  `BDAY;VALUE=DATE,X-APPLE-OMIT-YEAR=1604:1604-04-13` — пометка внутри `VALUE`,
  отдельного параметра нет. Карточку, посланную в 4.0, Google хранит в 3.0
  (живой набор 04.11.2026).
- **Чем грозило.** С v0.2.80 (SV-53) новое событие без конца пишется с
  `DTEND`, равным `DTSTART`, и Google его выбрасывал: страница вместо события
  отвечала формой и «The server did not keep … DTEND» (живой набор
  01.10.2026, `TestGoogleEventInTheExistingCalendar`). Новый контакт с
  телефоном и почтой — то же про `EMAIL` и `TEL`. Оба отчёта о потере ложные:
  по RFC 5545 §3.6.1 событие со временем без конца кончается в начале, то
  есть это то же событие (S-11), а метки, выбранные человеком, на месте —
  `PREF` поставил сервер.
- **Carrel (v0.2.91).** Сравнение после записи (S-7.1) не считает потерей
  `DTEND`, который сервер выбросил, если его подразумевает `DTSTART`: равный
  ему у времени (те же параметры, тот же пояс), следующий день у даты, — при
  условии, что `DTSTART` вернулся как послан и ни `DTEND`, ни `DURATION` у
  записи нет (`model.impliedEndLeftOut`). И не считает потерей `PREF` среди
  значений `TYPE`, которого не было в посланном (`model.withoutAddedPref`);
  `PREF`, посланный и не сохранённый, — потеря по-прежнему. Регистр значений
  `TYPE` сравнение и раньше не различало (SV-32).
- **День рождения без года (v0.2.400).** Решение владельца 01.10.2026
  («потеря остаётся»: другие клиенты увидят год 1604) пересмотрено
  04.11.2026, **вариант А**: владелец посмотрел пробную карточку с
  `BDAY;VALUE=DATE,X-APPLE-OMIT-YEAR=1604:1604-04-13` — Google Контакты в
  браузере показывают «13 апреля · День рождения», на Android — «--04-13 ·
  День рождения», года 1604 нигде нет. Правка карточки 3.0 и так писала вид
  Apple (v0.2.229, Б30), но живой набор нашёл три ошибки: новая карточка
  уходила в 4.0 с `--0413` и теряла день; правка, которую Google сохранил,
  называлась потерей BDAY — сравнение не узнавало пометку внутри `VALUE`; и
  такая карточка показывалась с годом 1604. Теперь: пометка читается в
  обоих местах (`model.AppleOmitYear`); дни карточки (`BDAY`, `ANNIVERSARY`,
  `X-ABDATE`) сравниваются как дни — `--0413`, `1604-04-13` с пометкой и
  вид Google один день, `VALUE=DATE` — вид дня, а не потеря
  (`model.sameCardDay`); новая карточка, которую сервер сохранил в 3.0 без
  дня без года, пишется ещё раз с днём видом Apple (`web.yearlessAgain`).
  Год, набранный к такому дню, снимает пометку и там, где Google её держит
  (`VALUE=DATE`). Час, который Google дописывает событию без конца, Carrel
  больше не встречает: конец пишется всегда (SV-53).
- **Сторож.** `model.TestAnEndTheStartImpliesIsNotLostWhenTheServerLeavesItOut`,
  `model.TestAPreferredMarkTheServerAddsIsNotALoss`; на `davtest`
  (`DropsZeroLengthEnd`, `MarksThePreferred`) —
  `web.TestAnEndTheServerKeepsAsNoEndIsNotALoss` и
  `web.TestAPreferredMarkTheServerAddsIsNotALoss`; живьём —
  `TestGoogleEventInTheExistingCalendar` и `TestGoogleContactFields` (метку
  телефона читает без регистра и с `PREF` сервера, `mustLabel`). День
  рождения без года: `model.TestADayWithoutAYearInAnotherFormIsNotALoss`,
  `model.TestAppleOmitYearInEitherPlace`; на `davtest` (`StoresVCard3`) —
  `web.TestABirthdayWithoutAYearOnGoogle`; живьём —
  `TestGoogleBirthdayWithoutAYear` (зелёный 04.11.2026).

### SV-61. Двоеточие в значении карточки экранировано

Найдено 16.11.2026 полной сверкой «поле × трудная запись» на живых
серверах (решение владельца того же дня: «Бери полную сверку»).

- **Сервер.** Google хранит двоеточие в значении экранированным:
  `item2.URL;TYPE=PREF:https\://ada.example`,
  `X-SOCIALPROFILE;TYPE=TWITTER:https\://x.example/ada`. RFC 6350 §3.4
  экранирует только обратную косу, запятую, точку с запятой и перевод
  строки; go-vcard оставляет `\:` как есть. Сверх того при записи Google
  разбирает группы и собирает по-своему: `item1.TEL` приходит `TEL`,
  `URL` — `item2.URL` рядом с `item2.X-ABLabel:Work`; из двух `TITLE` и
  двух `ORG` оставляет по одному.
- **Были ошибки Carrel.** Карточка показывала «Website · pref:
  https\://ada.example» — с обратной косой и не ссылкой; форма — так же;
  новый адрес, сохранённый формой, отчёт о потере называл несохранённым
  (Carrel отправлял `https://`, читал `https\://`). Первая правка (`\:`
  — двоеточие при чтении записи) открыла вторую ошибку: список и поиск по
  `UID` читают ответ `REPORT` разобранным клиентом DAV и перечитывали его
  своим чтением только при параметре без имени — форма открывалась со
  значением `https\://`, проверка перед записью читала `https://`, и
  сохранить такую карточку было нельзя вовсе («Pre-save check failed: the
  field Address would not be saved as entered»).
- **Что делает Carrel** (`v0.2.416`). `\:` в значении — двоеточие, `\\:` —
  обратная коса и двоеточие (`card.plainColons`, до разбора go-vcard);
  ответ списка перечитывается и тогда, когда в нём есть `\:`
  (`card.ownRead`): одна карточка читается одинаково везде. Нетронутая
  строка уходит обратно, как пришла. Разбор групп и один `TITLE`/`ORG` —
  выбор сервера; сверка сравнивает имена свойств без группы.
- **Проверка.** `card.TestAnEscapedColonIsAColon` (запись и список, отчёт о
  потере); живьём — `TestGoogleContactFields/matrix`. На `davtest` вид
  Google не повторить: go-webdav отдаёт карточку через go-vcard, и `\:`
  приходит как `\\:`.

---

## Библиотеки

### SV-11. go-webdav, клиент

- `caldav.TextMatch` без поля collation (и в master, 15.09.2026);
  `carddav.AddressDataRequest` не передаёт `content-type` и `version`.
- **Carrel.** Свои REPORT в `internal/dav`: `CalSearcher` (`v0.0.57`),
  `CardQuerier` (`v0.0.60`). Появятся поля — свои REPORT можно снять.
- **Сторож.** `TestSearchCalendarNamesItsCollation`,
  `TestQueryCardsNamesTheVersion`.

### SV-12. go-webdav, сервер (под `davtest`)

- text-match с регистром, collation игнорирует; `carddav.Filter` без
  prop-filter не находит ничего (RFC 6352 §10.5 — всё); `MOVE` записи — 501;
  `DELETE` календаря уходит в удаление записи и отвечает 404; `calendar-color` в
  PROPFIND глубины 0 не отдаёт.
- **Carrel.** `davtest` всё это делает как настоящие серверы: регистр под
  `i;unicode-casemap`, все карточки на пустой фильтр, `MOVE` записи с 412 и 409
  (`v0.0.68`), `DELETE` коллекции со всем содержимым, `displayname` и
  `calendar-color` коллекции (`v0.0.70`). То же поведение живьём держат
  `TestRadicaleMove`, `TestBaikalMove`, `TestRadicaleBaikalCollectionMove`,
  `TestRadicaleCollectionDelete`, `TestBaikalCollectionDelete`.

### SV-14. net/http не повторяет PROPFIND и REPORT

- **Как ломается.** Сервер закрывает соединение, простоявшее в пуле (SV-9 —
  Apache через 5 с; у SFTPGo видели то же). Следующий запрос уходит в уже
  закрытое соединение и возвращает `EOF`. `net/http` пересылает запрос сам,
  но только для методов из своего списка безопасных — `GET`, `HEAD`,
  `OPTIONS`, `TRACE` (`Request.isReplayable`). `PROPFIND` и `REPORT` туда не
  входят **ни при каком** `GetBody`.
- **Чем грозило.** Случайный отказ на ровном месте: «сервер не отвечает» на
  экране, хотя сервер жив. Так падал один прогон `TestBaikalMove` из пяти
  (причина числилась ненайденной) и так не записалось вложение в живом S-20:
  `PROPFIND …-attach/: EOF` на первом обращении к файловому серверу в запросе,
  где перед этим уже ходили на календарный.
- **Carrel.** `limitTransport` в страже посылает запрос второй раз, если первый
  кончился до ответа (`EOF`, `ECONNRESET`, `EPIPE`, закрытое соединение) и тело
  перематывается (`GetBody`). Читающий запрос (`GET`, `HEAD`, `OPTIONS`,
  `PROPFIND`, `REPORT`) — при любом таком отказе. Повтор один: второй отказ
  подряд — это ответ сервера, а не гонка.
- **Запись тоже, но только с простоявшего соединения** (18.09.2026, решение
  владельца; спека S-5.1). На общем хостинге так же умирает и запись: в полном
  живом прогоне `MKCALENDAR` на Nextcloud упал с `EOF`, хотя в одиночку тот же
  тест зелёный, и на сервер ничего не ушло. Страж спрашивает у `httptrace`,
  откуда взялось соединение: взято из пула **простоявшим** (`Reused` и
  `WasIdle`) — сервер закрыл свою сторону до запроса и не видел его, повтор
  безопасен. Поднято ради этого запроса — не повторяем: там отказ ничего не
  говорит о том, что сервер успел сделать. И второе основание: всякая запись
  Carrel условна (S-8) или идемпотентна, поэтому долетевшая дважды отвергается
  предусловием (412, 405), а не удваивается. Тело, которое нельзя отдать
  заново, — файл в пути к другому серверу — не повторяется никогда.
- **Поэтому такое тело в пул не ходит** (`v0.0.127`, ревью 19.09.2026). Полный
  живой прогон потерял вложение на SFTPGo:
  `PUT …report-2.pdf: EOF (not sent again: its body cannot be produced a second
  time)` — загрузка ушла в соединение, которое сервер как раз закрыл, и
  повторить её было нечем. Запрос с неперематываемым телом теперь идёт через
  отдельный транспорт стража без пула (`DisableKeepAlives`): соединение
  поднимается ради него, и закрыть его заранее сервер не может. Цена — одно
  рукопожатие TLS на загрузку файла; записи с перематываемым телом ходят
  через пул, как раньше.
- **Сторож.** `TestAStreamedBodyTakesAConnectionOfItsOwn`,
  `TestReadOnlyRequestIsSentAgainAfterAnIdleClose`,
  `TestWriteOnAnIdleConnectionIsSentAgain`,
  `TestWriteOnAFreshConnectionIsNotSentAgain`,
  `TestAStreamedBodyIsNotSentAgain` в `internal/dav/retry_test.go`; живьём —
  весь набор, который до этого плавал.

## SoGo

### SV-16. text-match в календаре не соблюдается

- **Сервер.** `calendar-query` с `prop-filter` и `text-match` отдаёт **всю**
  коллекцию: три события в календаре, и на `Event 2`, на `zzzznotfound` и на
  запрос без text-match приходят одни и те же три (16.09.2026). Collation роли
  не играет — без неё то же самое. CardDAV при этом фильтрует правильно: на
  несуществующее слово `addressbook-query` отвечает ноль ответов.
- **Чем грозило.** Поиск по календарям возвращал бы коллекцию целиком: на
  «launch» приезжала задача «Встреча с Ковалёвой», на «LOVELACE» — все события.
- **Carrel.** `cal.Search` просеивает пришедшее по тем же свойствам, по которым
  спрашивал, без учёта регистра; у сервера, который отфильтровал сам, не
  отнимается ничего. Источник помечается заметкой «сервер не ищет по своим
  календарям, Carrel просеял ответ» (`cal.ErrServerDoesNotFilter`).
- **Сторож.** `TestSoGoSearch`.

### SV-17. Имя книги при создании и версия карточки

- **Сервер.** MKCOL адресной книги отвечает 201, но `displayname` берёт из
  адреса: книга, созданная с именем «Probe Book», называется `carrel-probe-…-a`.
  Отдельный `PROPPATCH` имя ставит (207). У календаря MKCALENDAR имя берёт.
  Карточка отдаётся в `VERSION:3.0` и с `Accept: text/vcard; version=4.0`, и
  без него, **но `KIND` и `GENDER` при этом целы** — свойства 4.0 не теряются,
  в отличие от sabre/dav (SV-1).
- **Чем грозило.** Каталог держал бы имя, которого на сервере нет, а лист
  удаления просил бы набрать его — при том, что человек видит другое.
- **Carrel.** После создания имя читается; разошлось — дошлётся `PROPPATCH`, и
  в каталог идёт то, что в итоге показывает сервер (`web.settleName`). Версию
  карточки Carrel не трогает: терять нечего, пишем по RFC (решение человека
  16.09.2026).
- **Сторож.** `TestSoGoCollections`, `TestSoGoCollectionDelete`,
  `TestSoGoVCard4Edit` — последний проверяет сохранность свойств, а не номер
  версии.

### SV-18. MOVE между коллекциями меняет UID записи

- **Сервер.** `MOVE` карточки в другую книгу отвечает 201, но в приёмнике лежит
  ресурс с именем сервера — и **с другим UID**: положили `UID:keep-me-917`,
  получили `139-6AAACB00-7-6910AA80.vcf` с `UID:139-6AAACB00-7-6910AA80`
  внутри. `FN` и чужие свойства при этом целы, прежнего имени в приёмнике нет
  (404). Проверено прямой пробой 16.09.2026.
- **Чем грозит.** UID — ключ, по которому Carrel находит запись и по которому
  S-16 считает дубликаты, а S-18 обещает не менять его ни при переносе, ни при
  копии. После такого переноса запись по прежнему адресу не находится:
  `moveOnServer` перечитывает приёмник по UID, не находит и уводит человека на
  адрес, которого больше нет.
- **Carrel.** Перенос на таком сервере называет случившееся вместо молчаливого
  редиректа: запись на месте, но под новым идентификатором. С 24.09.2026
  (`v0.2.30`) карточка под новым uid находится: до переноса запоминается,
  что лежит в книге-приёмнике, и после переноса перенесённая карточка —
  единственный ресурс, которого там не было. Уведомление ведёт на неё, а не
  в коллекцию. И главное: всё, что указывало на старый uid через
  `RELATED-TO`, перенаправляется на новый (S-15.4). Заметки и задачи,
  созданные из хронологии человека, держатся только на этом uid и без
  перенаправления молча выпадали бы из неё; заметил владелец. Если новых
  ресурсов не один, а ноль или несколько (кто-то писал туда одновременно),
  Carrel не угадывает: уведомление по-прежнему ведёт в коллекцию. Событие в календаре тот же сервер переносит, UID
  сохраняя, — подмена замечена только у карточек.
- **Сторож.** `TestMoveWhereTheServerChangesTheUID` на `davtest` с
  `MoveRewritesUID`: уведомление открывает карточку под новым uid;
  `TestAMovedCardKeepsWhatPointedAtIt`: задачи со ссылкой на старый uid
  указывают на новый, а `RELTYPE=SIBLING` при этом сохраняется. Живьём
  `TestSoGoMove`: живые тесты ищут запись по тому, что в ней написано
  (`liveRecordWhere`), а не по имени ресурса и не по идентификатору, принимают
  обе развязки (редирект там, где UID уцелел; уведомление там, где нет) и
  печатают найденные UID, когда не находят.

### SV-27. Хвостовые пустые части составного значения отбрасываются

- **Сервер.** Послали `N:Lovelace;Ada;;;` — вернулось `N:Lovelace;Ada`.
  Значение то же: RFC 6350 §6.2.2 читает короткое `N` как «остальное пусто».
  Хранит SoGo карточку в 3.0 в любом случае (SV-17). Проба 17.09.2026.
- **Чем грозило.** Контроль потерь (S-7.1) сравнивал значение текстом, поэтому
  правка любой карточки на SoGo заканчивалась отчётом «сервер не сохранил:
  `N`» — при целом имени. Ложная тревога хуже молчания: она учит закрывать
  отчёт не читая, а ради него он и сделан. Появилось 17.09.2026 вместе с полями
  имени (U-6.1): до них Carrel `N` не писал вовсе, и сравнивать было нечего.
  Поймал живой тест `TestSoGoVCard4Edit` — на фикстуре такого сервера не было.
- **Carrel.** Составные значения сравниваются по частям, хвостовые пустые не
  считая (`model.isStructured`: `N`, `ADR`, `ORG`, `GENDER`). Часть, которая
  действительно изменилась, по-прежнему потеря; у простого текстового свойства
  точка с запятой остаётся данными.
- **Сторож.** `TestCompareIgnoresTrailingEmptyComponents` — наша сторона;
  `TestSoGoTrimsEmptyNameComponents` — серверная: краснеет в тот день, когда
  SoGo оставит присланные части.

### SV-37. Заметок сервер не держит вовсе

- **Сервер.** Календарь заводится с `VEVENT`, `VTODO` и `VJOURNAL` в составе
  компонентов, `MKCALENDAR` отвечает успехом — а `PUT` заметки в эту же
  коллекцию отвечает **404**. Не 403 и не 415: адреса, по которому заметка
  могла бы лежать, для сервера просто нет.
- **Следствие для проверок.** Живая хронология сеет карточку, событие с
  адресом карточки в `ATTENDEE` и заметку с `RELATED-TO` на неё. На SoGo
  заметку посеять не выходит, и прогон, который на этом падал бы, проверял
  бы состав компонентов SoGo, а не хронологию Carrel. `trySeed` пишет в лог,
  чего сервер не взял, и снимает соответствующее ожидание; остальное
  проверяется как везде.
- **Проверка.** `integration.TestSoGoTimeline`.

### SV-60. Планировщик пишет организатора и состояние приглашения

Найдено 16.11.2026 полной сверкой «поле × трудная запись» (Baikal, SoGo;
Nextcloud — тот же sabre).

- **Сервер.** Событию, где владелец ящика добавил участников, сервер
  планирования (RFC 6638) пишет `ORGANIZER` — адрес владельца
  (`ORGANIZER:mailto:carrel@carrel.mixdep.ru`), отправленному приглашению —
  `SCHEDULE-STATUS=1.1` на строке участника. SoGo, став организатором,
  сверх того сбрасывает ответы участников в `NEEDS-ACTION` и разбирает
  передачу участия (`DELEGATED-TO`, участник, которому передали). Davis
  (тот же sabre) рассылает приглашения почтой и без почтового транспорта
  отвечает на всякую смену участников `500`
  (`Symfony\Component\Mailer\Exception\TransportException`), ничего не
  записав; Carrel так и говорит — «The server refused the change. Nothing
  was saved.» с ответом сервера в подробностях. Стенд шлёт приглашения в
  никуда (`MAILER_DSN: null://null`).
- **Была ошибка Carrel.** Сравнение после записи считало параметры строки
  целиком: участник, к которому сервер дописал `SCHEDULE-STATUS`, шёл в
  отчёт о потере — «the server did not keep: ATTENDEE», хотя сервер хранил
  его как отправлено.
- **Что делает Carrel** (`v0.2.416`). `SCHEDULE-STATUS` — параметр сервера
  (RFC 6638 §7.3: его ставит и меняет сервер), в сравнении не участвует
  (`model.withoutScheduleStatus`). Дописанный `ORGANIZER` — не потеря:
  сравниваются свойства, которые Carrel отправил. Что SoGo переписал у
  участников, отчёт называет — так и есть. Записи с участниками сверка на
  личных ящиках владельца (Yandex, Google, iCloud, Mail.ru) не сеет:
  сервер разослал бы приглашения (`personalAccount`).
- **Проверка.** `model.TestAScheduleStatusTheServerWritesIsNotALoss`;
  живьём — `TestBaikalForms/matrix/meeting`, `TestSoGoForms/matrix/meeting`.


### SV-64. Календарь с одними событиями держит и задачи

Найдено 17.11.2026 стендом приёмки на тестовых серверах (U-15.1).

- **Сервер.** «Проекты» и «Дежурства» набора заведены экраном Carrel с одним
  `VEVENT` в составе — `MKCALENDAR` ответил успехом, а состав, прочитанный
  после создания, шире: оба календаря стоят и в рейле задач, с нулём.
- **Carrel.** Ничего не меняет: каталог держит то, что сервер назвал
  (`settleComponents`, S-12.2), — экран говорит правду о сервере. Кадры
  набора (`s-tasks-1`) рисуют рейл задач без них: там календари Radicale,
  которые состав держат.
- **Сторожа нет.**

---

## Yandex

### SV-19. Новая коллекция уходит по адресу сервера

- **Сервер.** `MKCALENDAR` по запрошенному адресу — **201 Created**, но
  коллекции там нет: `PROPFIND` того же адреса отвечает 404, а в календарном
  доме появляется ровно один новый ребёнок с именем сервера:
  `events-38658948` вместо `carrel-probe-…`. Ни `Location`, ни
  `Content-Location` не приходят — куда положил, сервер не говорит. Прямая
  проба 16.09.2026. В доме, кроме календарей, лежат `inbox`, `outbox` и
  ребёнок с именем самого ящика.
- **Чем грозило.** Каталог держал бы адрес, которого на сервере нет: страница
  коллекции открывала бы пустоту. Хуже: перенос коллекции (S-12.3) берёт адрес
  приёмника до копирования и сложил бы туда запись за записью — все мимо.
- **Carrel.** После создания адрес перечитывается; там ничего — дом читается
  заново, и единственный ребёнок, которого до создания не было, и есть новая
  коллекция (`web.settlePath`, оба места: создание коллекции и перенос). Ноль
  или больше одного — не угадываем: человеку говорится, что сервер положил
  коллекцию по своему адресу, а какому именно — неизвестно, и что повторное
  обнаружение внесёт её в каталог (`web.lostCollection`). Имена сверяются без
  учёта регистра, а в каталог идёт то написание, которое показал сервер.
- **Сторож.** `TestCreateWhereTheServerPicksTheAddress` на `davtest` с
  `CreateAssignsItsOwnAddress`; живьём — `TestYandexCalendarLifecycle`: каталог,
  адрес на экране и сервер обязаны сойтись на одном адресе.

### SV-20. Адресную книгу создать нельзя

- **Сервер.** `MKCOL` книги в `/addressbook/<ящик>/` — **405 Not Allowed**, и
  отвечает nginx, а не CardDAV: метод закрыт на входе. Одинаково обоими
  паролями приложения, календарным и контактным (16.09.2026). Книга у ящика
  одна — `/addressbook/<ящик>/1/` «Personal», — и писать в неё можно: PUT
  карточки 201, GET возвращает её, DELETE убирает.
- **Чем грозило.** Общий живой набор начинается с засева книги: все семь
  обёрток падали на первом же `MKCOL`, не дойдя до того, что сервер умеет.
- **Carrel.** Кода это не меняет: отказ сервера и так приходит содержимым листа
  — метод, адрес, код. Живой набор для Yandex написан под то, что сервер
  разрешает, вместо общих обёрток.
- **Сторож.** `TestYandexAddressBookRefused`.

### SV-21. FN сервер строит из N, а присланный FN выбрасывает

- **Сервер.** Карточка возвращается с `FN`, собранным из `N`: послали
  `FN:Probe 4.0` при `N:Probe;Carrel;;;` — получили `FN:Carrel Probe`. То же с
  `VERSION:3.0` и с `4.0` (обе пробы 16.09.2026; хранит сервер в 3.0 в любом
  случае). Остальное цело: `N`, `EMAIL`, `UID` — как послали.
- **Чем грозит.** Форма контакта Carrel знает одно имя — `FN`
  (`web.editFields`: `fn`, `email`, `tel`, `org`, `title`), а `N` не пишет
  вовсе. Значит карточка уходит с `N:;;;;`, сервер строит из пустого `N` пустое
  `FN`, и контакт, заведённый на Yandex через Carrel, остаётся **без имени**: в
  книге лежит `FN:` и `N:;;;;` с целым `UID`.
- **Carrel.** Обхода не нужно: сервер сходится с тем, что у нас нарисовано.
  В кадрах правка карточки держит три поля имени — `Display name`, `Given`,
  `Family`, — а лист быстрого создания режет `Display name` слева направо
  (первое слово в `Given`, остальное в `Family`) и говорит об этом подсказкой.
  Читать `N` Carrel умеет давно: `web.contactName` берёт имя из `N`, когда
  `FN` пуст, а `card.knownProps` хранит `N` при записи чужой карточки. Не
  не хватало одного — полей ввода: `web.editFields` знал одно `fn` и писал
  только `FN`, поэтому карточка уходила с пустым `N`, и сервер строил из
  пустого пустое. Дыра была наша, не серверная. Закрыта 17.09.2026: спека
  первой (U-6.1), затем редактор — `Given` и `Family` пишутся в `N`, пустое
  отображаемое имя собирается из них, а заполненное при пустых частях режется
  слева направо. Живьём карточка теперь приезжает с `FN:Carrel Probe …` и
  `N:Probe …;Carrel;;;`, то есть сервер строит имя ровно из того, что мы
  прислали.
- **То же у Google.** Послали `FN:Carrel Echo Probe` при `N:Probe;Echo;;;` —
  вернулось `FN:Echo Probe` (проба 17.09.2026). Обход тот же и не понадобился
  второй раз: редактор пишет `N`, и имя приезжает целым.
- **Сторож.** `TestYandexContactInTheOnlyBook` держит поведение сервера как оно
  есть: краснеет в тот день, когда Yandex оставит присланное имя. У Google —
  `TestGoogleContactInTheOnlyBook`.

### SV-23. Отчёта addressbook-query нет вовсе

- **Сервер.** `REPORT addressbook-query` — **400 Bad Request**; тот же отчёт с
  фильтром — **403** с телом `<D:error><supported-report/></D:error>`, то есть
  предусловием RFC 3253: такого отчёта здесь нет. Версия в `address-data` ни
  при чём, без неё тот же 400. При этом `PROPFIND` глубины 1 перечисляет
  карточки, `addressbook-multiget` отдаёт их целиком с `address-data`, а
  `address-data`, запрошенное через `PROPFIND`, приходит с 404. Проба
  17.09.2026.
- **Чем грозило.** Книгу Carrel читал только этим отчётом, поэтому источник
  падал целиком: на экране «Personal is unavailable — сервер ответил не так,
  как ожидалось», ни имени, ни действий над карточкой. Запись при этом шла
  обычным PUT и проходила — карточка лежала на сервере, недоступная тому, кто
  её только что завёл. Четыре прогона из пяти, и я считал это случайностью,
  пока не спросил сам сервер.
- **Carrel.** Отказано дважды, и второй раз без версии — значит дело не в
  формате карточки, а в самом отчёте: книга перечисляется `PROPFIND` глубины 1
  и забирается `addressbook-multiget` (`dav.FetchCards`). Фильтр при этом на
  сервер не уезжает, поэтому поиск по книге просеивается здесь
  (`card.siftCards`), а источник говорит «этот сервер не ищет по своим адресным
  книгам, Carrel просеял ответ» (`card.ErrServerDoesNotQuery`) — так же, как с
  календарями SoGo (SV-16). Списку контактов не теряется ничего: multiget
  отдаёт всё.
- **Сторож.** `TestSearchFetchesWhatAServerCannotQuery` на `davtest` с
  `RefuseCardQuery`; живьём — `TestYandexContactInTheOnlyBook`: страница записи
  показывает имя и не несёт уведомлений.

### SV-34. Событие переписывается: правило, пояс и свойства, которых нет

- **Сервер.** Yandex принимает `PUT` события и хранит не то, что прислали:
  `RRULE:FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6` возвращается как
  `FREQ=WEEKLY;BYDAY=MO;UNTIL=20260216T080000Z;INTERVAL=1` — `COUNT`
  пересчитан в `UNTIL`, `INTERVAL=1` дописан, `WKST` снят. Время
  переписывается в свой пояс: `DTSTART;TZID=Asia/Yekaterinburg` приходит
  обратно иначе, вместе с `DTEND`. `STATUS` и `CATEGORIES` у события не
  хранятся вовсе, как и `X-CUSTOM` — последнее не берётся даже при прямой
  записи мимо Carrel. `PUT` задачи (`VTODO`) принимается с успехом, а
  ресурса по адресу потом нет.
- **Что делает Carrel.** Ничего особенного и не должен: каждое из этих
  расхождений — потеря по S-7.1, и человек видит отчёт C-9 с именами
  свойств. Отчёт правдив, обхода нет, и обход был бы враньём.
- **Следствие для человека.** После того как сервер переписал `COUNT` в
  `UNTIL`, форма приходит с заполненным «Until». Попытка задать «Times», не
  очистив «Until», упирается в ошибку на поле: «A series ends after a number
  of times or on a day, not both». Это верно — и это то, как переписывающий
  сервер выглядит с той стороны экрана.
- **Проверка.** `integration.TestYandexForms` (живьём). Части про контакты и
  задачи пропускаются с записью в лог: адресную книгу Yandex не заводит
  (SV-19), задачу не хранит.

### SV-35. Порядок частей правила — не потеря

- **Сервер.** SoGo хранит правило переставленным:
  `FREQ=WEEKLY;COUNT=6;BYDAY=MO;WKST=MO` там, где прислали
  `FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6`. Серия та же.
- **Была ошибка Carrel.** `model.CompareAfterPut` сравнивал значение
  свойства как строку, и на каждой правке повторяющегося события SoGo
  сообщал человеку, что сервер не сохранил `RRULE`. Серия была цела;
  сообщение было ложным, и вдобавок оно подменяло переход на карточку
  формой с отчётом.
- **Что делает Carrel.** Правило читается как множество частей: `RRULE` и
  `EXRULE` сравниваются по частям, перестановка потерей не считается,
  пропавшая или изменившаяся часть — считается.
- **Проверка.** `model.TestARuleReorderedIsTheSameRule` и два её соседа;
  живьём — `integration.TestSoGoForms`.


### SV-38. `ATTENDEE` события не хранится, вместо него ставится свой `ORGANIZER`

- **Сервер.** Событие, записанное через CalDAV с
  `ATTENDEE;PARTSTAT=ACCEPTED:mailto:…`, возвращается без `ATTENDEE` вовсе.
  На его месте появляется `ORGANIZER;CN=<ящик>:mailto:<ящик>@yandex.ru` —
  владелец учётной записи, кого бы ни прислали. Заодно переписываются время
  и правило (SV-34).
- **Следствие для человека.** Хронология ищет человека в трёх строках:
  `ORGANIZER`, `ATTENDEE` и `RELATED-TO` (S-15.4). На Yandex первая из них
  всегда указывает на владельца ящика, вторая стёрта. Поэтому хронология
  коллеги, которого позвали на встречу, на этом сервере пуста — и это
  правда, а не сбой: сервер не хранит того, о чём спрашивают.
- **Что делает Carrel.** Ничего особенного: опрос заканчивается, каждый
  источник отвечает «пусто», отчёт не поднимается, человеку пишется, что в
  этом промежутке ничего не нашлось, и предлагается расширить промежуток.
  Обхода нет: выдумать участника, которого сервер не хранит, значило бы
  соврать.
- **Проверка.** `integration.TestYandexTimeline` — обе половины на одном
  событии: карточка участника не находит ничего, карточка с адресом,
  который сервер вписал в `ORGANIZER`, находит событие со словом
  «organiser». Вторая — единственная живая проверка этой ветки, какая у нас
  есть. День, когда Yandex начнёт хранить `ATTENDEE`, красит первую.

### SV-62. Длительность хранится концом; запись меняется и пропадает на миг после записи

Найдено 16.11.2026 полной сверкой «поле × трудная запись».

- **Сервер.** Событие, присланное с `DURATION:PT1H`, Yandex хранит с
  `DTEND` — событие по длительности становится событием по концу. И
  запись, которую ему только что прислали, он иногда меняет сам: метка
  версии сдвигается без чьей-либо правки (два раза за четыре прогона —
  прямой записью теста и сохранением Carrel). Живой прогон 17.11.2026
  нашёл третье: на миг после `PUT` (ответ 201) Yandex отвечает на `GET`
  той же записи 404 и не отдаёт её в `REPORT`, а позже отдаёт снова (одну
  такую запись — через несколько минут, при проверке); новый календарь
  после `MKCALENDAR` (201) так же не сразу виден в доме. Случается не в
  каждом прогоне: два прогона форм Yandex из трёх в ночь на 17.11, затем
  ни одного в шести; в прогоне вехи (`a90c748f`) — две записи, у каждой
  один ответ 404 и запись через 0,6 с. Тела, которые послал Carrel,
  повторённые curl, Yandex держит — дело не в содержимом.
- **Что делает Carrel.** Длительность: форма показывает конец, правка
  пишет `DTEND` — того, что Yandex держит. Смена записи под формой: Carrel
  отвечает листом конфликта, как на чужую правку (C-9) — для него это
  неотличимо; сверка такой случай пропускает с записью в лог. Запись, не
  найденная при чтении после сохранения, — сохранение стоит, отчёта о
  потере нет (`reportWrite`); открытая в этот миг запись — 404,
  перечитывание её находит. Живой набор перечитывает 404 до 10 с
  (`getSettled`, `matrixSave`, `makeCollection`): запись, которой нет на
  самом деле, нет и после ожидания. Правки Carrel нет.
- **Новый календарь** (18.11.2026, стенд приёмки): `MKCALENDAR` по своему
  адресу (SV-19), а дом, прочитанный сразу, нового ещё не называет — лист
  писал «Made, but not at the address it was given … could not tell which»
  на каждый календарь. Созданный перед ним и показанный с опозданием
  оказывался новым рядом — два новых, и выбрать было нельзя. С `v0.3.8`
  дом перечитывается до 5 с (по 0,5 с), а из двух новых берётся тот, чьё
  имя отправлено (`web.settlePath`, `namedAmong`). Сторожа —
  `TestCreateFindsACalendarTheServerListsLate` (`davtest.ListsNewLate`),
  `TestCreateTellsItsCalendarByNameAmongTwoNew` (`davtest.ListLate`).
  Страница нового календаря, открытая сразу, может получить на `REPORT`
  404 — «is unavailable», «Try again» снимает.
- **Проверка.** Живьём — `TestYandexForms/matrix`.

---

## Mail.ru

### SV-22. Ссылку выдают на календарь, а не на дом

- **Сервер.** Человеку выдаётся адрес одного календаря:
  `/principals/mail.ru/<ящик>/calendars/<uuid>/`. `current-user-principal`
  там — 404 внутри 207, на корне сервера — 400. Зато `PROPFIND` самого адреса
  отвечает `<D:collection/><C:calendar/>`, `displayname` «Основной» и составом
  компонентов `VEVENT`, а `OPTIONS` признаёт `calendar-access`, `MKCALENDAR` и
  `REPORT` (проба 17.09.2026). То есть ссылка — это коллекция, и сервер об этом
  прямо говорит.
- **Чем грозило.** Обнаружение уходило в файловый откат по одной лишь ошибке
  принципала: человек подключал календарь и получал папку с файлами. При этом
  S-6 уже требовала различать их по роду ресурса — «файловые коллекции: род
  ресурса без календарного и адресного признака», — а код рода не спрашивал
  вовсе. Это было невыполненное требование, а не пробел в замысле.
- **Carrel.** Прежде чем счесть адрес файловым корнем, спрашивается род
  ресурса (`discover.addressIsCollection`): календарный или адресный признак
  означает выданную ссылку на одну коллекцию. Она берётся с именем и составом
  компонентов, домашних наборов у такого подключения нет, `FileOnly` не
  ставится, а в след идёт «the entered address is itself a calendar» —
  диагностика по S-6 обязательна. Отказ этого запроса ничего не ломает:
  поведение остаётся прежним, откат к файлам.
- **Файловые серверы не задеты.** У SFTPGo и OpenCloud род ресурса без
  признаков, и живой отчёт после правки показывает у обоих прежнее
  `fileOnly=true` с одной файловой коллекцией.
- **Сторож.** `TestEnteredAddressIsACalendar` и
  `TestEnteredAddressIsAnAddressBook` на фикстуре — оба с непустым списком
  папок, чтобы доказать, что файловый обход не сработал; живьём —
  `TestLiveDiscoveryReport`, где mailru показывает один календарь «Основной».

### SV-36. Единственный календарь, который переписывает почти всё

Первая живая проба самих записей (20.09.2026): до сих пор у Mail.ru была
проверена только выдача ссылки (SV-22), а что сервер делает с событием —
никто не смотрел.

- **Сервер.** Из присланного события Mail.ru не берёт `X-CUSTOM` — ни
  через Carrel, ни прямой записью мимо него. `EXDATE` и `RECURRENCE-ID`
  он **хранит**, переписав время в UTC (`EXDATE:20260119T070000Z` вместо
  `EXDATE;TZID=Europe/Moscow:20260119T100000`); до 11.10.2026 здесь было
  написано «не берёт» — проверка искала строку как присланную.
  `CATEGORIES` не хранится. Правило серии
  переписывается: `FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6` возвращается как
  `FREQ=WEEKLY;COUNT=6;BYDAY=MO` — части переставлены и `WKST` снят. Смена
  пояса переписывает время: `DTSTART`/`DTEND` приходят обратно не такими,
  как их отправили. И сверх того сервер **дописывает своё**:
  `X-MOZ-LASTACK` — когда в последний раз подтвердили напоминание.
- **Была ошибка Carrel.** `X-MOZ-LASTACK` сервер дописывает на одной записи
  и снимает на следующей. Carrel, который хранит всякую строку, какую сам не
  правит, отправлял её обратно и видел, что её нет, — и сообщал человеку о
  потере свойства, которого тот никогда не писал. Отчёт о потере, полный
  чужой бухгалтерии, учит закрывать отчёты о потере не глядя.
- **Что делает Carrel.** `X-MOZ-LASTACK`, `X-MOZ-GENERATION` и
  `X-MOZ-SNOOZE-TIME` считаются серверной отметкой наравне с `DTSTAMP` и
  `SEQUENCE` (`model.isServerStamp`). Остальное остаётся потерей по S-7.1 и
  показывается человеком в отчёте C-9: перестановка частей правила — не
  потеря (SV-35), а снятый `WKST` — потеря, и так и пишется.
- **Проверка.** `integration.TestMailruEventFields` (живьём),
  `model.TestTheBookkeepingAServerAddsIsNotALoss` и её соседка про свойство
  человека — на фикстуре.
- **SV-59, 11.10.2026.** Отступление, которое после смены времени серии
  ни на одно её повторение не приходится, Mail.ru выбрасывает — единственный
  из двенадцати. Так нашлась ошибка Carrel (ниже, «Серия с отступлениями»).
  С `v0.2.186`, когда отступления идут за серией, перенесённое повторение
  на Mail.ru остаётся; пропуск дня, которого в серии больше нет (серию
  сделали «через неделю»), он убирает, и отчёт о потере называет `EXDATE`.

---

## Nextcloud

### SV-63. После неверных паролей всякий ответ — через 25 секунд

Найдено 16.11.2026: полная сверка на Nextcloud шла час, а утром того же
дня весь его набор шёл 7,5 минуты.

- **Сервер.** Защита Nextcloud от подбора: после неверных паролей с адреса
  он задерживает каждый запрос с него — с верным паролем тоже — до 25 с,
  и держит задержку часами. Замер: три `PROPFIND` с верным паролем подряд
  — 25,5 с каждый. Неверные пароли шёл живой набор сам: проверка
  подключения нарочно ставит неверный (`liveConnectionEdit`), и каждый
  прогон замедлял следующие.
- **Что делает Carrel.** Ждёт: предел ответа DAV по умолчанию — 30 с
  (`CARREL_DAV_REQUEST_TIMEOUT`), запрос в 25 с проходит, медленно.
  Человек, ошибившийся паролем несколько раз, увидит Carrel медленным, а
  не сломанным. Живой набор неверного пароля Nextcloud больше не шлёт
  (`holdsAGrudge`, по пути `/remote.php/`), как iCloud (SV-58).
- **Проверка.** Замер выше; сверка `TestNextcloudForms/matrix` прошла
  все 34 случая без расхождений, шаг `disconnect` не успел до предела
  прогона.

### SV-65. Где лежат файлы, принципал не говорит

Найдено 17.11.2026 стендом приёмки на тестовых серверах (U-15.1).

- **Сервер.** Файлы пользователя — по `/remote.php/dav/files/<id>/`, где
  `<id>` — последний сегмент принципала `/remote.php/dav/principals/users/<id>/`
  (у тестовой учётной записи — «MIX_1 for tests», с пробелами; логин — адрес
  почты, и он не `<id>`). Свойства, которое называло бы этот адрес, у
  принципала нет. Адрес файлов, введённый как адрес сервера, `.well-known`
  уводит к принципалу, и обход дальше идёт по календарям и книгам.
- **Чем грозило.** Учётная запись Nextcloud в Carrel оставалась без файлов
  при любом введённом адресе; кадры (`s-files-6`, `s-plan-5`) рисуют её
  хранилище «Files».
- **Carrel.** Принципала вида `…/principals/users/<id>/` обход спрашивает и
  об адресе `…/files/<id>/` (`PROPFIND` глубины 0); папка там — хранилище
  «Files» той же учётной записи, её папки — папки в нём (S-6, `v0.3.5`).
  Принципал другого вида о файлах не спрашивают.
- **Davis** (18.11.2026, стенд приёмки на стендах Linux-машины) — то же:
  файлы по `/dav/home/<id>/` (`WEBDAV_ENABLED`) рядом с принципалом
  `/dav/principals/<id>/`. Учётная запись Davis тоже стояла без «Files».
  Принципала вида `…/principals/<id>/` обход спрашивает об `…/home/<id>/`
  (`v0.3.7`). Тот же вид у Baikal (`/dav.php/principals/<id>/`), а файлов у
  него нет — `/dav.php/home/<id>/` отвечает 404; поэтому отсутствие папки
  у такого принципала не сбой: след пишет «files: none at …» спокойно.
  Сторож — `discover.TestAnAccountOnDavisHasItsFiles`. Адрес файлов Davis,
  введённый сам (`/dav/home/<id>/…`), отвечает принципалом, и подключение
  берёт учётную запись (её «Files», календари и книги), а не папки этого
  адреса — по общему правилу S-6.
- **Сторож.** `discover.TestAnAccountOnNextcloudHasItsFiles` — davtest за
  прокси, который отдаёт его по адресам Nextcloud; живьём — `TestLiveStand`
  (`cmd/carrel`, `-tags integration`).

### SV-30. Удалённый календарь называет себя календарём, пока не спросишь

- **Сервер.** `DELETE` календаря — **204**, и он попадает в свою корзину. Но по
  прежнему адресу он отвечает **207** и остаётся среди детей календарного дома,
  а род ресурса зависит от того, о чём спросили: `PROPFIND` только про
  `resourcetype` отдаёт `<D:collection/><cal:calendar/>` — живой календарь;
  тот же `PROPFIND`, где рядом попросили `nc:deleted-at`, отдаёт
  `<D:collection/><nc:deleted-calendar/>` и время удаления. Второй `DELETE` по
  тому же адресу — снова 204, и ничего не меняется. Адресная книга так себя не
  ведёт: её удаляют сразу, и адрес отвечает 404. Nextcloud 28.0.3, проба
  18.09.2026.
- **Чем грозило.** Человек удалял календарь, Carrel его удалял и убирал из
  каталога — а следующее обнаружение находило его снова, живым, и предлагало
  вернуть в список. Удаление, которое отменяется само.
- **Carrel.** Спека первой (S-6): коллекция, которую сервер считает удалённой,
  — не коллекция. Обход дома уже делает свой `PROPFIND` за цветом, метками и
  правами; туда добавлено `nc:deleted-at` (`discover.propDeletedAt`), и
  коллекция с непустой отметкой в список не попадает. Заодно сервер начинает
  говорить правду и про род ресурса — вопрос о удалении включает её, — так что
  прежняя логика рода работает как была. След пишет, сколько пропущено: «1 in
  the server's trash bin, left out».
- **Живой набор.** Проверка после удаления больше не требует 404: коллекция
  либо исчезает, либо остаётся, перестав быть коллекцией своего рода. Так
  честнее для всех трёх поведений — Baikal и SoGo удаляют сразу, Nextcloud
  прячет в корзину.
- **Сторож.** `TestDiscoverySkipsWhatTheServerPutInItsTrashBin` на `davtest`
  (`Trash`, который отвечает `nc:deleted-at` только спросившему); живьём —
  `TestNextcloudCollectionDelete`.
- **Новая коллекция по тому же адресу** (17.11.2026, стенд приёмки). Корзина
  держит адрес удалённого календаря: календарь с тем же именем лист Carrel
  не заводит — «Something already lives at lichnyi/ on this server. Give it
  another name.», хотя ничего с таким именем человек у себя не видит.
  Адрес можно вписать другой. Решение владельца 18.11.2026: отказ говорит
  о корзине. Список занятых адресов листа (`homeChildren`) спрашивает и
  `nc:deleted-at`; адрес из корзины отказывается у Name — «Something
  deleted at lichnyi/ is still in this server's trash bin. Give it another
  name, or empty the trash bin on the server.», вписанный руками — у
  Address: «Something deleted at this address is still in the server's
  trash bin.» Сторож — `TestAnAddressHeldByTheServersTrashBinSaysSo`.

### SV-57. Число в папке говорит только Nextcloud, размеров картинки не даёт никто

- **Проба.** 02–03.10.2026, решение владельца по ФБ11 (строки папки «folder ·
  34 items» и плитка картинки «4032×3024» без запроса на строку). На каждом
  сервере — папка с вложенной (два файла и папка внутри), PNG 300×200 и JPEG
  640×480 с сегментом Exif в 20 КБ перед заголовком кадра. Один `PROPFIND`
  глубины 1 спрашивал `DAV:childcount`, `visiblecount`, `objectcount`,
  `hassubs`, `iscollection`, `oc:size`, `oc:fileid`, `oc:image`, `oc:photo`,
  `nc:contained-folder-count`, `nc:contained-file-count`,
  `nc:metadata-photos-size`, `nc:file-metadata-size`, `nc:metadata-photos-exif`,
  `nc:metadata-blurhash`, `nc:has-preview`; потом `propname` у папки и у JPEG;
  потом `GET` с `Range: bytes=0-65535` и `bytes=0-99`.

  | Сервер | Число в папке | Размеры картинки | `Range` на `GET` |
  |---|---|---|---|
  | Nextcloud 28.0.3 | `nc:contained-folder-count` и `nc:contained-file-count` у каждой папки ответа, и у читаемой тоже: у вложенной 1 и 3 (папка и файлы порознь); `DAV:childcount` и прочие — 404 | нет: `nc:metadata-photos-size` и `nc:file-metadata-size` — 404 и у свежих, и у картинок `Photos/` из набора самого Nextcloud; метаданные фото этот экземпляр не собирает, и имя свойства живьём не подтверждено | 206, `Content-Range` верный |
  | OpenCloud (за Apache 2.4.65) | нет: всё 404; `oc:size` — объём папки, не число | нет: `oc:image`, `oc:photo` — 404 | 206, `Accept-Ranges: bytes` |
  | SFTPGo 2.7.5 | нет: всё 404 | нет | 206, `Accept-Ranges: bytes` |
  | Stalwart (стенд Linux-ворот, 03.10.2026) | нет | нет | 206 — заголовок JPEG прочитан |
  | Davis (стенд Linux-ворот, 03.10.2026) | нет | нет | 206 — заголовок JPEG прочитан |
  | Yandex Диск | **не проверено**: `PROPFIND` — 402 (WebDAV Диска — только Яндекс 360) | — | — |
  | Mail.ru Облако | **не проверено**: 403 паролем приложения CalDAV; адреса и пароля WebDAV у нас нет | — | — |

  Radicale, Baikal, Xandikos, DAViCal, SoGo, Google и iCloud файлов не держат.
- **Что из этого следует.** Число в папке приходит даром только у Nextcloud —
  сумма двух чисел; Stalwart и Davis (проверены 03.10.2026 по решению
  владельца) молчат, как OpenCloud и SFTPGo. `DAV:childcount` (черновик свойств коллекций,
  draft-hopmann-collection-props, так отвечают IIS и Exchange) спрашивается
  заодно: в линейке его не дал никто, но вопрос ничего не стоит. Размеров
  картинки в `PROPFIND` нет ни у кого, а неподтверждённое свойство Nextcloud
  Carrel не спрашивает. Диапазон соблюли все пятеро.
- **Чем грозило.** Без этого — запрос на каждую строку (`PROPFIND` каждой
  подпапки, чтение начала каждой картинки) или «0 items» там, где сервер
  просто промолчал.
- **Carrel.** `file.List` просит три свойства в своём единственном `PROPFIND`;
  `Entry.Items` — `nil`, когда сервер не сказал, и тогда число берётся из
  списка этой папки, уже прочитанного сессией, иначе его нет (S-17.1).
  Размеры — только у одной выбранной картинки: `file.Provider.Dimensions`, один
  `GET` с `Range: bytes=0-65535`, чтение не дальше `conf.ImageHeadBytes` и
  тогда, когда сервер ответил 200 целиком, — ответ закрывается; разбор
  заголовка — `file.ImageSize`.
- **Сторож.** Без сети — `davtest` с `FolderCounts` (как Nextcloud),
  `ChildCount` и `FilesIgnoreRange` (200 и весь файл на любой `Range`):
  `TestListTakesWhatAFolderHoldsFromTheSameAnswer`,
  `TestDimensionsAskForTheHeadOnly`, `TestImageSizeReadsNoFurtherThanTheCap`,
  `TestFolderRowsSayWhatTheyHold`, `TestChosenImageSaysItsSize`. Живьём —
  `TestNextcloudFolderHeads` (число есть), `TestSFTPGoFolderHeads` и
  `TestOpenCloudFolderHeads` (числа нет; заговорит сервер — сторож упадёт), у
  всех трёх диапазон — 206.

---

## Серия с отступлениями: что делают серверы и что делал Carrel

Проба 11.10.2026 (`CARREL_SERIES_REPORT=1`, `integration/series_probe_test.go`)
— один сценарий на двенадцати серверах и на сервере-заглушке, который хранит
ровно присланное. Серия по понедельникам в 10:00 по Москве, 19 января
пропущено (`EXDATE`), 26 января перенесено на 12:00 (второй `VEVENT` с
`RECURRENCE-ID`). Запись прямо на сервер; три правки через Carrel — статус,
правило, пояс на Екатеринбург; контроль — то же смещение времени серии
прямой записью мимо Carrel.

| Сервер | После прямой записи | После смены времени серии | Перенос без `RECURRENCE-ID` |
|---|---|---|---|
| Baikal, Nextcloud, Davis | как прислано | отступления остаются со старым временем | не принимает (415) |
| Radicale | как прислано | остаются со старым временем | не принимает (400) |
| Stalwart, Xandikos, DAViCal, SoGo, iCloud | как прислано | остаются со старым временем | принимает и хранит |
| Yandex | как прислано; правило переписывает (`COUNT` → `UNTIL`) | остаются со старым временем; серию держит в московском поясе | 504 (сбой или отказ — не разобрано) |
| Mail.ru | хранит; время отступлений переводит в UTC | **убирает и пропуск, и перенос** | не принимает (412) |
| Google | — | — | в живом наборе этого сценария нет |

**Carrel** на всех шагах отправлял и серию, и перенос — ничего не
выбрасывал. Но при смене пояса или часа серии он переписывал только её
начало: `EXDATE` и `RECURRENCE-ID` оставались на 10:00 по Москве, а серия
шла уже в 10:00 по Екатеринбургу. Пропуск больше ничего не пропускал,
перенос ничего не заменял — на одиннадцати серверах молча, на Mail.ru с
потерей переноса.

**Решение владельца 11.10.2026:** отступления сдвигаются вместе с серией, а
если сервер что-то теряет или ломает — Carrel предупреждает. Сделано
`v0.2.186`: пропуски и привязки переносов получают то же смещение, что
начало серии, и пишутся так, как теперь написано начало; время, на которое
день перенесён, — слово человека, оно остаётся. После записи отступления
сверяются по мгновению, а не по написанию: те же дни в UTC — не потеря,
пропавший пропуск — `EXDATE` в отчёте, пропавший или отвязанный перенос —
`RECURRENCE-ID`.

**Чего правка не делает.** Серия, у которой меняют правило («каждую
неделю» → «через неделю»), может оставить отступление без своего дня — как
19 января в этой пробе; Carrel его не убирает и не предупреждает, пока
сервер его хранит. Отчёт называет отступления именами свойств, как рисует
кадр; слов «пропал перенос такого-то дня» в нём нет.

## iCloud

### SV-58. Неверный пароль закрывает календари и для верного

Найдено 10.10.2026 живым набором. Проверка подключения нарочно шлёт
неверный пароль, чтобы увидеть отказ на карточке, и сразу за ним — верный.
`caldav.icloud.com` после этого отвечает 401 и на верный: правка
подключения и пять календарных проверок следом упали, через пять минут
опроса верным паролем отказ ещё стоял; позже в тот же день те же проверки
прошли. `contacts.icloud.com` всё это время работал. Пароль приложения
при этом цел — владелец в ту же минуту подключил им адресную книгу в
Thunderbird.

Живой набор неверный пароль iCloud больше не шлёт (`holdsAGrudge`, список
из одного имени: спросить сервер здесь значит послать тот самый неверный
пароль, а ряд таких может закрыть учётную запись); половина проверки с
верным паролем идёт.

**Решение владельца 10.10.2026:** Carrel говорит как есть. Для него второй
отказ неотличим от первого — оба раза 401, а верен ли пароль, знает только
iCloud; совет «подождите» был бы домыслом. С `v0.2.185` сообщение «Could
not connect» при 401 называет отказ в логине и пароле (прежде — «Discovery
stopped at the principal step»), шаги остаются в трассе.

### SV-28. Календарный хост не пускает тем же паролем приложения

**Снята 30.09.2026.** Те же логин и пароль приложения (из
`credentials.txt`) календарный хост теперь принимает: `PROPFIND /` —
207, принципал `/<dsid>/principal/` тот же, что у адресного хоста, дом
`/<dsid>/calendars/`, в нём три календаря — «Напоминания» (только `VTODO`),
«Домашний» и «Рабочий» (только `VEVENT`), у всех `calendar-color`; принципал
объявляет `schedule-outbox-URL` и два своих адреса в
`calendar-user-address-set`. `/.well-known/caldav` по-прежнему 400, и
обнаружение идёт прямым адресом, как и раньше. Хост адресов не держит
(`addressbook-home-set` — 404), так что подключений два, по протоколу, как у
Google (SV-24). Что изменилось на стороне Apple — не видно: владелец, судя по
записи ниже, включил календарь в учётной записи с устройства Apple.
Сторож — `TestICloudCalendarDiscovery`: дом, оба рода календарей, цвета,
планирование «да» и свои адреса. Что встретилось дальше — SV-52, SV-53,
SV-54.

Как было до того:

- **Сервер.** `contacts.icloud.com` с логином Apple ID и паролем приложения
  отвечает: `PROPFIND /` — 207, принципал `/<dsid>/principal/`, адресный дом
  `/<dsid>/carddavhome/`, одна книга. `caldav.icloud.com` теми же данными —
  **401** на всё: `/`, `/.well-known/caldav`, `/<dsid>/principal/`,
  `/<dsid>/calendars/`, `p01-caldav.icloud.com`; `WWW-Authenticate: Basic
  realm="MMCalDav"`. `User-Agent` ни при чём — пробовали пустой, свой,
  браузерный и `dataaccessd`. `GET /.well-known/caldav` — 400 и с
  авторизацией, и без. Адрес с числовым идентификатором из CardDAV отвечает
  400, а не 401: путь сервер разбирает, но идентификатор не его. Проба
  18.09.2026.
- **Чем грозит.** Календарей iCloud в Carrel нет.
- **Carrel.** Обхода нет и быть не может: это отказ службы, а не особенность
  протокола. Пароль приложения у Apple один на учётную запись, и адресная
  служба его принимает — значит дело не в пароле, а в том, что календарная
  служба для этого Apple ID не включена. Подключение заводится на протокол, как
  у Google (SV-24): контакты работают, а календарный адрес честно отказывает —
  в следе обнаружения «принципал: 401», и подключения не возникает.
- **Подтверждено владельцем** (18.09.2026): учётная запись ограниченная, Apple
  просит войти в неё с устройства Apple, чтобы включить календарь. То есть дело
  и правда не в Carrel.
- **Сторожа нет**, и он тут ни к чему: держать надо не поведение сервера, а
  состояние учётной записи. Включат календарь — запись снимается.

### SV-29. У единственной книги нет имени, а у рода ресурса нет collection

- **Сервер.** `PROPFIND allprop` на книге не возвращает `displayname` вовсе —
  ни пустым, ни своим. `resourcetype` там — только
  `<addressbook xmlns="urn:ietf:params:xml:ns:carddav"/>`, без `<collection/>`,
  которого RFC 6352 §5.2 требует рядом. `MKCOL` второй книги — **403**.
  `max-resource-size` — 563200. Всё прочее образцово: карточка хранится как
  прислана (4.0, `UID`, `N` со всеми частями, чужое `X-CUSTOM`),
  `addressbook-query` отвечает и с фильтром, и без, `PUT`/`GET`/`DELETE` по
  адресу, который дал Carrel, работают.
- **Чем грозило.** Книга без имени: в списке подключений человек видит подпись,
  собранную из адреса.
- **Carrel.** Род ресурса определяется по адресному признаку, а не по
  `<collection/>` (`discover.resourceKind`), поэтому книга находится и
  приходит в каталог. А имя ей даёт обнаружение: сервер не сказал, как она
  называется, — значит именем становится то, **что она есть**: «Address book»
  (`discover.named`). Последний сегмент адреса, «card», человеку, чьи контакты
  в ней лежат, не говорит ничего. Несколько безымянных одного рода в одном
  доме различаются тем же сегментом — «Address book (card)», — потому что две
  одинаковые строки хуже технического слова (решение владельца, 18.09.2026).
  Вторую книгу завести нельзя, поэтому живой набор пишет в единственную, как у
  Google.
- **Сторож.** `TestICloudContactLifecycle`: карточка заводится страницей,
  открывается по адресу, правится по одному полю и удаляется.

### SV-52. Один компонент на календарь

- **Сервер.** `caldav.icloud.com`, проба 30.09.2026. `MKCALENDAR` с
  `supported-calendar-component-set` из нескольких компонентов отвечает
  **201** и оставляет **первый**: `VEVENT, VTODO` → `VEVENT`;
  `VTODO, VEVENT` → `VTODO`; `VEVENT, VTODO, VJOURNAL` → `VEVENT`. Тела у 201
  нет, и о выброшенном сервер не говорит — ни в ответе, ни в 404-propstat,
  как Xandikos (SV-41). `MKCALENDAR` с одним `VJOURNAL` — **500**: заметок
  iCloud не держит вовсе. Свои календари учётной записи такие же:
  «Напоминания» — только `VTODO`, остальные — только `VEVENT`. Сам дом
  `/<dsid>/calendars/` — простая коллекция с именем владельца и набором
  `VEVENT, VTODO, VFREEBUSY`; рядом `inbox/`, `outbox/`, `notification/`.
- **Попутно.** Имя ресурса помнит род того, что под ним лежало: `PUT` задачи
  в «Напоминания» под именем, которое минуту назад носило удалённое событие
  другого календаря, — **403** с телом `invalid attempt to update record
  from type 'EventArchiveV1' to 'ReminderArchiveV1'`. Carrel называет новую
  запись свежим случайным UID, так что его это не касается; тесты берут
  разные имена для событий и задач.
- **Что делает Carrel.** Каталог берёт набор из ответа сервера
  (`settleComponents`, S-12.2), поэтому календарь, заведённый со страницы
  «события и задачи», честно числится календарём событий и на экране задач
  не появляется. До v0.2.80 человеку об этом не было сказано: он просил
  задачи, а их там не будет, и узнавал он это, только не найдя календаря
  среди задач.
- **Обход (v0.2.80).** После создания набор, прочитанный с сервера,
  сравнивается с тем, что просили; то, чего сервер не взял, сеанс помнит
  про эту коллекцию (`Session.NoteRefusal`, `create:<компонент>`), и списки
  календаря (агенда, задачи, заметки) несут уведомление словами того, что
  сделал сервер (`web.droppedNotice`): «The server did not take tasks» —
  «The server made «Trips» for events only and said nothing of the tasks
  you asked for. Tasks need another calendar: make one for them on this
  server or another.» Второй календарь Carrel сам не заводит — это решение
  владельца, а не обход. `MKCALENDAR` с одним `VJOURNAL` (500) — отказ
  сервера на листе создания, как любой другой. Своего кадра у такого
  уведомления нет — задача 53 в [plan/to-typecase.md](plan/to-typecase.md).
- **Сторож.** `web.TestCreateSaysWhatTheServerDidNotTake` на стенде
  `davtest` с `CreateKeepsFirstComponent` (держит первый компонент набора,
  на `VJOURNAL` один — 500): уведомление называет задачи и то, что сервер
  оставил, календарь, взятый целиком, молчит, заметки одни — отказ 500 на
  листе.
- **Живьём.** `TestICloudCollections`: календарь «события и задачи»
  заводится страницей, в каталоге ровно те роды, что держит сервер, имя и
  цвет правятся, удаление по имени. `TestICloudCalendarDiscovery`: у каждого
  календаря один компонент. Живые наборы заводят события и задачи в разные
  календари (`icloudCalendars`), заметки не проверяются.

### SV-53. Событие без конца — 404

- **Сервер.** `caldav.icloud.com`, проба 30.09.2026, `PUT` с
  `If-None-Match: *` в календарь «Домашний»:

  | в `VEVENT` | ответ |
  |---|---|
  | `DTSTART:20310311T093000Z`, конца нет | **404 Not Found**, тело пустое |
  | `DTSTART;VALUE=DATE:20310311`, конца нет | **404** |
  | + `DTEND:20310311T103000Z` | 201 |
  | + `DURATION:PT1H` | 201 |
  | + `DTEND:20310311T093000Z` (равен началу) | 201 |
  | `DTSTART;VALUE=DATE:20310311` + `DTEND;VALUE=DATE:20310312` | 201 |

  RFC 5545 §3.6.1 такое событие разрешает: без конца событие со временем
  длится ноль, а с датой — один день. Отказ 404 на `PUT` по адресу, которого
  ещё нет, к тому же читается как «такой коллекции нет».
- **Что делало Carrel.** До v0.2.80 форма нового события без заполненного
  конца писала событие без `DTEND` (`DTSTART`, `DTSTAMP`, `LAST-MODIFIED`,
  `SUMMARY`, `UID`), и страница отвечала «The server refused the change …
  PUT … 404». То есть на iCloud событие, у которого человек не указал
  конец, не сохранялось вовсе.
- **Обход (v0.2.80).** Конец пишется всегда, когда его нет: `DTEND`,
  равный `DTSTART`, у события со временем и следующий день
  (`VALUE=DATE`) у события на дату — по RFC 5545 §3.6.1 это то же самое
  событие (S-11). Всегда, а не после 404: так дешевле на запрос, и одно и
  то же событие на всех серверах. Касается новой записи (полная форма и
  быстрый лист) и правки, которая стирает конец; событие другого клиента
  без конца, правленное в другом, остаётся как пришло (S-7), с `DURATION` —
  с ним (`web.endless`).
- **Попутно.** Поле `end.time` без `end.date` полная форма пропускала:
  конец писался только с датой, а без неё — равным началу. С v0.2.81
  (решение ведущего 30.09.2026) правило быстрого листа действует и в
  полной форме: пустая дата конца со временем — день начала, время раньше
  начала — следующий день (S-11).
- **Сторож.** `web.TestAnEventWithoutAnEndIsWrittenWithOne` на стенде
  `davtest` с `RefuseEventWithoutEnd` (404 на `VEVENT` без `DTEND` и
  `DURATION`): событие со временем и на дату с полной формы и с быстрого
  листа, правка, стирающая конец.
- **Живьём.** `TestICloudEventWithoutAnEnd` — событие со временем и на
  дату без конца через форму (30.09.2026 до обхода: 404, как в таблице);
  `TestICloudRecords` заводит событие без конца.

### SV-54. `text-match` в `calendar-query` не работает

- **Сервер.** `caldav.icloud.com`, проба 30.09.2026, `REPORT` на календарь,
  `Depth: 1`:
  - `comp-filter VEVENT` с `prop-filter UID` / `text-match` — **412
    Precondition Failed**, тело пустое, какой предусловие — не сказано;
  - `prop-filter SUMMARY` / `text-match` («carrel» с
    `collation="i;unicode-casemap"` и «Carrel» без неё) при двух событиях
    «Carrel probe» — **207 без единого совпадения**;
  - без фильтра и с `time-range` — 207, совпадения правильные;
    `calendar-multiget` — 207 с телами.

  Каждый ответ `calendar-query` несёт ещё и `<response>` самой коллекции
  (её адрес и `getetag`, без `calendar-data`).
- **Что делает Carrel.** Поиск по тексту на iCloud работает: пустой ответ
  перепроверяется чтением компонента целиком и отсевом у себя — путь SV-47,
  найденное идёт с `ErrServerDoesNotFilter`. А **запись по UID не
  находится вовсе**: `cal.Provider.FindUID` шлёт `calendar-query` с
  `text-match` по `UID`, получает 412 (повтор — тот же 412), и экран говорит
  «<календарь> is unavailable … REPORT <календарь> … HTTP 412». Так
  встречают карточка события и задачи, правка, «Mark done», перенос и
  удаление через страницы — то есть ни одна запись iCloud в Carrel не
  открывается, хотя списки, повестка и поиск их показывают. Создание
  работает: это `PUT`.
- **Обход (v0.2.80).** 412 (или 400, 403, 501) на `text-match` по `UID` —
  это «фильтра нет», и запись ищется без него (`cal.Provider.FindUID`,
  `findUnfiltered`): сперва `GET` по адресу `<коллекция>/<UID>.ics` (так
  Carrel сам называет записи), затем компонент, прочитанный целиком без
  фильтра, — через кэш сеанса (S-4.2, `ReadWhole`), — и сверка `UID` у
  себя, как у Yandex с `addressbook-query` (SV-23). Отказ сеанс помнит про
  коллекцию (`cal.Refusals`, `Session.NoteRefusal`, `uid-text-match`), и
  следующий поиск записи там фильтра уже не просит. Ответ коллекции о самой
  себе в отчёте запись не порождает.
- **Сторож.** `cal.TestAUIDTheServerWillNotFilterIsFoundWithoutTheFilter` и
  `web.TestARecordOpensWhereTheServerRefusesAUIDFilter` на стенде `davtest` с
  `RefuseUIDTextMatch` (412 с пустым телом на всякий `text-match` по
  `UID`): запись под своим именем находится `GET`, под чужим — в чтении
  целиком, UID сравнивается целиком, отказ спрашивается один раз за сеанс;
  карточки события и задачи открываются, правка сохраняется.
- **Живьём.** `TestICloudSearch` — поиск по двум соединениям.
  `TestICloudRecords`, `TestICloudForms` (событие и задача),
  `TestICloudMove` и открытие записей в `TestICloudInTheExistingCalendars`
  до обхода пропускались (30.09.2026: «reminders = []» — карточка события
  не открылась, «task done: status 200» и «move: status 200» с «REPORT …
  412» в подробностях); с v0.2.80 идут без пропуска.

### SV-55. Один ресурс на UID во всей учётной записи

- **Сервер.** `caldav.icloud.com`, проба 30.09.2026. Событие с UID `X`
  лежит в «Рабочем»; `PUT` события с тем же UID в «Домашний» — **412
  Precondition Failed**, тело пустое, под именем `X.ics` и под `X-2.ics` с
  `If-None-Match: *`, и под третьим именем без него. В самом «Домашнем»
  этих имён нет (`DELETE` — 404). То есть UID занят на всю учётную запись,
  а не на календарь, и 412 говорит не «имя занято», а «UID занят».
  Возможно, той же природы и 403 «EventArchiveV1 → ReminderArchiveV1» из
  SV-52: имя помнит род того, что под ним лежало.
- **Чем грозит.** Копия записи в другой календарь того же iCloud (S-18,
  «Copy here») несла тот же UID и до v0.2.81 не писалась: «Nothing was
  copied — The event is where it was», в подробностях `PUT … 412`. Перенос
  (`MOVE`) работает, импорт файла, чей UID уже есть в другом календаре,
  упрётся так же.
- **Обход (v0.2.81).** Решение владельца 30.09.2026: копия внутри такой
  учётной записи получает **свой UID**, как карточка при переносе в SoGo
  (SV-18). Для других клиентов это другая запись, с исходной она не
  складывается («linked ×2»: дубликаты сравниваются по UID), и
  экран говорит это словами: страница исходной записи с уведомлением
  «Copied, with a UID of its own» — «The server holds one record per UID
  across the whole account, so the copy in «Home» has a UID of its own. For
  other apps it is another event, not this one twice; duplicates are matched
  by that identifier.» — и ссылкой «Open it there» на копию. Связи не
  перевязываются: исходная остаётся, и что указывало на неё, указывает на
  неё же.
  **Когда.** Не «всегда внутри iCloud»: такое правило пришлось бы вешать на
  адрес сервера, а Carrel узнаёт особенности по ответам, не по имени хоста;
  и копия с другого подключения, чей UID в этой учётной записи ещё не занят,
  получила бы новый UID зря. Поэтому по ответу: копия пишется с прежним UID,
  как везде; **412 на имя, под которым ничего нет** (`GET` — 404), — это не
  «имя занято», а «UID занят», и копия уходит ещё раз под новым UID и
  своим именем. Сервер, отвечающий 412 на занятое имя, идёт, как прежде, к
  «-2»: его имя занято, и `GET` это показывает. Сеанс помнит учётную запись
  (`Session.NoteRefusal`, коллекция — пусто, `uid-account-wide`), и
  следующая копия внутри того же подключения идёт с новым UID сразу, без
  лишнего 412, а лист копии говорит это **до** начала: «…with a UID of its
  own: this server holds one record per UID across the account, so for
  other apps the copy is another event». Перенос (`MOVE`) UID не трогает и
  работает.
- **Импорт (v0.2.84).** То же правило: запись файла, чей UID уже лежит в
  другом календаре той же учётной записи, после 412 на свободное имя пишется
  ещё раз со своим UID, и её строка в отчёте говорит «Created under …: this
  server holds one record per UID across the account, and another of its
  collections holds this one»; она считается среди «under a new identity».
  Записи того же файла, что ссылаются на неё (`RELATED-TO`), идут за ней:
  ещё не записанные — сразу, уже записанные — правкой после последней.
  Проверка — `TestAnImportGetsAUIDOfItsOwnWhereTheAccountHoldsOneRecordPerUID`
  на `davtest.OneUIDPerAccount` (красная до правки: «Not created … 412»).
- **Сторож.** `web.TestACopyGetsAUIDOfItsOwnWhereTheAccountHoldsOneRecordPerUID`
  на стенде `davtest` с `OneUIDPerAccount` (412 с пустым телом на `PUT`
  календарного объекта, чей UID держит другой ресурс учётной записи, под
  любым именем): до обхода — «Nothing was copied»; после — копия с новым
  UID и всем, что было в записи, уведомление ведёт на неё, исходная не
  тронута, вторая копия не получает ни одного 412, лист её называет новый
  UID заранее. Сервер, отвечающий 412 на занятое имя, — прежний
  `TestMoveTakenNameAndTakenUID`: копия ложится под «-2» с тем же UID.
- **Живьём.** `TestICloudMove`: перенос `MOVE`, затем шаг «copy back» без
  пропуска (`CARREL_ICLOUD_SV55` снята) — копия находится в исходном
  календаре под новым UID, с повтором и исключением, уведомление её
  открывает, оригинал на месте, второй лист копии говорит о новом UID.
  30.09.2026: зелёный, все `TestICloud*` зелёные.

---

## Библиотеки

### SV-31. Ни одна из библиотек не сворачивает строки

- **Библиотеки.** `ical.Encoder.encodeProp` пишет `имя;параметры:значение
`
  без свёртки; `vcard.formatLine` — то же. Значит встроенное вложение или фото
  уходило одной строкой в сотни килобайт, тогда как RFC 5545 §3.1 и RFC 6350
  §3.2 просят не длиннее 75 октетов.
- **Наверх писать нечего: там уже отказали, дважды.**
  [go-ical#18](https://github.com/emersion/go-ical/issues/18) закрыт 25.02.2023
  словами автора: «I would rather not. This is just a SHOULD. I've implemented
  it for go-message and it caused a ton of regressions», а на вопрос, примет ли
  он PR, — «No.»; до него заброшен
  [PR #5](https://github.com/emersion/go-ical/pull/5) (2021).
  В go-vcard та же история: PR
  [#37](https://github.com/emersion/go-vcard/pull/37) и
  [#38](https://github.com/emersion/go-vcard/pull/38) закрыты — «It doesn't
  serve any actual purpose, and from my experience with go-message, is very
  difficult to get right». Проверено 18.09.2026.
- **Чем грозило.** Разбирающая сторона имеет право отказать в строке сверх
  предела: наши три сервера и jtx принимают, а следующий клиент — как повезёт.
  Узнать об этом от человека, у которого не открылась запись, — худший способ.
- **Carrel.** Одна свёртка на оба формата (`model.FoldLines`), потому что
  правило одно: не длиннее 75 октетов, перенос — `CRLF` и пробел, который
  разворачивание убирает. Применяется там, где документ уходит на провод и в
  выгрузку — `cal.Encode` и `card.Encode`, — а внутренние сборки остаются
  развёрнутыми. Свёртка не попадает внутрь символа (RFC 6350 §3.2 это
  рекомендует, а разрезанная руна — это испорченная запись, а не длинная
  строка), и повторная свёртка ничего не меняет.
- **Заодно понадобилось обновление.** В нашем пине go-vcard разворачивал
  свёрнутую строку конкатенацией в цикле — квадратично; свёрнутое фото на 500
  КиБ это ~7000 строк и около 1,7 ГБ копирования при чтении. В `v0.1.0` их
  же правка (issue #41) заменила это на `strings.Builder`, поэтому
  зависимость поднята в том же коммите, что и свёртка.
- **Сторож.** `TestFoldLines` и соседние в `internal/model`;
  `TestEncodeFoldsLongLines` у обоих провайдеров — 60 КиБ вложения и 40 КиБ
  фото уходят строками не длиннее 75 и возвращаются целыми. Живьём — весь
  набор, который пишет фото и вложения: он же показал, что серверы принимают
  свёрнутое как своё.

### SV-13. go-vcard экранирует запятую в data: URI

- **Библиотека.** Пишет `PHOTO:data:image/jpeg;base64\,…`.
- **Carrel.** `card.Encode` возвращает запятую у `PHOTO`, `LOGO`, `SOUND`, `KEY`,
  иначе любая правка ломала чужое фото (`v0.0.52`). С `v0.0.124` чужое фото
  уходит своей строкой (SV-32), а запятую сохраняет только фото, записанное
  заново.
- **Сторож.** Тесты фото в `internal/provider/card` и `internal/web`.

### SV-32. Группы, значения параметров и списки не переживают go-vcard

- **Библиотеки.** go-vcard читает `item1.EMAIL;type=INTERNET;type=HOME;type=pref`
  в поле с группой и тремя значениями `TYPE`, а пишет запятую внутри значения
  всегда экранированной: `CATEGORIES:work,friends` уходит
  `CATEGORIES:work\,friends` — одна категория. Кавычки параметра он режет по
  запятой: `LABEL="Street 1, Town"` становится двумя значениями. go-ical все
  значения параметра хранит.
- **Чем грозило.** Наш перевод в модель брал у параметра первое значение и не
  брал группу ни у одной библиотеки, а карточку писал целиком заново. Любая
  правка карточки из Apple Contacts отвязывала подписи (`X-ABLabel`) от адресов
  и телефонов, теряла `HOME`, `pref`, `VOICE`, склеивала категории, у события —
  второй адрес `DELEGATED-TO`. Проверка потерь этого не видела: перечитанная
  карточка сворачивалась так же. Найдено ревью 19.09.2026.
- **Carrel.** `model.RawValue` несёт группу и строку, в которой значение пришло
  (`Wire`); строка живёт, пока значение никто не тронул. Карточка пишется
  своими строками: нетронутое — как пришло, байт в байт; изменённое —
  заново, со всеми значениями параметра (`model.FormatParams`: списком —
  только у списочных параметров RFC, остальные в кавычках). У событий
  нетронутое значение собирается из своей строки со всеми параметрами.
  `CATEGORIES` и `NICKNAME`, записанные заново, остаются списком. Проверка
  потерь сравнивает значения параметра множеством без регистра.
- **Чего нет.** Строка списка, которую человек изменил, пишется без группы:
  подпись Apple у неё отвяжется. Остальные строки того же списка — нет.
- **Сторож.** `TestUntouchedLinesGoBackAsTheyCame`,
  `TestListEditKeepsTheOtherRows`, `TestFreshCategoriesStayAList`,
  `TestLossCheckReadsTypeAsASet`, `TestEventEditKeepsEveryParameterValue`.

### SV-33. go-ical паникует, go-webdav роняет весь отчёт

- **Библиотеки.** go-ical на строке `X-FOO;A=b` (параметр, после которого нет
  двоеточия) падает с `index out of range`, на `A="x"y:` — своей `panic`.
  `caldav.Client.QueryCalendar` разбирает все тела ответа и на первой
  нечитаемой записи возвращает ошибку за весь календарь.
- **Чем грозило.** Паника в горутине веера — конец процесса: все сессии,
  все ключи. Такую запись может прислать чужой сервер, подписка или файл
  импорта. Одна кривая запись, даже без паники, делала весь календарь
  «недоступным». Найдено ревью 19.09.2026.
- **Carrel.** Свой `calendar-query` в `dav` (`calquery.go`, тот же разбор
  ответа, что у поиска): нечитаемое тело — запись без данных, и провайдер
  называет её «не прочитана», как запись, которую отказался кодировать go-ical.
  Всякий разбор iCalendar — `dav.DecodeCalendar` и `cal.decode` — ловит панику
  и возвращает ошибку. `MultiGetCalendar` go-webdav обёрнут так же.
- **Сторож.** `TestCalendarQueryKeepsTheReadableRecords`,
  `TestMalformedRecordIsAnErrorNotAPanic`.


---

## Оборванные соединения

### SV-39. Повторённая запись встречает то, что сделала первая

- **Сервер.** Соединение обрывается **после** того, как сервер прочитал
  запрос, и **до** того, как пришёл ответ. Полный живой прогон 21.09.2026
  поймал это на Nextcloud (`77-222-37-121.swtest.ru`, Apache) дважды за
  один прогон: `PUT` с `If-None-Match: *` по адресу, куда никогда ничего не
  писали, ответил **412**, а `MKCOL` новой книги — **405**. Оба ответа
  значат «оно уже здесь», и оно было здесь, потому что это же самое
  обращение долетело первым разом. Там же третий случай — `MKCALENDAR`,
  оборванный на соединении, поднятом ради него: такой запрос не повторяется
  вовсе, и человек видит `EOF`.
- **Была ошибка Carrel.** Сторож повторяет запись, оборванную на
  соединении, которое пул держал простаивающим, и обосновывал это тем, что
  «сервер закрыл свою сторону до запроса и не видел его». Посылка неверна:
  соединение умирает и после чтения запроса. Данные это не портит —
  повторную копию отвергает предусловие (S-8), для того оно и есть, — но
  отказ доходил до человека как есть. Человек заводил календарь и читал
  «The server would not make it» про календарь, который уже стоял на
  сервере; заводил запись и читал «The server refused the change» про
  запись, которая уже лежала.
- **Что делает Carrel.** Второй ответ помечается сторожем как ответ на
  повтор (`dav.WasResent`); пометка ставится только изнутри и снимается со
  всякого первого ответа, так что сервер не может её себе приписать. Место,
  которое что-то создавало, читает 412 и 405 на помеченном ответе как «моя
  же запись, пришедшая дважды», и идёт дальше обычным путём — а обычный
  путь везде перечитывает адрес, так что неверная догадка обернулась бы
  честным «его нет в списке», а не выдуманной записью. Тронуты создание
  записи, создание коллекции, создание папки и копия при переносе; у
  последней повтор иначе завёл бы человеку два экземпляра вместо одного.
- **Чего это не чинит.** Запись, оборванную на соединении, поднятом ради
  неё, по-прежнему никто не повторяет: что сервер с ней сделал, неизвестно,
  и догадка тут была бы опаснее отказа.
- **Проверка.** `dav.TestARefusalAfterAResendSaysItWasResent` и
  `dav.TestAServerCannotClaimItsAnswerWasAResend`;
  `web.TestACollectionMadeOnAConnectionThatDiedIsStillMade` на стенде,
  который берёт запрос и рвёт соединение (`davtest.Server.HangUpOnce`).

### SV-40. `match-type` принят и не совпал ни с чем

- **Сервер.** Xandikos (`v0.2.12`). `addressbook-query` с
  `<text-match match-type="equals">` отвечает **207 без единого ответа** —
  тем же самым, чем отвечают на карточку, которой нет. Тот же фильтр без
  `match-type` ту же карточку отдаёт. Проверено на стенде 23.09.2026:
  `contains` по `UID` — карточка, `contains` по `FN` — карточка,
  `prop-filter` на наличие свойства — карточка, `is-not-defined` — карточка,
  а `equals` — ноль, и с точным `UID`, и с точным `FN`.
- **Почему это ловушка.** RFC 6352 §8.6.1 велит отказывать в
  неподдержанном `match-type` **403** с предусловием `supported-filter`.
  Ответ «207 и ничего» неотличим от правды, и откатываться не от чего: на
  проводе нет ни слова о том, что виноват фильтр.
- **Была ошибка Carrel.** `FindUID` просил `equals`. Список карточек
  открывался (он идёт без фильтра), а **одна** карточка не находилась —
  страница записи отвечала 404, и правка тоже. Ровно это и поймал живой
  прогон на Xandikos.
- **Что делает Carrel.** `FindUID` просит совпадение по умолчанию —
  подстроку, — а точный `UID` сверяет у себя, как и сверял раньше. Просить
  у сервера более узкое совпадение, чем нам нужно, незачем: подстрока даёт
  надмножество, а точность обеспечивается на нашей стороне. Возможность
  послать `equals` из `dav` никуда не делась, но ею никто не пользуется, и
  у поля написано, чего от неё ждать.
- **Проверка.** `card.TestFindUIDAsksForNoOptionalMatchType` на стенде,
  который отвечает пустотой на любой `match-type`
  (`davtest.Server.DeafToMatchType`).

### SV-41. Коллекция создана, цвет выброшен, ответ — 201

- **Сервер.** Xandikos (`v0.2.12`). `MKCALENDAR` с `displayname`,
  `calendar-color` и `supported-calendar-component-set` отвечает **201** и
  кладёт в тело `mkcalendar-response`, где `displayname` — 200, а
  `calendar-color` и набор компонентов — **404**. Календарь создан, цвета
  на нём нет. `PROPPATCH` тем же цветом сразу после — 207/200, и цвет
  стоит и читается.
- **Чем это отличается от SV-17 и SV-19.** Тем же самым по сути: сервер
  берёт не всё, о чём его просили, — но здесь он **честно говорит, что
  именно не взял**, и всё равно отвечает 201. RFC 5689 §3 описывает отказ
  в свойстве как отказ в создании; здесь создано и не поставлено.
- **Была ошибка Carrel.** `MkColProps` считал всякий 2xx полным успехом —
  и в комментарии у него было написано, что «отказ в свойстве не создаёт
  ничего». Создание коллекции клало в каталог тот цвет, который ввёл
  человек, ни разу его не перечитав. На Xandikos человек завёл бы синий
  календарь, каталог показывал бы синий, а всякий другой клиент — никакой;
  C-10 при этом называет этот цвет «тем, что прислал сервер».
- **Что делает Carrel.** Цвет улаживается так же, как имя (SV-17) и адрес
  (SV-19): перечитывается, при расхождении посылается ещё раз отдельным
  `PROPPATCH`, перечитывается снова, и в каталог идёт то, что держит
  сервер. Не удержал — в каталоге пусто, и C-10 берёт тон от адреса, как и
  положено, когда сервер цвета не прислал.
- **Проверка.** `web.TestCreateSettlesAColourTheServerDroppedAtCreation` на
  стенде, который роняет цвет при создании и называет его в 404-propstat
  (`davtest.Server.CreateDropsColour`). В живом наборе цвет перед проверкой
  досеивается (`seedColour`): что сервер сделал с `MKCALENDAR` — дело
  стенда, а проверяется, что каталог показывает то, что держит сервер.

### SV-42. Фильтр роняет сервер, если в книге есть не-ASCII

- **Сервер.** Xandikos (`v0.2.12`). `addressbook-query` с любым
  `text-match` по книге, где **хоть в одном значении** есть символ вне
  ASCII, отвечает **500**; в журнале сервера — его собственный
  `UnicodeEncodeError: 'ascii' codec can't encode characters`. Тот же
  запрос **без фильтра** те же карточки отдаёт спокойно, `PROPFIND` тоже,
  и сама карточка читается `GET`-ом.
- **Опыт по одной оси, 23.09.2026.** Одна и та же книга, одно и то же тело
  запроса; менялось только содержимое книги:

  | в книге | `anyof`-поиск |
  |---|---|
  | только ASCII-карточка | 207 |
  | + карточка со значением на кириллице | **500** |
  | + карточка с латинским `é` (кириллицы нет) | **500** |

  Текст, который ищут, значения не имеет: падало на ASCII-запросе
  `launch`. Значит это не про язык поиска и не про кириллицу, а про любой
  символ вне ASCII **в самой книге**.
- **Насколько это больно.** Поиск по контактам на таком сервере не
  работает ни у кого, у кого в записях есть хоть одно имя с диакритикой.
  То есть практически ни у кого.
- **Что делает Carrel.** Падение на фильтре — третья причина прочитать
  книгу окольным путём, рядом с «отчёта нет вовсе» (SV-23) и «отчёт пуст,
  хотя книга полна» (SV-26): книга перечитывается без фильтра, отсев идёт
  у нас, и рядом с находками стоит `ErrServerDoesNotQuery` — экран честно
  говорит, что искали мы. Условие узкое: **500 только на запросе, который
  нёс фильтр.** Без фильтра ронять было нечему, и такой отказ уходит
  тому, кто спрашивал, как уходил раньше.
- **Проверка.** `card.TestASearchSurvivesAServerThatBreaksOnTheFilter` на
  стенде, который отвечает 500 на всякий `text-match`
  (`davtest.Server.BreaksOnTextMatch`); там же проверено, что 500 без
  фильтра по-прежнему доходит до спрашивающего.

### SV-43. Набор компонентов не задать: календарь держит всё

- **Сервер.** Xandikos (`v0.2.12`). `MKCALENDAR` с
  `supported-calendar-component-set` отвечает 201 и называет это свойство в
  404-propstat — набор выброшен. `PROPPATCH` того же свойства отвечает
  **409 Conflict**: оно защищённое. `PROPFIND` всякого календаря отвечает
  `VEVENT`, `VTODO`, `VJOURNAL`, `VFREEBUSY`. И это не формальность: в
  календарь, заведённый под одни события, сервер принимает `VTODO` с 201.
- **Починить нечем.** В отличие от имени (SV-17), адреса (SV-19) и цвета
  (SV-41), послать ещё раз тут нечего: календаря «только под события» на
  этом сервере не существует.
- **Что делает Carrel.** Каталог берёт набор и выведенные из него виды из
  ответа сервера. Виды решают, на каких экранах коллекция показывается
  **вообще**, поэтому набор уже, чем у сервера, спрятал бы настоящие
  записи: задача, написанная в этот календарь другим клиентом, не
  появилась бы в Carrel нигде. Обнаружение всегда выводило виды так;
  создание было единственным местом, где они брались из формы. Правило
  записано в S-12.2: **в каталог идёт ответ сервера, а не то, что ввели в
  форму** — каталог это кэш сервера, а не данные человека.
- **Проверка.** `web.TestCreateTakesTheKindsTheServerReports` на стенде,
  который держит все компоненты, о чём бы ни просили
  (`davtest.Server.CreateIgnoresComponentSet`).

### SV-44. MOVE нет вовсе

- **Сервер.** Xandikos (`v0.2.12`). `MOVE` отвечает **405** и между
  коллекциями, и при переименовании внутри одной; `COPY` — тоже 405. Это
  написано в его README прямо: «RFC 4918 (Core WebDAV) — implemented, except
  for COPY/MOVE/LOCK operations». Я выяснял это пробами, хотя мог прочитать. В
  `OPTIONS` заголовок `Allow` перечисляет
  `DELETE, GET, HEAD, MKCALENDAR, MKCOL, OPTIONS, POST, PROPFIND, PROPPATCH, PUT, REPORT`
  — ни `MOVE`, ни `COPY` там нет. То есть это не отказ про конкретную
  запись, а отсутствие метода.
- **Что делал Carrel.** Говорил правду и останавливался: «Ничего не
  перенесено, запись там же», с кодом 405 в подробностях. Честно, но на
  таком сервере перенос записи был недоступен совсем.
- **Что делает Carrel теперь.** 405 или 501 на `MOVE` — это «метода
  здесь нет», и запись идёт тем же путём, каким она идёт на **другой**
  сервер, где `MOVE` неприменим по определению: копия пишется на новое
  место, перечитывается, сверяется на потери (S-7), и только копия,
  которая действительно там, стоит исходной. Не атомарно — но порядок
  такой, что оборвись он в любой точке, у человека остаётся запись, а не
  пустота; а каждый исход, кроме успешного, говорит о себе сам («Скопировано,
  исходная оставлена» с причиной). Отказ **про эту запись** — предусловие,
  права, занятое имя — по-прежнему доходит до человека как отказ.
- **Чем перенос отличается от переноса.** Копия — новый объект: её пишут,
  и она получает свой штамп (`REV`). На сервере с `MOVE` запись
  переезжает как есть.
- **Проверка.** `web.TestMoveOnAServerWithoutMoveCopiesThenDeletes` на
  стенде, где `MOVE` отвечает 405 (`davtest.Server.Refuse`).

### SV-45. Адрес из `.well-known` защищён, и это нормально

- **Сервер.** Stalwart (`v0.16.23`). `/.well-known/caldav` отвечает `307` на
  `/dav/cal`, а сам `/dav/cal` без входа отвечает `401`. Ничего
  неправильного тут нет: RFC 6764 §6 просит у этого запроса **адрес**, а не
  ресурс, и контекстный путь сервер вправе закрывать.
- **Была ошибка Carrel.** Проба `.well-known` идёт без учётных данных —
  так и надо, — но клиент **шёл по перенаправлению до конца** и судил по
  итоговому коду. Полезный `307`, в котором адрес уже назван, превращался в
  «шаг плохой», и обнаружение откатывалось на введённый адрес. У Stalwart
  корень — это веб-интерфейс, и `PROPFIND` по нему отвечает `200`, так что
  откат не спасал: все семь живых наборов падали на обнаружении.
- **Что делает Carrel.** Перенаправление **читается, а не проходится**:
  берётся `Location`, как и просит спецификация. Отказ от понижения
  HTTPS→HTTP при этом никуда не делся — раньше его формулировал клиент,
  теперь сама проба, и причина по-прежнему попадает в подпись шага трассы
  (SV-10), а не только в ошибку.
- **Проверка.** `dav.TestResolveWellKnownTakesTheAddressNotTheResource` —
  стенд перенаправляет на адрес, отвечающий `401`, и проверяется, что проба
  туда **не пошла**; `dav.TestResolveWellKnownSaysWhyItFailed` — понижение
  через TLS-стенд, с причиной в подписи.

### SV-46. Режим восстановления отдаёт только управление

- **Сервер.** Stalwart (`v0.16.23`). При `STALWART_RECOVERY_MODE=1`
  поднимается **единственный** слушатель — `http-recovery`, — и он
  обслуживает управляющий API. Обычная учётная запись на нём входит
  успешно и тут же получает `403`: в журнале сервера рядом стоят
  `Authentication successful` и `Unauthorized access … details =
  "authenticate"`. Даже у администратора дома DAV в этом режиме отвечают
  `404`, хотя `/dav/pal` их исправно называет.
- **Это не про Carrel,** а про то, как поднимать стенд, и записано, чтобы
  следующий не потратил на это вечер: сначала поднять **в** режиме
  восстановления и настроить через API, затем поднять **без** него — и
  работает тот слушатель, который создал план. В
  `docker-compose.linux.yml` режим стал переключателем
  (`CARREL_STALWART_RECOVERY`), а не постоянной настройкой.
- **Чего стоило не знать этого.** Все отказы `403` и `404` читались как
  нехватка прав, и час ушёл на роли и разрешения, которых там нет.

### SV-47. Календарный `text-match` совпадает только с целым значением без пробела

- **Сервер.** Xandikos (`v0.2.12`). `calendar-query` с `prop-filter` и
  `text-match` отвечает `207`, но находит запись, только если искомый текст
  **равен всему значению** свойства — и только если в этом значении **нет
  пробела**. Проверено 23.09.2026 с контролем (тот же запрос без фильтра
  отдаёт все события):

  | в календаре | ищем | нашлось |
  |---|---|---|
  | `SUMMARY:Oneword` | `Oneword` | 1 |
  | `SUMMARY:Oneword` | `Onewor` | 0 |
  | `SUMMARY:Two words` | `Two words` | **0** |
  | `SUMMARY:Two words` | `Two` | 0 |
  | `UID:launch-probe` | `launch-probe` | 1 |

  С коллацией `i;unicode-casemap` и без неё — одинаково. Фильтр по времени
  (`time-range`) работает, поэтому лента не страдает; страдает только поиск,
  и на настоящих названиях он не находит ничего.
- **Почему это ловушка.** Как и в SV-40, «207 и ничего» неотличимо от
  честного промаха.
- **Что делает Carrel.** Пустой ответ календарного поиска перепроверяется
  чтением компонента целиком и отсевом у себя. Если отсев нашёл то, чего не
  нашёл сервер, — находки идут с пометкой `ErrServerDoesNotFilter`, и экран
  говорит, что искал Carrel. Если и отсев ничего не нашёл — это честный
  промах, и экран молчит. Решение владельца 23.09.2026.
- **Цена, и сколько она продержится.** Полное чтение компонента на каждый
  поиск, не нашедший ничего, — на **всех** серверах, а не только на
  битых. Владелец заметил, что прочитанное должно ложиться в кэш, и следующий
  поиск шёл бы по нему; S-4.2 это и предписывает — карта версий, сверяемая
  по метке изменения, и тела по версии. С v0.2.35 так и есть: чтение
  перепроверки идёт через кэш сессии и остаётся в нём, а следующий поиск по
  неизменному календарю просеивается из хранимого и стоит серверу один
  PROPFIND за меткой. Цена платится раз на изменение календаря, а не на
  каждый поиск (`web.TestTheCheckOfAnEmptySearchIsPaidOnce`).
- **Проверка.** `cal.TestAnEmptySearchIsCheckedAgainstTheCollection` на
  стенде, где календарный `text-match` не находит ничего
  (`davtest.Server.CalendarTextMatchFindsNothing`), и
  `cal.TestAnHonestMissSaysNothingMore` — на здоровом стенде промах остаётся
  промахом, без пометки.

### SV-48. Отсутствие сказано статусом ответа, а не статусом свойств

- **Сервер.** Xandikos (`v0.2.12`). `PROPFIND` ресурса, которого нет, —
  события, карточки или чего угодно — отвечает **207** с одним
  `<response>`, у которого **свой** `<status>` `404 Not Found` и ни одного
  `<propstat>`. `href` при этом содержит весь URL в percent-кодировке:
  `http%3A//127.0.0.1%3A8081/user/calendars/calendar/no-such-event.ics`.
  `HEAD` того же адреса честно отвечает 404. Проба 23.09.2026. Нашлось это
  на файлах, которые Xandikos принимает, не обещая, но тот же ответ дают
  отсутствующее событие и отсутствующая карточка, то есть то, что он
  заявляет.
- **Это законно.** RFC 4918 §14.24: `response` несёт **либо** блоки
  `propstat`, **либо** собственный `status`. Большинство серверов на
  отсутствующий ресурс отвечают 404 целиком, и до этого дня второй формы нам
  не попадалось.
- **Была ошибка Carrel.** Разборщик multistatus читал только `propstat`. У
  ответа без них получались код `0` и пустые свойства, а `Stat` пропускал
  только **ненулевые** не-2xx коды — и строил запись из `href`. То есть
  **файл, которого нет, считался существующим.** Carrel проверяет
  свободность имени именно через `Stat` (часть серверов игнорирует
  `If-None-Match`, SV-8), так что на таком сервере каждое свободное имя
  выглядело бы занятым. Касалось это не только файлов: так читается всякий
  `PROPFIND`.
- **Что делает Carrel.** Ответ без `propstat` берёт код из собственного
  `status`. Правка в самом разборе, поэтому её получают все, кто читает
  `PROPFIND`.
- **Проверка.** `file.TestStatReadsAStatusThatIsNotInAPropstat` — тело
  ответа Xandikos в точности, как сервер его прислал, вместе со странным
  `href`. Живой набор по файлам Xandikos был и убран: файлов этот сервер не
  обещает.

### SV-49. Адресная книга без имени — 500

- **Сервер.** Davis (`ghcr.io/tchapi/davis-standalone:latest`, образ от
  06.09.2026, sabre/dav 4.7.1, база SQLite). `MKCOL` адресной книги без
  `displayname` отвечает **500**: в схеме Davis столбец
  `addressbooks.displayname` объявлен `NOT NULL`, а sabre/dav пишет туда
  `null` (`Integrity constraint violation: 19 NOT NULL constraint failed`).
  У Baikal и Nextcloud, на том же sabre/dav, такой книге место есть. Проба
  24.09.2026, первый прогон Davis.
- **Чем грозит.** Carrel — ничем: форма коллекции требует имя и всегда
  шлёт его в `MKCOL`. Упала затравка живого набора, которую тест заводит
  сам, без имени.
- **Что делает Carrel.** Ничего особого. Затравка набора коллекций теперь с
  именем, как у формы.
- **Проверка.** `TestDavisCollections`: затравка без имени снова дала бы
  500.

### SV-50. Имя коллекции экранирует ещё раз

- **Сервер.** DAViCal 1.1.12. `displayname`, присланный в `MKCALENDAR` или
  `PROPPATCH` как `A &amp; B` (то есть «A & B»), в `PROPFIND` приходит как
  `A &amp;amp; B`: любой клиент читает «A &amp; B». Так же `&lt;` и `&gt;`.
  Проба 24.09.2026, первый прогон DAViCal.
- **Carrel.** Экранирует один раз — это видно по шести другим серверам
  линейки на том же коде и по пробе curl с тем же телом. Имя, которое
  показывает сервер, и есть имя коллекции: каталог держит его (S-12.2), и
  человек увидит в рейле «Поездки &amp; планы», как увидит в любом другом
  клиенте. Раскодировать ответ за сервер Carrel не станет: это значило бы
  додумывать, что сервер «имел в виду».
- **Проверка.** `TestDAViCalCollections` с отметкой `escapesNamesTwice`:
  ждёт имя в том виде, в каком его показывает сервер, и требует, чтобы
  каталог держал именно его. У остальных серверов отметки нет, и имя
  сверяется буква в букву.

### SV-51. Поиск по книге с EMAIL или TEL теряет регистр кириллицы

- **Сервер.** DAViCal 1.1.12. `addressbook-query` с `test="anyof"` и
  несколькими `prop-filter` не находит **ничего**, если текст — кириллица
  в другом регистре и среди свойств есть `EMAIL` или `TEL`. Даже карточку,
  чей `FN` содержит текст: «ковал» по одному `FN` находит «Ковалёва Анна»,
  по `FN` и `EMAIL` — ничего, по `FN` и `ORG` — находит. В том же регистре
  («Ковал») и латиницей в другом регистре ищется правильно. Ответ — 207 без
  ответов, то есть неотличим от честного промаха. Проба 24.09.2026.
- **Что делает Carrel.** То же, что с календарями (SV-47): пустой ответ
  поиска по книге перепроверяется чтением книги целиком и отсевом у себя.
  Нашлось — находки идут с пометкой `card.ErrServerMissed`, и источник
  говорит, что книгу просеял Carrel. Не нашлось — честный промах, экран
  молчит. Чтение идёт через кэш сессии (S-4.2): следующий поиск по
  неизменной книге просеивается из хранимого.
- **Проверка.** `card.TestAnEmptyCardSearchIsCheckedAgainstTheBook`,
  `card.TestAnHonestCardMissSaysNothingMore`,
  `card.TestTheCardCheckReadsThroughReadWhole` на `davtest`
  (`CardTextMatchFindsNothing`); живьём — `TestDAViCalSearch`.

## Стенды Linux-машины через экраны Carrel (18.11.2026)

Набор кадров (`docs/visual/fixtures`) заведён экранами Carrel поочерёдно
на Radicale, Stalwart, Xandikos, Davis и DAViCal — каждый побывал главной
учётной записью набора — и на OpenCloud, SFTPGo и Davis для файлов
(`TestLiveStand`, `CARREL_STAND_ACCOUNTS`; стенды — `scripts/linux-stands.sh`
на своих портах). Строки списков и страницы записей сверены с тем же набором
на davtest; расхождения — ниже и в SV-65.

### SV-66. Заметка с двумя DESCRIPTION — 400

- **Сервер.** Radicale 3.5.4. `PUT` заметки с двумя `DESCRIPTION` («Заметка
  со встречи 10.08» набора) — 400 «Bad Request»; в журнале сервера:
  «VJOURNAL components cannot contain more than 1 DESCRIPTION». RFC 5545
  §3.6.3 разрешает `DESCRIPTION` у `VJOURNAL` несколько раз.
- **Чем грозило.** Отчёт импорта писал «dav: HTTP 400: Bad Request» — ни
  запроса, ни адреса.
- **Carrel.** Запись не пишется — она такая, какой пришла, и склеивать её
  `DESCRIPTION` за человека Carrel не станет. Строка отказа в отчёте — как в
  меню отказов (states-16): «Заметка со встречи 10.08 · PUT
  /test/zapisi/5c81-77aa-0d64.ics → 400 Bad Request» (`v0.3.7`).
- **Сторож.** `web.TestImportSaysWhatTheServerRefused`; живьём —
  `TestLiveStand`.

### SV-67. Вложение-текст хранится текстом; запись крупнее 512 КБ — 412

- **Сервер.** Stalwart 0.16. Вложение в записи (`ATTACH;VALUE=BINARY;
  ENCODING=BASE64`), чьи байты читаются как текст UTF-8, хранит и отдаёт
  текстом: `ENCODING` выброшен, `VALUE=BINARY` остался, значение — с
  экранированием TEXT (`# Release checklist\n…`). У PDF-заглушки набора — то
  же. Коллекции объявляют `max-resource-size` 524288; запись крупнее —
  412 с условием `CALDAV:max-resource-size` в теле.
- **Чем грозило.** Carrel читал такое значение как base64: размер — три
  четверти («96 KB» вместо «128 KB», «1.9 KB» вместо «3 KB»), скачанный
  файл — мусор. Заметка набора в 573 КБ получала 412, импорт принимал его за
  занятое имя, писал под другим и получал 412 снова.
- **Carrel.** Значение при `VALUE=BINARY` без `ENCODING=BASE64`, которое
  base64 быть не может, — файл текстом: экранирование снимается, размер и
  данные верные (`cal.keptAsText`, `v0.3.7`). Импорт сверяет размер записи с
  `max-resource-size` коллекции до записи (как вложения, S-20): крупная в
  предпросмотре «Too large: 573 KB — nextcloud · Записи takes records up to
  512 KB», в «Import N» не входит, на сервер не уходит, в отчёте — под
  «Failed» с теми же словами.
- **Сторож.** `cal.TestAnInlineFileKeptAsTextIsItsText`,
  `web.TestImportLeavesARecordTheCollectionCannotTake`; живьём —
  `TestLiveStand`.

### SV-68. Ящики планирования — обычные календари

- **Сервер.** DAViCal 1.1.12. В календарном доме — `.in/` («Carrel Inbox»)
  и `.out/` («Carrel Outbox»), `resourcetype` у обоих — `collection` и
  `calendar`, без `schedule-inbox`/`schedule-outbox` RFC 6638. Ящики
  называют только `schedule-inbox-URL` и `schedule-outbox-URL` принципала.
- **Чем грозило.** Оба стояли в рейлах календаря, задач и заметок как
  календари человека.
- **Carrel.** Обход спрашивает у принципала и `schedule-inbox-URL` (в том же
  запросе, что `schedule-outbox-URL`) и оставляет оба ящика вне списка; след
  — «2 scheduling boxes (inbox, outbox) left out» (S-6, `v0.3.7`).
- **Сторож.** `discover.TestTheSchedulingBoxesAreNotCollections`; живьём —
  `TestLiveStand`.

## Публичные сервисы через экраны Carrel (18.11.2026)

Набор кадров заведён экранами Carrel на тестовых учётных записях Yandex,
Google, iCloud и Mail.ru (решение владельца 18.11.2026: учётные записи
тестовые, данные в них — норма, после теста всё убирается). Каждый сервис —
главной учётной записью набора (`CARREL_STAND_ACCOUNTS="nextcloud=yandex"` и
т. д.), остальные места — SOGo, Baikal, SFTPGo. Где коллекцию завести
нельзя, стенд кладёт записи в уже существующую того же рода и после теста
удаляет из неё новое. Экраны сверены текстом с davtest без колонки
источника.

- **Yandex.** Любой из двух адресов (`caldav.yandex.ru`, `carddav.yandex.ru`)
  отдаёт и календари, и книгу — хватает одного подключения. Вторую книгу не
  заводит (`MKCOL` — 405, SV-20). Календарь держит один род: «Работа»
  (`VEVENT` и `VTODO`) стала календарём событий, блокнот — тоже («The server
  did not take notes»). **Задачи теперь хранит** (в списке, заведённом для
  `VTODO`; прежняя запись SV-34 «задачу не хранит» устарела), но без
  `DTSTAMP` (SV-69), без напоминаний, тегов, вложения и `RELATED-TO`. Новый
  календарь — SV-62. Запись с UID, который учётная запись уже держала, —
  SV-70.
- **Google.** Ничего не заводит (`MKCALENDAR` — 404, `MKCOL` — 400, SV-24);
  задач и заметок нет. Дописывает своё — SV-73. События и карточки в
  остальном совпали.
- **iCloud.** Заводить перестал: `MKCALENDAR` — 403 «no DAV:bind on the
  calendar home-set» (до 30.09.2026 календари заводились, SV-52), `MKCOL`
  книги — 403. Заметок нет (SV-52). Остальное — контакты, события с
  «Синк-апом» целиком, задачи — совпало с davtest.
- **Mail.ru.** Ссылка на один календарь (SV-22); книги, задач и заметок нет.
  Запрос о задачах — 400 (SV-71); событие с чужим организатором — 403
  (SV-72); владельца дописывает участником и теряет `STATUS` (SV-36).

### SV-69. Задача без DTSTAMP

- **Сервер.** Yandex. Задача, записанная с `DTSTAMP`, читается без него.
- **Чем грозило.** go-ical отказывается кодировать компонент без `DTSTAMP`,
  и Carrel показывал «4 records not shown: invalid iCalendar» — ни одной
  задачи Yandex.
- **Carrel.** `DTSTAMP` — отметка записи, не её содержание (S-7.1): записи
  без него дописывается отметка из `LAST-MODIFIED`, иначе `CREATED`, иначе
  времени ответа (`cal.stampMissing`, `v0.3.8`). Запрещённое иначе (конец и
  длительность сразу) по-прежнему «invalid iCalendar».
- **Сторож.** `cal.TestARecordWithoutItsStampIsRead`.

### SV-70. UID удалённой записи не берётся снова

- **Сервер.** Yandex. `PUT` записи с UID, который учётная запись держала и
  удалила (UID «Синк-апа» набора — от прежних прогонов), — 201, а `GET` через
  4 и 8 с — 404. Та же запись с новым UID хранится.
- **Carrel.** Отчёт импорта честен: «Not kept by the server · Синк-ап по
  релизу · not there after it was written». Правки нет.

### SV-71. Вопрос о задачах в календаре событий — 400

- **Сервер.** Mail.ru. `calendar-query` с `comp-filter` `VTODO` или
  `VJOURNAL` — 400 `{"title":"bad request","type":"caldav"}`; с `VEVENT` — 207.
- **Чем грозило.** Импорт спрашивал коллекцию обо всех трёх родах, и отказ
  на одном делал её «did not answer» — «Import N events» выключен, в
  Mail.ru ничего не импортировать.
- **Carrel.** Коллекция спрашивается только о том, что держит
  (`destRecords`, `v0.3.8`).
- **Сторож.** `web.TestImportAsksACalendarOnlyAboutWhatItHolds`
  (`davtest.EventQueriesOnly`).

### SV-72. Событие с чужим организатором — 403

- **Сервер.** Mail.ru. «Синк-ап» набора (организатор `mix@example.org`,
  участники) — 403; без `ATTENDEE` и `ORGANIZER` та же запись — 201.
- **Carrel.** «Failed · Синк-ап по релизу · PUT …/8f1c-4a02-b7d3.ics → 403
  Forbidden». Правки нет.

### SV-73. Google дописывает своё

- **Сервер.** Событию — свои напоминания («10 minutes before · 30 minutes
  before»; у события на весь день — «1 day before, 23:30»),
  `STATUS:CONFIRMED`, организатора участником («5 attendees» вместо 4).
  Карточке — `TYPE=pref` у телефона и почты; `CATEGORIES` карточки не
  хранит.
- **Carrel.** Дописанное показывается как есть — это запись сервера. Подпись
  значения карточки показывала «Phone · mobile, pref»: `pref` (и `INTERNET`,
  `VOICE` vCard 3) — не вид значения; с `v0.3.8` подпись их не несёт, как и
  форма правки (`notLabels`).
- **Сторож.** `web.TestATypedLabelLeavesOutWhatIsNoKind`.
