package widget

// Bar — tc-bar
//
// Роль. Строка кнопок и переключателей над содержимым или в панели.
// Вход. Bar{ Left []Node, Right []Node, Rank2, Separators, Collapsed, MoreLabel }.
// Состояния. full | collapsed.
// Инварианты. Одна строка при любой ширине; лишнее под «ещё» по рангу.
// Поведение. Пересчёт при изменении ширины, «ещё» как tc-menu.
// Геометрия. Высота 26px.
// Содержимое. tc-btn, tc-seg, tc-menu, tc-range, tc-in, tc-num, разделители.
// Чего нет. Нет второй строки и горизонтальной прокрутки.
type Bar struct {
	Left       []Node
	Right      []Node
	Rank2      []Action
	Separators bool
	Collapsed  bool
	MoreLabel  string
}

func (b Bar) Render() string {
	cls := "tc-bar"
	if b.Collapsed {
		cls = classes(cls, "is-collapsed")
	}
	left := renderNodes(b.Left)
	right := renderNodes(b.Right)
	sep := ""
	if b.Separators {
		sep = `<span class="tc-bar__sep" aria-hidden="true"></span>`
	}
	more := ""
	if len(b.Rank2) > 0 {
		label := b.MoreLabel
		if label == "" {
			label = labels.More
		}
		var items []MenuItem
		for _, a := range b.Rank2 {
			items = append(items, MenuItem{Label: a.Label, Icon: a.Icon, Href: a.Href})
		}
		more = Menu{Label: label, Items: items}.Render()
	}
	moreLabel := b.MoreLabel
	if moreLabel == "" {
		moreLabel = labels.More
	}
	return `<div` + classAttr(cls) + ` data-tc-bar` + attr("data-tc-more", moreLabel) + `><div class="tc-bar__left">` + left + `</div>` + sep + `<div class="tc-bar__right">` + right + more + `</div></div>`
}

// Seg — tc-seg
//
// Роль. Выбор одного из нескольких равноправных значений.
// Вход. Seg{ Options, Chosen, Name }.
// Состояния. нет; выбранное — одно из значений.
// Инварианты. Выбран ровно один; при нехватке ширины становится меню.
// Поведение. Стрелки влево и вправо, Home, End.
// Геометрия. Высота строки, в которой стоит.
// Содержимое. Подписи и числа.
// Чего нет. Нет множественного выбора.
type Seg struct {
	Options []SegOption
	Chosen  string
	Name    string
}

func (s Seg) Render() string {
	var opts string
	for _, o := range s.Options {
		cls := "tc-seg__opt"
		if o.Value == s.Chosen {
			cls = classes(cls, "is-chosen")
		}
		opts += `<button type="button" class="` + cls + `" data-tc-seg` + attr("data-value", o.Value) + ` name="` + esc(s.Name) + `"` + ariaBool("aria-pressed", o.Value == s.Chosen) + `>` + esc(o.Label) + `</button>`
	}
	return `<div class="tc-seg" role="group" data-tc-seg` + attr("aria-label", s.Name) + `>` + opts + `</div>`
}

// Chip — tc-chip
//
// Роль. Значение, которое включают и выключают.
// Вход. Chip{ Label, Href, On, Count }.
// Состояния. on | off.
// Инварианты. Включённость видна не только цветом.
// Поведение. Нажатие, Enter, пробел.
// Геометрия. Одна строка, длинная подпись обрезается многоточием.
// Содержимое. tc-num.
// Чего нет. Нет крестика удаления.
type Chip struct {
	Label string
	Href  string
	On    bool
	Count *int
}

func (c Chip) Render() string {
	cls := "tc-chip"
	if c.On {
		cls = classes(cls, "is-on")
	}
	inner := `<span class="tc-chip__label">` + esc(c.Label) + `</span>` + Num{Value: c.Count}.Render()
	if c.Href != "" {
		return `<a href="` + esc(c.Href) + `"` + classAttr(cls) + `>` + inner + `</a>`
	}
	return `<button type="button"` + classAttr(cls) + attr("aria-label", c.Label) + `>` + inner + `</button>`
}

// Range — tc-range
//
// Роль. Две даты и готовые варианты.
// Вход. Range{ From, To, Presets, NameFrom, NameTo }.
// Состояния. нет.
// Инварианты. Конец не раньше начала; ширина полей по содержимому.
// Поведение. Правка полей, выбор готового варианта, проверка порядка.
// Геометрия. Ширина поля даты фиксирована.
// Содержимое. tc-in, tc-btn.
// Чего нет. Нет сетки месяца.
type Range struct {
	From     string
	To       string
	Presets  []Action
	NameFrom string
	NameTo   string
}

func (r Range) Render() string {
	from := r.NameFrom
	if from == "" {
		from = "from"
	}
	to := r.NameTo
	if to == "" {
		to = "to"
	}
	var presets string
	for _, p := range r.Presets {
		presets += p.Render()
	}
	return `<div class="tc-range" data-tc-range><div class="tc-range__fields">` + Input{Name: from, Label: labels.From, Kind: InputDate, Value: r.From}.Render() + `<span class="tc-range__dash" aria-hidden="true">–</span>` + Input{Name: to, Label: labels.To, Kind: InputDate, Value: r.To}.Render() + `</div><div class="tc-range__presets">` + presets + `</div></div>`
}

func renderActions(actions []Action) string {
	var out string
	for _, a := range actions {
		aa := a
		if aa.Size == "" {
			aa.Size = ButtonSizeBar
		}
		out += aa.Render()
	}
	return out
}
