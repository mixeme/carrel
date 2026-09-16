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
| Nextcloud | — | вживую не проверялся; на sabre/dav, как Baikal |

Все три гоняются одним живым набором `-tags=integration` на Linux-машине
(`scripts/linux-gates.sh`); Baikal и SFTPGo — и с Windows. Учётные данные —
`dev/credentials.txt`, не в репозитории.

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
| SV-14 | net/http + любой сервер | закрытое простаивающее соединение: `PROPFIND` и `REPORT` не повторяются | повтор один раз в страже | `TestReadOnlyRequestIsSentAgainAfterAnIdleClose` |

---

## sabre/dav (Baikal, Nextcloud)

### SV-1. vCard отдаётся в 3.0 и теряет свойства 4.0

- **Сервер.** Хранит как прислано, но GET без `Accept` и REPORT без `version`
  отдают 3.0 с конверсией: `KIND` пропадает, появляется `PRODID:-//Sabre//…`.
  RFC 6352 §10.4.2 это разрешает. Незнакомую версию отвергает
  `supported-address-data`.
- **Чем грозило.** Правка записывала 3.0 обратно, и карточка 4.0 теряла `KIND`
  насовсем — отчёт о потере этого не видел.
- **Carrel.** Все чтения карточек просят `text/vcard; version=4.0`
  (`dav.CardQuerier`), отказ — повтор без версии. Карточка 3.0 после правки
  хранится в 4.0: меняется формат, не данные (`v0.0.60`).
- **Сторож.** `TestBaikalVCard4Edit`; без сети — `TestCardReadsAskForVCard4`.

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
  кончился до ответа (`EOF`, `ECONNRESET`, `EPIPE`, закрытое соединение),
  метод только читает (`GET`, `HEAD`, `OPTIONS`, `PROPFIND`, `REPORT`) и тело
  перематывается (`GetBody`). Пишущий запрос не повторяется никогда: сервер мог
  его принять. Повтор один — второй отказ подряд это ответ сервера, а не гонка.
- **Сторож.** `TestReadOnlyRequestIsSentAgainAfterAnIdleClose` и соседние в
  `internal/dav/retry_test.go`; живьём — весь набор, который до этого плавал.

### SV-13. go-vcard экранирует запятую в data: URI

- **Библиотека.** Пишет `PHOTO:data:image/jpeg;base64\,…`.
- **Carrel.** `card.Encode` возвращает запятую у `PHOTO`, `LOGO`, `SOUND`, `KEY`,
  иначе любая правка ломала чужое фото (`v0.0.52`).
- **Сторож.** Тесты фото в `internal/provider/card` и `internal/web`.
