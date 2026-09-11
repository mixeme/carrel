package widget

// Scrim — tc-scrim
//
// Роль. Слой между страницей и тем, что лежит поверх.
// Вход. Scrim{ For, On, MustAnswer }.
// Состояния. on | off.
// Инварианты. Принадлежит одному наложению; нажатие закрывает, кроме MustAnswer.
// Поведение. Нажатие, Esc.
// Геометрия. Всё окно.
// Содержимое. нет.
// Чего нет. Нет анимации дольше мгновения.
type Scrim struct {
	For        string
	On         bool
	MustAnswer bool
}

func (s Scrim) Render() string {
	cls := "tc-scrim"
	if s.On {
		cls = classes(cls, "is-on")
	}
	data := ` data-tc-scrim` + attr("data-tc-for", s.For)
	if s.MustAnswer {
		data += ` data-tc-must-answer="true"`
	}
	hidden := ""
	if !s.On {
		hidden = ` hidden`
	}
	return `<div` + classAttr(cls) + data + hidden + ` aria-hidden="true"></div>`
}

// Overlay — tc-overlay
//
// Роль. Общий контейнер для всего, что ложится поверх.
// Вход. Overlay{ Kind, Anchor, Open, ID, Body }.
// Состояния. sheet | dialog | popover.
// Инварианты. Порядок слоёв принадлежит библиотеке; всплывающее привязано к контролу.
// Поведение. Позиционирование, переворот, сдвиг внутрь окна.
// Геометрия. Считается по прямоугольнику контрола и окна.
// Содержимое. tc-sheet, tc-dialog, tc-menu.
// Чего нет. Нет наложения без хозяина.
type Overlay struct {
	Kind   OverlayKind
	Anchor string
	Open   bool
	ID     string
	Body   Prose
}

func (o Overlay) Render() string {
	cls := classes("tc-overlay", "tc-overlay--"+string(o.Kind))
	if o.Open {
		cls = classes(cls, "is-open")
	}
	hidden := ""
	if !o.Open {
		hidden = ` hidden`
	}
	return `<div` + classAttr(cls) + ` data-tc-overlay` + attr("data-tc-anchor", o.Anchor) + attr("id", o.ID) + ` role="dialog" aria-modal="true"` + hidden + `>` + o.Body.Render() + `</div>`
}

// Sheet — tc-sheet
//
// Роль. Короткое взаимодействие, после которого возвращаются на тот же экран.
// Вход. Sheet{ Title, Target, Body, Actions, MustAnswer, Open, ID }.
// Состояния. open | closed.
// Инварианты. Экран под листом остаётся; Target обязателен; фокус заперт.
// Поведение. Открытие, Esc, нажатие мимо, запертый фокус.
// Геометрия. Ширина по содержимому; тело прокручивается.
// Содержимое. tc-form, tc-list, tc-tree, tc-btn.
// Чего нет. Нет вложенных листов.
type Sheet struct {
	Title      string
	Target     string
	Body       Prose
	Actions    []Action
	MustAnswer bool
	Open       bool
	ID         string
}

func (s Sheet) Render() string {
	cls := "tc-sheet"
	if s.Open {
		cls = classes(cls, "is-open")
	}
	var acts string
	for _, a := range s.Actions {
		acts += a.Render()
	}
	id := s.ID
	if id == "" {
		id = uniqueID("tc-sheet")
	}
	scrim := Scrim{For: id, On: s.Open, MustAnswer: s.MustAnswer}.Render()
	body := `<header class="tc-sheet__head"><div class="tc-sheet__target">` + esc(s.Target) + `</div><h2 class="tc-sheet__title">` + esc(s.Title) + `</h2>` + Close{For: id, Label: labels.Close}.Render() + `</header><div class="tc-sheet__body">` + s.Body.Render() + `</div><footer class="tc-sheet__foot">` + acts + `</footer>`
	return scrim + renderOverlay(OverlaySheet, s.Open, id, "", `<div`+classAttr(cls)+` data-tc-trap>`+body+`</div>`)
}

// Dialog — tc-dialog
//
// Роль. Вопрос, на который нельзя не ответить.
// Вход. Dialog{ Title, Text, Confirm, Cancel, Danger, Typed, TypedLabel, Open, ID }.
// Состояния. open | closed.
// Инварианты. Опасное подтверждение называет последствие; барьер набором по флагу.
// Поведение. Esc, нажатие мимо, запертый фокус, начальный фокус на безопасном действии.
// Геометрия. Узкий и широкий вариант.
// Содержимое. Текст, tc-in, tc-btn.
// Чего нет. Нет третьей кнопки.
type Dialog struct {
	Title      string
	Text       string
	Confirm    Action
	Cancel     Action
	Danger     bool
	Typed      bool
	TypedLabel string
	Open       bool
	ID         string
}

func (d Dialog) Render() string {
	cls := classes("tc-dialog", "tc-dialog--narrow")
	if d.Danger {
		cls = classes(cls, "tc-dialog--danger")
	}
	if d.Open {
		cls = classes(cls, "is-open")
	}
	id := d.ID
	if id == "" {
		id = uniqueID("tc-dialog")
	}
	confirm := d.Confirm
	if d.Danger && confirm.Kind == "" {
		confirm.Kind = ButtonDanger
	}
	cancel := d.Cancel
	if cancel.Label == "" {
		cancel.Label = labels.Cancel
	}
	typed := ""
	if d.Typed {
		typed = `<div class="tc-dialog__typed">` + Input{Name: "confirm", Kind: InputText, Placeholder: d.TypedLabel}.Render() + `</div>`
	}
	body := `<header class="tc-dialog__head"><h2 class="tc-dialog__title">` + esc(d.Title) + `</h2>` + Close{For: id, Label: labels.Close}.Render() + `</header><div class="tc-dialog__body"><p>` + esc(d.Text) + `</p>` + typed + `</div><footer class="tc-dialog__foot">` + cancel.Render() + confirm.Render() + `</footer>`
	scrim := Scrim{For: id, On: d.Open}.Render()
	return scrim + renderOverlay(OverlayDialog, d.Open, id, "", `<div`+classAttr(cls)+` data-tc-trap>`+body+`</div>`)
}

// Menu — tc-menu
//
// Роль. Список действий, открывающийся у своего контрола.
// Вход. Menu{ Label, Icon, Items, Align, State, ID }.
// Состояния. open | closed | disabled.
// Инварианты. Стрелка есть всегда; всплывающее под контролом; Esc и мимо закрывают.
// Поведение. Открытие, стрелки, Home/End, первая буква, Esc, мимо.
// Геометрия. Ширина всплывающего не меньше контрола.
// Содержимое. Пункты и разделители.
// Чего нет. Нет вложенности.
type Menu struct {
	Label string
	Icon  string
	Items []MenuItem
	Align MenuAlign
	State MenuState
	ID    string
}

func (m Menu) Render() string {
	if m.Label == "" && m.Icon == "" && len(m.Items) == 0 {
		return ""
	}
	id := m.ID
	if id == "" {
		id = uniqueID("tc-menu")
	}
	cls := "tc-menu"
	switch m.State {
	case MenuOpen:
		cls = classes(cls, "is-open")
	case MenuDisabled:
		cls = classes(cls, "is-disabled")
	default:
		cls = classes(cls, "is-closed")
	}
	align := m.Align
	if align == "" {
		align = MenuAlignStart
	}
	dis := ""
	if m.State == MenuDisabled {
		dis = " disabled"
	}
	exp := "false"
	if m.State == MenuOpen {
		exp = "true"
	}
	var items string
	for _, it := range m.Items {
		if it.Separator {
			items += `<li class="tc-menu__sep" role="separator"></li>`
			continue
		}
		icls := "tc-menu__item"
		disabled := it.Reason != ""
		if disabled {
			icls = classes(icls, "is-disabled")
		}
		title := ""
		if disabled {
			title = ` title="` + esc(it.Reason) + `"`
		}
		icon := renderIcon(it.Icon)
		items += `<li role="none"><a href="` + esc(it.Href) + `" class="` + icls + `" role="menuitem"` + title + `>` + icon + esc(it.Label) + `</a></li>`
	}
	pop := `<ul class="tc-menu__popover tc-menu__popover--` + string(align) + `" role="menu" data-tc-menu-popover` + attr("id", id+"-popover") + `>` + items + `</ul>`
	btn := `<button type="button" class="tc-menu__control" data-tc-menu` + attr("data-tc-menu-id", id) + ` aria-haspopup="menu" aria-expanded="` + exp + `" aria-controls="` + esc(id) + `-popover"` + dis + `>` + renderIcon(m.Icon) + `<span class="tc-menu__label">` + esc(m.Label) + `</span><span class="tc-menu__arrow" aria-hidden="true"></span></button>`
	return `<div` + classAttr(cls) + attr("id", id) + ` data-tc-overlay-anchor>` + btn + pop + `</div>`
}

// UserMenu — tc-usermenu
//
// Роль. Имя, отметка времени и пункты, относящиеся к вошедшему.
// Вход. UserMenu{ Name, Note, Items, Mark, State, ID }.
// Состояния. open | closed.
// Инварианты. Как tc-menu по открытию и фокусу; признак открытой выдачи виден.
// Поведение. Как tc-menu.
// Геометрия. Прижато к правому краю верхней панели.
// Содержимое. Пункты, tc-badge.
// Чего нет. Нет вложенных меню.
type UserMenu struct {
	Name  string
	Note  string
	Items []MenuItem
	Mark  Badge
	State MenuState
	ID    string
}

func (u UserMenu) Render() string {
	if u.Name == "" && len(u.Items) == 0 {
		return ""
	}
	id := u.ID
	if id == "" {
		id = uniqueID("tc-usermenu")
	}
	cls := "tc-usermenu"
	if u.State == MenuOpen {
		cls = classes(cls, "is-open")
	}
	exp := "false"
	if u.State == MenuOpen {
		exp = "true"
	}
	var items string
	for _, it := range u.Items {
		if it.Separator {
			items += `<li class="tc-usermenu__sep" role="separator"></li>`
			continue
		}
		items += `<li role="none"><a href="` + esc(it.Href) + `" class="tc-usermenu__item" role="menuitem">` + esc(it.Label) + `</a></li>`
	}
	head := `<div class="tc-usermenu__head"><span class="tc-usermenu__name">` + esc(u.Name) + `</span>` + u.Mark.Render() + `<span class="tc-usermenu__note">` + esc(u.Note) + `</span></div>`
	pop := `<ul class="tc-usermenu__popover" role="menu" data-tc-menu-popover` + attr("id", id+"-popover") + `>` + head + items + `</ul>`
	btn := `<button type="button" class="tc-usermenu__control" data-tc-menu` + attr("data-tc-menu-id", id) + ` aria-haspopup="menu" aria-expanded="` + exp + `" aria-controls="` + esc(id) + `-popover"><span class="tc-usermenu__avatar" aria-hidden="true">` + esc(initials(u.Name)) + `</span><span class="tc-usermenu__label">` + esc(u.Name) + `</span><span class="tc-menu__arrow" aria-hidden="true"></span></button>`
	return `<div` + classAttr(cls) + attr("id", id) + ` data-tc-overlay-anchor>` + btn + pop + `</div>`
}

func initials(name string) string {
	if name == "" {
		return "?"
	}
	runes := []rune(name)
	if len(runes) > 0 {
		return string(runes[0])
	}
	return "?"
}

func renderOverlay(kind OverlayKind, open bool, id, anchor, inner string) string {
	cls := classes("tc-overlay", "tc-overlay--"+string(kind))
	if open {
		cls = classes(cls, "is-open")
	}
	hidden := ""
	if !open {
		hidden = ` hidden`
	}
	return `<div` + classAttr(cls) + ` data-tc-overlay` + attr("data-tc-anchor", anchor) + attr("id", id) + ` role="dialog" aria-modal="true"` + hidden + `>` + inner + `</div>`
}
