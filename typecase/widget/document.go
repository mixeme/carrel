package widget

// Doc — tc-doc
//
// Роль. Сплошной текст, который читают и правят.
// Вход. Doc{ Mode, Width, Body, Bar, Meta }.
// Состояния. reading | editing.
// Инварианты. Переключатель меры — часть виджета; тихого сохранения нет.
// Поведение. Переключение меры, переход в правку, предупреждение при уходе.
// Геометрия. Полная ширина или мера 72–78 знаков.
// Содержимое. tc-docbar, tc-prose, tc-docmeta.
// Чего нет. Нет панели форматирования с разметкой.
type Doc struct {
	Mode  DocMode
	Width DocWidth
	Body  Prose
	Bar   DocBar
	Meta  DocMeta
}

func (d Doc) Render() string {
	mode := d.Mode
	if mode == "" {
		mode = DocReading
	}
	width := d.Width
	if width == "" {
		width = DocWidthMeasure
	}
	return `<article` + classAttr("tc-doc", "tc-doc--"+string(mode), "tc-doc--"+string(width)) + `>` + d.Bar.Render() + `<div class="tc-doc__layout">` + d.Body.Render() + d.Meta.Render() + `</div></article>`
}

// Prose — tc-prose
//
// Роль. Показ размеченного текста, пришедшего деревом блоков.
// Вход. Prose{ Blocks }.
// Состояния. нет.
// Инварианты. Готовую разметку принять невозможно.
// Поведение. нет.
// Геометрия. Мера строки от tc-doc.
// Содержимое. Блоки объявленных родов.
// Чего нет. Нет разбора разметки.
type Prose struct {
	Blocks []ProseBlock
}

func (p Prose) Render() string {
	var blocks string
	for _, b := range p.Blocks {
		blocks += renderProseBlock(b)
	}
	return `<div class="tc-prose">` + blocks + `</div>`
}

func renderProseBlock(b ProseBlock) string {
	switch b.Kind {
	case ProseH1, ProseH2, ProseH3:
		tag := string(b.Kind)
		return `<` + tag + ` class="tc-prose__` + tag + `">` + esc(b.Text) + `</` + tag + `>`
	case ProseP:
		return `<p class="tc-prose__p">` + esc(b.Text) + `</p>`
	case ProseUL:
		var items string
		for _, it := range b.Items {
			items += `<li>` + esc(it) + `</li>`
		}
		return `<ul class="tc-prose__ul">` + items + `</ul>`
	case ProseCode:
		return `<pre class="tc-prose__code"><code>` + esc(b.Code) + `</code></pre>`
	case ProseLink:
		return `<p class="tc-prose__p"><a href="` + esc(b.Href) + `" class="tc-prose__link">` + esc(b.Text) + `</a></p>`
	default:
		return `<p class="tc-prose__p">` + esc(b.Text) + `</p>`
	}
}

// DocBar — tc-docbar
//
// Роль. Действия над документом: сохранить, переключить меру, сосредоточиться.
// Вход. DocBar{ Left, Right, Rank2, Dirty, Collapsed }.
// Состояния. full | collapsed.
// Инварианты. Свой порог отступления; одна строка.
// Поведение. Как tc-bar, плюс признак несохранённого.
// Геометрия. Высота панели инструментов.
// Содержимое. tc-btn, tc-menu, tc-kbd.
// Чего нет. Нет кнопок, не относящихся к документу.
type DocBar struct {
	Left      []Action
	Right     []Action
	Rank2     []Action
	Dirty     bool
	Collapsed bool
}

func (d DocBar) Render() string {
	cls := "tc-docbar"
	if d.Dirty {
		cls = classes(cls, "is-dirty")
	}
	if d.Collapsed {
		cls = classes(cls, "is-collapsed")
	}
	dirty := ""
	if d.Dirty {
		dirty = `<span class="tc-docbar__dirty" aria-label="` + esc(labels.UnsavedChanges) + `">●</span>`
	}
	return `<div` + classAttr(cls) + `>` + dirty + Bar{Left: ActionsAsNodes(d.Left), Right: ActionsAsNodes(d.Right), Rank2: d.Rank2}.Render() + `</div>`
}

// DocMeta — tc-docmeta
//
// Роль. Дата, метки, связи, вложения сбоку от текста.
// Вход. DocMeta{ Sections }.
// Состояния. нет.
// Инварианты. Не превращается в форму.
// Поведение. нет.
// Геометрия. Своя колонка; в узкой раскладке под текстом.
// Содержимое. tc-tag, tc-fields, tc-row.
// Чего нет. Нет прокрутки отдельно от документа.
type DocMeta struct {
	Sections []MetaSection
}

func (m DocMeta) Render() string {
	var secs string
	for _, s := range m.Sections {
		var tags string
		for _, t := range s.Tags {
			tags += t.Render()
		}
		var rows string
		for _, r := range s.Rows {
			rows += FieldRowHTML(r)
		}
		for _, r := range s.Rows2 {
			rows += r.Render()
		}
		secs += `<section class="tc-docmeta__section"><h3 class="tc-docmeta__title">` + esc(s.Title) + `</h3><div class="tc-docmeta__tags">` + tags + `</div><div class="tc-docmeta__rows">` + rows + `</div></section>`
	}
	return `<aside class="tc-docmeta">` + secs + `</aside>`
}
