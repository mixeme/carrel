# Библиотека компонентов

Отсюда собираются экраны. Здесь лежит и разметка компонента, и его стили, рядом
друг с другом, потому что порознь они разъезжаются: почти каждый дефект, найденный
в разборах макетов, был одним и тем же — **размер и устройство знала соседняя
разметка, а не компонент**.

```
component/
  tmpl/   один файл на компонент — {{define "m-…"}}
  css/    один файл на компонент — числовой префикс задаёт порядок каскада
  README.md
```

## Имена

Имена — из макетов, дословно: `.m-bar`, `.m-head`, `.m-row`. Раздел 6
[carrel-ui-mockups.html](../../../docs/visual/carrel-ui-mockups.html) объясняет,
**почему** примитив устроен так; здесь лежит то, что из этого следует.

Префикс `m-` достался от макетов, где он разводил примитивы с типографикой самого
документа. В продакшене такой коллизии нет, и имя оставлено ровно затем, чтобы
макет и экран были **одной разметкой**: кадр из макета и живой экран сравниваются
по узлам, а не на глаз. Наборы данных для такой сверки лежат в
[docs/visual/fixtures](../../../docs/visual/fixtures/README.md).

## Как этим пользоваться

Компонент вызывается через `{{template}}`, а вход собирается типизированной
функцией — не `map`, чтобы опечатка в имени поля роняла отрисовку, а не тихо
давала пустую строку:

```gotemplate
{{template "m-head" (head "Title" "Combined calendar" "Subtitle" "5 of 7 calendars")}}
    <div class="m-acts">…действия экрана…</div>
{{template "m-headend"}}

{{template "m-bar" (bar)}}
    …поля и переключатели экрана…
    {{template "m-more"}}
    {{template "m-right" (right "Second" true)}}…счётчик…{{template "m-rightend"}}
{{template "m-barend"}}

{{template "m-list" (list)}}
{{template "m-group" (group "Label" "Monday · 10 August" "Num" "3")}}
{{template "m-row" (row "Kind" "agenda")}}
    <span class="m-bar3" data-swatch="…"></span>
    <span class="mono">10:00</span>
    <a class="t detail-link" href="…">…</a>
    <span class="s">…</span>
{{template "m-rowend"}}
{{template "m-listend"}}

<form method="post" class="m-form" action="…">
    <div class="m-f w-md">
        <label for="port">Port</label>
        <input class="m-in is-mono" id="port" name="port">
        <p class="m-hint">Chosen by the system at each start.</p>
    </div>
    <label class="m-check"><input type="checkbox" name="stop">Stop the server on exit</label>
    <div class="m-formfoot">
        <button type="submit" class="m-btn is-primary">Save</button>
    </div>
</form>

{{template "m-rail" (rail "Nav" true "Label" "Sections" "App" true)}}
    {{template "m-nav" (nav)}}
        <a href="…">{{template "ico" "cal"}}Calendar</a>
    {{template "m-navend"}}
{{template "m-railend" (rail "Nav" true)}}

{{template "m-rail" (rail "Sec" true)}}
    {{template "m-src" (src "Href" $allURL "All" true "On" true)}}
        <span class="m-box is-part"></span>
        <span class="m-bar3 is-multi" aria-hidden="true"></span>
        <span class="t">Combined calendar</span>
    {{template "m-srcend" (src "Href" $allURL)}}
    {{template "m-rail-sec" (railSec "Label" "Nextcloud")}}
    {{template "m-src" (src "Item" true)}}
        <label>
            <input type="checkbox" name="source" value="…" checked>
            <span class="m-box"></span>
        </label>
        <span class="m-bar3" data-swatch="…" aria-hidden="true"></span>
        <a class="t" href="…">Личный</a>
        <span class="m-meta">ro</span>
    {{template "m-srcend" (src "Item" true)}}
    {{template "m-rail-secend"}}
    {{template "m-rail-foot" (railFoot)}}
        <button type="submit" class="m-btn">Apply</button>
    {{template "m-rail-footend"}}
{{template "m-railend" (rail)}}
```

Парные компоненты (`m-head` / `m-headend`) открывают и закрывают тег: между ними
экран кладёт своё содержимое. Одиночные рисуют себя целиком.

**Вход передаётся всегда**, даже пустой: `{{template "m-bar" (bar)}}`, а не
`{{template "m-bar"}}`. Тогда внутри компонента `.` — типизированный ноль, а не
`nil`, и `{{if .Attrs}}` не падает.

## Что нельзя

- **Писать класс компонента руками в экране.** `<div class="m-bar">` в шаблоне
  экрана роняет `TestComponentClassesStayInComponentFiles`. Класс принадлежит
  своему файлу в `css/`, разметка — своему файлу в `tmpl/`.
- **Заводить второе имя тому же самому.** Именно так появились шестнадцать классов
  для одной полосы действий и двадцать восемь для шапки. Если нужного вида нет —
  это модификатор существующего компонента (`is-…`), а не новый компонент.
- **Задавать размер компонента снаружи.** Отступ, высота строки и число колонок —
  дело компонента. Экран говорит, что показать, а не какого оно размера.
- **Передавать `Attrs` через `safeHTML`.** Атрибуты экрана (`data-files-list`,
  `data-columns-root`, `data-files-ops hidden`) попадают туда, где html/template
  ждёт **имя атрибута**, а эта позиция фильтруется по типу: всё, кроме
  `template.HTMLAttr`, выходит буквальным `ZgotmplZ`, и атрибут просто
  исчезает. Ничего не падает — класс верный, страница рисуется, ворота
  зелёные, и замечает только браузер: так файловый менеджер разом остался без
  панели выделения, без панели инструментов и без сортировки таблицы. Поэтому
  вход — `(safeAttr "…")`, поле — `template.HTMLAttr`, и держат это
  `TestComponentAttrsAreAttributeTyped` и `TestScreensPassAttrsAsAttributes`.

## Состав

| Компонент | Вход | Шаблон · стили | Что это |
|---|---|---|---|
| `m-head` / `m-headend` | `head` | `tmpl/m-head.html` · `css/10-head.css` | Шапка раздела. `Side` — панель справа (h2, 19px); `Plain` — диалог без своего отступа; `Doc` — хром чтения заметки без h1 |
| `m-bar` / `m-barend` | `bar` | `tmpl/m-bar.html` · `css/20-bar.css` | Полоса инструментов; `Sel` — заливка выделения (`.m-sel`) |
| `m-barform` / `m-barformend` | `barForm` | `tmpl/m-bar.html` · `css/20-bar.css` | Та же полоса настоящей формой GET — диапазон дат в агенде |
| `m-right` / `m-rightend` | `right` | `tmpl/m-bar.html` · `css/20-bar.css` | Правая группа полосы; `Second` — уходит под «⋯» |
| `m-sep` | `sep` | `tmpl/m-bar.html` · `css/20-bar.css` | Вертикальный разделитель на полосе |
| `m-range` / `m-rangeend` | `barRange` | `tmpl/m-bar.html` · `css/20-bar.css` | Диапазон дат как один элемент полосы |
| `m-more` | — | `tmpl/m-bar.html` · `css/20-bar.css` | Кнопка «⋯»: скрыта, пока колонка широкая |
| `m-list` / `m-listend` | `list` | `tmpl/m-row.html` · `css/30-row.css` | Колонка строк; `Attrs` — id и `data-columns-root` экрана |
| `m-row` / `m-rowend` | `row` | `tmpl/m-row.html` · `css/30-row.css` | Строка списка. `Kind` — модификатор макетов (`contact` / `agenda` / `task` / `note` / `find` / `file`); `Done` / `Overdue` / `On` — состояния; `Class` — маркеры JS (`is-contact`, `is-merged`) |
| `m-group` | `group` | `tmpl/m-row.html` · `css/30-row.css` | Рубрикатор над пачкой строк и счётчик справа |
| `m-form` / `m-formend` | `form` | `tmpl/m-form.html` · `css/40-form.css` | Форма как ряд полей. Экран чаще пишет класс на своём `<form>` (слот): method, action и enctype всегда его |
| `m-f` / `m-fend` | `field` | `tmpl/m-form.html` · `css/40-form.css` | Ячейка поля. `Width` — `xs`/`sm`/`md`/`lg`/`xl`/`full`; `OwnLine` — без пустой строки подписи |
| `m-formfoot` / `m-formfootend` | `formFoot` | `tmpl/m-form.html` · `css/40-form.css` | Подвал формы: кнопки экрана, высота 26 px |
| `m-seg` / `m-segend` | `seg` | `tmpl/m-form.html` · `css/40-form.css` | Сегмент (сортировка, плотность, Week/Month). `Menu` — компактный в меню пользователя |
| `m-fset` / `m-fsetend` | `fieldSet` | `tmpl/m-form.html` · `css/40-form.css` | Группа полей с легендой |
| `m-rail` / `m-railend` | `rail` | `tmpl/m-rail.html` · `css/50-rail.css` | Левая колонка. `Nav` — `<nav>` (разделы, настройки); иначе `<aside>` (источники). `App` — `data-app-rail`; `Sec` — `data-section-rail` |
| `m-nav` / `m-navend` | `nav` | `tmpl/m-rail.html` · `css/50-rail.css` | Стек ссылок раздела |
| `m-src` / `m-srcend` | `src` | `tmpl/m-rail.html` · `css/50-rail.css` | Строка источника. `Href` — ссылка, `Item` — пункт списка; `All` / `On` / `Off` / `Root` / `Ext` / `Error` — состояния макетов |
| `m-rail-sec` / `m-rail-secend` | `railSec` | `tmpl/m-rail.html` · `css/50-rail.css` | Группа источников под рубрикатором (`Label`) |
| `m-rail-foot` / `m-rail-footend` | `railFoot` | `tmpl/m-rail.html` · `css/50-rail.css` | Подвал рейла: Apply, «New calendar» |
| `m-side` / `m-sideend` | `side` | `tmpl/m-panel.html` · `css/60-panel.css` | Колонка подробностей справа. Экран чаще пишет класс на своём `<aside>` (`#app-details`, `#files-props`): id и hidden всегда его |
| `m-dialog` / `m-dialogend` | `dialog` | `tmpl/m-panel.html` · `css/60-panel.css` | Рамка диалога. `Wide` / `Narrow` — 1080 / 520 px |
| `m-card` / `m-cardend` | `card` | `tmpl/m-panel.html` · `css/60-panel.css` | Карточка. `Accent` — выделенная; экран чаще пишет класс на своём узле |
| `m-menu` / `m-menuend` | `menu` | `tmpl/m-panel.html` · `css/60-panel.css` | Выпадающее меню. `Pop` — всплывает и места в раскладке не занимает |
| `m-table` / `m-tableend` | `dataTable` | `tmpl/m-table.html` · `css/70-table.css` | Таблица записей. `Class` — маркер экрана (`install-check-table`); колонки и строки — дело экрана |
| `importexportmenu` | `importExportMenu` | `tmpl/m-panel.html` · `css/60-panel.css` | Состав: «⋯» шапки единого вида — Import/Export через `.m-menu`. Не примитив макетов, а именованная сборка |

Классы, которые эти компоненты приносят с собой:

| Класс | Чей | Что это |
|---|---|---|
| `.m-head` | `m-head` | Сама шапка. `is-side` — панель справа; `is-plain` — диалог; `is-doc` — хром чтения заметки |
| `.m-h1` | `m-head` | Заголовок раздела |
| `.m-sub` | `m-head` | Подзаголовок под заголовком |
| `.m-crumbs` | `m-head` | Хлебные крошки |
| `.m-bar3` | `m-head`; слот строки и рейла | Полоска коллекции. Знает свой размер сама. В шапке её пишет компонент; в строке и в источнике экран пишет цвет |
| `.m-acts` | `m-head` | **Слот**: действия экрана. Содержимое — дело экрана, раскладка — дело шапки, поэтому этот класс экран пишет сам |
| `.m-bar` | `m-bar` | Сама полоса; `is-form` — вариант формой; `.m-sel` — панель выделения |
| `.m-right` | `m-bar` | Правая группа: счётчик, плотность, «Clear selection» |
| `.m-sep` | `m-bar` | Разделитель 1×20 |
| `.m-range` | `m-bar` | From/To и Show — один узел, чтобы диапазон уходил целиком |
| `.m-more` | `m-bar` | «⋯»; содержимое меню — ушедшие `is-2nd`, их подставляет `carrel.js` |
| `.m-list` | `m-list` | Колонка строк |
| `.m-row` | `m-row` | Сама строка; `--contact` / `--agenda` / `--task` / `--note` / `--find` / `--file` — сетка колонок. `is-on` — открытая, `is-done` / `is-overdue` — состояния |
| `.m-group` | `m-group` | Рубрикатор группы и счётчик |
| `.m-rubric` | `m-group` | Капитель рубрикатора (h2 для оглавления) |
| `.m-num` | `m-group` | Счётчик справа |
| `.m-form` | `m-form` | Ряд полей. `is-tight` — меньше отступ сверху |
| `.m-f` | `m-form` | Ячейка: подпись, поле, подсказка. `is-own-line` — без пустой строки подписи |
| `.m-fset` | `m-form` | Группа полей на всю ширину, с легендой-рубрикатором |
| `.m-in` | `m-form` | Поле. `is-pick` / `select.m-in` — стрелка списка; `is-area` — многострочное; `is-mono` — моноширинное |
| `.m-btn` | `m-form` | Кнопка. `is-primary` / `is-quiet` / `is-danger` / `is-danger-solid` / `is-sm` / `is-on` |
| `.m-seg` | `m-form` | Сегмент; выбранное — `.is-on` у ребёнка. `is-menu` — на всю ширину в выпадающем |
| `.m-check` | `m-form` | Подписанный флажок или радио |
| `.m-hint` | `m-form` | Подсказка под полем, 11 px |
| `.m-lbl` | `m-form` | Подпись поля вне `m-f > label` |
| `.m-formfoot` | `m-form` | Подвал: кнопки 26 px, как на полосе |
| `.m-rail` | `m-rail` | Левая колонка. `App` ставит `data-app-rail` (навигация), `Sec` — `data-section-rail` (источники): два узла одной сетки |
| `.m-nav` | `m-nav` | Стек ссылок; текущая — `aria-current="page"` или `.is-on`. `is-back` — «Back to app» |
| `.m-src` | `m-src` | Строка источника. `is-all` — единый вид; `is-root` — корень файлов (не единый вид); `is-on` / `is-off` / `is-error` / `is-ext` — состояния. Ячейки — прямые дети, хвост раскладывается сам |
| `.m-box` | `m-src` | **Слот**: флажок-квадрат. `is-off` / `is-part` — не опрошен / частично |
| `.m-meta` | `m-src` | **Слот**: хвост строки (счётчик, `ro`) |
| `.m-rail-sec` | `m-rail-sec` | Группа под рубрикатором — один аккаунт |
| `.m-rail-foot` | `m-rail-foot` | **Слот**: подвал; какие кнопки — дело экрана |
| `.m-side` | `m-side` | **Слот**: колонка подробностей; id, hidden и aria-label — дело экрана |
| `.m-fields` | `m-side` | **Слот**: сетка подпись/значение; какие dt/dd — дело экрана |
| `.m-sec` | `m-side` | **Слот**: секция под шапкой панели; рубрика и тело — дело экрана |
| `.m-dialog` | `m-dialog` | **Слот**: рамка диалога. `is-wide` / `is-narrow` |
| `.m-card` | `m-card` | **Слот**: карточка. `is-accent` / `is-ignored` — состояния экрана |
| `.m-msg` | `m-card` | **Слот**: баннер. `is-alert` / `is-accent`. Текст — дело экрана |
| `.m-empty` | `m-card` | **Слот**: пустое состояние; слова и кнопка — дело экрана |
| `.m-poll` | `m-card` | **Слот**: ход опроса; сводка и Stop — дело экрана |
| `.m-prog` | `m-card` | Полоска прогресса внутри `.m-poll`; ширину пишет `data-fill` |
| `.m-menu` | `m-menu` | **Слот**: выпадающее меню; какие строки — дело экрана. `is-pop` / `is-right` |
| `.m-badge` | `m-card` | **Слот**: значок (`is-linked` / `is-alert` / `is-local`) |
| `.m-tag` | `m-card` | **Слот**: категория. `is-on` — выбранная |
| `.m-tick` | `m-card` | **Слот**: флажок задачи 13 px |
| `.m-av` | `m-card` | **Слот**: аватар строки списка |
| `.m-table` | `m-table` | **Слот**: таблица записей; колонки и строки — дело экрана. Импортное превью и конфликт пишут класс сами |
| `.m-fname` | `m-table` | **Слот**: имя файла со значком |
| `.m-grid` | `m-table` | **Слот**: сетка плиток файлов |
| `.m-tile` | `m-table` | **Слот**: плитка файла. `is-up` — «наверх» |
| `.m-thumb` | `m-table` | **Слот**: превью плитки |
| `.m-transfers` | `m-table` | **Слот**: очередь загрузок; строки пишет `carrel.js` |
| `.m-tree` | `m-table` | **Слот**: дерево папок; узлы — дело экрана |
| `.m-title` | — | **Слот**: заголовок документа 27 px в теле заметки, не `.m-h1` |
| `.m-prose` | — | **Слот**: текст заметки |
| `.m-doc` | — | **Слот**: рамка документа заметки |
| `.m-docbody` | — | **Слот**: колонка текста заметки |
| `.m-metacol` | — | **Слот**: боковая колонка заметки |
| `.m-editbar` | — | **Слот**: панель правки заметки |
| `.m-steps` | — | **Слот**: шаги мастера импорта |
| `.m-diag` | — | **Слот**: диагностический дамп |
| `.m-kbd` | — | **Слот**: подсказка клавиши |

Высоту строки знает система, не экран: **28 px** там, где есть поле, **26 px** на полосе инструментов, **22 px** в таблице. Стрелку у `<select>` рисует `.m-in`, разметка только ставит класс.

Что экрану **можно** писать руками, перечислено поимённо в
`component_allowlist_test.go`: `slotClasses` — слоты вроде `.m-acts`,
`standaloneUse` — примитив, взятый отдельно от своего компонента, с указанием
экрана. Всё остальное — через `{{template}}`.

Новый компонент добавляется тремя файлами сразу — шаблон, стили, строка в этой
таблице. Ворота `TestComponentLibraryIsComplete` требуют всех трёх: компонент, о
котором негде прочитать, находят переписыванием заново под новым именем.

## Порядок каскада

Файлы в `css/` склеиваются на старте в один ответ `/static/component.css` в
порядке имён, поэтому у имени числовой префикс. Собранный файл не коммитится:
закоммиченный расходится с исходниками, и никто этого не замечает.

`component.css` подключается **до** `carrel.css`. Экранное правило может
перебить компонент — но тогда оно видно как исключение, а не растворено в общей
куче.
