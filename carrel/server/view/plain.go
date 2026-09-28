package view

// Plain — общая часть страниц без оболочки: ни рейла, ни панели, ни поиска.
type Plain struct {
	Title   Text     `json:"title"`
	Lede    Text     `json:"lede"`
	Notices []Notice `json:"notices"`
}

type LoginPage struct {
	Plain
	Form   Form   `json:"form"`
	Forgot Action `json:"forgot"` // ведёт на /forgot и обещает только то, что там есть
	About  Action `json:"about"`
}
type SetupPage struct {
	Plain
	Form Form `json:"form"`
}
type RegisterPage struct {
	Plain
	Form Form `json:"form"`
	Sent bool `json:"sent"`
}
type InvitePage struct {
	Plain
	Form  Form `json:"form"`
	Valid bool `json:"valid"`
}
type ConfirmEmailPage struct {
	Plain
	OK   bool   `json:"ok"`
	Next Action `json:"next"`
}
type ForgotPage struct {
	Plain
	Form    Form     `json:"form"`
	Options []Action `json:"options"` // разрушительная очистка или запрос администратору
	HasMail bool     `json:"hasMail"` // подтверждённого адреса нет — помочь нельзя, и так и сказано
	Sent    bool     `json:"sent"`
	Asked   bool     `json:"asked"`
}

type WipeStep string

const (
	WipeConfirm  WipeStep = "confirm"  // очистка ещё не сделана, предупреждение на экране
	WipePassword WipeStep = "password" // очищено, тем же токеном задают новый пароль
)

type ForgotWipePage struct {
	Plain
	Step    WipeStep `json:"step"`
	Login   Text     `json:"login"`   // логин остаётся прежним и показывается, а не спрашивается
	Form    Form     `json:"form"`    // подтверждение на первом шаге, пароль и повтор на втором
	Warning Notice   `json:"warning"` // на первом шаге называет последствие, на втором — цену пароля
}
type AboutPage struct {
	Plain
	Version Text   `json:"version"`
	Commit  Text   `json:"commit"`
	Source  Action `json:"source"` // видимая ссылка на исходники запущенной версии
	Third   Action `json:"third"`
	Back    Action `json:"back"`
}

type PublishedPage struct {
	Plain
	Kind     Kind             `json:"kind"`
	Owner    Text             `json:"owner"`
	ReadOnly bool             `json:"readOnly"` // метка read-only рядом с логотипом
	Range    *Range           `json:"range"`
	Contacts []ContactRow     `json:"contacts"`
	Days     []AgendaDay      `json:"days"`
	Tasks    []TaskRow        `json:"tasks"`  // опубликованный список задач
	Notes    []NoteRow        `json:"notes"`  // опубликованная тетрадь
	Fields   []Field          `json:"fields"` // одна запись: контакт, событие, задача
	Body     Text             `json:"body"`   // описание события или задачи
	Note     *NoteDoc         `json:"note"`
	File     *PublishedFile   `json:"file"`
	Folder   *PublishedFolder `json:"folder"`
	Actions  []Action         `json:"actions"` // Download · Download folder
}
type PublishedFile struct {
	Name    Text        `json:"name"`
	Size    Size        `json:"size"`
	Type    Text        `json:"type"`
	Preview PreviewKind `json:"preview"` // C-8: изображение, PDF, текст, разметка, ничего
	Body    Text        `json:"body"`
	Blocks  []Block     `json:"blocks"`
	Href    string      `json:"href"`
}
type PublishedFolder struct {
	Crumbs []Crumb   `json:"crumbs"`
	Rows   []FileRow `json:"rows"`
	DAV    Text      `json:"dav"` // тот же секрет монтируется как диск
}

// Block — разобранная разметка: заголовки, списки, задачи с отметкой,
// выделение, ссылки, код, цитаты, таблицы.
// Сырой HTML исходника экранируется и блоком не становится.
type Block struct {
	Kind     BlockKind `json:"kind"`
	Level    int       `json:"level"`
	Text     Text      `json:"text"`
	Href     string    `json:"href"`
	Checked  bool      `json:"checked"`          // задача отмечена; только у BlockTask
	Header   bool      `json:"header,omitempty"` // строка заголовка таблицы; только у BlockRow
	Align    CellAlign `json:"align,omitempty"`  // выравнивание столбца; только у BlockCell, пусто — не задано
	Children []Block   `json:"children"`
}
