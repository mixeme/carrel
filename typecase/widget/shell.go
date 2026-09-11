package widget

// Shell — tc-shell
//
// Роль. Четыре зоны страницы и переходы между раскладками по ширине.
// Вход. Shell{ Top, Rail, Main, Side, Density, Narrow }.
// Состояния. wide | narrow; по ширине окна.
// Инварианты. Пятой зоны нет; раскладка считается виджетом.
// Поведение. Следит за шириной; рейл становится выдвижной панелью.
// Геометрия. Рейл 264/232/56px; side 400px.
// Содержимое. tc-top, tc-rail, tc-main, tc-side.
// Чего нет. Нет своей прокрутки страницы.
type Shell struct {
	Top     Top
	Rail    Rail
	Main    Main
	Side    Side
	Density Density
	Narrow  bool
}

func (s Shell) Render() string {
	dens := s.Density
	if dens == "" {
		dens = DensityNormal
	}
	cls := classes("tc-shell", "tc-shell--"+string(dens))
	if s.Narrow {
		cls = classes(cls, "tc-shell--narrow")
	}
	return `<div` + classAttr(cls) + ` data-tc-shell data-tc-root>` + s.Top.Render() + `<div class="tc-shell__body"><div class="tc-shell__rail">` + s.Rail.Render() + `</div><div class="tc-shell__main">` + s.Main.Render() + `</div><div class="tc-shell__side">` + s.Side.Render() + `</div></div></div>`
}

// Top — tc-top
//
// Роль. Знак, поиск, создание, меню учётной записи. Одна на страницу.
// Вход. Top{ Brand, Nav, Search, Create, User, Mark, Narrow }.
// Состояния. normal | narrow.
// Инварианты. Знак один раз слева; поиск виден всегда.
// Поведение. Отступление правой группы; меню создания и учётной записи.
// Геометрия. Высота фиксирована; поиск тянется.
// Содержимое. tc-brand, tc-search, tc-menu, tc-usermenu, tc-nav.
// Чего нет. Нет второй строки.
type Top struct {
	Brand  Brand
	Nav    Nav
	Search Search
	Create Menu
	User   UserMenu
	Mark   Mark
	Narrow bool
}

func (t Top) Render() string {
	cls := "tc-top"
	if t.Narrow {
		cls = classes(cls, "tc-top--narrow")
	}
	create := ""
	if t.Create.Label != "" || t.Create.Icon != "" || len(t.Create.Items) > 0 {
		create = t.Create.Render()
	}
	user := ""
	if t.User.Name != "" || len(t.User.Items) > 0 {
		user = t.User.Render()
	}
	nav := t.Nav.Render()
	// Поиск рисуется всегда: «поле не убирается параметром» — инвариант tc-top,
	// а не недосмотр. Ширину ему задаёт геометрия панели, а не наличие входа.
	search := t.Search.Render()
	mark := t.Mark.Render()
	return `<header` + classAttr(cls) + `>` + t.Brand.Render() + nav + search + `<div class="tc-top__actions">` + create + user + `</div>` + mark + `</header>`
}

// Main — tc-main
//
// Роль. Всё, что осталось после рейла и панели.
// Вход. Main{ Label, Body []Node }.
// Состояния. нет.
// Инварианты. Ширину не ограничивает.
// Поведение. Своя прокрутка.
// Геометрия. Занимает остаток.
// Содержимое. Любые виджеты.
// Чего нет. Нет отступов «на всякий случай».
type Main struct {
	Label string
	Body  []Node
}

func (m Main) Render() string {
	return `<main class="tc-main"` + attr("aria-label", m.Label) + `><div class="tc-main__body">` + renderNodes(m.Body) + `</div></main>`
}

// Rail — tc-rail
//
// Роль. Сгруппированный список с подвалом действий и фильтром.
// Вход. Rail{ Sections, Chips, Foot, Note, Layout, Drawer, Open }.
// Состояния. column | icons | drawer.
// Инварианты. В узкой раскладке — выдвижная панель; подвал прижат вниз.
// Поведение. Открытие/закрытие панели, Esc, возврат фокуса.
// Геометрия. Ширины из tc-shell.
// Содержимое. tc-railsec, tc-chip, tc-railfoot.
// Чего нет. Не хранит выбор.
type Rail struct {
	Sections []RailSection
	Chips    []Chip
	Foot     RailFoot
	Note     string
	Layout   RailLayout
	Drawer   bool
	Open     bool
}

func (r Rail) Render() string {
	cls := "tc-rail"
	layout := r.Layout
	if layout == "" && r.Drawer {
		layout = RailLayoutDrawer
	}
	if layout == "" {
		layout = RailLayoutColumn
	}
	cls = classes(cls, "tc-rail--"+string(layout))
	if r.Open {
		cls = classes(cls, "is-open")
	}
	var chips string
	for _, c := range r.Chips {
		chips += c.Render()
	}
	if chips != "" {
		// Пустая полоса чипов рисовала линовку и отступ из ничего.
		chips = `<div class="tc-rail__chips">` + chips + `</div>`
	}
	var secs string
	for _, s := range r.Sections {
		secs += s.Render()
	}
	note := ""
	if r.Note != "" {
		note = `<p class="tc-rail__note">` + esc(r.Note) + `</p>`
	}
	return `<aside` + classAttr(cls) + ` data-tc-rail` + ariaBool("aria-expanded", r.Open) + `>` + chips + `<div class="tc-rail__sections">` + secs + `</div>` + note + r.Foot.Render() + `</aside>`
}

// RailSection — tc-railsec
//
// Роль. Заголовок группы и её строки.
// Вход. RailSection{ Title, State, Rows }.
// Состояния. ok | muted | alert.
// Инварианты. Недоступная группа помечена цветом и словом; пустой Title не выпускает заголовок.
// Поведение. нет.
// Геометрия. Заголовок — мелкий кегль с разрядкой.
// Содержимое. tc-tickrow.
// Чего нет. Не сворачивается.
func (rs RailSection) Render() string {
	cls := "tc-railsec"
	if rs.State != "" {
		cls = classes(cls, "tc-railsec--"+string(rs.State))
	}
	var rows string
	for _, r := range rs.Rows {
		rows += r.Render()
	}
	title := ""
	if rs.Title != "" {
		title = `<h3 class="tc-railsec__title">` + esc(rs.Title) + `</h3>`
	}
	return `<section` + classAttr(cls) + `>` + title + `<div class="tc-railsec__rows">` + rows + `</div></section>`
}

// RailFoot — tc-railfoot
//
// Роль. Действия колонки и подпись сборки.
// Вход. RailFoot{ Actions, Build }.
// Состояния. нет.
// Инварианты. Подпись сборки всегда последняя; пустой Build — видимо.
// Поведение. нет.
// Геометрия. Действия одна строка; подпись под ними.
// Содержимое. tc-btn.
// Чего нет. Нет меню.
type RailFoot struct {
	Actions []Action
	Build   Build
}

func (rf RailFoot) Render() string {
	var acts string
	for _, a := range rf.Actions {
		aa := a
		aa.Size = ButtonSizeBar
		acts += aa.Render()
	}
	build := rf.Build.Render()
	return `<footer class="tc-railfoot"><div class="tc-railfoot__actions">` + acts + `</div>` + build + `</footer>`
}

func (b Build) Render() string {
	if b.Name == "" && b.Version == "" {
		return `<p class="tc-railfoot__build tc-railfoot__build--missing" role="alert">` + esc(labels.BuildMissing) + `</p>`
	}
	text := esc(b.Name) + " " + esc(b.Version)
	if b.Commit != "" {
		text += " (" + esc(b.Commit) + ")"
	}
	if b.Href != "" {
		return `<a href="` + esc(b.Href) + `" class="tc-railfoot__build">` + text + `</a>`
	}
	return `<p class="tc-railfoot__build">` + text + `</p>`
}

// Side — tc-side
//
// Роль. Колонка справа, отвечающая на выделение.
// Вход. Side{ Label, Title, Blocks, Actions, Open }.
// Состояния. open | closed.
// Инварианты. Закрывается крестиком, кнопкой и Esc.
// Поведение. Открытие, закрытие, возврат фокуса.
// Геометрия. 400px; содержимое прокручивается.
// Содержимое. tc-sec, tc-fields, tc-field, tc-btn, tc-bar.
// Чего нет. Не редактирует.
type Side struct {
	Label   string
	Title   string
	Blocks  []SideBlock
	Actions []Action
	Open    bool
}

func (s Side) Render() string {
	cls := "tc-side"
	if s.Open {
		cls = classes(cls, "is-open")
	} else {
		cls = classes(cls, "is-closed")
	}
	var blocks string
	for _, b := range s.Blocks {
		blocks += b.Render()
	}
	id := uniqueID("tc-side")
	return `<aside` + classAttr(cls) + ` data-tc-side` + attr("id", id) + attr("aria-label", s.Label) + `><header class="tc-side__head"><h2 class="tc-side__title">` + esc(s.Title) + `</h2>` + Close{For: id, Label: labels.ClosePanel}.Render() + `</header><div class="tc-side__body">` + blocks + `</div><footer class="tc-side__foot">` + Bar{Left: ActionsAsNodes(s.Actions)}.Render() + `</footer></aside>`
}

func (b SideBlock) kindCount() int {
	n := 0
	if b.Sec != nil {
		n++
	}
	if b.Fields != nil {
		n++
	}
	if b.Field != nil {
		n++
	}
	return n
}

// Render emits one side-panel block. Zero kinds render nothing; more than one is invalid.
func (b SideBlock) Render() string {
	switch b.kindCount() {
	case 0:
		return ""
	case 1:
		if b.Sec != nil {
			return b.Sec.Render()
		}
		if b.Fields != nil {
			return b.Fields.Render()
		}
		return b.Field.Render()
	default:
		return ""
	}
}

// Nav — tc-nav
//
// Роль. Стопка ссылок верхнего уровня со значками и подписями.
// Вход. Nav{ Items, Current, Layout }.
// Состояния. column | icons | bottom.
// Инварианты. Отмечен ровно один текущий пункт.
// Поведение. Ходьба клавишами, подсказки в раскладке значков.
// Геометрия. Высота строки фиксирована.
// Содержимое. tc-num.
// Чего нет. Не сворачивается в меню.
type Nav struct {
	Items   []NavItem
	Current string
	Layout  NavLayout
}

func (n Nav) Render() string {
	if len(n.Items) == 0 {
		return ""
	}
	layout := n.Layout
	if layout == "" {
		layout = NavLayoutColumn
	}
	var items string
	for _, it := range n.Items {
		cls := "tc-nav__item"
		if it.Href == n.Current {
			cls = classes(cls, "is-current")
		}
		items += `<a href="` + esc(it.Href) + `"` + classAttr(cls) + `><span class="tc-nav__icon" aria-hidden="true">` + esc(it.Icon) + `</span><span class="tc-nav__label">` + esc(it.Label) + `</span>` + Num{Value: it.Count}.Render() + `</a>`
	}
	return `<nav` + classAttr("tc-nav", "tc-nav--"+string(layout)) + ` aria-label="` + esc(labels.Main) + `">` + items + `</nav>`
}

// Head — tc-head
//
// Роль. Заголовок рабочей поверхности с путём, подзаголовком и действиями.
// Вход. Head{ Crumbs, Title, Sub, Mark, Actions, Rank2 }.
// Состояния. нет.
// Инварианты. Действия справа не наезжают на заголовок.
// Поведение. Отступление действий по рангу.
// Геометрия. Одна строка заголовка и подзаголовка.
// Содержимое. tc-crumbs, tc-btn, tc-menu, tc-mark.
// Чего нет. Нет поиска.
type Head struct {
	Crumbs  Crumbs
	Title   string
	Sub     string
	Mark    Mark
	Actions []Action
	Rank2   []Action
}

func (h Head) Render() string {
	return `<header class="tc-head">` + h.Crumbs.Render() + h.Mark.Render() + `<div class="tc-head__titles"><h1 class="tc-head__title">` + esc(h.Title) + `</h1><p class="tc-head__sub">` + esc(h.Sub) + `</p></div>` + Bar{Right: ActionsAsNodes(h.Actions), Rank2: h.Rank2}.Render() + `</header>`
}

// Crumbs — tc-crumbs
//
// Роль. Цепочка ссылок от корня к текущему месту.
// Вход. Crumbs{ Items }.
// Состояния. нет.
// Инварианты. Последний элемент — не ссылка; при сжатии корень и текущее остаются.
// Поведение. Средние звенья раскрываются нажатием.
// Геометрия. Одна строка.
// Содержимое. нет.
// Чего нет. Нет редактирования пути строкой.
type Crumbs struct {
	Items []Crumb
}

func (c Crumbs) Render() string {
	if len(c.Items) == 0 {
		return ""
	}
	var parts string
	for i, item := range c.Items {
		if i > 0 {
			parts += `<span class="tc-crumbs__sep" aria-hidden="true">/</span>`
		}
		if i == len(c.Items)-1 {
			parts += `<span class="tc-crumbs__current" aria-current="page">` + esc(item.Label) + `</span>`
		} else {
			parts += `<a href="` + esc(item.Href) + `" class="tc-crumbs__link">` + esc(item.Label) + `</a>`
		}
	}
	return `<nav class="tc-crumbs" aria-label="` + esc(labels.Breadcrumb) + `">` + parts + `</nav>`
}
