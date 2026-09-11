# Собрать экраны как кадры макета

**Composer.** Рисунок собранных экранов (`typecase/screens`) против
[carrel-ui-mockups.html](../visual/carrel-ui-mockups.html). Макет — источник
рисунка; спека и библиотека догоняют. Календарь и задачи в этот план не входят:
маршрутов нет.

> **Правило находки.** Если кадр макета не собирается из библиотеки или
> собирается, но рисунок другой — это дыра виджета или спеки, не правило в
> `screens.css`. В `screens.css` нет селекторов `.tc-`. Находки этого плана —
> `Э-`, продолжение нумерации после сборки (`Э-1`…`Э-6` закрыты).

Один коммит на раздел `### N.` (семь коммитов кода). Сначала проверка, которая
краснеет на текущем коде, потом правка. Этот файл и changelog — отдельный
коммит до кода.

---

## Почему это осталось

Сборка экранов закрыла состав и числа токенов: покрытие 72/72, геометрия бандла,
`screens-compose`, кадры заметок без ручного `<main>` / `<aside>`. Ворота не
меряли кадр против кадра. Собранный экран при этом чужой.

Замеры 08.09.2026, `/notes` против кадра `s-notes` при 1280 px:

- знак: макет — SVG 56×22 Literata; собрано — `◆` + «Carrel» в Public Sans 13 px
- разделы: макет — 29 px, фон `--card`, stroke-иконки; собрано — 38 px, зелёный
  текст, эмодзи
- источники в рейле: макет — 26 px; собрано — `tc-tickrow` 38 px
- список: макет — без коробки, линовка волосяная; собрано — рамка 1 px и радиус
- рабочая поверхность: макет — `--bg` `#F5F1E8`; собрано — `--tc-surface` и
  padding 20 px (спека L-4.1 уже запрещает и фон, и «отступы на всякий случай»)
- выделение строки: макет — `--card`; собрано — `color-mix` акцента
- панель заметок: макет — 440 px (`--side:440px`); собрано — 400 px
- подвал рейла: макет — квадрат ~29×26 только со значком; собрано — кнопка
  122 px с подписью и `<use href="#+">`
- чипы тегов: макет — под коллекциями; собрано — над ними
- `Rank2` (Import / Export): макет — кнопки в шапке; собрано — сразу в «ещё»
- обвязка демо (нав экранов, Light/Dark, `h1`) сидит над оболочкой, и
  `.tc-shell { min-height: 100vh }` считает её частью продукта

---

## Закрытые развилки

Имена полей, значения и списки ниже — закон плана. Другого входа, другого
символа, другого hex нет.

### Значки

- `Icon` — id символа целиком, как в спрайте: `tc-i-plus`. Не `+`, не `✎`, не
  `plus`, не `href="#+"`. Синонимов нет. Каталог и экраны правятся в том же
  коммите, что спрайт.
- `renderIcon("tc-i-plus")` → `<svg class="tc-icon"><use href="#tc-i-plus">`.
  Пустая строка — нет узла.
- Спрайт библиотеки встраивается в `screens.PageHTML` и в `catalog.PageHTML`
  (тело `typecase.Sprite()`).
- Дефолтный `typecase/icons/sprite.svg` — подача плюс ролевой набор. Уже есть:
  `tc-i-close`, `tc-i-chevron`, `tc-i-chevron-right`, `tc-i-search`,
  `tc-i-more`, `tc-i-check`, `tc-i-minus`, `tc-i-plus`, `tc-i-dot`,
  `tc-i-square`. Добавить (имена = `tc-i-` + имя без `i-` макета): `cols`,
  `qr`, `shield`, `server`, `user`, `theme`, `gear`, `drag`, `device`,
  `archive`, `all`, `cal`, `con`, `task`, `note`, `file`, `dup`, `adm`,
  `refresh`, `print`, `up`, `down`, `back`, `clock`, `link`, `menu`, `mail`,
  `img`, `doc`, `trash`, `pencil`, `move`, `grid`, `panel`, `expand`, `quill`.
  Контуры — из символов макета `i-*`. `i-chev` не дублировать: это
  `tc-i-chevron`. `i-dots` не дублировать: это `tc-i-more`. `i-davloom` в
  спрайт не входит (чужой знак).
- Кнопка только со значком (подпись словом не рисуется; `Label` → `aria-label`
  и `title`): close, plus, search, more, expand, refresh, back, print, panel,
  chevron, server, link, cols, pencil, up, down. Список полный. В подвале
  рейла, если у `Action` есть `Icon`, подпись словом не рисуется всегда: так
  устроен кадр. Спека L-4.1 `tc-railfoot`: квадраты не делят ширину, не
  растягиваются.
- Логотип Carrel в `typecase/icons/` не кладётся.

### Знак и шапка

- Поле `Brand.SVG string`: разметка `<svg>…</svg>`, вставляется как HTML, не
  через `esc`. Если непустое — `Mark` и `Word` не рисуются. Экраны задают
  только `SVG`. `Mark` остаётся символом для клеток каталога без логотипа.
- Символы `#logo-word` и `#logo-shade` — в `typecase/screens`, файл
  `carrel-logo.svg` (контуры из макета), инлайн в `PageHTML` рядом со спрайтом
  библиотеки. `Brand.SVG` ссылается на них через `<use href="#logo-word">` и
  `#logo-shade`. Размер в шапке: 56×22. Заливки: слово `--tc-ink`, лампа
  `--tc-accent`.
- `Top.Refresh Action`: кнопка опроса справа, после Create, до User. `Label` —
  текст времени (`2 min ago`), `Icon` — `tc-i-refresh`. Нулевой `Action` —
  кнопки нет (кадры `/states`, где её нет). Спека L-4.1: `Top{ Brand, Search,
  Create, Refresh, User, Mark }`.
- `Create` остаётся `Menu`, не split-кнопка. Поле `Menu.Kind ButtonKind`; на
  экранах `Kind: ButtonPrimary`, `Icon: "tc-i-quill"`, `Label: "New note"`,
  стрелка виджета как сейчас.
- Поиск: `Placeholder: "Search everything…"`. Ширина по-прежнему тянется
  (L-4.1); 560 px кадра не токен.
- Пользователь: `UserMenu{ Name: "mix" }`, `Note` пустой, без аватара и без
  `Mark`.

### Палитра и кегль

- `--tc-edge`: светлая `#C9BFA6`, тёмная `#3A463D`.
- `--tc-hair`: светлая `#E2DACA`, тёмная `#28332B`.
- `--tc-dusty`: светлая `#6F6856`, тёмная `#99927C`.
- `--tc-danger` не трогать (`#A84232` / `#D5786B`). Слот `--tc-alert` не
  заводить. `#B04A3A` из таблицы макета — не кадр: в `:root` кадра `#A84232`.
- Базовый `--tc-font-size` остаётся 13 px. Нав и заголовок строки списка —
  13.5 px в CSS виджета, не новым токеном.

### Геометрия виджетов

- Нав: высота 29 px; `.is-current` — фон `--tc-card`, цвет `--tc-ink`,
  `font-weight: 600`. Не акцент.
- `tc-tickrow` в рейле: 26 px. Поле `TickRow.Current bool` → класс
  `is-current`, фон `--tc-card`. Combined notebook: `Current: true`.
- `Mark.Colours []string`: больше одного значения — равные горизонтальные
  полосы (как `.m-bar3.is-multi`). Одно значение или пусто — как сейчас
  `Colour`. `Label` обязателен.
- Строка списка: 38 px. `.tc-row.is-selected` — фон `--tc-card`.
- `.tc-list`: `border: none`, без радиуса. Линовка строки — `--tc-hair`.
- `.tc-main`: `background: transparent`, без padding. Gutter у `tc-head`,
  `tc-bar` и `.tc-row`.
- `.tc-head__title`: 22 px Literata, `line-height: 1.15`. Чтобы В-1 не
  вернулся — `padding-block` по замеру ink overflow, не `line-height: 1.5`.
- `Side.Size SideSize`: `""` / `SideSizeDefault` → 400 px; `SideSizeWide`
  (`"wide"`) → 440 px, класс `tc-side--wide`. Заметки (`/notes`) — wide.
  Других размеров нет.
- Чипы рейла после `Sections`, до `Foot`.
- Недоступная строка: слово состояния остаётся (L-4). «—» кадра — это
  `Count == nil` (`tc-num` unknown), не замена слова.

### Rank2, сцена, проба

- `Bar.Render` рисует `Rank2` как `tc-btn` в `.tc-bar__right` до меню «ещё».
  Меню «ещё» на первой отрисовке пустое (или скрыто). `tc.js` переносит
  кнопки Rank2 в stash при нехватке ширины, как пункты `tc-top`. Без JS на
  1280 px Import и Export видны.
- Обвязка демо: `header.screens-chrome` (нав экранов, Light/Dark, `h1`) над
  `.screens-stage`, `flex: none`. Сцена `flex: 1; min-height: 0`.
  `.screens-stage .tc-shell { min-height: 100%; }`, не `100vh`. `html, body`
  — колонка на высоту окна.
- Проба — файл `typecase/browser/screens_look_test.go`, не секция в
  `screens_test.go`. Каждый id из `screens.All()`, 1280×900, светлая тема.
  `/states` — каждый `.screens-frame__mock`. Сначала красная.
- Таблица пробы (±1 px): `.tc-nav__item` высота 29; `.is-current` фон =
  `--tc-card`; `.tc-tickrow` 26; `.tc-main` фон = `--tc-bg` (прозрачность на
  `--tc-bg` родителя); `.tc-list` border-width 0; `.tc-brand svg` есть;
  `/notes` `.tc-side` ширина 440; каждый `use[href^="#"]` имеет `symbol` с
  тем id в документе; в `.tc-nav__icon` нет эмодзи.
- Overflow / contrast / ink в `screens_test.go` не выкидывать. Этот план на
  1280 px; 375 px не расширять.

### Пункт 7

- `/contacts`, `/files`, `/io`, `/duplicates`, `/states` готовы, когда проба
  пункта 6 зелёная. Отдельного «добить кадр» нет.
- Дополнительный чеклист только у заметок (кадры 7.4): `/notes`,
  `/notes-read`, `/notes-edit`, `/notes-quick` — состав ниже. Если проба
  зелёная, а кадра всё равно нет из-за дыры входа — `Э-N` в «Находках» и
  стоп по этому кадру, не `screens.css`.
- Экраны не появляются в этом плане.

---

## Задача

### 1. Спека и токены

- [ ] L-5.1: `--tc-edge`, `--tc-hair`, `--tc-dusty` со значениями из развилок;
      `token.Palette` включает их; `[data-tc-theme="dark"]` задаёт все три
- [ ] L-4.1: `tc-main` прозрачен (спека уже так; ворота геометрии краснеют,
      пока CSS врёт); `Top{ …, Refresh Action, … }`; `Side.Size`;
      `tc-railfoot` — квадраты со значком, не делят ширину
- [ ] L-4.3: нав 29 px, текущий — `--tc-card` + `--tc-ink`; `Brand.SVG`
- [ ] L-4.4: `Rank2` сначала кнопки; список «только значок» — из развилок;
      `Menu.Kind`
- [ ] L-4.5: tickrow в рейле 26 px; `TickRow.Current`; `Mark.Colours`
- [ ] Ворота: тёмная тема переопределяет каждый слот палитры; бандл не
      ссылается на имя вне `token.Required`

### 2. Подача значков

- [ ] `renderIcon` только `tc-i-*`; `Nav.Icon` → `<use>`, не текст
- [ ] Спрайт в `screens.PageHTML` и `catalog.PageHTML`
- [ ] Спрайт дополнен списком из развилок; каталог и экраны переведены на
      `tc-i-*` (в том числе бывший `Icon: "+"`)
- [ ] Ворота: нет эмодзи в `.tc-nav__icon`; каждый `use[href]` имеет `symbol`

### 3. Знак и верхняя панель

- [ ] `Brand.SVG`; `typecase/screens/carrel-logo.svg`; 56×22
- [ ] `Top.Refresh`; поиск «Search everything…»; `Create` primary +
      `tc-i-quill`; пользователь только имя
- [ ] Ворота: в `.tc-brand` есть `svg`; нет текста «◆»

### 4. Рисунок оболочки и списков

- [ ] CSS и разметка по развилкам (main, нав, tickrow, list, selected, rail
      chips, railfoot, edge, dusty, head title)
- [ ] `TickRow.Current`, `Mark.Colours`
- [ ] `gate/geometry.go`: нав 29, tickrow 26, список без border

### 5. `Rank2` и ширина панели

- [ ] `Bar.Render` + `tc.js` по развилкам
- [ ] `Side.Size` / `tc-side--wide`; `/notes` — wide
- [ ] Ворота: `/notes` side 440 px на 1280; Rank2 — кнопки, не только «ещё»

### 6. Сцена демо и ворота рисунка

- [ ] `PageHTML`: `screens-chrome` + сцена, `min-height: 100%` у шелла
- [ ] `typecase/browser/screens_look_test.go`, таблица из развилок, сначала
      красная
- [ ] `/states` — каждый `.screens-frame__mock`

### 7. Сборка заметок

- [ ] `/notes`: подзаголовок панели (источник · время), поля на всю ширину,
      дата и время в ряд, вложения строками, `Mark.Colours` у Combined, теги
      под коллекциями, `Side.Size: SideSizeWide`
- [ ] `/notes-read`: шапка чтения — Edit, Full width / Reading width,
      metacol как в кадре 7.4
- [ ] `/notes-edit`: фокус-правка, нет рейла и панели
- [ ] `/notes-quick`: открытый лист
- [ ] Changelog Typecase: слоты палитры; `Brand.SVG`; `Top.Refresh`;
      `Side.Size`; `Menu.Kind`; `TickRow.Current`; `Mark.Colours`; `Icon`
      только `tc-i-*` (бывший `"+"` → `"tc-i-plus"`)

---

## Что не чинить

- Вертикальных отступов у ячеек таблицы нет намеренно: высота
  `--tc-row-height`.
- Пустые закрытые состояния (`tc-sheet` closed, `tc-dialog` closed,
  `tc-scrim` off, `tc-side` closed) и `tc-num — unknown` невидимы по
  определению.
- «Build information missing» — верное поведение при пустом
  `RailFoot.Build`.
- Вылет `tc-top` / `tc-seg` на 375 px — не этот план.
- Календарь (7.2) и задачи (7.3) — нет маршрутов в `screens.All()`.
- `screens.css` не содержит селекторов `.tc-`.
- Split-кнопка Create, фиксированные 560 px поиска, слот `--tc-alert`,
  третье `--tc-row-height` — отвергнуты выше.

---

## Находки

Пусто. Сюда `Э-N`, когда пункт 7 упирается во вход, которого нет в пунктах
1–6.
