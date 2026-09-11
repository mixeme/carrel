package widget

// List — tc-list
//
// Роль. Ряд строк, разделённых линовкой, с рубрикаторами между группами.
// Вход. List{ Groups, Density, Empty }.
// Состояния. normal | empty.
// Инварианты. Пустой список показывает действие; плотность от оболочки.
// Поведение. Ходьба стрелками по строкам, Enter открывает выделенную.
// Геометрия. Высота строки 22px / 52px.
// Содержимое. tc-rubric, строки tc-row.
// Чего нет. Не сортирует и не фильтрует.
type List struct {
	Groups  []ListGroup
	Density Density
	Empty   Empty
}

func (l List) Render() string {
	if len(l.Groups) == 0 {
		return `<div class="tc-list tc-list--empty">` + l.Empty.Render() + `</div>`
	}
	dens := l.Density
	if dens == "" {
		dens = DensityNormal
	}
	var groups string
	for _, g := range l.Groups {
		var rows string
		for _, r := range g.Rows {
			rows += r.Render()
		}
		groups += g.Rubric.Render() + rows
	}
	return `<div` + classAttr("tc-list", "tc-list--"+string(dens)) + ` role="list" data-tc-list>` + groups + `</div>`
}

// Row — tc-row
//
// Роль. Одна запись в списке: ведущая метка, содержимое, хвост.
// Вход. Row{ Href, Mark, Cells, Badges, Menu, Selected, State, Variant }.
// Состояния. normal | selected | muted | alert.
// Инварианты. Метка — полоса слева; меню у своей строки; хвост прижат вправо.
// Поведение. Наведение, выделение, меню, Enter.
// Геометрия. Одна строка / две в узкой раскладке.
// Содержимое. Ячейки, tc-badge, tc-menu.
// Чего нет. Нет перетаскивания и вложенных строк.
type Row struct {
	Href     string
	Mark     Mark
	Cells    RowCells
	Badges   []Badge
	Menu     []MenuItem
	Selected bool
	State    RowState
	Variant  RowVariant
}

func (r Row) Render() string {
	cls := classes("tc-row", rowVariantClass(r.Variant))
	if r.Selected || r.State == RowSelected {
		cls = classes(cls, "is-selected")
	}
	switch r.State {
	case RowMuted:
		cls = classes(cls, "is-muted")
	case RowAlert:
		cls = classes(cls, "is-alert")
	}
	mark := r.Mark.Render()
	cells := renderRowCells(r.Variant, r.Cells)
	var badges string
	for _, b := range r.Badges {
		badges += b.Render()
	}
	menu := ""
	if len(r.Menu) > 0 {
		menu = Menu{Label: labels.Actions, Items: r.Menu}.Render()
	}
	inner := mark + `<div class="tc-row__cells">` + cells + `</div><div class="tc-row__badges">` + badges + `</div><div class="tc-row__tail">` + menu + `</div>`
	if r.Href != "" {
		return `<a href="` + esc(r.Href) + `"` + classAttr(cls) + ` role="listitem" data-tc-row>` + inner + `</a>`
	}
	return `<div` + classAttr(cls) + ` role="listitem" data-tc-row>` + inner + `</div>`
}

func rowVariantClass(v RowVariant) string {
	if v == "" || v == RowDefault {
		return ""
	}
	return "tc-row--" + string(v)
}

func renderRowCells(v RowVariant, c RowCells) string {
	switch v {
	case RowAvatar:
		av := c.Avatar
		if av.Initials == "" && av.Image == "" {
			av = Avatar{Initials: initials(c.Title)}
		}
		return av.Render() + `<div class="tc-row__text"><span class="tc-row__title">` + esc(c.Title) + `</span><span class="tc-row__subtitle">` + esc(c.Subtitle) + `</span></div>`
	case RowLead:
		return `<span class="tc-row__lead">` + esc(c.Lead) + `</span><div class="tc-row__text"><span class="tc-row__title">` + esc(c.Title) + `</span><span class="tc-row__subtitle">` + esc(c.Subtitle) + `</span></div>`
	case RowTick:
		tick := Tick{Done: c.Done, Label: c.Title}.Render()
		return tick + `<span class="tc-row__title">` + esc(c.Title) + `</span><span class="tc-row__due">` + esc(c.Due) + `</span>`
	case RowExcerpt:
		var tags string
		for _, t := range c.Tags {
			tags += t.Render()
		}
		return `<span class="tc-row__title">` + esc(c.Title) + `</span><span class="tc-row__excerpt">` + esc(c.Excerpt) + `</span><span class="tc-row__tags">` + tags + `</span>`
	case RowMeta:
		return `<span class="tc-row__icon" aria-hidden="true">` + esc(c.Icon) + `</span><span class="tc-row__name">` + esc(c.Name) + `</span><span class="tc-row__size">` + esc(c.Size) + `</span><span class="tc-row__time">` + esc(c.Time) + `</span>`
	case RowFind:
		return `<span class="tc-row__icon" aria-hidden="true">` + esc(c.Icon) + `</span><div class="tc-row__find"><span class="tc-row__title">` + esc(c.Title) + `</span><span class="tc-row__detail">` + esc(c.Detail) + `</span><span class="tc-row__caption">` + esc(c.Caption) + `</span></div>`
	default:
		return `<span class="tc-row__title">` + esc(c.Title) + `</span><span class="tc-row__subtitle">` + esc(c.Subtitle) + `</span>`
	}
}

// TickRow — tc-tickrow
//
// Роль. Отмечаемая строка: галка, метка, подпись, счётчик, только чтение.
// Вход. TickRow{ Href, Label, Mark, Count, Tick, ReadOnly, State, Menu }.
// Состояния. ok | read-only | unavailable | gone; галка on | off | partial.
// Инварианты. Три состояния галки — одно поле; счётчик пуст при отсутствии.
// Поведение. Галка и подпись — разные зоны.
// Геометрия. Зона галки 44px в узкой раскладке.
// Содержимое. tc-mark, tc-num, tc-menu.
// Чего нет. Не хранит состояние.
type TickRow struct {
	Href     string
	Label    string
	Mark     Mark
	Count    *int
	Tick     TickState
	ReadOnly bool
	State    TickRowState
	Menu     []MenuItem
}

func (t TickRow) Render() string {
	cls := "tc-tickrow"
	switch t.State {
	case TickRowReadOnly:
		cls = classes(cls, "is-read-only")
	case TickRowUnavailable:
		cls = classes(cls, "is-unavailable")
	case TickRowGone:
		cls = classes(cls, "is-gone")
	}
	tickCls := "tc-tickrow__tick"
	switch t.Tick {
	case TickOn:
		tickCls = classes(tickCls, "is-on")
	case TickPartial:
		tickCls = classes(tickCls, "is-partial")
	case TickDisabled:
		tickCls = classes(tickCls, "is-disabled")
	}
	menu := ""
	if len(t.Menu) > 0 {
		menu = Menu{Label: labels.Actions, Items: t.Menu}.Render()
	}
	stateWord := ""
	if t.State == TickRowUnavailable {
		stateWord = `<span class="tc-tickrow__state">` + esc(labels.Unavailable) + `</span>`
	}
	inner := `<button type="button" class="` + tickCls + `" aria-label="` + esc(labels.Toggle) + ` ` + esc(t.Label) + `"></button>` + t.Mark.Render() + `<span class="tc-tickrow__label">` + esc(t.Label) + `</span>` + stateWord + Num{Value: t.Count}.Render() + menu
	if t.Href != "" {
		return `<div` + classAttr(cls) + ` data-tc-tickrow><a href="` + esc(t.Href) + `" class="tc-tickrow__link">` + inner + `</a></div>`
	}
	return `<div` + classAttr(cls) + ` data-tc-tickrow>` + inner + `</div>`
}

// Rubric — tc-rubric
//
// Роль. Мелкая надпись с разрядкой, открывающая группу.
// Вход. Rubric{ Text, Count }.
// Состояния. нет.
// Инварианты. Не заголовок раздела и не кликается.
// Поведение. нет.
// Геометрия. Прописные, разрядка, приглушённый цвет.
// Содержимое. tc-num.
// Чего нет. Нет уровней.
type Rubric struct {
	Text  string
	Count *int
}

func (r Rubric) Render() string {
	return `<div class="tc-rubric"><span class="tc-rubric__text">` + esc(r.Text) + `</span>` + Num{Value: r.Count}.Render() + `</div>`
}
