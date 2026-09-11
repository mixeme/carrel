package widget

import "strconv"

// Search — tc-search
//
// Роль. Поле, под которым живут недавние запросы и переходы к известному.
// Вход. Search{ Query, Action, History, Jump, Note, Suggesting }.
// Состояния. idle | suggesting.
// Инварианты. Подсказки под полем; пустой запрос никуда не ведёт.
// Поведение. Открытие подсказок, стрелки, Enter, Esc, крестик очищает.
// Геометрия. Поле тянется; выпадающее не уже поля.
// Содержимое. tc-row--find.
// Чего нет. Не ищет само.
type Search struct {
	Query      string
	Action     string
	History    []SearchRow
	Jump       []JumpRow
	Note       string
	Suggesting bool
}

func (s Search) Render() string {
	cls := "tc-search"
	if s.Suggesting {
		cls = classes(cls, "is-suggesting")
	}
	var history string
	for _, h := range s.History {
		r := Row{Variant: RowFind, Href: h.Href, Cells: RowCells{Title: h.Query, Caption: "Recent"}}
		history += r.Render()
	}
	var jump string
	for _, j := range s.Jump {
		r := Row{Variant: RowFind, Href: j.Href, Cells: RowCells{Title: j.Label, Detail: j.Hint}}
		jump += r.Render()
	}
	pop := ""
	if s.Suggesting {
		pop = `<div class="tc-search__popover" data-tc-search-popover><div class="tc-search__history">` + history + `</div><div class="tc-search__jump">` + jump + `</div><div class="tc-search__all"><button type="submit" class="tc-search__submit">All results</button></div></div>`
	}
	note := ""
	if s.Note != "" {
		note = `<span class="tc-search__note">` + esc(s.Note) + `</span>`
	}
	return `<form` + classAttr(cls) + ` action="` + esc(s.Action) + `" method="get" data-tc-search role="search"><div class="tc-search__field"><input type="search" name="q" class="tc-search__input" value="` + esc(s.Query) + `" aria-label="Search">` + note + `<button type="button" class="tc-search__clear" data-tc-search-clear aria-label="Clear">×</button></div>` + pop + `</form>`
}

// RunBar — tc-runbar
//
// Роль. Ход длинной операции со сводкой и перечнем участников.
// Вход. RunBar{ Summary, At, Running, Percent, Items, Actions }.
// Состояния. участников: waiting | running | done | empty | error | timeout | cancelled | unavailable.
// Инварианты. Индикатор исчезает по завершении; «пусто» и «отказ» разные.
// Поведение. Обновление, отмена, повтор участника.
// Геометрия. Одна строка сводки.
// Содержимое. tc-meter, tc-btn, tc-mark.
// Чего нет. Нет вращающегося кружка без счёта.
type RunBar struct {
	Summary string
	At      string
	Running bool
	Percent *int
	Items   []RunItem
	Actions []Action
}

func (r RunBar) Render() string {
	cls := "tc-runbar"
	if r.Running {
		cls = classes(cls, "is-running")
	}
	meter := ""
	if r.Running && r.Percent != nil {
		meter = Meter{Value: *r.Percent, Label: r.Summary}.Render()
	}
	var items string
	for _, it := range r.Items {
		items += `<div class="tc-runbar__item tc-runbar__item--` + esc(it.State) + `"><span class="tc-runbar__item-label">` + esc(it.Label) + `</span>` + it.Action.Render() + `</div>`
	}
	var acts string
	for _, a := range r.Actions {
		acts += a.Render()
	}
	at := ""
	if r.At != "" {
		at = `<span class="tc-runbar__at">` + esc(r.At) + `</span>`
	}
	return `<div` + classAttr(cls) + ` data-tc-runbar>` + meter + `<div class="tc-runbar__summary">` + esc(r.Summary) + at + `</div><div class="tc-runbar__items">` + items + `</div><div class="tc-runbar__actions">` + acts + `</div></div>`
}

// Diag — tc-diag
//
// Роль. То, что показывают тому, кто станет разбираться.
// Вход. Diag{ Summary, Lines, Steps, Expanded }.
// Состояния. collapsed | expanded.
// Инварианты. Свёрнут по умолчанию; содержимое моноширинное.
// Поведение. Раскрытие, копирование.
// Геометрия. Прокручивается по горизонтали внутри себя.
// Содержимое. нет.
// Чего нет. Нет разбора и приукрашивания.
type Diag struct {
	Summary  string
	Lines    []string
	Steps    []Step
	Expanded bool
}

func (d Diag) Render() string {
	cls := "tc-diag"
	if d.Expanded {
		cls = classes(cls, "is-expanded")
	}
	var lines string
	for _, l := range d.Lines {
		lines += esc(l) + "\n"
	}
	var steps string
	for _, s := range d.Steps {
		steps += `<li class="tc-diag__step tc-diag__step--` + esc(s.Status) + `">` + esc(s.Label) + `</li>`
	}
	return `<details` + classAttr(cls) + ` data-tc-diag` + ariaBool("open", d.Expanded) + `><summary class="tc-diag__summary">` + esc(d.Summary) + `</summary><pre class="tc-diag__body" tabindex="0">` + lines + `</pre><ol class="tc-diag__steps">` + steps + `</ol></details>`
}

// Diff — tc-diff
//
// Роль. Одно расхождение между двумя версиями значения и выбор одного из них.
// Вход. Diff{ Property, Options, Chosen, Agreed }.
// Состояния. open | agreed.
// Инварианты. Варианты столбиком; выбран не более одного.
// Поведение. Выбор варианта, стрелки между вариантами.
// Геометрия. Одна карточка на свойство.
// Содержимое. tc-fields, tc-check.
// Чего нет. Не решает ничего само.
type Diff struct {
	Property string
	Options  []DiffOption
	Chosen   string
	Agreed   bool
}

func (d Diff) Render() string {
	cls := "tc-diff"
	if d.Agreed {
		cls = classes(cls, "is-agreed")
	}
	var opts string
	for _, o := range d.Options {
		chk := Check{Name: d.Property, Label: o.Label, Value: o.Value, Kind: CheckRadio, Checked: o.Value == d.Chosen}
		opts += `<div class="tc-diff__option">` + chk.Render() + `<div class="tc-diff__value">` + esc(o.Label) + `</div></div>`
	}
	return `<div` + classAttr(cls) + ` data-tc-diff><h3 class="tc-diff__property">` + esc(d.Property) + `</h3><div class="tc-diff__options">` + opts + `</div></div>`
}

// Crop — tc-crop
//
// Роль. Выбор квадрата из картинки.
// Вход. Crop{ Href, Zoom, OffsetX, OffsetY, Rotate, Narrow }.
// Состояния. full | centre.
// Инварианты. Рамка показывает результат; числовые поля всегда.
// Поведение. Перетаскивание, ползунок, поворот, пересчёт.
// Геометрия. Квадрат; холст масштабируется.
// Содержимое. tc-btn, tc-in.
// Чего нет. Не режет и не кодирует картинку.
type Crop struct {
	Href    string
	Zoom    int
	OffsetX int
	OffsetY int
	Rotate  int
	Narrow  bool
}

func (c Crop) Render() string {
	cls := "tc-crop"
	if c.Narrow {
		cls = classes(cls, "tc-crop--centre")
	}
	return `<div` + classAttr(cls) + ` data-tc-crop><div class="tc-crop__canvas"><img src="` + esc(c.Href) + `" alt="" class="tc-crop__img"` + attr("data-tc-zoom", itoa(c.Zoom)) + attr("data-tc-rotate", itoa(c.Rotate)) + attr("data-tc-offset-x", itoa(c.OffsetX)) + attr("data-tc-offset-y", itoa(c.OffsetY)) + `><div class="tc-crop__frame" aria-hidden="true"></div></div><div class="tc-crop__controls">` + Input{Name: "zoom", Kind: InputNumber, Value: itoa(c.Zoom)}.Render() + Input{Name: "offsetX", Kind: InputNumber, Value: itoa(c.OffsetX)}.Render() + Input{Name: "offsetY", Kind: InputNumber, Value: itoa(c.OffsetY)}.Render() + Button{Label: "Rotate 90°"}.Render() + `</div></div>`
}

// Steps — tc-steps
//
// Роль. Показ того, сколько шагов и на каком мы.
// Вход. Steps{ Items, Current }.
// Состояния. нет.
// Инварианты. Текущий ровно один; пройденные не кликаются без объявления.
// Поведение. нет.
// Геометрия. Одна строка; в узкой — «шаг N из M».
// Содержимое. нет.
// Чего нет. Нет проверки порядка.
type Steps struct {
	Items   []string
	Current int
}

func (s Steps) Render() string {
	var items string
	for i, label := range s.Items {
		cls := "tc-steps__item"
		if i == s.Current {
			cls = classes(cls, "is-current")
		} else if i < s.Current {
			cls = classes(cls, "is-done")
		}
		items += `<span` + classAttr(cls) + `>` + esc(label) + `</span>`
	}
	return `<div class="tc-steps" aria-label="Progress" data-tc-steps>` + items + `<span class="tc-steps__count">Step ` + esc(itoa(s.Current+1)) + ` of ` + esc(itoa(len(s.Items))) + `</span></div>`
}

// Card — tc-card
//
// Роль. Блок с рамкой.
// Вход. Card{ Rubric, Body }.
// Состояния. нет.
// Инварианты. Пары выравниваются по колонке подписей.
// Поведение. нет.
// Геометрия. Ширина колонки подписей по самой длинной.
// Содержимое. Любые виджеты.
// Чего нет. Нет теней.
type Card struct {
	Rubric Rubric
	Body   string
}

func (c Card) Render() string {
	return `<div class="tc-card">` + c.Rubric.Render() + `<div class="tc-card__body">` + c.Body + `</div></div>`
}

// Sec — tc-sec
//
// Роль. Именованный подраздел внутри блока.
// Вход. Sec{ Rubric, Body }.
// Состояния. нет.
// Инварианты. Как tc-card без рамки.
// Поведение. нет.
// Геометрия. Как tc-card.
// Содержимое. Любые виджеты.
// Чего нет. Нет теней.
type Sec struct {
	Rubric Rubric
	Body   string
}

func (s Sec) Render() string {
	return `<section class="tc-sec">` + s.Rubric.Render() + `<div class="tc-sec__body">` + s.Body + `</div></section>`
}

// Fields — tc-fields
//
// Роль. Сетка «подпись — значение».
// Вход. Fields{ Rows }.
// Состояния. нет.
// Инварианты. Пары выравниваются; значение-путь обрезается с середины.
// Поведение. нет.
// Геометрия. Ширина колонки подписей по самой длинной.
// Содержимое. нет.
// Чего нет. Нет теней.
type Fields struct {
	Rows []FieldRow
}

func (f Fields) Render() string {
	var rows string
	for _, r := range f.Rows {
		rows += FieldRowHTML(r)
	}
	return `<dl class="tc-fields">` + rows + `</dl>`
}

func FieldRowHTML(r FieldRow) string {
	valCls := "tc-fields__value"
	if r.Path {
		valCls = classes(valCls, "tc-fields__value--path")
	}
	return `<div class="tc-fields__row"><dt class="tc-fields__label">` + esc(r.Label) + `</dt><dd` + classAttr(valCls) + `>` + esc(r.Value) + `</dd></div>`
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
