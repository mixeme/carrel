package widget

// Form — tc-form
//
// Роль. Набор полей с общими кнопками и общими ошибками.
// Вход. Form{ Action, Method, Sets, Errors, Foot, Busy }.
// Состояния. normal | busy.
// Инварианты. Ошибка поля у своего поля; кнопки в подвале; повторная отправка невозможна при busy.
// Поведение. Отправка, блокировка, фокус на первое поле с ошибкой.
// Геометрия. Выравнивание по строке подписи.
// Содержимое. tc-fset, tc-field, tc-formfoot.
// Чего нет. Нет проверки значений.
type Form struct {
	Action string
	Method string
	Sets   []FieldSet
	Errors []FieldError
	Foot   FormFoot
	Busy   bool
}

func (f Form) Render() string {
	method := f.Method
	if method == "" {
		method = "POST"
	}
	cls := "tc-form"
	if f.Busy {
		cls = classes(cls, "is-busy")
	}
	var sets string
	for _, s := range f.Sets {
		sets += s.Render()
	}
	var errs string
	for _, e := range f.Errors {
		errs += `<div class="tc-form__error" data-tc-field="` + esc(e.Name) + `">` + esc(e.Error) + `</div>`
	}
	dis := ""
	if f.Busy {
		dis = " aria-busy=\"true\""
	}
	return `<form action="` + esc(f.Action) + `" method="` + esc(method) + `"` + classAttr(cls) + dis + `>` + errs + sets + f.Foot.Render() + `</form>`
}

// FieldSet — tc-fset
//
// Роль. Именованный блок формы.
// Вход. FieldSet{ Legend, Rows, Note }.
// Состояния. нет.
// Инварианты. Заголовок группы — рубрикатор.
// Поведение. нет.
// Геометрия. Отступ между группами больше, чем между полями.
// Содержимое. tc-field.
// Чего нет. Не сворачивается.
type FieldSet struct {
	Legend string
	Rows   []Field
	Note   string
}

func (fs FieldSet) Render() string {
	var rows string
	for _, r := range fs.Rows {
		rows += r.Render()
	}
	note := ""
	if fs.Note != "" {
		note = `<p class="tc-fset__note">` + esc(fs.Note) + `</p>`
	}
	return `<fieldset class="tc-fset"><legend class="tc-fset__legend">` + esc(fs.Legend) + `</legend>` + rows + note + `</fieldset>`
}

// Field — tc-field
//
// Роль. Подпись, ввод, подсказка, ошибка и кнопка, относящаяся к полю.
// Вход. Field{ Name, Label, Kind, Value, Options, Hint, Error, Extra, ReadOnly, Required, Disabled }.
// Состояния. normal | error | read-only | disabled.
// Инварианты. Подпись связана с полем; ширина по роду содержимого.
// Поведение. Показ ошибки, связь подсказки с полем.
// Геометрия. Высота 28px; в узкой раскладке кегль не меньше 16px.
// Содержимое. tc-in, tc-check, tc-btn.
// Чего нет. Нет плавающей подписи внутри поля.
type Field struct {
	Name     string
	Label    string
	Kind     InputKind
	Value    string
	Options  []SelectOption
	Hint     string
	Error    string
	Extra    Action
	ReadOnly bool
	Required bool
	Disabled bool
	Check    *Check
}

func (f Field) Render() string {
	cls := "tc-field"
	if f.Error != "" {
		cls = classes(cls, "is-error")
	}
	if f.ReadOnly {
		cls = classes(cls, "is-read-only")
	}
	if f.Disabled {
		cls = classes(cls, "is-disabled")
	}
	req := ""
	if f.Required {
		req = `<span class="tc-field__req" aria-hidden="true">*</span>`
	}
	var input string
	if f.Check != nil {
		chk := *f.Check
		chk.Name = f.Name
		chk.Label = f.Label
		input = chk.Render()
	} else {
		input = Input{Name: f.Name, Kind: f.Kind, Value: f.Value, Options: f.Options, ReadOnly: f.ReadOnly || f.Disabled}.Render()
	}
	hint := ""
	if f.Hint != "" {
		hint = `<span class="tc-field__hint" id="` + esc(f.Name) + `-hint">` + esc(f.Hint) + `</span>`
	}
	err := ""
	if f.Error != "" {
		err = `<span class="tc-field__error" id="` + esc(f.Name) + `-error" role="alert">` + esc(f.Error) + `</span>`
	}
	extra := ""
	if f.Extra.Label != "" {
		extra = `<div class="tc-field__extra">` + f.Extra.Render() + `</div>`
	}
	return `<div` + classAttr(cls) + `><label class="tc-field__label" for="` + esc(f.Name) + `">` + esc(f.Label) + req + `</label><div class="tc-field__control">` + input + extra + `</div>` + hint + err + `</div>`
}

// FormFoot — tc-formfoot
//
// Роль. Кнопки формы в объявленном порядке.
// Вход. FormFoot{ Primary, Rest, Danger }.
// Состояния. нет.
// Инварианты. Первичное действие ровно одно; опасное отделено.
// Поведение. нет.
// Геометрия. Одна строка; в узкой раскладке — столбец.
// Содержимое. tc-btn.
// Чего нет. Нет кнопок, ведущих на другие экраны.
type FormFoot struct {
	Primary Action
	Rest    []Action
	Danger  []Action
}

func (ff FormFoot) Render() string {
	var rest, danger string
	for _, a := range ff.Rest {
		rest += a.Render()
	}
	for _, a := range ff.Danger {
		da := a
		if da.Kind == "" {
			da.Kind = ButtonDanger
		}
		danger += da.Render()
	}
	primary := ff.Primary
	if primary.Kind == "" {
		primary.Kind = ButtonPrimary
	}
	return `<div class="tc-formfoot"><div class="tc-formfoot__primary">` + primary.Render() + `</div><div class="tc-formfoot__rest">` + rest + `</div><div class="tc-formfoot__danger">` + danger + `</div></div>`
}

// Danger — tc-danger
//
// Роль. Блок разрушающих действий внизу страницы настроек.
// Вход. Danger{ Title, Rows }.
// Состояния. нет.
// Инварианты. Каждое действие называет последствие; блок всегда последний.
// Поведение. нет; подтверждение — tc-dialog.
// Геометрия. Отбит от остальной страницы.
// Содержимое. tc-btn с состоянием danger.
// Чего нет. Нет действий без подтверждения.
type Danger struct {
	Title string
	Rows  []DangerRow
}

func (d Danger) Render() string {
	var rows string
	for _, r := range d.Rows {
		a := r.Action
		if a.Kind == "" {
			a.Kind = ButtonDanger
		}
		rows += `<div class="tc-danger__row"><div class="tc-danger__text"><strong>` + esc(r.Title) + `</strong><p>` + esc(r.Body) + `</p></div><div class="tc-danger__action">` + a.Render() + `</div></div>`
	}
	return `<section class="tc-danger"><h2 class="tc-danger__title">` + esc(d.Title) + `</h2>` + rows + `</section>`
}
