# Скан U-6.9 «Импорт» после записки 111 (задача 188) — что кадры рисуют, а сервер не несёт

Только чтение. Рабочее дерево `D:\Local\wt\t53\carrel` (ветка `wip/import-111` =
main `795469ce` + копия макета Typecase `a78141d`, 510 кадров). Черновики
Typecase — `D:\Local\wt\t53\typecase\internal\screens\io.go` (`Io1`…`Io19`).
Записка 111 — `D:\Local\Git\carrel\docs\plan\from-typecase.md:3663–3730`.
Прежний скан — `docs/plan/scans/import.md` (Им1–Им20, В1–В12): нумерация
продолжена (Им21…), вопросы В1–В11 старого скана записка 111 закрыла (см. §6).

**Метод.** Кадры `s-io-1…19`, `s-contacts-1/-6/-16`, `s-publish-3`,
`mentions-1/-9` вырезаны по `data-frame=` до следующего `data-frame=` (свой
резак в черновой папке) и прочитаны как HTML; черновик `io.go` — для
`FileField{Accept, Many}`, `Button{Title, State}`. Против них — `internal/web/import.go`,
`import_drop.go`, `import_zip.go`, `middleware.go`, `csrf_body.go`, `files_ops.go`,
`provider/card/split.go`, `provider/cal/split.go`, `view/cross.go`, `view/lists.go`,
фикстуры `docs/spec/fixtures/app-*-import/*.json`, `check.py`,
C-5/C-6 (`carrel-common.md:421–422`, `:1095–1104`, `:2219–2300`), U-6.9
(`carrel-client.md:1046–1080`), S-19 (`carrel-server.md:2080…`), S-29.4 (`:2783`).
`check.py` прогнан (только чтение): красное одно — 1b `/app/contacts/import`.
Сервер не запускался; где «по коду» — из чтения, не из ответа.

---

## 1. Кадры импорта

| Кадр | data-screen | data-part | Что рисует | Фикстура сейчас |
|---|---|---|---|---|
| `s-io-1` | `/app/contacts/{account}/{col}` | `drop` | Накладка броска на список: «Drop to import» · «One ‹.vcf› file into **nextcloud · Контакты**» · «The preview opens first…»; в подписи — слова отказа броска (п. 4) | — (часть списка; `ListBar.Drop` в фикстурах списков) |
| `s-io-2` | `/app/calendar/{account}/{col}` | `drop-card` | Бросок на открытую карточку → вложение, мастер не открывается | — |
| `s-io-3` | contacts·calendar·tasks `…/import` | `source` | Шаг 1 контактов: «Import contacts», Into ▍nextcloud · Контакты, «vCard file», `accept=".vcf"`, строка «‹.vcf› — vCard 2.1, 3.0 and 4.0. One file holds any number of cards; the calendar and the list of tasks take ‹.ics› here instead.» | `app-contact-import/normal` (`STATE_FRAME`, `check.py:477`) |
| `s-io-4` | `/app/notes/{account}/{col}/import` | `source` | Шаг 1 заметок: «Markdown, a zip of them, or .ics», `accept=".md,.zip,.ics"`, **`multiple`**, строка «‹.md› — Markdown, ‹.zip› — any number of them, ‹.ics› — iCalendar 2.0 journal entries. Several files may be picked at once; events and tasks in an ‹.ics› are skipped and named in the report.»; дерево WebDAV | `app-note-import/normal` (`:483`) |
| `s-io-5` | contacts `…/import` | — | Предпросмотр контактов: вопрос «5 records…», отказ «line 12: cannot decode quoted-printable», имя «—» блёкло, UID «—» | `app-contact-import/preview` (`:478`) |
| `s-io-6` | calendar `…/import` | — | Предпросмотр событий; **новая строка** «Untitled event» (блёкло) · «2026-05-04 · all day» · «Ready — no end: a whole day; no title» | `app-event-import/preview` (`:480`) |
| `s-io-7` | tasks `…/import` | — | Предпросмотр задач; **«Untitled task»** · «Ready — no title»; отказ «line 88: cannot read DUE «2026-13-01»» | `app-task-import/preview` (`:482`) |
| `s-io-8` | notes `…/import` | — | Предпросмотр заметок (zip, 64 files) | `app-note-import/preview` (`:484`) |
| `s-io-9` | все четыре `…/import` | `report` | Отчёт: подзаголовок **▍nextcloud · Контакты** (метка); Created · Already here, skipped · **With a new UID** · **Not kept by the server** · Failed «line 12: cannot decode quoted-printable» | `app-contact-import/report` (`:479`) |
| `s-io-10…13` | — | фрагменты | Меню Into по роду, ro с плашкой; **`-11` теперь с ro «nextcloud · Общие с семьёй»** | — (фрагменты) |
| `s-io-14` | calendar `…/import` | `report-skipped` | Отчёт с пропущенным; Failed «line 418: cannot read RRULE «FREQ=FORTNIGHTLY»»; подзаголовок с меткой | `app-event-import/report` (`:481`) |
| **`s-io-15`** | calendar `…/import` | `source-calendar` | Шаг 1 событий: «Import events», Into ▍nextcloud · Личный, «Any calendar you may write to, on every connected server.», «iCalendar file», `accept=".ics"`, строка «‹.ics› — iCalendar 2.0. One file holds any number of events; tasks and journal entries in it are skipped and named in the report.» | **нет** |
| **`s-io-16`** | tasks `…/import` | `source-tasks` | Шаг 1 задач: «Import tasks», «Any list of tasks you may write to…», «iCalendar file», `accept=".ics"`, «…any number of tasks; events and journal entries in it are skipped…» | **нет** |
| **`s-io-17`** | **`/app/contacts/import`** | `into-empty` | Вход из сведённого вида: Into пустое `is-ph` «Choose an address book», имя взятого файла **значением** поля файла (`is-mono`, «Разное (mix).vcf»), Preview file `disabled`, `title="Choose under «Into» where the records go."` | **нет** |
| **`s-io-18`** | contacts `…/import` | `refused` | Шаг 1 + одно уведомление над шагом `is-alert`: «**Nothing was read.** «Разное (mix).vcf» is not a vCard file: its first line is not ‹BEGIN:VCARD›. Choose the file again.» | **нет** |
| **`s-io-19`** | contacts `…/import` | `unanswered` | Предпросмотр: уведомление «**nextcloud · Контакты did not answer**, so what is already there cannot be told. Nothing is written; try again, or choose another address book under «Into».»; вопроса «already here» нет; счёт «117 ready · 3 with errors»; таблица стоит; «Import 117 contacts» `disabled`, `title="nextcloud · Контакты did not answer"` | **нет** |

Рейлы новых кадров: `-15` — Дни рождения (—), Личный (on), Работа; `-16` — Дела
(on), Работа; `-17`…`-19` — Контакты (on), Архив. `s-io-6` теперь рисует «Дни
рождения» в рейле (первым) — строка `FRAME_TODO` «app-event-import: коллекции
рейла» (`check.py:307`, задача Typecase 199 п. 1) похоже устарела: сверить и
снять вместе с п. 1 задачи 199 в `to-typecase.md:2487`.

---

## 2. Пункты записки 111 против сервера

### п. 1 — шаг 1 календаря и задач (`s-io-15`, `-16`, `-3`)

- **Кадр.** Заголовки «Import events», «Import tasks»; подсказки Into (см.
  таблицу); поле «iCalendar file», `accept=".ics"`; строки под полем —
  с моноширинным расширением в начале и « — » (см. таблицу).
- **Сервер.** Заголовок — `"Import "+noun+"s"` (`import.go:461`) — **сходится**.
  `Source.Kinds` `[".ics"]` (`import.go:226–234`) — сходится. Строки под
  полем — `importHints` (`import.go:437–441`): у контактов нет начала «‹.vcf› — »
  (фикстура `app-contact-import/normal`: `hint` начинается с «vCard 2.1…»);
  у событий и задач старые слова: «…tasks and notes in it are named and left
  out. An address book takes ‹.vcf› here instead.» Подсказка Into и подпись
  поля файла («vCard file», «iCalendar file») — слова клиента по роду
  (`plan/server.md:755–762`, `v0.2.252`; старый скан, «Клиенту»).
- **С1. Строки под полем файла — кадров, целиком.** `importHints`:
  контакты — `join(monoRun(".vcf"), " — vCard 2.1, 3.0 and 4.0. One file holds any number of cards; the calendar and the list of tasks take ", monoRun(".ics"), " here instead.")`;
  события — `join(monoRun(".ics"), " — iCalendar 2.0. One file holds any number of events; tasks and journal entries in it are skipped and named in the report.")`;
  задачи — то же с «tasks; events and journal entries». Начало «‹ext› — »
  несёт сервер (иначе строку заметок, где расширений три, клиент не
  соберёт); комментарии `view/cross.go:670–672` и C-6 `carrel-common.md:2243`.

### п. 2 — заметки берут `.ics` (`s-io-4`)

- **Кадр.** `accept=".md,.zip,.ics"` (в этом порядке), подпись «Markdown, a
  zip of them, or .ics», строка — см. таблицу (три моноширинных куска).
- **Сервер.** `importFormats` у заметок — `[".ics", ".md", ".zip"]`
  (`import.go:230–231`; фикстура `app-note-import/normal`: `kinds` в этом
  порядке); строки под полем у заметок **нет вовсе** — `importHints` без
  `KindNote` (`import.go:437–441`; фикстура: `"hint": []`). Одиночный `.ics`
  у заметок разбирается по коду (`readImport` → `cal.Split`, VJOURNAL →
  `KindNote`, `cal.go:150`), VEVENT/VTODO — «Skipped: a notebook takes notes —
  import it into a calendar» (`import.go:899–905`). VJOURNAL в календаре —
  «…import it into a notebook» (`kindHolders`, `import.go:1040–1044`) — сходится.
- **С2.** `importFormats(KindNote)` → `[".md", ".zip", ".ics"]` (порядок
  `accept` кадра; тем же порядком пойдут `ImportDrop.Accept` и `formatsRuns`);
  `importHints[KindNote]` = `join(monoRun(".md"), " — Markdown, ", monoRun(".zip"), " — any number of them, ", monoRun(".ics"), " — iCalendar 2.0 journal entries. Several files may be picked at once; events and tasks in an ", monoRun(".ics"), " are skipped and named in the report.")`.
  Подпись поля «Markdown, a zip of them, or .ics» — клиенту.

### п. 3 — несколько файлов (`s-io-4`, подпись: «3 files · 12 KB»)

- **Кадр.** Поле заметок `multiple` (`io.go:1129` `Many: true`); у набора
  подзаголовок предпросмотра «3 files · 12 KB»; один файл — как было.
- **Сервер.** У `ImportSource` признака «несколько» нет (`view/cross.go:668–692`;
  у броска он есть — `ImportDrop.Many`, `lists.go:47`). Набор: `readNoteFiles`
  (`import.go:1798–1815`) кладёт `Source = "3 files"`, `Files = 3`,
  `Format = "Markdown"` → `importSubtitle` (`import.go:560–568`) даёт
  «‹3 files› · 12 KB · 3 files» — число дважды и моноширинно. В наборе берётся
  только `.md`: `.zip` и `.ics` набора — «Not Markdown.» (`import.go:1804–1806`),
  хотя кадр разрешает их в наборе.
- **С3.** `ImportSource.Many bool` (true у заметок) — поле `multiple` у
  Browse; C-6 и `view/cross.go`.
- **С4.** Подзаголовок набора — «3 files · 12 KB»: у набора `Source` не
  выводится моноширинно и число файлов не повторяется (`importSubtitle`:
  ветка «набор», напр. `Format == ""` и `Source` — счёт). Моноширинно ли
  «3 files» — кадра нет (вопрос В12).
- **С5.** Набор берёт всё, что берёт одиночное поле: `.zip` — его `.md`
  (как `readZipNotes`, строки называют `архив/файл`), `.ics` — VJOURNAL
  (как `readImport`), прочее — «Skipped: not Markdown». `readNoteFiles`
  переписать через общий разбор одного файла.

### п. 3 (второй) — пустое «Into» (`s-io-17`)

- **Кадр.** Into `is-ph` «Choose an address book» (у календаря «Choose a
  calendar», у задач «Choose a list of tasks», у заметок «Choose a notebook»);
  имя взятого файла — значение поля файла, моноширинно, только имя; Preview
  file `disabled`, `title` «Choose under «Into» where the records go.».
- **Сервер.** Причина — есть (`import.go:546–550`: `st.entry`). Вход GET
  `/app/{kind}/import` (`import.go:624–647`) и бросок POST (`:671–703`) — есть.
  `Source.Taken` — `importSubtitle(held)` = «‹имя› · размер · формат»
  (`import.go:536–538`; C-6 `:2246`). Слов пустого Into в модели нет
  (`TargetPicker`, `view/cross.go:241–248`).
- **С6.** `Source.Taken` — только имя файла (моноширинно), без размера и
  формата; комментарий C-6 / `view/cross.go:682–685`. (Размер и формат
  человек прочтёт в шапке предпросмотра — подпись `s-io-1`.)
- Слова «Choose an address book…» — клиенту по `Kind` при пустом
  `Into.Chosen`, по образцу подсказки Into (`v0.2.252`). Если решено
  держать их на сервере — `TargetPicker.Placeholder Text`; по прецеденту —
  клиенту.

### п. 4 — отказ броска (подпись `s-io-1`)

- **Кадр (подпись).** Красная строка накладки вместо «Drop to import»:
  чужое расширение — «Only .vcf files are imported here» (календарь и задачи —
  «.ics», заметки — «.md, .zip or .ics»); несколько файлов не у заметок —
  «One file at a time: only notes take several».
- **Сервер.** `ImportDrop.Elsewhere []ImportElsewhere{Ext, Says}`
  (`import_drop.go:58–79`, `lists.go:44–57`, C-6 `:1095–1104`): по фразе на
  чужое расширение — «Contacts take ‹.vcf› — the calendar, the list of tasks
  and a notebook take this one». Фразы «по одному» нет.
- **С7.** `ImportDrop.Elsewhere` → `ImportDrop.Refused Runs` — одна фраза
  «Only ‹.vcf› files are imported here» / «Only ‹.ics› files…» / «Only ‹.md›,
  ‹.zip› or ‹.ics› files…» (моноширинность расширений — по образцу `Line`;
  в подписи без разметки — вопрос В1); `ImportDrop.OneOnly Text` «One file at
  a time: only notes take several» (у заметок пусто). Снять `ImportElsewhere`,
  `dropTaker`, `dropHolder`, `takesFor`, `joinAnd` (если больше нигде),
  C-6 `:1095–1104`. Отказ формы шага 1 несколькими файлами
  (`errOneFileOnly`, `import.go:1826`) — те же слова «One file at a time: only
  notes take several.» (сейчас «Only notes are imported several files at once…»).

### п. 5 — Р-72: одно правило отказа (`s-io-6`, `-7`, `-14`)

- **Кадр/записка.** Отказ — только то, что нельзя записать вовсе (сломанная
  строка, нечитаемая кодировка). DTSTART без DTEND и DURATION — по RFC 5545:
  дата — целый день, дата и время — миг; запись без названия — с названием
  по умолчанию «Unnamed contact», «Untitled event», «Untitled task» (в таблице
  блёкло, `tc-dim`). Строка — «Ready — no end: a whole day; no title»
  (`s-io-6`), «Ready — no title» (`s-io-7`).
- **Сервер.** Событие без конца и безымянные записи **пишутся** и сейчас (по
  коду: ни `cal.Split`, ни `card.Split` их не отказывают). Но: заглушка
  названия — «— (no FN)», «— (no SUMMARY)», «—» (`importTitle`,
  `import.go:1099–1116`; фикстура `app-task-import/preview`: «— (no SUMMARY)»,
  `untitled: true`, итог «Ready»); пометок «no end: …» и «no title» нет
  (`readyNote`, `import.go:1008–1036`, склейка «, »). Отказ целого файла из-за
  одной записи: `cal.Split` возвращает ошибку `g.object` → весь файл отвергнут
  (`provider/cal/split.go:192–195`); `d.add` с ошибкой `c.body` — тоже весь
  файл (`import.go:878–881`, `:907–909`).
- **С8. Названия по умолчанию.** `importTitle`: готовая запись без названия —
  «Unnamed contact», «Untitled event», «Untitled task» (как `titleOf`,
  `import.go:1118–1129`), `Untitled = true` (блёкло); заметка — как есть
  («—», `ParseNote` и так называет её по имени файла). C-6 `ImportLine`
  (`carrel-common.md:2270–2276`), `view/cross.go:707–713`.
- **С9. Пометки готовой записи.** `readyNote`: «no title» у безымянной
  (у контакта — слово кадра не нарисовано, вопрос В4); у события без DTEND и
  DURATION — «no end: a whole day» (DTSTART — дата), «no end: a moment»
  (дата-время; слов в кадре нет — вопрос В4). Склейка: кадр `s-io-6` —
  «no end: a whole day; no title» через «; », `s-io-7` — «40 % done, priority 1»
  через «, » (сервер склеивает всё «, », фикстура задач: «40 % done, priority 5,
  a task without a date is a task») — правило склейки — вопрос В4.
- **С10. Отказ — по записи, не по файлу.** Ошибка разбора одной записи
  календаря (`group.object` → `Parse`) и `c.body` одной записи — строка
  «failed» с номером строки (С13), остальные записи файла идут. Отказ всего
  файла — только когда файл не того вида (п. 9, С15).
- Записывает ли сервер название по умолчанию в саму запись (FN/SUMMARY) —
  не сказано; по коду не пишет (`card.Split` FN не добавляет,
  `provider/card/split.go:26–67`), а vCard 3.0/4.0 требует FN — вопрос В10.

### п. 6 — одна фраза на отказ (`s-io-5`, `-7`, `-9`, `-14`)

- **Кадр.** Предпросмотр и отчёт пишут одно: «line 12: cannot decode
  quoted-printable», «line 88: cannot read DUE «2026-13-01»», «line 418:
  cannot read RRULE «FREQ=FORTNIGHTLY»». Имя непрочтённой записи — «—»
  (блёкло), UID/срок — «—».
- **Сервер.** Предпросмотр: итог — `sk.Reason` как есть (`import.go:1287`) —
  «the card has no END:VCARD» или текст ошибки go-vcard (`card/split.go:37`,
  `:48`), номера строки нет; название без имени — `sk.named()` «— (no FN)» /
  «— (no SUMMARY)» (`import.go:170–180`, `:1307–1311`). Отчёт: `sk.named() + ": " + sk.Reason`
  (`import.go:1473`) — другая фраза, чем в предпросмотре. Номеров строк
  разбор не знает: `card.cutCards` идёт по развёрнутым строкам
  (`model.LogicalLines`, `card/split.go:80–101`), `cal.partition` режет
  физические строки, но номер в `chunk` не несёт (`cal/split.go`). Разбора
  quoted-printable vCard 2.1 нет нигде (`grep -i quoted` по `provider/card` —
  только комментарий теста): значение QP по коду уходит сырым «=D0=90…».
  Значения DUE/DTSTART/RRULE разбор не проверяет — `Parse` их принимает,
  `DueOf` молча даёт пустой срок (`import.go:1073–1076`).
- **С11.** Имя непрочтённой записи без названия — «—» (`named()` →
  «—» у всех родов; `Untitled = true`). Своё имя, если строка FN/SUMMARY
  прочлась, — как сейчас.
- **С12.** `ImportReport.Failures` — та же фраза, что ячейка Result
  предпросмотра (без «название: »). Отказ сервера при записи — «‹имя›:
  ‹слова сервера›» как сейчас (`import.go:1539`) или только слова — вопрос В9.
- **С13. Номер строки.** `card.Split` и `cal.Split` несут номер физической
  строки начала ошибки: `Broken{Line int}`, `Stray`/новая `Failed{Line}`;
  фраза — «line N: ‹что›»: «cannot decode quoted-printable», «cannot read
  DUE «…»», «cannot read RRULE «…»», «the card has no END:VCARD» (последнее —
  по образцу; кадра нет).
- **С14. vCard 2.1.** Разбор `ENCODING=QUOTED-PRINTABLE` (и `CHARSET=`,
  мягкий перенос «=») при чтении карточки 2.1; неразборчивое — отказ записи
  «line N: cannot decode quoted-printable». Проверка значений дат и RRULE
  событий и задач (DTSTART, DTEND, DUE, RRULE) — нечитаемое — отказ записи
  «line N: cannot read ‹PROP› «‹значение›»». По Р-72 это «сломанная строка».

### пп. 7, 8 — отчёт: «With a new UID», «Not kept by the server» (`s-io-9`)

- **Кадр.** Карточка «With a new UID ‹2›»: «Вера Дорошенко · 2b7e-11aa →
  9d04-77c1 · imported as a copy», «Виктор Панов · 51c0-3a8e → baikal-0412 ·
  the server gave its own». Карточка «Not kept by the server ‹1›»: «Галина
  Мещерякова · PHOTO». Пустых карточек нет. Подпись: вторая карточка по
  ответу — «Already here, skipped» / «Replaced» / «Imported as copies».
  (Под ответом Skip строка «imported as a copy» законна: «Вторая запись того
  же файла под уже занятым UID — копия при любом выборе», S-19,
  `carrel-server.md:2108–2109`.)
- **Сервер.** `Report.Renamed` (число) и `Report.Renames` — «старый → новый»
  без имени и без «почему» (`import.go:1555–1560`; `view/cross.go:741–744`).
  Почему — только в `written`-строке `Rows` («Created under …: the collection
  already held…», `:1495`; «…this server holds one record per UID across the
  account…», `:1532`; «…the server gave it an identity of its own», `:1675`).
  Что сервер не сохранил — только приклейкой к ячейке `Rows`
  (`importLanded`, `import.go:1646–1653`) и числом в `Note` (`:1596–1598`);
  ни числа, ни строк в карточку модель не несёт. `Rows` и `Note` кадры не
  рисуют (Им17, открыт).
- **С15а. `ImportReport.NewUIDs []Text`** (и число — `Renamed` как есть):
  «‹имя› · ‹старый› → ‹новый› · ‹почему›»: «imported as a copy» (ответ Copy и
  вторая запись того же UID в файле), «the server gave its own» (SV-25,
  Google — `importLanded`), iCloud SV-55 (`withOwnUID`, `:1527–1533`) — слов
  кадр не дал (вопрос В3). `Renames` снять или переименовать.
- **С15б. `ImportReport.Lost int`, `Losts []Text`**: «‹имя› · ‹свойства через
  «, »›» из `writeLoss` в `importLanded` (`:1650–1653`). Запись, которую
  сервер не показал вовсе (`:1667`), — тоже сюда? (вопрос В9).
- **С15в. Им17 закрыть.** `Report.Rows` и `Report.Note` больше не нужны:
  что сменило UID и что сервер не сохранил — в своих карточках (В8, В9 старого
  скана ответил кадр). Снять из модели, C-6 (`carrel-common.md:2277–2300`),
  `importNote` (`import.go:1579–1603`), `skippedTable` (`:1339–1356`),
  `written`/`refused` в `runImport`.
- Карточка «Imported as copies» при ответе Copy и «With a new UID» — не
  повторят ли одни и те же строки (подпись называет обе) — вопрос В2.

### п. 9 — состояния мастера (`s-io-18`, `-19`), Р-73

- **Кадр `s-io-18`.** Одно уведомление `is-alert` над шагом: заголовок
  «Nothing was read.» + тело ««Разное (mix).vcf» is not a vCard file: its
  first line is not ‹BEGIN:VCARD›. Choose the file again.» (моноширинно
  `BEGIN:VCARD`). Слова других отказов шага 1 (подпись): «No file was
  chosen.», «This file holds no record of this kind.», «The archive holds no
  Markdown file.», «The folder could not be read: ‹причина›.», «There is no
  file to import: the session ended. Choose it again.»
- **Сервер (`importRefusal`, `import.go:1819–1838`).** Заголовок «Nothing was
  imported»; «No file was chosen» / «Choose the file to import and send the
  form again.»; пустой файл — «Empty file: nothing to store…»; потолки (см.
  Р-73); прочее — «Carrel could not read this file.» + `Details` с
  текстом ошибки («card: the file holds no vCard», «cal: the file holds no
  VCALENDAR», `card/split.go:65`, `cal/split.go:81`). «This file holds no
  record of this kind.» — не отказ шага 1, а причина выключенной Import на
  предпросмотре (`import.go:524–525`). «The archive holds no Markdown file.»,
  «The folder holds no Markdown file.», «None of these files is Markdown.» —
  приписка к сообщению предпросмотра (`d.Note`, `import_zip.go:91–93`,
  `import.go:1193–1195`, `:1811–1813`). Папка не прочлась — уведомление
  «Documents is unavailable» / «The folder could not be read, so nothing was
  taken from it.» + `Details` PROPFIND (`import.go:1155–1161`). Сессия
  кончилась — «There is no file to import» / «The file is no longer in hand —
  the session may have ended. Choose it again.» (`import.go:1406–1411`).
  Правило «первая строка не BEGIN:VCARD» сервер не держит: `cutCards` ищет
  BEGIN:VCARD где угодно (`card/split.go:80–101`).
- **С16. Отказ шага 1 — одна форма.** `importRefusal`: `Title` «Nothing was
  read.», тело `Runs` — слова кадра: «No file was chosen.»; ««‹имя›» is not a
  vCard file: its first line is not ‹BEGIN:VCARD›. Choose the file again.»
  (первая непустая строка файла не BEGIN:VCARD — проверка до разбора;
  у `.ics` — по образцу «is not an iCalendar file: its first line is not
  ‹BEGIN:VCALENDAR›» — вопрос В7); «This file holds no record of this kind.»
  (ни одной записи рода и ни одной непрочтённой — переносится с
  предпросмотра на шаг 1; вопрос В6); «The archive holds no Markdown file.»
  (zip без `.md` — отказ шага 1, не предпросмотр; набор и папка без `.md` —
  вопрос В6); «The folder could not be read: ‹причина›.» (`takeFolder`;
  `Details` — оставить ли, вопрос В5); «There is no file to import: the
  session ended. Choose it again.» (`runImport`). `Details` у прочих — снять
  или оставить свёрнутым (В5). Тело «Nothing was read.» — как `Title`
  (жирно), тело — `Runs` (моноширинные куски).
- **Кадр `s-io-19`.** Предпросмотр стоит (счёт без «already here», таблица),
  вопроса нет, уведомление — см. таблицу (жирное «‹stamp› did not answer» и
  продолжение «, so what…» — одна фраза), Import `disabled` с `title`
  «nextcloud · Контакты did not answer». «Try again» и технического блока
  кадр не рисует.
- **Сервер.** `importPreview` при ошибке `destUIDs` — `unavailableNotice`
  (`import.go:1243–1247` → `records.go:614–626`): «Контакты is unavailable» /
  «The server did not answer as expected, so nothing on this screen comes
  from it.» + `Details` + «Try again». Import при этом **включена**
  (`import.go:521–530`: `dest != nil`, записи есть). `runImport` при той же
  ошибке рисует предпросмотр с **пустой** таблицей (`import.go:1420–1426`,
  `:1449–1454`).
- **С17. Коллекция не ответила.** `importState.unanswered`: уведомление
  `Runs` — `strongRun("‹stamp› did not answer")`, «, so what is already there
  cannot be told. Nothing is written; try again, or choose another ‹address
  book | calendar | list of tasks | notebook› under «Into».» (stamp —
  «nextcloud · Контакты», не `collectionLabel`); без `Details`/`Actions` —
  если кадр прав (В8); `import.run` выключена, `Reason` — «‹stamp› did not
  answer»; `Existing` — nil (так и есть). `runImport` при отказе коллекции —
  тот же экран с таблицей предпросмотра, не пустой.
- **Р-73 — потолков у импорта нет.** Где сейчас:
  - `import.go:820` — `uploadedImports(r, s.cfg.ImportMaxBytes)`, сам потолок
    — `:1734–1748` (`LimitReader`, `MaxBytesError`);
  - `import.go:828–829` — файлов набора больше `ImportMaxRecords`;
  - `import.go:869–870` (карточки), `:890–891` (календарь) — записей больше
    `ImportMaxRecords`;
  - `import.go:1165`, `:1178–1179`, `:1182`, `:1187`, `:1208–1227`
    (`readAtMost`) — папка: байты и записи;
  - `import_zip.go:15–16` (комментарий), `:53–54` (члены архива), `:57`,
    `:85–87` (распакованное);
  - `import.go:1829–1832` — слова отказов «A file of up to …», «the file
    holds N records; an import takes M»;
  - `middleware.go:107–119` — `bodyLimit` импорта `ImportMaxBytes +
    MaxFormBodyBytes`; **по коду** только для `/app/{kind}/{account}/{col}/import`
    (`len(seg) == 6`), а POST `/app/{kind}/import` (бросок на сведённый вид)
    получает 1 MiB формы — файл больше ≈1 MiB туда сейчас отвечает 413;
  - тесты: `import_test.go:307–330` (`TestImportRefusesAFileOverTheCeiling`),
    `import_zip_test.go:101–114`.
  - **Оставить** (восстановление, не импорт): `conf.go:52–53, 147–148,
    325–331` (ключи живут ради restore), `restore.go:234, 700–738, 843–848,
    1002`, `restore_parts.go:161`, тип `errTooManyRecords` (`import.go:1720–1726`
    — им пользуется `restore.go:738`: перенести к restore).
- **С18. Снять потолки импорта** по списку выше; файл — потоком: импорт —
  в `isFileUpload` (`files_ops.go:220–242`, оба адреса: `/app/{kind}/import`
  и `/app/{kind}/{a}/{c}/import`), `_csrf` читается до файла
  (`readUploadFields`), без `MaxBytesReader` у тела. Сейчас CSRF импорта —
  `csrfTokenFromBody` → `r.ParseMultipartForm(1 MiB)` (`csrf_body.go:16`): всё
  больше 1 MiB Go кладёт **временным файлом на диск** — с Р-73 это любой
  крупный импорт, против «на диск Carrel содержимое не пишется» (память
  `no-content-on-server-disk`). Разбор — по мере чтения; черновик остаётся в
  памяти сеанса (`DraftPendingImport`, `import.go:212–217`). Отказ «больше
  потолка» исчезает; запись, которую не взял DAV-сервер, — строка Failed со
  словами сервера (так и есть, `import.go:1536–1540`).
- **С19. Спецификация под Р-73.** S-19 (`carrel-server.md:2088–2091` —
  «элементы считаются … против `CARREL_IMPORT_MAX_RECORDS`, распакованное … против
  `CARREL_IMPORT_MAX_BYTES`»), **S-29.4** (`:2785–2788`: «Всё, что приходит
  извне, разбирается с потолками до разбора: размер, число записей, число
  элементов архива…» — прямо противоречит Р-73), C-12 `limits.md:133, 150,
  194`, C-11 `carrel-common.md:4220–4221` («Потолок файла импорта», «Записей
  в одном импорте» → только восстановление). Защита от zip-бомбы
  (`import_zip_test.go:101–114`) без потолка байтов: предложение — член
  архива, распаковавшийся больше объявленного размера, — непрочтённая строка
  (`Failed`, «the archive says N, holds more»), а не потолок импорта; это
  решение сервера, не владельца, но записать в S-29.4.

### п. 10 — тетрадь ro в Into заметок (`s-io-11`)

- **Сервер.** `importTargets` (`import.go:282–310`) показывает ro-коллекции
  с `ReadOnly`, `Reason` «Read-only» для всех родов; фикстура
  `app-note-import/normal` несёт «nextcloud · Общие с семьёй» `readOnly: true`.
  **Закрыто по коду и фикстуре**, серверу делать нечего.

### Метка коллекции (владелец, 22.10)

- **Into шага 1 и предпросмотра** (`s-io-3…8`, `-15…-19`): ▍ перед именем —
  `TargetOption.Colour` есть (`view/cross.go:257`; фикстуры: `"colour":
  "#4a6b52"`) — **клиенту**.
- **Подзаголовок отчёта** (`s-io-9`, `-14`: `tc-mark--h11` + «nextcloud ·
  Контакты»): сервер шлёт `Subtitle = line(stamp)` без цвета
  (`import.go:497–500`). **С20.** `Subtitle` — один `Run{Text: stamp, Colour:
  catalogColour(dest.col)}` (`Run.Colour` есть, `view/common.go:25–28`, как
  s-conflict-11). C-6 `ImportPage.Subtitle` (`carrel-common.md:2224`).
- **Подзаголовок контакта** (`s-contacts-1` — карточка в панели, `-6`, `-16`,
  `s-publish-3` — карточка под листом, `mentions-1`, `-9`): второй
  `tc-head__sub` — `tc-mark--h11` + «nextcloud · Контакты». Модель
  `ContactPage.Source` (`SourceBlock`) несёт и `stamp`, и `colour` (фикстура
  `app-contact/normal`: `"source": {"colour": "#4a6b52", "stamp": "nextcloud ·
  Контакты", …}`) — **только клиент**: метка из `Source.Colour` перед
  `Source.Stamp`. `mentions-*` — функция-кандидат («Для сведения» записки 111),
  серверу строить нечего.

### Уведомления вровень с колонкой (`tc-msg--mr0-ml0`)

- Раскладка — **клиенту**; сервер ничего не несёт.

---

## 3. Фикстуры, check.py, U-6

**Красное 1b** (`check.py:48–55`): `s-io-17` ставит `data-screen="/app/contacts/import"`;
C-5 его знает (`carrel-common.md:421`, проверка 1 зелёная через шаблон рода),
а U-6 шаблона `/app/{kind}/import` не называет — U-6.9 (`carrel-client.md:1046–1080`)
разбирает только `/app/{kind}/{account}/{col}/import`. **С21.** В U-6.9
своя строка/заголовок `` `/app/{kind}/import` `` — вход из сведённого вида и
бросок туда: шаг 1 с взятым файлом (`Source.Taken`) и пустым Into
(`s-io-17`). Это снимает красное.

**Новые фикстуры** (генерация — `fixture_test.go:3725–3783`, `TestFixtureImport`;
шаг 1 контактов — `:2437–2446`; реестр типов — `view/fixtures_test.go:54–57`):

| Фикстура | Кадр | Как получить | Строка `check.py` |
|---|---|---|---|
| `app-event-import/normal` | `s-io-15` | GET `…/calendar/…/lichnyj/import` | `STATE_FRAME["app-event-import/normal"] = "s-io-15"` (у `-15` есть `data-part` — `canon_frame` его не возьмёт) |
| `app-task-import/normal` | `s-io-16` | GET `…/tasks/…/dela/import` | `STATE_FRAME["app-task-import/normal"] = "s-io-16"` |
| `app-contacts-import/normal` (новый каталог, адрес `/app/contacts/import`) | `s-io-17` | POST файла на `/app/contacts/import` (Им4) — шаг 1 с `Taken` и пустым Into | `FIXTURE_SCREEN["app-contacts-import"] = "/app/contacts/import"`; `STATE_FRAME[…/normal] = "s-io-17"`; реестр `view/fixtures_test.go` |
| `app-contact-import/refused` | `s-io-18` | POST «Разное (mix).vcf», первая строка не BEGIN:VCARD | `STATE_FRAME["app-contact-import/refused"] = "s-io-18"` |
| `app-contact-import/unanswered` | `s-io-19` | предпросмотр, Into на коллекции, чей сервер не отвечает (в наборе mockup — нужна коллекция «лежащего» подключения; есть ли она в `newMockupApp` — **не проверено**) | `STATE_FRAME["app-contact-import/unanswered"] = "s-io-19"` |

**Существующие — дополнить данными кадров:** `app-contact-import/preview` —
карточка 2.1 с битым QP («line 12: cannot decode quoted-printable», имя «—»);
`app-contact-import/report` — вторая запись того же UID («imported as a
copy») и свойство, которое сервер не держит («Not kept by the server»;
теряет ли davtest хоть что-то — **не проверено**; «the server gave its own» в
наборе не получить — Google); `app-event-import/preview` — событие без
SUMMARY и без DTEND с DTSTART-датой («Untitled event» · «Ready — no end: a
whole day; no title»); `app-event-import/report` (`s-io-14`) — битый RRULE
(«line 418: cannot read RRULE «FREQ=FORTNIGHTLY»»); `app-task-import/preview`
— «Untitled task» · «Ready — no title» и битый DUE («line 88: cannot read DUE
«2026-13-01»»); `app-note-import/normal` — строка под полем (С2), `kinds`
в порядке кадра, `many`. Набора заметок («3 files · 12 KB») кадром нет —
фикстура не обязательна.

**Рейл `s-io-17`** — Контакты отмечены, Архив нет (вход из сведённого вида,
`combinedBackRail`, `import.go:1842–1844`): сойдутся ли отметки фикстуры —
**не проверено** (проверка 8 у `normal` сверяет отметки).

**`FRAME_TODO` «app-event-import: коллекции рейла»** (`check.py:307–308`) —
`s-io-6` теперь рисует «Дни рождения»: строку снять, задачу 199 п. 1 закрыть
(сверить).

---

## 4. Серверу — сводка пунктов

| № | Что | Где | Записка |
|---|---|---|---|
| С1 | Строки под полем файла контактов, событий, задач — слова кадров, с «‹ext› — » | `importHints`, `import.go:437–441`; C-6 | п. 1 |
| С2 | Заметки: `kinds` `.md,.zip,.ics`, строка под полем | `import.go:226–234`, `:437–441` | п. 2 |
| С3 | `ImportSource.Many` | `view/cross.go:668`, C-6 `:2241` | п. 3 |
| С4 | Подзаголовок набора «3 files · 12 KB» | `importSubtitle`, `import.go:560–568` | п. 3 |
| С5 | Набор берёт `.zip` и `.ics` | `readNoteFiles`, `import.go:1798–1815` | п. 3 |
| С6 | `Source.Taken` — только имя | `import.go:536–538`, C-6 `:2246` | п. 3 (`s-io-17`) |
| С7 | Отказ броска одной фразой + «One file at a time…» | `import_drop.go:58–79`, `lists.go:44–57`, C-6 `:1095–1104`; `import.go:1826` | п. 4 |
| С8 | Названия по умолчанию «Unnamed contact / Untitled event / Untitled task» | `importTitle`, `import.go:1099–1116` | п. 5 (Р-72) |
| С9 | Пометки «no end: a whole day», «no title» | `readyNote`, `import.go:1008–1036` | п. 5 |
| С10 | Отказ по записи, не по файлу | `cal/split.go:192–195`; `import.go:878–881`, `:907–909` | п. 5 |
| С11 | Имя непрочтённой — «—» | `importSkip.named`, `import.go:170–180` | п. 6 |
| С12 | `Failures` — фраза предпросмотра | `import.go:1470–1474` | п. 6 |
| С13 | Номер строки в отказе разбора | `card/split.go`, `cal/split.go` | п. 6 |
| С14 | QP vCard 2.1; проверка DTSTART/DTEND/DUE/RRULE | `provider/card`, `provider/cal` | п. 6, Р-72 |
| С15а | `Report.NewUIDs` «имя · старый → новый · почему» | `import.go:1555–1560`, `view/cross.go:741` | п. 7 |
| С15б | `Report.Lost`, `Losts` | `importLanded`, `import.go:1646–1653` | п. 8 |
| С15в | Снять `Report.Rows`, `Report.Note` (Им17) | `import.go:1479–1573`, `:1579–1603` | пп. 7–8 |
| С16 | Отказ шага 1 — «Nothing was read.» + слова кадра; перенос «holds no record», «archive holds no Markdown» на шаг 1; правило первой строки | `importRefusal`, `import.go:1819–1838`; `:524–525`; `import_zip.go:91–93`; `:1155–1161`; `:1406–1411` | п. 9 |
| С17 | Коллекция не ответила: уведомление кадра, Import выключена, таблица на месте | `import.go:1240–1250`, `:521–530`, `:1420–1454` | п. 9 |
| С18 | Снять потолки импорта; поток (`isFileUpload`), без временных файлов | см. список Р-73 | Р-73 |
| С19 | S-19, S-29.4, C-12, C-11 под Р-73; zip-бомба — непрочтённая строка | `carrel-server.md:2088–2091, 2785–2788`; `limits.md:133,150,194`; `carrel-common.md:4220–4221` | Р-73 |
| С20 | Подзаголовок отчёта с меткой (`Run.Colour`) | `import.go:497–500` | метка |
| С21 | U-6.9 называет `/app/{kind}/import` (красное 1b) | `carrel-client.md:1046` | `s-io-17` |
| С22 | Фикстуры §3, строки `STATE_FRAME`/`FIXTURE_SCREEN`, снять `FRAME_TODO` | `fixture_test.go:3725–3783`, `check.py:253–258, 307, 477–484`, `view/fixtures_test.go:54–57` | — |

Тесты, которые поедут вместе: `import_frames_test.go` (слова кадров),
`import_lines_test.go` (заглушки названий), `import_broken_card_test.go`
(«END:VCARD» в строке → «line N: …»), `import_drop_test.go` (Elsewhere),
`import_test.go:307–330`, `import_zip_test.go:101–114` (потолки),
`import_loss_test.go` (потери — теперь карточка).

Клиенту (не сервер): подпись поля файла по роду («vCard file», «iCalendar
file», «Markdown, a zip of them, or .ics»); подсказка Into шага 1 календаря
и задач (`s-io-15`, `-16`) и «Choose a …» пустого Into (`s-io-17`); метка ▍ в
Into и в подзаголовке контакта; `tc-msg--mr0-ml0`; имя взятого файла в поле
(`data-tc-filename`). Им11 (форма сроков «Mon 09:00 · weekly until …»,
«· all day») — открыт, записка 111 его не трогает.

---

## 5. Пакеты (по одной версии)

Пакет 1 — сервер `v0.2.357`: **слова и модель шага 1, броска и отчёта** —
С1–С9, С11, С12, С15а–в, С20, С21 и фикстуры `app-event-import/normal`,
`app-task-import/normal`, `app-contacts-import/normal`, дополнения
`preview`/`report` там, где не нужны номера строк (безымянные записи,
«imported as a copy», «Not kept»). Всё — в `web/import*.go`, `view/`, C-6,
U-6.9; без провайдеров.

Пакет 2 — сервер `v0.2.358`: **состояния и Р-73** — С16, С17, С18, С19 и
фикстуры `app-contact-import/refused`, `/unanswered`. Трогает
`middleware.go`, `files_ops.go`, `csrf_body.go`-путь импорта, `conf`
(комментарии), S-19/S-29.4/C-12/C-11; тесты потолков снимаются.

Пакет 3 — сервер `v0.2.362` (сделан 23.10.2026): **разбор с номерами строк** — С10, С13, С14 в
`provider/card` и `provider/cal` (отказ по записи, «line N: …», QP 2.1,
проверка дат и RRULE) и дополнения фикстур `preview`/`report` битыми
строками кадров (`s-io-5`, `-7`, `-9`, `-14`).

Пакеты 1 и 2 малы и в одних файлах — по памяти «Bigger versions» их можно
собрать одной версией (`v0.2.357`), пакет 3 — отдельно: он трогает
провайдеры и их тесты.

---

## 6. Вопросы к кадрам (кандидаты в задачи Typecase, не владельцу)

Старые В1–В11 (`scans/import.md`) записка 111 закрыла: В1 — п. 1, В2 — п. 2,
В3 — п. 3, В4 — `s-io-17`, В5 — п. 4, В6 — Р-72, В7 — п. 6, В8 — «With a new
UID», В9 — «Not kept by the server», В10 — `s-io-18`, `-19`, Р-73, В11 — п. 10.
В12 (лист экспорта) — открыт, дубль `scans/sheets.md` В8.

- **В1. Подпись `s-io-1` держит обе фразы отказа.** Старая — «Contacts take
  ‹.vcf› — the calendar and the list of tasks take this one», новая — «Only
  .vcf files are imported here». Какая стоит? Расширения в красной строке —
  моноширинно, как в «One ‹.vcf› file into …»?
- **В2. «Imported as copies» и «With a new UID».** Подпись `s-io-9` называет
  обе; при ответе «Import as copies» строки «старый → новый · imported as a
  copy» окажутся в двух карточках. Остаётся ли «Imported as copies» вариантом
  второй карточки?
- **В3. «Почему» для UID, сменённого не ответом.** Нарисованы «imported as a
  copy» и «the server gave its own». Слова для iCloud (SV-55: учётная запись
  держит этот UID в другой своей коллекции; Carrel дал новый сам) и для второй
  записи того же UID в файле (под любым ответом — копия; «imported as a copy»?).
- **В4. Пометки «Ready — …».** Склейка: `s-io-6` «no end: a whole day; no
  title» («; »), `s-io-7` «40 % done, priority 1» («, ») — правило? Слова для
  события без конца с датой-временем («no end: a moment»?) и для контакта без
  имени («no name»? «no title»?).
- **В5. Отказ шага 1.** «Nothing was read.» — заголовок каждого отказа
  («**Nothing was read.** No file was chosen.»), или у остальных свой?
  «The folder could not be read: ‹причина›.» — причина словами сервера (как
  `Details` сейчас: «PROPFIND …», текст ошибки)? Технический блок у отказа
  остаётся свёрнутым или его нет?
- **В6. Когда «This file holds no record of this kind.»** Файл только с
  задачами в календарь — отказ шага 1 теряет «import it into a list of
  tasks», которое даёт предпросмотр. Отказ — только когда в файле нет вовсе
  записей рода (ни чужих, ни битых)? Папка без `.md` и набор без `.md` —
  «The folder holds no Markdown file.» / «None of these files is Markdown.»
  или общие слова?
- **В7. «Не тот файл» у прочих родов.** Нарисовано для vCard. `.ics` —
  ««x.ics» is not an iCalendar file: its first line is not ‹BEGIN:VCALENDAR›.»?
  Битый zip? Пустой файл (`.vcf`, `.ics`, одиночный `.md`) — сейчас «Empty
  file: nothing to store and nothing to name it by.»
- **В8. `s-io-19`.** Нет «Try again» и технического блока — сервер их не
  шлёт? Уведомление — одна фраза с жирным началом («**… did not answer**, so
  …»): `Title` + `Body` клиент склеит через пробел/точку; сервер пошлёт
  `Runs` с жирным куском — так?
- **В9. Строки отчёта при отказе сервера.** «Failed» записи, которую не взял
  DAV-сервер, — «‹имя›: ‹слова сервера›» или только слова? Запись, которой
  сервер после записи не показывает вовсе, — «Failed» или «Not kept by the
  server» (запись без свойства)?
- **В10. Название по умолчанию — в записи или только на экране?** «Unnamed
  contact», «Untitled event», «Untitled task»: пишет ли импорт FN/SUMMARY в
  запись? vCard 3.0/4.0 без FN — невалиден (RFC 6350 §6.2.1), строгий сервер
  его не возьмёт.
- **В11. «—» блёкло или нет.** C-6 `ImportLine`: «пустая ячейка — «—»
  приглушённо»; кадры: `s-io-7` срок готовой задачи — `--dusty`, а UID
  непрочтённой (`s-io-5`), срок пропущенного (`s-io-6`) и непрочтённой
  (`s-io-7`) — обычным. Правило (сейчас — забота клиента, но фраза C-6)?
- **В12. Подзаголовок набора.** «3 files · 12 KB» — «3 files» моноширинно,
  как имя файла, или обычным? Zip внутри набора считается одним файлом или
  своими членами?

---

## 7. Не проверено

- Сервер не запускался; поведение — по коду. Что пишет davtest для карточки
  без FN и события без DTEND — не смотрел.
- Отдаёт ли mockup-набор (`newMockupApp`) коллекцию, чей сервер не отвечает
  (для `/unanswered`), и теряет ли davtest свойство (для «Not kept»).
- Отметки рейла `s-io-17` против `combinedBackRail` фикстуры.
- Разбор `vcard.Decoder` на строках QP 2.1 с мягким переносом «=» — по коду
  QP не разбирается, но отказ ли он или сырой текст — не пробовал.
- `cutCards` при мусоре до первого `BEGIN:VCARD` — по коду пропускает и
  читает карточки дальше (правило «первая строка» кадра `s-io-18` не держит).
