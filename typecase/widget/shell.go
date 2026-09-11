package widget

// Shell — tc-shell
//
// Роль. Четыре зоны страницы и переходы между раскладками по ширине.
// Вход. Shell{ Top, Rail, Main, Side, Density }.
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
}

func (s Shell) Render() string {
	dens := s.Density
	if dens == "" {
		dens = DensityNormal
	}
	return `<div` + classAttr("tc-shell", "tc-shell--"+string(dens)) + ` data-tc-shell data-tc-root>` + s.Top.Render() + `<div class="tc-shell__body"><div class="tc-shell__rail">` + s.Rail.Render() + `</div><div class="tc-shell__main">` + s.Main.Render() + `</div><div class="tc-shell__side">` + s.Side.Render() + `</div></div></div>`
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
	create := t.Create.Render()
	user := t.User.Render()
	return `<header` + classAttr(cls) + `>` + t.Brand.Render() + t.Nav.Render() + t.Search.Render() + `<div class="tc-top__actions">` + create + user + `</div>` + t.Mark.Render() + `</header>`
}

// Main — tc-main
//
// Роль. Всё, что осталось после рейла и панели.
// Вход. Main{ Label, Body }.
// Состояния. нет.
// Инварианты. Ширину не ограничивает.
// Поведение. Своя прокрутка.
// Геометрия. Занимает остаток.
// Содержимое. Любые виджеты.
// Чего нет. Нет отступов «на всякий случай».
type Main struct {
	Label string
	Body  string
}

func (m Main) Render() string {
	return `<main class="tc-main"` + attr("aria-label", m.Label) + `><div class="tc-main__body">` + m.Body + `</div></main>`
}

// Rail — tc-rail
//
// Роль. Сгруппированный список с подвалом действий и фильтром.
// Вход. Rail{ Sections, Chips, Foot, Note, Drawer, Open }.
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
	Drawer   bool
	Open     bool
}

func (r Rail) Render() string {
	cls := "tc-rail"
	if r.Drawer {
		cls = classes(cls, "tc-rail--drawer")
	}
	if r.Open {
		cls = classes(cls, "is-open")
	}
	var chips string
	for _, c := range r.Chips {
		chips += c.Render()
	}
	var secs string
	for _, s := range r.Sections {
		secs += s.Render()
	}
	note := ""
	if r.Note != "" {
		note = `<p class="tc-rail__note">` + esc(r.Note) + `</p>`
	}
	return `<aside` + classAttr(cls) + ` data-tc-rail` + ariaBool("aria-expanded", r.Open) + `><div class="tc-rail__chips">` + chips + `</div><div class="tc-rail__sections">` + secs + `</div>` + note + r.Foot.Render() + `</aside>`
}

// RailSection — tc-railsec
//
// Роль. Заголовок группы и её строки.
// Вход. RailSection{ Title, State, Rows }.
// Состояния. ok | muted | alert.
// Инварианты. Недоступная группа помечена цветом и словом.
// Поведение. нет.
// Геометрия. Заголовок — мелкий кегль с разрядкой.
// Содержимое. tc-tickrow.
// Чего нет. Не сворачивается.
func (rs RailSection) Render() string {
	cls := "tc-railsec"
	if rs.State != "" {
		cls = classes(cls, "tc-railsec--"+rs.State)
	}
	var rows string
	for _, r := range rs.Rows {
		rows += r.Render()
	}
	return `<section` + classAttr(cls) + `><h3 class="tc-railsec__title">` + esc(rs.Title) + `</h3><div class="tc-railsec__rows">` + rows + `</div></section>`
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
		return `<p class="tc-railfoot__build tc-railfoot__build--missing" role="alert">Build information missing</p>`
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
// Содержимое. tc-sec, tc-fields, tc-btn, tc-bar.
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
		blocks += b.Sec.Render()
	}
	id := "tc-side"
	return `<aside` + classAttr(cls) + ` data-tc-side` + attr("aria-label", s.Label) + `><header class="tc-side__head"><h2 class="tc-side__title">` + esc(s.Title) + `</h2>` + Close{For: id, Label: "Close panel"}.Render() + `</header><div class="tc-side__body">` + blocks + `</div><footer class="tc-side__foot">` + Bar{Left: s.Actions}.Render() + `</footer></aside>`
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
	Layout  string
}

func (n Nav) Render() string {
	layout := n.Layout
	if layout == "" {
		layout = "column"
	}
	var items string
	for _, it := range n.Items {
		cls := "tc-nav__item"
		if it.Href == n.Current {
			cls = classes(cls, "is-current")
		}
		items += `<a href="` + esc(it.Href) + `"` + classAttr(cls) + `><span class="tc-nav__icon" aria-hidden="true">` + esc(it.Icon) + `</span><span class="tc-nav__label">` + esc(it.Label) + `</span>` + Num{Value: it.Count}.Render() + `</a>`
	}
	return `<nav` + classAttr("tc-nav", "tc-nav--"+layout) + ` aria-label="Main">` + items + `</nav>`
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
	return `<header class="tc-head">` + h.Crumbs.Render() + h.Mark.Render() + `<div class="tc-head__titles"><h1 class="tc-head__title">` + esc(h.Title) + `</h1><p class="tc-head__sub">` + esc(h.Sub) + `</p></div>` + Bar{Right: h.Actions, Rank2: h.Rank2}.Render() + `</header>`
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
	return `<nav class="tc-crumbs" aria-label="Breadcrumb">` + parts + `</nav>`
}
