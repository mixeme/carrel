package widget

// Table — tc-table
//
// Роль. Данные по колонкам: сравнение, предпросмотр, журнал.
// Вход. Table{ Columns, Rows, Selectable, Empty }.
// Состояния. normal | empty.
// Инварианты. Ширина колонки по роду; числовые колонки вправо; прокручивается сама.
// Поведение. нет.
// Геометрия. Высота строки 22px; шапка не прокручивается.
// Содержимое. Ячейки, tc-btn в колонке действий.
// Чего нет. Не сортирует и не подгружает; выделение строк — у потребителя.
type Table struct {
	Columns    []Column
	Rows       []TableRow
	Selectable bool
	Empty      Empty
}

func (t Table) Render() string {
	if len(t.Rows) == 0 {
		return `<div class="tc-table tc-table--empty">` + t.Empty.Render() + `</div>`
	}
	hasActions := false
	for _, r := range t.Rows {
		if len(r.Actions) > 0 {
			hasActions = true
			break
		}
	}
	var head string
	for _, c := range t.Columns {
		head += `<th class="tc-table__th tc-table__th--` + string(c.Kind) + `">` + esc(c.Label) + `</th>`
	}
	if hasActions {
		head += `<th class="tc-table__th tc-table__th--action">` + esc(labels.Actions) + `</th>`
	}
	var body string
	for _, r := range t.Rows {
		cls := "tc-table__tr"
		if r.Selected {
			cls = classes(cls, "is-selected")
		}
		var cells string
		for i, cell := range r.Cells {
			kind := ColText
			if i < len(t.Columns) {
				kind = t.Columns[i].Kind
			}
			cells += `<td class="tc-table__td tc-table__td--` + string(kind) + `">` + esc(cell) + `</td>`
		}
		if hasActions {
			acts := ""
			if len(r.Actions) > 0 {
				for _, a := range r.Actions {
					aa := a
					aa.Size = ButtonSizeTable
					acts += aa.Render()
				}
			}
			cells += `<td class="tc-table__td tc-table__td--action">` + acts + `</td>`
		}
		body += `<tr` + classAttr(cls) + `>` + cells + `</tr>`
	}
	return `<div class="tc-table"><table class="tc-table__grid"><thead><tr>` + head + `</tr></thead><tbody>` + body + `</tbody></table></div>`
}

// Grid — tc-grid
//
// Роль. Обзор того, что само по себе картинка.
// Вход. Grid{ Tiles }.
// Состояния. нет.
// Инварианты. Плитки одного размера; имя не больше двух строк.
// Поведение. нет.
// Геометрия. Размер плитки фиксирован.
// Содержимое. tc-thumb, tc-badge.
// Чего нет. Нет предпросмотра при наведении; выделение и открытие — у потребителя.
type Grid struct {
	Tiles []Tile
}

func (g Grid) Render() string {
	var tiles string
	for _, t := range g.Tiles {
		tiles += t.Render()
	}
	return `<div class="tc-grid">` + tiles + `</div>`
}

// Tile — tc-tile
//
// Роль. Одна плитка в сетке.
// Вход. Tile{ Href, Name, Thumb, Icon, Selected, Badges }.
// Состояния. image | icon.
// Инварианты. Нет миниатюры — значок рода.
// Поведение. Выделение, открытие.
// Геометрия. Фиксированный размер.
// Содержимое. tc-thumb, tc-badge.
// Чего нет. Нет предпросмотра при наведении.
type Tile struct {
	Href     string
	Name     string
	Thumb    Thumb
	Icon     string
	Selected bool
	Badges   []Badge
}

func (t Tile) Render() string {
	cls := "tc-tile"
	if t.Selected {
		cls = classes(cls, "is-selected")
	}
	thumb := t.Thumb
	if thumb.Href == "" && t.Icon != "" {
		thumb = Thumb{Icon: t.Icon}
	}
	var badges string
	for _, b := range t.Badges {
		badges += b.Render()
	}
	inner := thumb.Render() + `<span class="tc-tile__name">` + esc(t.Name) + `</span>` + badges
	if t.Href != "" {
		return `<a href="` + esc(t.Href) + `"` + classAttr(cls) + `>` + inner + `</a>`
	}
	return `<div` + classAttr(cls) + `>` + inner + `</div>`
}

// Thumb — tc-thumb
//
// Роль. Уменьшенное изображение; при отсутствии — значок рода.
// Вход. Thumb{ Href, Icon, Alt, Size, Loading }.
// Состояния. image | icon | loading.
// Инварианты. Место занято всегда; не загрузилось — значок.
// Поведение. Подгрузка по видимости.
// Геометрия. Размер из объявленного набора.
// Содержимое. нет.
// Чего нет. Не увеличивает по наведению.
type Thumb struct {
	Href    string
	Icon    string
	Alt     string
	Size    ThumbSize
	Loading bool
}

func (t Thumb) Render() string {
	size := t.Size
	if size == "" {
		size = ThumbSizeTile
	}
	cls := classes("tc-thumb", "tc-thumb--"+string(size))
	if t.Loading {
		return `<div` + classAttr(cls, "tc-thumb--loading") + ` aria-busy="true"></div>`
	}
	if t.Href != "" {
		return `<div` + classAttr(cls, "tc-thumb--image") + `><img src="` + esc(t.Href) + `" alt="` + esc(t.Alt) + `" class="tc-thumb__img" loading="lazy"></div>`
	}
	return `<div` + classAttr(cls, "tc-thumb--icon") + `><span class="tc-thumb__icon" aria-hidden="true">` + esc(t.Icon) + `</span></div>`
}

// Tree — tc-tree
//
// Роль. Иерархия, в которой выбирают один узел.
// Вход. Tree{ Nodes, Chosen, Disabled }.
// Состояния. normal | disabled.
// Инварианты. Выбран не более одного; недоступный узел виден.
// Поведение. Раскрытие, стрелки, Enter выбирает.
// Геометрия. Одна строка на узел.
// Содержимое. tc-mark, tc-badge.
// Чего нет. Нет множественного выбора.
type Tree struct {
	Nodes    []TreeNode
	Chosen   string
	Disabled []string
}

func (t Tree) Render() string {
	return `<div class="tc-tree" role="tree" data-tc-tree>` + renderTreeNodes(t.Nodes, t.Chosen, t.Disabled, 0) + `</div>`
}

func renderTreeNodes(nodes []TreeNode, chosen string, disabled []string, depth int) string {
	var out string
	for _, n := range nodes {
		dis := contains(disabled, n.ID)
		cls := "tc-tree__node"
		if n.ID == chosen && !dis {
			cls = classes(cls, "is-chosen")
		}
		if dis {
			cls = classes(cls, "is-disabled")
		}
		exp := "false"
		if n.Open {
			exp = "true"
		}
		var badges string
		for _, b := range n.Badges {
			badges += b.Render()
		}
		children := ""
		if len(n.Children) > 0 {
			children = `<div class="tc-tree__children" role="group">` + renderTreeNodes(n.Children, chosen, disabled, depth+1) + `</div>`
		}
		toggle := ""
		if len(n.Children) > 0 {
			toggle = `<button type="button" class="tc-tree__toggle" data-tc-tree-toggle aria-expanded="` + exp + `" aria-label="` + esc(labels.Toggle) + `"></button>`
		} else {
			toggle = `<span class="tc-tree__spacer" aria-hidden="true"></span>`
		}
		btnDis := ""
		if dis {
			btnDis = " disabled"
		}
		out += `<div` + classAttr(cls) + ` role="treeitem" data-tc-tree-item` + attr("data-id", n.ID) + `>` + toggle + n.Mark.Render() + `<button type="button" class="tc-tree__label"` + btnDis + `>` + esc(n.Label) + `</button>` + badges + children + `</div>`
	}
	return out
}

func contains(list []string, id string) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}

// Queue — tc-queue
//
// Роль. Ряд идущих операций с ходом, отменой и вопросами.
// Вход. Queue{ Rows, Note }.
// Состояния. idle | running | done | failed | partial | cancelled.
// Инварианты. Ожидающая строка показывает вопрос; ход — числом.
// Поведение. нет.
// Геометрия. Прижата к нижнему краю рабочей поверхности.
// Содержимое. tc-meter, tc-btn.
// Чего нет. Не хранит очередь между заходами; отмена и ответ — у потребителя.
type Queue struct {
	Rows []QueueRow
	Note string
}

func (q Queue) Render() string {
	var rows string
	for _, r := range q.Rows {
		cls := "tc-queue__row tc-queue__row--" + string(r.State)
		meter := ""
		if r.Percent != nil {
			meter = Meter{Value: *r.Percent, Label: r.Label}.Render()
		}
		question := ""
		if r.Question != "" {
			var choices string
			for _, c := range r.Choices {
				choices += c.Render()
			}
			question = `<div class="tc-queue__question"><p>` + esc(r.Question) + `</p>` + choices + `</div>`
		}
		action := ""
		if r.Action.Label != "" {
			action = r.Action.Render()
		}
		rows += `<div` + classAttr(cls) + `>` + meter + `<span class="tc-queue__label">` + esc(r.Label) + `</span>` + question + action + `</div>`
	}
	note := ""
	if q.Note != "" {
		note = `<p class="tc-queue__note">` + esc(q.Note) + `</p>`
	}
	return `<div class="tc-queue">` + rows + note + `</div>`
}
