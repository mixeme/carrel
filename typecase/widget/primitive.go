package widget

import "strconv"

// Button — tc-btn
//
// Роль. Действие: кнопка формы или ссылка, выглядящая как кнопка.
// Вход. Button{ Label, Icon, Href, Method, Kind, Size, Enabled, Reason }.
// Состояния. normal | primary | danger | disabled; по умолчанию normal.
// Инварианты. Выключенная несёт причину; опасная не бывает первичной; подпись не обрезается до неузнаваемости.
// Поведение. Нажатие, Enter и пробел на фокусе, подсказка причины у выключенной.
// Геометрия. Три высоты: form 28px, bar 26px, table 22px; в узкой раскладке зона нажатия не меньше 44px.
// Содержимое. нет.
// Чего нет. Нет кнопки только со значком, кроме узнаваемых вслепую.
type Button struct {
	Label   string
	Icon    string
	Href    string
	Method  string
	Kind    ButtonKind
	Size    ButtonSize
	Enabled bool
	Reason  string
}

func (b Button) Render() string {
	disabled := b.Reason != ""
	cls := classes("tc-btn", "tc-btn--"+string(b.sizeOrDefault()), btnKindClass(b.Kind, !disabled))
	if disabled {
		cls = classes(cls, "is-disabled")
	}
	var title string
	if disabled && b.Reason != "" {
		title = ` title="` + esc(b.Reason) + `"`
	}
	icon := renderIcon(b.Icon)
	inner := icon + `<span class="tc-btn__label">` + esc(b.Label) + `</span>`
	if b.Href != "" && !disabled {
		method := ""
		if b.Method != "" && b.Method != "GET" {
			method = attr("data-tc-method", b.Method)
		}
		return `<a href="` + esc(b.Href) + `"` + classAttr(cls) + title + method + `>` + inner + `</a>`
	}
	dis := ""
	if disabled {
		dis = " disabled"
	}
	return `<button type="button"` + classAttr(cls) + title + dis + `>` + inner + `</button>`
}

func (b Button) sizeOrDefault() ButtonSize {
	if b.Size == "" {
		return ButtonSizeForm
	}
	return b.Size
}

func btnKindClass(k ButtonKind, enabled bool) string {
	if !enabled {
		return "tc-btn--disabled"
	}
	switch k {
	case ButtonPrimary:
		return "tc-btn--primary"
	case ButtonDanger:
		return "tc-btn--danger"
	default:
		return "tc-btn--normal"
	}
}

// Input — tc-in
//
// Роль. Строка, число, дата, время, выбор из списка, многострочный текст.
// Вход. Input{ Name, Kind, Value, Options, Placeholder, ReadOnly }.
// Состояния. по роду ввода.
// Инварианты. Подсказка не заменяет подписи; у select стрелка — часть виджета.
// Поведение. Ввод, выбор, Esc возвращает исходное значение до отправки.
// Геометрия. Ширина от рода; textarea тянется по высоте до предела.
// Содержимое. нет.
// Чего нет. Нет масок ввода и автодополнения из чужих источников.
type Input struct {
	Name        string
	Kind        InputKind
	Value       string
	Options     []SelectOption
	Placeholder string
	ReadOnly    bool
}

func (in Input) Render() string {
	kind := in.Kind
	if kind == "" {
		kind = InputText
	}
	ro := boolAttr("readonly", in.ReadOnly)
	ph := attr("placeholder", in.Placeholder)
	name := attr("name", in.Name)
	id := attr("id", in.Name)
	switch kind {
	case InputTextarea:
		return `<textarea` + classAttr("tc-in", "tc-in--textarea", "tc-in--"+string(kind)) + name + id + ph + ro + `>` + esc(in.Value) + `</textarea>`
	case InputSelect:
		var opts string
		for _, o := range in.Options {
			sel := ""
			if o.Selected {
				sel = " selected"
			}
			opts += `<option value="` + esc(o.Value) + `"` + sel + `>` + esc(o.Label) + `</option>`
		}
		return `<div class="tc-in-wrap tc-in-wrap--select"><select` + classAttr("tc-in", "tc-in--select") + name + id + ro + `>` + opts + `</select><span class="tc-in__arrow" aria-hidden="true"></span></div>`
	default:
		return `<input type="` + esc(string(kind)) + `"` + classAttr("tc-in", "tc-in--"+string(kind)) + name + id + attr("value", in.Value) + ph + ro + `>`
	}
}

// Check — tc-check
//
// Роль. Флажок или радиокнопка с подписью.
// Вход. Check{ Name, Label, Value, Kind, Checked, Hint, Disabled, Reason }.
// Состояния. on | off | disabled.
// Инварианты. Подпись — часть зоны нажатия; выключенный несёт причину.
// Поведение. Нажатие, пробел; для радиогруппы — стрелки.
// Геометрия. Зона нажатия 44px в узкой раскладке.
// Содержимое. нет.
// Чего нет. Нет промежуточного состояния.
type Check struct {
	Name     string
	Label    string
	Value    string
	Kind     CheckKind
	Checked  bool
	Hint     string
	Disabled bool
	Reason   string
}

func (c Check) Render() string {
	kind := c.Kind
	if kind == "" {
		kind = CheckCheckbox
	}
	cls := classes("tc-check", "tc-check--"+string(kind))
	if c.Disabled {
		cls = classes(cls, "is-disabled")
	}
	title := ""
	if c.Disabled && c.Reason != "" {
		title = ` title="` + esc(c.Reason) + `"`
	}
	chk := ""
	if c.Checked {
		chk = " checked"
	}
	dis := ""
	if c.Disabled {
		dis = " disabled"
	}
	hint := ""
	if c.Hint != "" {
		hint = `<span class="tc-check__hint">` + esc(c.Hint) + `</span>`
	}
	return `<label` + classAttr(cls) + title + `><input type="` + esc(string(kind)) + `"` + attr("name", c.Name) + attr("value", c.Value) + chk + dis + `><span class="tc-check__box" aria-hidden="true"></span><span class="tc-check__label">` + esc(c.Label) + `</span>` + hint + `</label>`
}

// Kbd — tc-kbd
//
// Роль. Аккорд в подсказке или в таблице клавиш.
// Вход. Kbd{ Keys []string }.
// Состояния. нет.
// Инварианты. Не кликается.
// Поведение. нет.
// Геометрия. Моноширинный, не переносится по строке.
// Содержимое. нет.
// Чего нет. Нет привязки к действию.
type Kbd struct {
	Keys []string
}

func (k Kbd) Render() string {
	var parts string
	for i, key := range k.Keys {
		if i > 0 {
			parts += `<span class="tc-kbd__sep" aria-hidden="true">+</span>`
		}
		parts += `<kbd class="tc-kbd__key">` + esc(key) + `</kbd>`
	}
	return `<span class="tc-kbd">` + parts + `</span>`
}

// Close — tc-close
//
// Роль. Закрыть то, что открыто.
// Вход. Close{ For, Label }.
// Состояния. нет.
// Инварианты. Всегда имеет доступное имя; стоит в шапке наложения.
// Поведение. Нажатие закрывает хозяина, Esc делает то же.
// Геометрия. Зона нажатия не меньше 44px в узкой раскладке.
// Содержимое. нет.
// Чего нет. Нет подписи словом.
type Close struct {
	For   string
	Label string
}

func (c Close) Render() string {
	return `<button type="button" class="tc-close" data-tc-close` + attr("data-tc-for", c.For) + attr("aria-label", c.Label) + `><span class="tc-close__icon" aria-hidden="true">×</span></button>`
}

// Brand — tc-brand
//
// Роль. Марка продукта в шапке и на страницах без оболочки.
// Вход. Brand{ Mark, Word, Href, Size }.
// Состояния. full | markOnly.
// Инварианты. Ниже предела ширины только знак; знак и слово не дублируются.
// Поведение. нет.
// Геометрия. Пропорции фиксированы.
// Содержимое. нет.
// Чего нет. Нет перекраски в произвольный цвет.
type Brand struct {
	Mark     string
	Word     string
	Href     string
	Size     string
	MarkOnly bool
}

func (b Brand) Render() string {
	cls := "tc-brand"
	if b.MarkOnly {
		cls = classes(cls, "tc-brand--mark-only")
	}
	mark := `<span class="tc-brand__mark" aria-hidden="true">` + esc(b.Mark) + `</span>`
	word := `<span class="tc-brand__word">` + esc(b.Word) + `</span>`
	inner := mark + word
	if b.Href != "" {
		return `<a href="` + esc(b.Href) + `"` + classAttr(cls) + `>` + inner + `</a>`
	}
	return `<span` + classAttr(cls) + `>` + inner + `</span>`
}

// Mark — tc-mark
//
// Роль. Узкая вертикальная полоса, обозначающая принадлежность.
// Вход. Mark{ Colour, Label }.
// Состояния. нет.
// Инварианты. Цвет приходит значением; всегда сопровождается доступным именем.
// Поведение. нет.
// Геометрия. 3px по ширине в строках, полная высота строки.
// Содержимое. нет.
// Чего нет. Нет формы кружка и заливки строки.
type Mark struct {
	Colour string
	Label  string
}

func (m Mark) Render() string {
	return `<span class="tc-mark" style="background-color:` + esc(m.Colour) + `"` + attr("aria-label", m.Label) + ` title="` + esc(m.Label) + `"></span>`
}

// Num — tc-num
//
// Роль. Счётчик в строке, в разделе, у чипа.
// Вход. Num{ Value *int }.
// Состояния. known | unknown.
// Инварианты. Неизвестное число не рисуется; ноль вместо неизвестного невыразим.
// Поведение. нет.
// Геометрия. Выровнено по правому краю своей колонки.
// Содержимое. нет.
// Чего нет. Нет форматирования больших чисел сокращением.
type Num struct {
	Value *int
}

func (n Num) Render() string {
	if n.Value == nil {
		return `<span class="tc-num tc-num--unknown" aria-hidden="true"></span>`
	}
	return `<span class="tc-num">` + esc(strconv.Itoa(*n.Value)) + `</span>`
}

// Badge — tc-badge
//
// Роль. Короткая пометка рядом с текстом.
// Вход. Badge{ Label, Role }.
// Состояния. neutral | linked | alert | local | plan.
// Инварианты. Всегда со словом; не кликается.
// Поведение. нет.
// Геометрия. Одна строка, не переносится.
// Содержимое. нет.
// Чего нет. Нет счётчика в бейдже.
type Badge struct {
	Label string
	Role  BadgeRole
}

func (b Badge) Render() string {
	role := b.Role
	if role == "" {
		role = BadgeNeutral
	}
	return `<span` + classAttr("tc-badge", "tc-badge--"+string(role)) + `>` + esc(b.Label) + `</span>`
}

// Avatar — tc-avatar
//
// Роль. Картинка или инициалы в квадрате со скруглением.
// Вход. Avatar{ Href, Initials, Colour, Size, Image }.
// Состояния. image | initials.
// Инварианты. Пустой аватар невозможен; форма квадратная со скруглением.
// Поведение. Не загрузилась картинка — показываются инициалы.
// Геометрия. Два размера: строка списка и карточка.
// Содержимое. нет.
// Чего нет. Нет загрузки и правки.
type Avatar struct {
	Href     string
	Initials string
	Colour   string
	Size     string
	Image    string
}

func (a Avatar) Render() string {
	cls := classes("tc-avatar", "tc-avatar--"+a.sizeOrDefault())
	var inner string
	if a.Image != "" {
		inner = `<img class="tc-avatar__img" src="` + esc(a.Image) + `" alt="">`
		cls = classes(cls, "tc-avatar--image")
	} else if a.Initials != "" {
		inner = `<span class="tc-avatar__initials" style="background-color:` + esc(a.Colour) + `">` + esc(a.Initials) + `</span>`
		cls = classes(cls, "tc-avatar--initials")
	} else {
		inner = `<span class="tc-avatar__empty" aria-hidden="true">?</span>`
		cls = classes(cls, "tc-avatar--initials")
	}
	if a.Href != "" {
		return `<a href="` + esc(a.Href) + `"` + classAttr(cls) + `>` + inner + `</a>`
	}
	return `<span` + classAttr(cls) + `>` + inner + `</span>`
}

func (a Avatar) sizeOrDefault() string {
	if a.Size == "" {
		return "row"
	}
	return a.Size
}

// Tick — tc-tick
//
// Роль. Маленькая галка внутри строки: отметить сделанным.
// Вход. Tick{ Done, Label, Action, Disabled, Reason }.
// Состояния. on | off | disabled.
// Инварианты. Имеет доступное имя; выключена несёт причину.
// Поведение. Нажатие, пробел на фокусе.
// Геометрия. 13px в обычной плотности, зона нажатия 44px в узкой.
// Содержимое. нет.
// Чего нет. Нет промежуточного состояния.
type Tick struct {
	Done     bool
	Label    string
	Action   string
	Disabled bool
	Reason   string
}

func (t Tick) Render() string {
	cls := "tc-tick"
	if t.Done {
		cls = classes(cls, "is-on")
	}
	if t.Disabled {
		cls = classes(cls, "is-disabled")
	}
	title := ""
	if t.Disabled && t.Reason != "" {
		title = ` title="` + esc(t.Reason) + `"`
	}
	dis := ""
	if t.Disabled {
		dis = " disabled"
	}
	return `<button type="button"` + classAttr(cls) + attr("aria-label", t.Label) + attr("data-tc-action", t.Action) + title + dis + `><span class="tc-tick__box" aria-hidden="true"></span></button>`
}

// Tag — tc-tag
//
// Роль. Категория записи в карточке и в строке.
// Вход. Tag{ Label, Href, On }.
// Состояния. on | off.
// Инварианты. Не редактируется на месте.
// Поведение. Нажатие ведёт на фильтр, если задан адрес.
// Геометрия. Переносится по строке группой.
// Содержимое. нет.
// Чего нет. Нет своего цвета.
type Tag struct {
	Label string
	Href  string
	On    bool
}

func (t Tag) Render() string {
	cls := "tc-tag"
	if t.On {
		cls = classes(cls, "is-on")
	}
	if t.Href != "" {
		return `<a href="` + esc(t.Href) + `"` + classAttr(cls) + `>` + esc(t.Label) + `</a>`
	}
	return `<span` + classAttr(cls) + `>` + esc(t.Label) + `</span>`
}

// Msg — tc-msg
//
// Роль. Сообщение над содержимым: уведомление, предупреждение, отказ.
// Вход. Msg{ Role, Title, Body, Details, Actions, Diag }.
// Состояния. info | accent | alert.
// Инварианты. Стоит над содержимым, не поверх; подробности — отдельным блоком.
// Поведение. Раскрытие подробностей, закрытие если задано.
// Геометрия. Во всю ширину своей зоны.
// Содержимое. tc-diag, tc-btn.
// Чего нет. Нет самоисчезновения по таймеру.
type Msg struct {
	Role    MsgRole
	Title   string
	Body    string
	Details string
	Actions []Action
	Diag    Diag
}

func (m Msg) Render() string {
	role := m.Role
	if role == "" {
		role = MsgInfo
	}
	var acts string
	for _, a := range m.Actions {
		acts += a.Render()
	}
	diag := ""
	if m.Diag.Summary != "" || len(m.Diag.Lines) > 0 {
		diag = m.Diag.Render()
	}
	details := ""
	if m.Details != "" {
		details = `<p class="tc-msg__details">` + esc(m.Details) + `</p>`
	}
	return `<div` + classAttr("tc-msg", "tc-msg--"+string(role)) + ` role="status"><div class="tc-msg__title">` + esc(m.Title) + `</div><div class="tc-msg__body">` + esc(m.Body) + `</div>` + details + diag + `<div class="tc-msg__actions">` + acts + `</div></div>`
}

// Empty — tc-empty
//
// Роль. Место, где данных ещё нет.
// Вход. Empty{ Title, Body, Action }.
// Состояния. нет.
// Инварианты. Несёт действие: пустое состояние без кнопки невыразимо.
// Поведение. нет.
// Геометрия. По содержимому.
// Содержимое. tc-btn.
// Чего нет. Нет иллюстраций.
type Empty struct {
	Title  string
	Body   string
	Action Action
}

func (e Empty) Render() string {
	return `<div class="tc-empty"><h3 class="tc-empty__title">` + esc(e.Title) + `</h3><p class="tc-empty__body">` + esc(e.Body) + `</p><div class="tc-empty__action">` + e.Action.Render() + `</div></div>`
}

// Skel — tc-skel
//
// Роль. Место будущего содержимого, пока содержимое едет.
// Вход. Skel{ Shape, Width, Lines }.
// Состояния. нет.
// Инварианты. Занимает ровно то место, которое займёт настоящее содержимое.
// Поведение. Спокойная пульсация; при выключенном движении — статичен.
// Геометрия. По размеру замещаемого элемента.
// Содержимое. нет.
// Чего нет. Не знает, чего ждёт.
type Skel struct {
	Shape SkelShape
	Width string
	Lines int
}

func (s Skel) Render() string {
	shape := s.Shape
	if shape == "" {
		shape = SkelLine
	}
	lines := s.Lines
	if lines < 1 {
		lines = 1
	}
	style := ""
	if s.Width != "" {
		style = ` style="width:` + esc(s.Width) + `"`
	}
	var inner string
	for i := 0; i < lines; i++ {
		inner += `<span class="tc-skel__line"></span>`
	}
	return `<div class="tc-skel tc-skel--` + esc(string(shape)) + `"` + style + ` role="status" aria-label="Loading">` + inner + `</div>`
}

// Meter — tc-meter
//
// Роль. Шкала хода операции.
// Вход. Meter{ Value, Max, Label }.
// Состояния. нет.
// Инварианты. Ход показан числом, а не одним вращением.
// Поведение. нет.
// Геометрия. Одна полоса фиксированной высоты.
// Содержимое. нет.
// Чего нет. Нет вращающегося кружка без счёта.
type Meter struct {
	Value int
	Max   int
	Label string
}

func (m Meter) Render() string {
	max := m.Max
	if max < 1 {
		max = 100
	}
	pct := float64(m.Value) / float64(max) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return `<div class="tc-meter"` + attr("aria-label", m.Label) + ` role="meter" aria-valuenow="` + esc(strconv.Itoa(m.Value)) + `" aria-valuemin="0" aria-valuemax="` + esc(strconv.Itoa(max)) + `"><div class="tc-meter__bar" style="width:` + esc(strconv.FormatFloat(pct, 'f', 1, 64)) + `%"></div><span class="tc-meter__label">` + esc(m.Label) + `</span></div>`
}

func renderIcon(name string) string {
	if name == "" {
		return ""
	}
	return `<svg class="tc-icon tc-btn__icon" aria-hidden="true"><use href="#` + esc(name) + `"></use></svg>`
}
