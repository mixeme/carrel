# Скан раздела U-6.15 «Публичные страницы» (`/p/{token}…`)

Сверено по Typecase 0fe0244 и серверу v0.2.174 (main 6a62d8a), только чтение.

**Что сверялось.** Маршруты C-5: `/p/{token}` (страница, `PublishedPage`; мёртвый секрет — `DeadLinkPage`), `/p/{token}/download`, `/p/{token}/file/{path…}`, `/p/{token}/dav/{path…}` (потоки, вью-моделей нет). Кадры: s-publish-6, -7, -8, -9, -10, -11, -12 (экраны, `data-screen="/p/{token}"`, части по `data-part`), s-publish-16 (фрагмент — одна запись), rest-1 (мёртвая ссылка).

**Метод.** Фикстур у `/p/{token}` **нет ни одной** (см. Пс1), поэтому всюду, кроме rest-1, — «кадр против структуры»: черновик `frame2go` кадра (s-publish-6…12, все семь дали черновик) и HTML кадра — против `view.PublishedPage` (`carrel/server/view/plain.go`) и против того, чем её наполняет сервер (`carrel/server/internal/web/publish.go`, `publish_files.go`: `getPublished`, `publicRecord`, `publicCollection`, `publishedFilesPage`). s-publish-16 — фрагмент, HTML против структуры. rest-1 — HTML и черновик против фикстуры `dead-link/normal.json`.

## 1. Экраны, маршруты, модели, кадры, фикстуры

| Экран | Что в модели | Кадр | Фикстура |
|---|---|---|---|
| Календарь | `Days`, `Range`, `Actions` | s-publish-6 (`calendar`) | нет |
| Адресная книга | `Contacts`, `Actions` | s-publish-7 (`addressbook`) | нет |
| Заметка (одна запись) | `Note`, `Lede`, `Actions` | s-publish-8 (`note`) | нет |
| Файл с картинкой | `File` (`Preview: image`), `Lede`, `Actions` | s-publish-9 (`file`) | нет |
| Файл: текст и Markdown | `File.Body`, `File.Blocks` — в структуре есть, сервер не заполняет (Пс14) | s-publish-10 (`text`) | нет |
| Файл без предпросмотра | `File` (`Preview: none`), `Actions` «Download 84.2 MB» | s-publish-11 (`download`) | нет |
| Папка | `Folder` (`Crumbs`, `Rows`, `DAV`), `Actions` | s-publish-12 (`folder`) | нет |
| Одна запись: событие | `Title`, `Lede`, `Fields`, `Body`, `Actions` | s-publish-16 (фрагмент: только шапка) | нет |
| Одна запись: контакт, задача | `Fields`, `Body` | **кадра нет** | нет |
| Список задач | `Tasks` | **кадра нет** | нет |
| Тетрадь | `Notes` | **кадра нет** | нет |
| Папка внутри папки (`?in=`) | `Folder.Crumbs` из нескольких | **кадра нет** | нет |
| Источник не ответил / удалено | `Notices` («The source did not answer», «No longer there»), `Actions` пусты | **кадра нет** | нет |
| Мёртвая ссылка | `DeadLinkPage` | rest-1 | `dead-link/normal.json` (общая с `/invite/`, сверяется с кадром приглашения) |

Все маршруты `/p/…` в `FIXTURE_SCREEN` (`docs/spec/check.py`) отсутствуют; в `STATE_FRAME` нет ни одной строки на s-publish-6…12, -16. `docs/spec/fixtures/README.md` (строка 26) называет каталог `p-token` для `/p/{token}` — каталога нет. В `view/fixtures_test.go` `PublishedPage` не зарегистрирована.

rest-1 против `dead-link/normal.json`: заголовок «This link does not work», строка «It may have expired, been used already, or been withdrawn. Ask whoever sent it for a new one.», кнопка «Sign in» (primary) — совпадает дословно, разрывов нет.

## 2. Разрывы для сервера

### Общее для всех страниц

**Пс1. Фикстур публичной страницы нет.** Ни одного записанного ответа `/p/{token}`; README обещает каталог `p-token`. Нужно: каталог с состояниями по `data-part` кадров (C-14.3) — `calendar`, `addressbook`, `note`, `file`, `text`, `download`, `folder`, плюс `record` (s-publish-16 строкой `STATE_FRAME`), `tasks`, `notebook`, `gone`, `trouble`; строка в `FIXTURE_SCREEN`; регистрация `PublishedPage` в `fixtures_test.go`. Пока их нет, ни одно утверждение ниже проверкой `check.py` не держится.

**Пс2. Подвал «ссылка действует до…».** Все семь кадров рисуют подвал (`tc-main__foot`): s-publish-6 «This link works until 8 September 2026. The owner can revoke it at any moment.», s-publish-7 «This link works until 9 September 2026. The owner can revoke it at any moment.», s-publish-8, -9, -11 «This link works until 19 August 2026.», s-publish-10 «This link works until 19 August 2026 · the file is read from the server on every visit», s-publish-12 «This link works until 1 September 2026 · the contents are read from the server on every visit». В `PublishedPage` поля срока нет; `user.OpenLink` (ID, Target, Conn) срока до обработчика не доносит, хотя `ExpiresAt` у ссылки хранится. Нужно: поле подвала готовой фразой (день — данные) и срок в `OpenLink`.

**Пс3. `Lede` страницы коллекции и папки — слова, которых нет в кадрах.** Сервер ставит `Lede` «Read-only. Published from Carrel.» (`getPublished`, `publishedFilesPage`); у календаря, книги, списка задач, тетради и папки она остаётся. Кадры под заголовком рисуют другое: s-publish-6 «10 – 31 August 2026 · the page is read from the server on every visit», s-publish-7 «13 contacts · the list is read from the server on every visit», s-publish-12 — крошки и «· 4 files · 6.6 MB». Нужно: убрать эту строку; под заголовком — то, что в Пс5, Пс8, Пс16.

**Пс4. `Owner` не заполняется.** Поле `PublishedPage.Owner` есть в структуре и в C-6, в `publish.go` и `publish_files.go` ему ничего не присваивается, ни один кадр владельца не рисует (S-25.1: «ни имени владельца сверх необходимого»). Нужно: убрать поле из контракта.

### Календарь — s-publish-6

**Пс5. Диапазон под заголовком.** Кадр: `tc-head__sub` «10 – 31 August 2026 · …» — диапазон одной сложенной фразой. Модель: `Range.From` «10 August 2026» и `Range.To` «31 August 2026» порознь (`tf.date`). Нужно: готовая фраза диапазона (как складывать общий месяц и год — данные, C-10) в `Lede` или в `Range`.

**Пс6. Строка подписки.** Кадр: `tc-main__strip` — «Subscribe in your own client», поле с адресом «webcal://carrel.example/p/7f3ac9/personal.ics», «Copy», «Open in calendar app», подсказка «read-only, updates on its own». В модели адреса подписки нет; маршрута `/p/{token}/{имя}.ics` в `routes.go` нет (есть только `/download`, который отдаёт вложением). Нужно: поле с абсолютным адресом `webcal://…` (он же `href` кнопки «Open in calendar app»), маршрут, отдающий `.ics` для подписки; при `NoDownload` строки нет (комментарий в `getPublished` это уже обещает).

**Пс7. Цвет метки строки.** Кадр: у каждой строки `tc-mark` цвета календаря (`#4A6B52`). Сервер зовёт `agendaDays(…, mark=false)` — `AgendaRow.Colour` пуст (штамп сервера при этом тоже пуст, что кадру соответствует: `tc-row__source` пуста). Цвета коллекции в `PublishTarget` нет. Нужно: `Colour` строк — цвет опубликованного календаря.

Сошлось: день-рубрика «Monday · 10 August» и счёт (`AgendaDay.Rubric`, `.Count`), время, название, место (`Time`, `Summary`, `Location`), «Download .ics» (`Actions`, значок `i-down`), метка read-only (`ReadOnly`).

### Адресная книга — s-publish-7

**Пс8. Счёт под заголовком.** Кадр: «13 contacts · the list is read from the server on every visit». В модели счёта фразой нет (см. Пс3). Нужно: «13 contacts» / «1 contact» готовой фразой.

**Пс9. Строка адреса книги.** Кадр: «Address of this book», поле «https://carrel.example/p/2b91f4/team.vcf», «Copy», подсказка «another Carrel can subscribe to it; other clients import it». В модели адреса нет, маршрута `/p/{token}/{имя}.vcf` нет. Нужно: то же, что Пс6, для книги (без кнопки «Open in…» — её кадр не рисует).

**Пс10. Группы по букве.** Кадр: `tc-list__group` «А» со счётом «2», «В» со счётом «1», строки под ними. Модель: плоский `Contacts []ContactRow`; в приложении для того же рисунка есть `ContactGroup{Rubric, Count, Rows}`. Нужно: группы (`[]ContactGroup`) вместо плоского списка.

**Пс11. Телефоны и почта отдаются всегда.** Подпись кадра s-publish-7 и лист s-publish-3 («What the visitor sees»: «Name and organisation», «Phone numbers», «Email addresses», «Photo», «Note» — последние четыре выключены): что видит гость, решается при выдаче, «скачивание .vcf подчиняется тем же флажкам». В `user.PublishTarget` таких полей нет (есть только `From`, `To`, `NoDownload`, `HideDescriptions`); `publicCollection` всегда кладёт `Phone` и `Email` в строки, `publicRecord` для контакта — все `contactFieldsOf`, выгрузка — все записи целиком. Нужно: флажки в цели ссылки и их соблюдение страницей книги, страницей контакта и `.vcf`. (Сам лист — не этот раздел; здесь — его следствие на публичной странице.)

**Пс12. Цвет метки строки.** Кадр: `tc-mark` `#6B4E7A` у всех строк (цвет книги). Сервер: `ContactRow.Colour` не ставит (ставит только `Avatar.Colour` от UID). Нужно: цвет книги в `Colour`.

Сошлось: инициалы, имя, телефон, почта, организация (`Avatar.Initials`, `Name`, `Phone`, `Email`, `Org`), «Download .vcf».

### Заметка — s-publish-8

**Пс13. Вложения.** Кадр: рубрика «Attachments» и две строки — «2026-08-10-sync-board.png · 412 KB», «release-checklist.md · 3 KB»; по подписи каждая — своя ссылка с тем же сроком, а без отметки «опубликовать и их» страница «сказала бы „два файла“ и не назвала бы их». В `PublishedPage` и в `NoteDoc` вложений нет; `publicRecord` их не читает; отметки листа s-publish-4 «Publish them too — each gets its own link» в `PublishTarget` нет. Нужно: список вложений (имя, размер, адрес) либо один счёт фразой, когда их не публиковали.

Сошлось: заголовок (`Title`, `Note.Title`), день моноширинно «10 August 2026, 11:05» (`Lede`, моно-кусок; проверено по коду `publicRecord`), тело блоками (`Note.Rendered`: абзац, заголовок «Что делаем», задачи с отметкой — `Block.Checked`), «Download .md».

### Файл — s-publish-9, -10, -11

**Пс14. Текст и Markdown телом страницы не приходят.** Кадр s-publish-10: файл `release-checklist.md` разобран — заголовки «Перед заморозкой», «После», список из трёх пунктов с кодом «go test -race ./...», абзац; кнопка в шапке — «Download», не главная. Сервер (`publishedFilesPage`): `Preview` становится `image` только для четырёх типов картинок (`inlineTypes`), иначе остаётся `none`; `File.Body` и `File.Blocks` не заполняются нигде, значения `PreviewText` и `PreviewMarkdown` (C-8) не используются. Следствие: для `.md` сервер отдаст экран s-publish-11 с кнопкой «Download 3.1 KB» (главной). Нужно: для текста — `Preview: text` и `Body`, для Markdown — `Preview: markdown` и `Blocks`, с потолком размера (подпись кадра: большой текст «не выводится, а предлагается скачать»); кнопка тогда «Download» без размера и без `Primary`.

**Пс15. PDF.** U-6.15 и таблица видов макета: «PDF — браузеру». `PreviewPDF` сервер не ставит; PDF получает экран «без предпросмотра». Кадра PDF нет (см. вопросы). Нужно: `Preview: pdf` и `Href` с `?inline=1` — после решения по кадру.

Сошлось: имя (`Title`, `File.Name`), строка «image/png · 412 KB · changed 2026-08-10» одним моноширинным куском (`Lede`), картинка (`File.Href` с `?inline=1`), «Download» главной у s-publish-9, «Download 84.2 MB» у s-publish-11 (`Actions[0].Label`).

### Папка — s-publish-12

**Пс16. Итог папки в крошках.** Кадр: после текущей крошки «Материалы к релизу» — приглушённо «· 4 files · 6.6 MB». В `PublishedFolder` нет ни счёта, ни суммы размеров. Нужно: поле итога готовой фразой.

**Пс17. Адрес монтирования — без хоста.** Кадр: поле «https://carrel.example/p/5c40e2/dav/». Сервер: `Folder.DAV = s.route("/p/"+token+"/dav/")` — путь с базовым префиксом, без схемы и хоста (`path.go`: `route` добавляет только `basePath`). Нужно: абсолютный адрес, тот, что вставляют в «Подключить сетевой диск».

**Пс18. Тип строки папки.** Кадр: «folder · 6 files». Сервер: `row.Type = "folder"`, `FileRow.Items` не ставится. Нужно: счёт вложенного у строки папки (и одно слово с разделом файлов — см. вопросы: там «34 items»).

**Пс19. Время правки строки.** Кадр: числами — «2026-08-10 11:07», «2026-08-09 18:44». Сервер: `formatWhen(e.Modified, false, now)` → «10 August, 11:07». В разделе файлов для этого уже есть `numStamp` (`timefmt.go`). Нужно: `Modified` числами.

**Пс20. Значок строки.** Кадр: значки по типу — `file` у папки, `doc` у `.md` и `.pdf`, `img` у `.jpg` (черновик: `TableFile{Icon: …}`). Сервер `FileRow.Icon` на публичной странице не ставит. Нужно: `Icon`, как в `/app/files`.

**Пс21. Страница файла внутри ссылки на папку.** Подпись кадра и таблица видов: «нажатие на файл открывает его страницу — ту же, что выше, но внутри этой ссылки». У сервера такой страницы нет: `row.Href` файла — поток `/p/{token}/file/{path}`, `FileRow.Open` пуст, `/p/{token}?in=` принимает только папку. Нужно: адрес страницы файла внутри папки (в `Open`), отвечающий `PublishedPage` с `File`; `Href` остаётся скачиванием (кнопка «Download» строки).

Сошлось: заголовок, текущая крошка без адреса, «Download folder» (`i-down`), имена, размеры, типы файлов, `Href` строк (папка — `?in=`, файл — поток), папки сверху.

Данные: порядок строк кадра (скриншоты, release-checklist.md, 2026-08-04-whiteboard.jpg, смета-подрядчик.pdf) не по имени; сервер сортирует по имени без регистра — «2026-08-04…» встанет первым. Набор фикстуры `folder` стоит взять в порядке сервера либо подобрать имена — «данные».

### Одна запись — s-publish-16

Разрывов в шапке нет: заголовок «Синк-ап по релизу» (`Title`), строка «Monday, 10 August 2026, 10:00 – 11:00 · Переговорная 2» (`Lede`, тест `TestPublicRecordHeadIsTheFrame`), «Download .ics». Что ниже шапки — см. вопросы (В3).

## 3. Своя работа клиента

- Оболочка без рейла: `Shell{Body{Main}}`, первым — `Banner` с логотипом крупнее, `Badge` «read-only» по `ReadOnly`, кнопки `Actions` справа; у s-publish-11 кнопка не в шапке, а в середине (`tc-main__stage`) — выбор по `File.Preview == none`.
- Неизменные слова экрана: «read-only»; подписи строк-полос «Subscribe in your own client», «Address of this book», «Mount as a drive» и их подсказки «read-only, updates on its own», «another Carrel can subscribe to it; other clients import it», «read-only WebDAV, no password»; «Copy», «Open in calendar app», «How to mount»; хвосты «the page / the list / the file / the contents … is read from the server on every visit» (зависят от рода, не от данных); заголовки столбцов «Name», «Size», «Type», «Changed»; «Open» у папки и «Download» у файла в строке; рубрика «Attachments»; «Full width» / «Reading width»; «Search this list»; текст s-publish-11 «This file cannot be shown in a browser, and Carrel does not try. It is streamed with its own type, transcoded nowhere and stored nowhere.».
- Прочерк «—» на месте пустого телефона (s-publish-7) и размера папки (s-publish-12).
- Виджеты по черновикам: календарь — `ListGroup` + строки `tc-row--timecols`; книга — `ListGroup` + `FaceRow` (`tc-row--avatarcols`), пустая ячейка источника; заметка — `DocLayout{Doc{DocTitle, DocSub(Mono), Prose(Lead), DocTasks, DocFiles}}`; файл — `Head` 19-го кегля + `tc-thumb--wide`; текст — `Doc` из блоков; папка — `Head` с `Crumbs{Current, Aside}`, `Strip`, `Table`; подвал — `MainFoot`.
- Заметку рисовать из `Note.Rendered`, не из `Note.Body`; `Bar`, `Form`, `Fields` у публичной пусты.
- Узкий экран — та же колонка (U-6.15).

## 4. Вопросы к кадрам и кандидаты в задачи Typecase

- **В1. Пресеты недели без кадра.** У календаря, опубликованного целиком, сервер отдаёт `Range.Presets` «Previous week», «This week», «Next week» и показывает одну неделю; s-publish-6 рисует только ссылку с днями («10 – 31 August 2026») и ни одной кнопки. Кадра «календарь целиком» нет — где стоят пресеты, неизвестно.
- **В2. Подвал разный в кадрах.** «The owner can revoke it at any moment.» есть у s-publish-6, -7 и нет у -8…-12; хвост «· … read from the server on every visit» в подвале у -10, -12, а у -6, -7 он под заголовком. Бессрочная ссылка («No expiry») кадра подвала не имеет вовсе. От этого зависит, одно поле в Пс2 или фраза целиком.
- **В3. Тело одной записи.** s-publish-16 — фрагмент из одной шапки; сервер отдаёт ещё `Fields` (у события «When» повторяет `Lede`) и `Body`. Кадров страницы контакта и задачи нет; как рисуются поля и описание — не нарисовано.
- **В4. Список задач и тетрадь** (`Tasks`, `Notes`) — кадров нет; таблица видов макета говорит о «пяти видах», U-6.15 — о семи, сервер публикует и их.
- **В5. Состояния без кадра:** источник не ответил, опубликованное удалено (`Notices` — слова сервера «The source did not answer», «No longer there», кадром не проверены), пустая папка, пустой календарь за диапазон, пустая книга, папка внутри папки (крошки из нескольких), PDF, большой текст сверх потолка, ссылка без скачивания (`NoDownload`: нет кнопки и строки подписки).
- **В6. Макет противоречит себе о тегах заметки.** Подпись s-publish-8: «Тегов на публичной странице нет ни в каком случае»; таблица видов ниже: заметка — «Страница с текстом, тегами и списком вложений». Кадр тегов не рисует; принято: тегов нет.
- **В7. «files» против «items».** s-publish-12: «folder · 6 files», «· 4 files · 6.6 MB»; раздел файлов (`FileRow.Items`, s-files-1): «34 items». Одно слово или два — решить до Пс16, Пс18.
- **В8. Размер одного файла в трёх кадрах.** `release-checklist.md`: «3.1 KB» в s-publish-10, «3 KB» в s-publish-12 и s-publish-8. `formatSize` даст одно.
- **В9. Имя файла в строке папки — не ссылка.** В HTML s-publish-12 имя — `<span class="tc-table__filename">`, а подпись обещает «нажатие на файл открывает его страницу» (Пс21); в s-files-1 имя — ссылка. Кадр и подпись расходятся.
- **В10. Главная кнопка.** «Download» главная в s-publish-9 и -11, не главная в s-publish-10; «Download .ics/.vcf/.md» и «Download folder» — не главные. Сервер ставит `Primary` у любого файла.
- **Typecase / без сценариев.** «Copy» трёх полос (буфер — только сценарием; без него поле должно выделяться целиком); «Search this list» (по подписи фильтрует загруженное сценарием; без сценариев ничего не шлёт — нужна ли форма GET `?q=`, в модели её нет); «Full width / Reading width» (в модели выбора нет, без сценариев не переключается; адресом `?width=` или только сценарием); «How to mount» (по подписи показывает «три строки для трёх систем» — ни кадра раскрытого вида, ни текста этих строк нет).
- **Наблюдение (не кадр).** Строки публичной страницы несут `UID` записей (`ContactRow.UID`, `AgendaRow.UID`, `TaskRow.UID`, `NoteRow.UID`) и бейджи `AgendaRow.Badges` («4 attendees», «repeats weekly») — кадр s-publish-6 в этой ячейке пуст. Нужны ли они гостю — решить.

## 5. Не проверено

- Живой ответ сервера не снимался: всё о том, что сервер отдаёт, — из чтения кода `publish.go`, `publish_files.go`, `timefmt.go`, `path.go`, не из запроса.
- `formatSize` на значениях кадров («412 KB», «84.2 MB», «3.1 KB») не прогонялся.
- Что `Note.Rendered` для текста кадра s-publish-8 даёт именно абзац-лид, рубрику и задачи — не прогонялось (`cal.RenderMarkdown` не читан).
- Какие бейджи реально попадут в строки публичного календаря — по коду `agendaDays` прочитана только ветка задачи.
- `/p/{token}/dav/` и потоки — вне скана (вью-моделей нет).
- Клиент (`carrel/client`) на наличие экрана публичной страницы не смотрен: поиск `PublishedPage` в нём ничего не нашёл.

## 6. Итог

- Разрывов для сервера: **21** (Пс1–Пс21): общих 4, календарь 3, книга 5, заметка 1, файл 2, папка 6.
- Из них «данные»: **0** среди нумерованных — фикстур нет, расходиться набору не с чем; одна пометка «данные» без номера (порядок строк папки).
- Вопросов к кадрам: **10** (В1–В10), плюс 4 элемента без сценариев (Copy, Search, ширина, How to mount) и одно наблюдение об UID и бейджах.
