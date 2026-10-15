# Разведка: расхождение версий (U-6.8) и коллекции (U-6.10)

**Что.** Повторный скан двух разделов по кадрам: что кадр рисует, чего
контракт сервера не несёт (или несёт иначе), и вопросы к кадрам, которые
становятся задачами Typecase. Старые сканы от 03.10.2026 («Раздел
расхождения версий: пробелы контракта», `client.md:3770`; «Раздел коллекций:
пробелы контракта», `client.md:4026`) взяты отправной точкой: у каждого
пункта сказано, держится он, закрыт или новый.

**По какому коммиту.** main `1c2aa48` (сервер `v0.2.254`, клиент `v0.4.40`);
макет `docs/visual/carrel-ui-mockups.html` — сборка Typecase `95453f6`.

**Как.** Только чтение. Кадры вырезаны по `data-frame="…"` до следующего
`data-frame` (не по точному `class="frame"`) и прочитаны как HTML вместе с
`<figcaption>`. Канонический кадр фикстуры — по `docs/spec/check.py`
(`FIXTURE_SCREEN`, `canon_frame` с шаблоном рода; `STATE_FRAME` строк для
этих разделов нет). Фикстуры — `docs/spec/fixtures/app-conflict*`,
`app-collections-*` (у каждого каталога одно состояние `normal`). Состояния
без фикстур читаны по коду обработчиков (помечено «по коду»):
`carrel/server/internal/web/record_conflict.go`, `handlers_collections.go`,
`collection_move.go`, `collection_delete.go`, `handlers_connections.go`.
Сервер не запускался; черновик `frame2go` не запускался (см. «Не проверено»).
Закрытое сверено с `carrel/server/CHANGELOG.md`, `docs/plan/server.md`,
`docs/plan/to-typecase.md` (до 186), `docs/plan/from-typecase.md` (до 68).

## Таблица: экран → маршрут → вью-модель → кадры → фикстуры

| Экран | Маршрут (C-5) | Вью-модель (C-6) | Кадры | Фикстуры |
|---|---|---|---|---|
| Расхождение, контакт | `/app/{kind}/{account}/{col}/{uid}/conflict` (`common.md:409`) | `ConflictPage`, `ConflictRow` (`view/records.go:373–405`) | `s-conflict-1` (маршрут, `mockups:10319`), `s-conflict-2` (планшет, `data-part="tablet"`, `:10563`) | `app-conflict/normal` |
| Расхождение, событие | `/app/calendar/…/conflict` | `ConflictPage` | `s-conflict-3` (`:10390`) | `app-conflict-event/normal` |
| Расхождение, задача | `/app/tasks/…/conflict` | `ConflictPage` | `s-conflict-4` (`:10448`) | `app-conflict-task/normal` |
| Расхождение, заметка | `/app/notes/…/conflict` | `ConflictPage` | `s-conflict-5` (`:10505`) | `app-conflict-note/normal` |
| Новая коллекция | `/app/collections/new` (`common.md:478`) | `CollectionNewSheet` (`view/cross.go:338`) | `s-profile-5` (маршрут, `:12897`), `-6` (отказ, `:13073`), фрагменты `-13` (выбор сервера, `:13060`), `-20` (новая папка, `:12649`), `-21` (никто не отвечает, `:12669`) | `app-collections-new/normal` |
| Переименование | `/app/collections/rename` (`common.md:479`) | `CollectionRenameSheet` (`cross.go:350`) | `s-profile-7` (`:13221`), фрагмент `-18` (папка, `:12595`) | `app-collections-rename/normal` |
| Перенос и копия | `/app/collections/move` (`common.md:480`) | `CollectionMoveSheet` (`cross.go:363`) | `s-profile-8` (`:13316`) | `app-collections-move/normal` |
| Удаление | `/app/collections/delete` (`common.md:481`) | `CollectionDeleteSheet` (`cross.go:377`) | `s-profile-9` (`:13406`), фрагмент `-19` (папка, `:12616`) | `app-collections-delete/normal` |
| Меню строки коллекции (вход в листы) | `/app/settings/connections` (U-6.11) | `ConnectionsPage`, `collectionRowMenu` (`handlers_connections.go:640`) | `s-profile-1` (`:12201`), фрагменты `-12` (папка, `:12394`), `-17` (хранилище целиком, `:12579`) | — (раздел настроек) |

Все четыре листа коллекций — в `SHEETS` (`check.py:250`): рейл не
сверяется, экран под листом у них — экран кадра (см. Кл1).

---

## Расхождение версий: пробелы серверу (Рв)

**Что закрыто из старого скана.** РБ2 (форма выбора без сценариев) и РБ9
(параметры в C-5) — `v0.2.153`: `ConflictPage.Form`, `ConflictRow.Name`,
`ConflictRow.Choice`, C-5 называет `op`, `etag`, `choice.<n>`. РЖ1 (`Diff` в
форме) — записка 35 (задача 132: `Diff.Action`, `Check.Radio`). РЖ3 (планшет
без «Use only the server version», New note без стрелки) — записка 35:
`s-conflict-2` сейчас — те же четыре свойства и четыре кнопки (проверено:
`mockups:10563…`). Строка «FN agrees» — записка 35: **остаётся**, правится
U-6.8 (см. «Спеке»). `ReadAt`/`ServerAt` кадры не рисуют — записка 35.
Держатся: РА1, РБ1, РБ3, РБ4, РБ5, РБ6, РБ7, РБ8 (`server.md:1030–1032` —
«Осталось… РБ3, РБ4, РБ6»).

- **Рв1 (РБ1, держится). Подзаголовок вместо сообщения.** (**закрыто сервером `v0.2.265`**: `ConflictPage.Subtitle`) Кадры
  `s-conflict-1…-5`: `tc-head__sub` «Another client changed this contact
  while you were editing it. Nothing has been overwritten.» (у события —
  «this event», задачи — «this task», заметки — «this note»), сообщения
  `tc-msg` над карточками нет. Модель: `Notices[0]` — `role: alert`,
  `title: "Conflict"`, `body` с этой фразой (`record_conflict.go:154–159`;
  все четыре фикстуры). Экран, нарисовав `Notices`, разойдётся с кадром.
  Нужно: `ConflictPage.Subtitle` (слово рода — данные), тревогу не слать;
  «Conflict» — слово экрана (`Shell.Title` «Changed elsewhere» — заголовок
  документа, не шапки).

- **Рв2 (РБ3, держится). Вид различия у свойства.** (**закрыто сервером `v0.2.267`, `v0.2.271`**: `ConflictRow.Diff` — `both`, `mine`, `theirs`; черновик держит всю исходную версию; поле, которое изменил только сервер, — своя карточка; чужие свойства и строки по параметру — Рв6) Кадры: справа от
  свойства `tc-rubric__aside` — «both changed», «only in your edit», «only
  on the server», и от него подписи вариантов — «Your edit» / «Server»,
  «Your edit · include» / «Leave it out», «Your edit» / «Server · include»
  (`s-conflict-1`: TEL;work, EMAIL;work, X-EVOLUTION-FILE-AS; `-3`
  LOCATION; `-4` DUE). Модель: `ConflictRow` — `Property`, `Mine`,
  `Theirs`, `Choice`, `Name`; признака вида нет (`records.go:393–405`).
  Вид требует третьей версии — той, с которой открыли форму: у черновика
  отказа её значений нет (`editDraft` — `Mine`, `ETag`, `ReadAt`,
  `record_conflict.go:21–38`), и «only on the server» сервер сейчас не
  различит вовсе (Рв6). Нужно: `ConflictRow.Diff` (C-8: `both`, `mine`,
  `theirs`) и база для него; подписи вариантов — слова экрана по виду.

- **Рв3 (РБ3, держится). Пустое значение — «—», не «Empty».** (**закрыто сервером `v0.2.265`**: пусто — пустая строка) Кадры:
  отсутствующее значение — «—» (`s-conflict-1` «Leave it out —», «Your edit
  —»). Модель: `showValues` возвращает «Empty» (`record_conflict.go:132–134`);
  фикстура `app-conflict`: `rows[1].theirs = "Empty"`. Нужно: пусто — пустая
  строка (слово «—» — экрана).

- **Рв4 (РБ4, держится). Что выбрано заранее.** (**закрыто сервером `v0.2.271`**: у `theirs` — серверное; при `Unsure` — вопрос Т1) Кадры: своё — у «both
  changed» и «only in your edit», серверное — у «only on the server»
  (`s-conflict-1` choice.4 `value="server" checked`, `s-conflict-4` DUE
  `checked` у Server). Модель: `Choice: view.ChoiceMine` у каждой строки
  (`record_conflict.go:168`), и тем же сервер пишет строку, которой нет в
  запросе (`chosenSide`, `:224–230`). К тому же сообщение «Some of these may
  not be your changes» (`:172–176`, `v0.2.204`) советует «choose the
  server's value» — а выбрано заранее своё везде. Нужно: выбор по виду
  (Рв2); при `Unsure` — см. вопрос Т1.

- **Рв5 (РБ5, держится; решено запиской 35). Совпавшее.** (**закрыто сервером `v0.2.265`**: `ConflictPage.Agree`) Кадры: строка
  `tc-diff__agree` «‹FN› agrees — kept as it is.» над карточками (`-3`
  DTSTART, `-4` STATUS, `-5` CATEGORIES; свойство моноширинно). Модель:
  поля нет; `conflictFields` такие поля отбрасывает (изменено и совпадает с
  сервером — `record_conflict.go:72`). Нужно: `ConflictPage.Agree []Text`
  (имена свойств); «agrees — kept as it is.» — слова экрана.

- **Рв6 (РБ6, держится). Строки по параметру и чужие свойства.** Кадры:
  свойство с параметром отдельной карточкой — «TEL;work», «EMAIL;work»;
  чужое свойство, которого в форме нет, — «X-EVOLUTION-FILE-AS» («only on
  the server», «Ковалёва, Анна»). Модель: строка на поле формы
  (`editFields`), значения нескольких строк через «, », метка — хвостом
  значения « · work» (`record_conflict.go:121–130`); свойства вне формы не
  сравниваются. Нужно: строка на свойство с параметром, и чужие свойства,
  которые разошлись.

- **Рв7 (РА1, держится). Обновление «2 min ago».** (**закрыто сервером `v0.2.265`**: `Top.Refresh` — GET этой страницы) Кадры `s-conflict-1`,
  `-3…-5`: кнопка `i-refresh` «2 min ago» в верхней панели. Модель:
  `Top.Refresh` пуст у всех четырёх фикстур (`action.enabled=false`, `label`
  пуст); `conflictPage` его не ставит (`record_conflict.go:140`), тогда как
  страницы записи ставят (`recordRefresh`, `record_view.go:252`;
  `record_edit.go:2686`). Нужно: обновление — GET этой же страницы
  (расхождение строится заново против нынешней версии сервера), время —
  когда прочитана серверная версия.

- **Рв8 (новый). Cancel — ссылка на запись.** (**закрыто сервером `v0.2.265`**: GET на запись) Кадры `-1…-5`: `<a
  class="tc-btn" href="/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7">Cancel</a>`.
  Модель: `conflict.cancel` — POST `?op=cancel` (`record_conflict.go:145`),
  он стирает черновик отказа и ведёт на запись (`:283–286`). Нужно: GET на
  запись (черновик живёт до конца сеанса и затирается следующим отказом —
  вреда нет) или вопрос кадру. Вес малый.

- **Рв9 (новый, малый). Значения радио и номера строк.** Кадры: радио
  `choice.1`, `choice.2`… (с единицы), значения `mine`, `server`, `none`
  («Leave it out»); «Use only my edit» / «… server version» — кнопки той же
  формы `name="all" value="mine|server"`. C-5 (`common.md:409`) и модель:
  `choice.0`… (с нуля, `choiceField`, `:218`), значения `mine`, `theirs`;
  `op=mine`, `op=theirs` — отдельными формами (`Actions`). Без сценариев
  работает и то и другое; кнопка кадра `all=mine` при нынешнем сервере ушла
  бы как `op=combine` с выбором строк — клиент кнопки кадра так собирать не
  должен (см. «Клиенту»). Нужно: решить, чьи имена — кадра или C-5 (вопрос
  Т3).

- **Рв10 (РБ8, держится; дополнен). Состояния без кадров и фикстур** (по
  коду): «Deleted elsewhere» — одна Cancel, `Form` nil (`:246–248`); сервер
  не ответил (`unavailableNotice`, `:251`); запись не прочлась
  (`readNotice`, `:256`); «Not moved yet» — правка заметки с переносом в
  другую тетрадь (`:184–193`); «Nothing differs» — «Write this
  combination» и «Use only my edit» выключены с причиной «The server
  version already has your change.» (`:209–212`); «This combination cannot
  be written» с техническим блоком «Fields» (`:325–332`); «не то, что
  выбрано» S-7.3 (`faultNotice`, `:348–356`); отказ записи
  (`writeFailure`, `:366–368`); **новое с `v0.2.204`** — «Some of these may
  not be your changes» (`Unsure`, `:172–176`). Фикстур нет ни у одного,
  строк `STATE_FRAME` нет. Кадров — вопросы Т1, Т2.

- **Рв11 (РБ7, данные).** (**событие, задача, заметка — закрыто `v0.2.271`**: фикстуры сняты с данными кадров; контакт ждёт Рв6) Строки: `app-conflict` — TITLE, NOTE против TEL;work,
  EMAIL;work, NOTE, X-EVOLUTION-FILE-AS кадра; у события, задачи, заметки —
  по одной строке SUMMARY против двух (`-3` + LOCATION, `-4` + DUE, `-5` +
  DESCRIPTION). Рейл: у задачи «Работа 2» против «Работа 3» (`s-conflict-4`),
  у заметки «Дневник 1» против «Дневник 2» (`-5`); порядок строк рейла —
  по алфавиту у фикстур («Архив», «Контакты»; «Дни рождения», «Личный»,
  «Работа»), у кадров — иной («Контакты», «Архив»; «Личный», «Работа»,
  «Дни рождения») — тот же вопрос порядка, что задача 183 п. 3.

Не пробел: `ReadAt`, `ServerAt` модель шлёт, кадры не рисуют (записка 35) —
оставить или убрать решает сервер. Скрытые `_csrf`, `etag`, `op` в форме
кадр не рисует — у модели они есть, это правильно.

---

## Коллекции: пробелы серверу (Кл)

**Что закрыто из старого скана.** КА1 (заголовки), КА2 (Cancel с `?back=`),
КА4 (параметры C-5), КГ2 частично (`TargetOption.Href`) — `v0.2.153`. КЖ1
(лист формой) и КЖ2 (`Swatches` с именем) — записка 35 (задачи 134, 135):
`Sheet.Action`, `Swatches.Name`. Держатся: КА3, КБ1–КБ4, КВ1–КВ2, КГ1, КГ2
(остаток), КГ3, КД1–КД3. Строка `Cancels` у переноса календаря — новое с
`v0.2.217` (А12).

### Общее для листов

- **Кл1 (новый). Экран под листом (`Under`).** (**закрыто сервером `v0.2.270`**: `Under` у четырёх листов, `Under.Connections`; откуда открыт — back или Referer) Решение 15.10.2026: лист,
  открытый переходом, несёт страницу под собой (`Under`,
  `common.md:1874–1884`); поле есть только у `MoveSheet` и `CopiesSheet`. У
  четырёх листов коллекций `Under` нет (`cross.go:338–389`), и сам `Under`
  умеет лишь страницы записей и списков (`cross.go:297–309`). Над чем
  рисуют кадры:
  - `s-profile-5`, `-6` — объединённая агенда `/app/calendar` (подзаголовок
    «5 of 7 calendars · 3 accounts · 10 – 16 August 2026 · …», в подвале
    рейла нажата «New calendar»; подпись `-5`: «за скримом остался тот самый
    календарь») → `Under.Agenda` (поле есть);
  - `s-profile-7`, `-8`, `-9` — настройки «Connections»
    `/app/settings/connections` (шапка «Connections», «4 servers · 15
    collections · 2 subscriptions») → нужен `Under.Connections`
    (`ConnectionsPage`) — его нет;
  - второй вход в новую коллекцию — «New collection» карточки аккаунта
    (подпись `-5`) и «New folder» карточки хранилища (подпись `-20`) — тоже
    Connections;
  - лист из «⋯» рейла (меню строки рейла шлёт Rename, Move, Copy, Delete
    без `back`, `combined.go:979–1000`) — под ним список коллекции
    (`Contacts`, `Agenda`, `Tasks`, `Notes` — поля есть), кадра нет (Т5);
  - из мастера импорта (`?back=…/import`, `collectionBack`) — `ImportPage`,
    поля нет, кадра нет;
  - фрагменты `-13`, `-17…-21` экрана не несут.

- **Кл2 (новый). Адрес коротко.** (**закрыто сервером `v0.2.272`**: путь от корня хоста с «…») Кадры показывают адрес без схемы и
  хоста: `-5` подсказка «…/remote.php/dav/calendars/mix/poezdki/», `-7`
  значение «…/remote.php/dav/calendars/mix/birthdays/». Модель — полностью:
  `Address` «https://nextcloud.example/dav.php/alice/contacts/kontakty/»
  (фикстура rename), подсказка новой «https://nextcloud.example/dav.php/alice/calendars/…/
  — …» (фикстура new; `handlers_collections.go:835–837`,
  `collectionRenameSheet` — `serverOrigin(conn.Server) + col.Path`). Нужно:
  путь от корня сервера с «…» вместо начала (или вопрос кадру, если начало
  нужно).

- **Кл3 (КА3, держится). Подсказки — куски и слова кадра.** (**слова цвета новой — как кадр, `v0.2.272`**; куски и слова состава держатся) `FormRow.Hint`
  — `Text` (`view/settings.go:46`), кусков нет. Расходится:
  - сервер (`-5`): кадр «Two of three are listed: ‹baikal› is not
    answering, …» — имя моноширинно; модель — те же слова текстом;
  - цвет (`-5`): кадр «Written to the server as ‹calendar-color› — DAVx5 and
    Thunderbird will show the same one.»; модель «Written to the server as
    calendar-color: other clients show the same one. Left unchosen, the
    colour follows from the address.» (фикстура new, `handlers_collections.go:820–821`);
  - состав (`-5`): кадр «Most servers settle this when the collection is
    made and will not change it afterwards. A collection holding only tasks
    appears in **Tasks**, not here — the rail follows what is inside, not
    where it was made.»; модель «… and do not change it afterwards. The
    collection appears under Calendar, Tasks or Notes by what it holds, not
    by where it was made.» (`:822–823`);
  - адрес (`-5`): кадр «‹…/remote.php/dav/calendars/mix/poezdki/› — taken
    from the name, yours to change. Other clients and restore-by-path go by
    the address; the name they only show.»; модель «… — Taken from the name
    when left empty; yours to change. Other clients and restore by path go
    by the address, and it does not change later; the name they only show.»
    (`:833–837`).
  Нужно: подсказка частями (`Runs`) и слова кадра.

- **Кл4 (новый, по коду). Save и Delete листа, открытого не из настроек,
  ведут в настройки.** Cancel идёт по `?back=` (`sheetCancel`,
  `handlers_collections.go:95`), а успех переименования —
  `/app/settings/connections` всегда (`:949`, `:973`), удаления — тоже
  (`collection_delete.go:225`); отказ удаления и неудача переноса
  рисуются страницей Connections (`collection_delete.go:209`, `:219`;
  `collection_move.go:254`), а не листом. Из рейла агенды «Rename» → Save
  уводит человека в настройки. Кадров итога нет. Нужно: после Save — туда,
  откуда открыт (`back`), иначе список коллекции; отказ — в листе (как
  `-6` у новой) — см. Т11.

### Новая коллекция (`CollectionNewSheet`)

- **Кл5 (КБ2, держится). Адрес из имени.** Кадр `-5`: поле Address
  «poezdki» (выведено из «Поездки») и под ним полный будущий путь
  моноширинно. Модель: поле пусто, адрес выводится только при записи
  (`postCollectionNew`, `uniqueSlug(collectionSlug(f.name))`,
  `handlers_collections.go:317–322`). Без сценариев до отправки адреса не
  показать. Нужно решение: живой слой выводит адрес на месте (адрес
  `collectionSlug` — правило сервера, клиенту его не знать: нужен
  `Swap`-адрес или правило в контракте) — или кадр показывает пустое поле
  с «taken from the name».

- **Кл6 (КБ3, держится; уточнён). Отказ сервера (`s-profile-6`).** Кадр:
  в шапке сервер «baikal» (`tc-dim` справа от «New calendar»); сообщение
  «The server would not make it. Nothing was created, and the form is as you
  left it.»; технический блок «[create]  MKCALENDAR
  /dav.php/calendars/m.yenuchenko/poezdki/ → 403», тело
  `<D:error><D:need-privileges/></D:error>`, «no DAV:bind on the calendar
  home-set»; под блоком пояснение «This account may sign in and read, but
  not create. Either the user has no such right on the server, or the
  account was connected with a password issued read-only. Neither is
  something Carrel can fix from here — but it can say which of the two it
  is.»; кнопки «Back to the form», «Try again»; **полей формы нет.**
  Модель (по коду, `davRefusal`, `handlers_collections.go:1085–1108`): у
  403 — `Title` «This account can read here but not create», `Body` —
  «Nothing was created… The server does not give this account the right:
  …» одним текстом; `Details.Summary` «MKCALENDAR path», строки
  «‹connLabel›‹path›» (сервер слитно с путём), «… → 403», `err.Error()`;
  шага «[create]» и тела ответа отдельно нет; сервера в шапке нет
  (`Stamp` у `CollectionNewSheet` нет); `Form` с видимыми полями приходит
  и с отказом (`collectionNewSheet` строит её всегда). Нужно: `Stamp`;
  пояснение отдельным полем (`SheetNote`); блок по C-9.1 (шаг, метод,
  адрес, код, тело); в состоянии отказа набранное — скрытыми полями
  (обе кнопки шлют форму, `afterRefusal`, `:872–890`). Фикстуры и строки
  `STATE_FRAME` нет. Слова пояснения — Т9.

- **Кл7 (КБ4, держится; дополнен). Состояния без фикстур** (по коду):
  новая папка (`-20`, `?kind=file&a=`); никто не отвечает (`-21`: `Note`,
  поле сервера опущено, Create выключена с той же причиной —
  `handlers_collections.go:745–747`, `:800–802`, совпадает с кадром); **ни одно подключение не
  держит род** — «No connected server holds calendars. Connect one that does
  first.» (`:744`, кадра нет); без `?kind` — «New collection» с полем
  «What it is» (`:766–775`, кадра нет — Т5); из мастера импорта (`back`);
  отказ по полю — «Give it a name.», «Choose at least one: events, tasks or
  notes.», «Choose a server that holds calendars.», «Something already lives
  at this address on the server.» (`:265–290`, `:321`); «не в списке» после
  создания (`lostCollection`, `:337–340`).

- **Кл8 (КБ1, данные).** Фикстура: имя пусто, отмечено одно Events, цвет не
  выбран, адрес пуст; кадр `-5`: «Поездки», Events и Tasks, третий цвет
  (`#6B4E7A`), адрес «poezdki». Палитра у фикстуры строчными
  (`#6b4e7a`), у кадра — прописными (сверке смысла безразлично, отмечаю).

Не пробел: имена полей кадра (`on.which.server`, `events`/`tasks`/`notes`
со значением `1`) — генератора; сервер шлёт `account`, `components`
(`VEVENT`, `VTODO`, `VJOURNAL`) по C-5 — см. Т6.

### Переименование (`CollectionRenameSheet`)

- **Кл9 (новый). Подсказка цвета, которой нет в кадре.** (**закрыто `v0.2.272`**) Кадр `-7`: под
  «Colour» подсказки нет. Модель: «Written to the server as calendar-color.
  Left unchosen, the colour stays as it is.» (`handlers_collections.go:988`).
  Кадр прав — подсказку убрать (или вопрос кадру, если она нужна).

- **Кл10 (КВ1, КВ2 — держатся; данные и состояния).** Фикстура — адресная
  книга «Контакты» (`title` «Rename», цвета нет), кадр `-7` — календарь «Дни
  рождения» с цветом (`#A05A2C` отмечен), «Rename and colour». Без фикстур:
  папка (`-18`), отказ (`Failure` — «The server refused the change» /
  «This account can read here but not change it», `:1076–1080`), отказ по
  полю («Give it a name.», «Choose one of the colours shown.»),
  переименование **подписки** — `Address` = адрес ленты, `Note` «The name
  is yours: it stays in Carrel and is never sent to the feed.», `Stamp`
  пуст (`handlers_collections.go:942–1011`) — кадра нет (Т12).

### Перенос и копия (`CollectionMoveSheet`)

- **Кл11 (новый). Лист формой.** Кадр `-8`: `<form class="tc-sheet"
  method="post">`, узлы — радио `name="to"` (`value="radicale"`, у
  закрытых `disabled`), Move — `type="submit"`. Модель: `Form` нет
  (`cross.go:363–374`); выбор — ссылки `TargetOption.Href` (лист с
  выбранным), Move — `Action` POST с `to` в адресе
  (`collection_move.go:64–93`, `:166`). Лист переноса записи стал формой
  в `v0.2.248` (`MoveSheet.Form`, радио `to`, задача 183) — нужно то же:
  `CollectionMoveSheet.Form` (скрытые `_csrf`, `copy`), `Target.Field`,
  `TargetOption.Value` — подключение.

- **Кл12 (новый). Выбранное при открытии.** Кадр `-8`: при открытии
  отмечен «radicale · vps» (`is-on`, `checked`) — единственный подходящий,
  и сообщение уже говорит «copies all **51** events». Модель: без `?to`
  ничего не выбрано, Move выключена «Choose a server.», `warning` пуст
  (фикстура move; `collectionMoveBlock`, `collection_move.go:124–127`). Нужно: подходящий
  один — он выбран (как у новой коллекции, где выбран первый подходящий,
  `handlers_collections.go:733–741`).

- **Кл13 (КГ2, остаток держится). Закрытая цель с причиной «password ·
  401».** Кадр `-8`: baikal — `disabled`, хвост «password · 401»
  (сломанный пароль; подпись кадра: «Сломанный пароль — тоже»). Модель:
  причины только «already there», «no file storage», «files only», «no
  calendars» (`:72–84`); baikal в фикстуре move — **подходящий**
  («address books», `href` есть), хотя фикстура new того же набора
  закрывает его «not answering». Нужно: та же проверка «отвечает ли»
  (`silentNow`), что у новой коллекции, и причина отказа входа.

- **Кл14 (КГ3, держится). «a address book».** (**закрыто `v0.2.272`**) `collection_move.go:184`,
  `:186`: «Carrel creates a "+noun+" there…» — у книги «a address book».
  Нужно «an address book».

- **Кл15 (КГ1, данные).** Фикстура — книга «Контакты», цель не выбрана; кадр
  — календарь «Дни рождения», выбран radicale, «51» жирным. Фикстур копии
  (`?copy=1`), выбранной цели, папки («Move folder»), строки `Cancels` нет.

### Удаление (`CollectionDeleteSheet`)

- **Кл16 (КД1, держится). «What is pointing at it now» — пары.** Кадр `-9`:
  строка на ссылку, задание, устройства — `<b>1 published link</b>
  <span class="tc-dim">— answers “not found” from that moment</span>`
  (так же «Backup job «Личное на домашний диск»», «3 devices through
  davloom»; `-19` — «Attachments»). Модель: `References []Text` одной
  строкой через « — » (`collection_delete.go:231–282`). Нужно: пары (что —
  жирно, что будет — приглушённо).

- **Кл17 (КД2, держится). «Download .ics first» частями.** Кадр `-9`:
  «Download ‹.ics› first» — расширение моноширинно (`tc-mono--t11`). Модель:
  `Export.Label` одной строкой «Download .vcf first» (фикстура;
  `collection_delete.go:139`). Нужно: расширение отдельно (как у меню «Export ‹.ics›»,
  `s-profile-1`).

- **Кл18 (новый). Строка об отмене у удаления календаря.** Решение А12
  (`scans/silent-choices.md:44`): предупреждать об отмене, когда
  «переносит, удаляет или импортирует событие с участниками, календарь таких
  событий»; сделано для переноса (`CollectionMoveSheet.Cancels`,
  `collection_move.go:194–196`), у удаления коллекции поля нет
  (`cross.go:377–389`), хотя DELETE календаря на сервере, который
  планирует, так же может разослать отмены. Нужно: `Cancels` у
  `CollectionDeleteSheet`; кадра нет ни у одного из двух — Т10.

- **Кл19 (новый, малый). Слово записей.** (**правило одно — по счёту**: перенос считает записи первого рода и называет их им, удаление считает всё, что держит коллекция, и у смешанной говорит «records»; у календаря одних событий оба говорят «events», как кадры) Удаление смешанного календаря
  говорит «all N records» (`recordNoun` — «record», когда родов больше
  одного, `:79–91`), перенос того же календаря — «all N events»
  (`spec.noun` первого рода, `collection_move.go:168–172`). Кадры `-8`, `-9`
  — «51 events» оба. Нужно одно правило.

- **Кл20 (КД3, данные и состояния).** Фикстура — книга «Контакты» (5
  контактов, ссылок, заданий, устройств нет, архива нет); кадр `-9` —
  календарь, 51 событие, три строки ссылок, архив «2026-08-14». Без фикстур:
  папка (`-19`, «Open in Files»), неверное имя («Type the name exactly as
  shown: …», «Type the name to confirm: …», `:200–205`), пустая коллекция
  («It holds nothing now…», `Export` nil, «There is nothing in it to
  download.», `:124–128`), счёт неизвестен («everything in it»), отказ
  сервера (страница Connections, Кл4).

---

## Клиенту

- Экранов расхождения и четырёх листов коллекций у клиента нет
  (`carrel/client/screens/` — ни `conflict`, ни `collection*`; в
  `zone/zone.go:90–93` листы только помечены). Собирать по кадрам.
- Расхождение: перевод `ConflictRow` в `Diff` (карточка, варианты
  столбиком, подписи по виду — после Рв2) — РЖ2 старого скана, держится.
  «Use only my edit» / «… server version» / Cancel — `Actions` модели,
  каждая своей формой; в одну форму с `choice.<n>` (как рисует кадр, `name="all"`)
  их не ставить, пока сервер читает `op` (Рв9).
- Листы коллекций — `Sheet` с `Action` (задача 134), цвет — `Swatches.Name`
  (135), дерево цели — `TargetPicker` (узел закрытый — без ссылки, как
  выключенная кнопка; КЖ3 старого скана), «What goes in it» — группа
  отметок. Лист поверх экрана — `Overlay` из `Under.*.Shell`, когда Кл1
  будет сделан.

## Спеке

- U-6.8 (`carrel-client.md:1040`): «Совпавшее не показывается» — по
  записке 35 строка «‹X› agrees — kept as it is.» остаётся; править на
  «совпавшее не спрашивают, а называют строкой над карточками».
- C-6 (`carrel-common.md:2071–2072`): «Actions — «Move here» или «Copy»» —
  кнопка давно «Move» (`collection_move.go:155`, задача 147).

---

## Вопросы к кадрам (кандидаты в задачи Typecase)

Ни один не повторяет задач 1–186; где близко — номер назван.

- **Т1. Расхождение, когда Carrel не знает, что менял человек.**
  `s-conflict-1…-5` рисуют вид различия и выбор по нему (Рв2, Рв4). С
  `v0.2.204` бывает, что версия, с которой открыли форму, уже не
  хранится, и сервер говорит «Some of these may not be your changes …
  Where you changed nothing, choose the server's value — «Use only my
  edit» would put the old one back.» (`record_conflict.go:172–176`). Нужно:
  кадр этого состояния — как подписаны строки без вида («differs»?), что
  выбрано заранее (своё, серверное, ничего — тогда «Write this
  combination» выключена до выбора?), где стоит сообщение (над
  карточками, под подзаголовком). Рекомендация: ничего заранее, кнопка
  выключена с причиной «Choose a side for every property.»

- **Т2. Расхождение: состояния без кадров.** Нужны части `s-conflict-*`
  (`data-part`) с их словами: (а) «Deleted elsewhere» — запись удалена на
  сервере, сейчас лишь сообщение и Cancel (`:246–248`): предлагать ли
  «Write it as a new record»? (решение владельца — правка иначе теряется
  молча); (б) «Nothing differs» — две кнопки выключены с причиной «The
  server version already has your change.» (`:209–212`); (в) «Not moved
  yet» — правка заметки с переносом в другую тетрадь (`:184–193`): где
  строка и что делают три кнопки; (г) «This combination cannot be written»
  с блоком «Fields» (`:325–332`); (д) сервер не ответил / отказал при
  записи (`:251`, `:366–368`). Без кадров у этих фикстур нет
  `STATE_FRAME`.

- **Т3. Расхождение: имена формы и Cancel.** `s-conflict-1…-5`: радио
  `choice.1…` со значениями `mine`/`server`/`none`, кнопки «Use only…» —
  `name="all" value="mine|server"` той же формы, Cancel — ссылка на запись.
  C-5 (`common.md:409`): `choice.0…`, `mine`/`theirs`, `op=mine|theirs|cancel`
  отдельными POST. Нарисовать по C-5 (кнопки «Use only…» — `name="op"`,
  значения `mine`/`theirs`) или сказать, что верно кадру — тогда сервер
  примет имена кадра. И кадра телефона (≤ 460 px) у расхождения нет —
  U-6.8 «узкий экран — тот же вид»: нужен ли `phone-*`?

- **Т4. Адреса форм листов коллекций против C-5.** `s-profile-7`, `-8`, `-9`
  при `data-screen="/app/collections/rename|move|delete"` шлют форму на
  `/app/collections/nextcloud/dni-rozhdeniya/rename|move|delete`; `-18`, `-19`
  — на `/app/files/sftpgo/documents/rename|delete`, `-20` — на
  `/app/files/sftpgo/mkcol`. C-5 (`common.md:478–481`) — `/app/collections/…?a=…&c=…`,
  новая папка — `/app/collections/new?kind=file&a=…`. Привести `action` к
  C-5 (и `-21`, `-5` уже верны).

- **Т5. Листы коллекции, открытые не из настроек, и «New collection» без
  рода.** Кадры рисуют листы над Connections (`-7…-9`) и над агендой
  (`-5`, `-6`). Меню строки рейла шлёт Rename, Move, Copy, Delete
  (`combined.go:979–1000`) — кадра листа над списком коллекции нет (нужен,
  например, «Rename and colour» над `s-agenda-17` с `data-screen`, как
  просит задача 183 п. 1 у листов копий). «New collection» в шапке
  карточки аккаунта (`s-profile-1`) открывает лист без рода — сервер
  отвечает «New collection» с полем «What it is» (Calendar / Address book,
  подсказка «Tasks and notes live in calendars: …»,
  `handlers_collections.go:766–775`) — кадра
  нет. Нужно: эти два кадра.

- **Т6. Имена полей в кадрах листов.** `-5`: `on.which.server`, отметки
  `events`/`tasks`/`notes` = `1`; `-7`: `address.stays.as.it.is`; `-9`,
  `-19`: `type.the.name.to.confirm`; `-8`: `to` = `radicale`. C-5 и
  сервер: `account`, `components` (`VEVENT`/`VTODO`/`VJOURNAL`, несколько
  значений), `name`, `to` = id подключения. Нарисовать именами C-5
  (генератор сейчас делает имя из подписи).

- **Т7. Адрес в «Rename and colour» — поле, которое можно править.** `-7`:
  `<input class="tc-in is-mono tc-in--dusty" name="address.stays.as.it.is"
  value="…/remote.php/dav/calendars/mix/birthdays/">` — без `readonly`/
  `disabled`, с именем: его можно набрать, и оно уйдёт с формой, хотя
  подпись «— stays as it is» и S-12.2 говорят «не правится». Модель —
  `Address Text`, не поле. Нужно: текст (или `readonly` без имени).

- **Т8. Кнопки `type="button"` внутри форм листов.** Без сценариев ничего не
  делают: «Download ‹.ics› first» (`-9`) — у модели GET-ссылка экспорта;
  «Open in Files» (`-19`) — GET-ссылка на корень папки; Cancel во всех
  листах (`-5`, `-7`, `-8`, `-9`, `-18…-21`) — у модели GET на `back`;
  «Back to the form» / «Try again» (`-6`, где лист вообще не форма) — у
  модели POST формы. Нарисовать ссылками (`a.tc-btn`, как Cancel у
  `s-conflict-*`) и `-6` формой. Сквозное: Cancel `type="button"` и у
  `s-contacts-2`, `-32`, `s-files-21…-23` — одной задачей на все листы.

- **Т9. Отказ создания (`s-profile-6`): поля и обещание.** (а) Полей формы в
  кадре нет, а обе кнопки отправляют форму — значит, набранное должно
  ехать скрытыми полями; подтвердить, что поля в отказе не рисуются. (б)
  Пояснение кончается «— but it can say which of the two it is», но лист
  этого не говорит, а Carrel по 403 не отличит право пользователя от
  пароля только для чтения. Какие слова, если различить нельзя? (в)
  Данные: `-5` говорит, что baikal не отвечает и в список не попал, а `-6`
  — отказ того же baikal на MKCALENDAR; взять другой сервер (radicale ·
  vps).

- **Т10. Строка об отмене у переноса и удаления календаря.** Решение А12:
  календарь с встречами уходит с сервера, который планирует, — «nextcloud
  may tell the attendees of its meetings that they are cancelled.»
  (`CollectionMoveSheet.Cancels`, `v0.2.217`). Ни `s-profile-8`, ни `-9`
  этой строки не рисуют; у удаления сервер её ещё не шлёт (Кл18). Нужно:
  где она в обоих листах и каким видом (`tc-msg`, `tc-dim`). Близко к
  задаче 166 (лист удаления записи), но это лист коллекции.

- **Т11. Отказ переименования, переноса, удаления — где.** Отказ новой
  коллекции нарисован в листе (`-6`); отказ переименования сервер тоже
  кладёт в лист (`Failure`), а отказ удаления и неудачу переноса —
  сообщением на странице Connections (`collection_delete.go:209`,
  `collection_move.go:254`). Кадров нет ни у одного. Нужно: кадр отказа
  удаления и итога переноса «записано не всё» (`collectionMove.kept`,
  `:565`), и куда ведёт успех листа, открытого из рейла (Кл4).

- **Т12. Переименование подписки.** Подпись `-7` разбирает разницу: у
  подписки имя местное, сервера в шапке нет. Сервер отвечает своим видом
  (`Address` — адрес ленты, `Note` «The name is yours: it stays in Carrel
  and is never sent to the feed.», у книги-ленты — «Rename», цвет с
  подсказкой «Kept here: a feed sends no colour of its own.»,
  `handlers_collections.go:990–1011`) — кадра нет. Нужен.

- **Т13. Папку хранилища переносят или нет?** `s-profile-12` (меню папки):
  «Move to another server» есть; подпись `s-profile-8`: «Папку WebDAV так не
  двигают — у неё дерево в 7.5». Сервер даёт Move папке
  (`collectionMoveItems`, `collection_move.go:95–121`; лист «Move folder»,
  «each file downloaded and uploaded again», `:179–182`). Какое верно;
  если Move есть — кадр листа «Move folder» (и «Copy folder»).

- **Т14. «Никто не отвечает» (`s-profile-21`): какие поля остаются.** Кадр
  рисует одно поле Name; сервер шлёт и Colour, «What goes in it», Address
  (`collectionNewSheet` убирает только поле сервера, `handlers_collections.go:800–802`). Решить,
  остаются ли они. И состояние «ни одно подключение не держит календари» —
  «No connected server holds calendars. Connect one that does first.»
  (`handlers_collections.go:744`): нарисовать с действием «Connect account»?

---

## Итог

- **Расхождение (Рв):** 11 пунктов серверу — Рв1–Рв10 пробелы контракта
  (из них Рв8, Рв9 малые), Рв11 — данные. Из старых 10 закрыто 2 (РБ2, РБ9)
  и РЖ1, РЖ3 записками; новых — 2 (Рв8, Рв9) и одно новое состояние в Рв10
  (`Unsure`).
- **Коллекции (Кл):** 20 пунктов серверу — пробелов контракта 16 (Кл1–Кл7,
  Кл9, Кл11–Кл14, Кл16–Кл19; новые — Кл1, Кл2, Кл4, Кл9, Кл11, Кл12, Кл18,
  Кл19), данных и состояний без фикстур — 4 (Кл8, Кл10, Кл15, Кл20). Из
  старых 16 закрыто 4 (КА1, КА2, КА4, КГ2 частично) и КЖ1, КЖ2 записками.
- **Клиенту:** 3 строки (экранов нет; `Diff`; листы через `Sheet.Action`,
  `Swatches`, `TargetPicker`, `Overlay`).
- **Спеке:** 2 правки (U-6.8 «совпавшее»; C-6 «Move here»).
- **Вопросов к кадрам:** 14 (расхождение — Т1–Т3; коллекции — Т4–Т14).

**Не проверено:** черновик `frame2go` не запускался — Typecase рядом
(`D:\Local\Git\typecase`) стоит на `bd667a3` с незакоммиченной правкой, не
на сборке `95453f6`, а отдельного дерева Typecase на `95453f6` эта
разведка не заводила (только чтение); виды кадров читаны по HTML. Сервер не
запускался: все состояния без фикстур — по коду. Не проверено, как
`check.py` сейчас сверяет листы коллекций с кадрами (запуск не делался).
Кадр подписок (`s-profile-10`) и меню рейла как кадр не сверялись — они
раздела настроек (U-6.11).
