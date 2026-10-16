package view

// Plain — общая часть страниц без оболочки: ни рейла, ни панели, ни поиска.
// Title — заголовок карточки и документа; у входа кадр его не рисует (на его
// месте знак) и он остаётся заголовком документа. Lede — строка под
// заголовком (у входа и «о программе» — первая строка теглайна), кусками:
// логин или адрес в ней кадр выделяет (s-auth-3, -6, -10, -11). Notices —
// над полями: предупреждение о пароле (s-auth-2, s-auth-4, s-auth-11,
// s-auth-12).
type Plain struct {
	Title   Text     `json:"title"`
	Lede    Runs     `json:"lede"`
	Notices []Notice `json:"notices"`
}

// LoginPage — вход (s-auth-1): знак, теглайн, Login и Password, Sign in, под
// кнопкой — ссылки.
type LoginPage struct {
	Plain
	Tagline Text `json:"tagline"` // вторая строка теглайна, после Lede
	Form    Form `json:"form"`
	// Register — «Create an account»; только пока регистрация открыта, nil —
	// закрыта: закрытый экземпляр форму не рекламирует
	Register *Action `json:"register"`
	Forgot   Action  `json:"forgot"` // ведёт на /forgot и обещает только то, что там есть
}
type SetupPage struct {
	Plain
	Form Form `json:"form"`
}
type RegisterPage struct {
	Plain
	Form Form `json:"form"` // после отправки пуста
	Sent bool `json:"sent"`
	// SignIn ведёт на /login: «Already have an account? Sign in» у формы
	// (s-auth-2), «Back to sign in» после отправки (s-auth-3)
	SignIn Action `json:"signIn"`
}
type InvitePage struct {
	Plain
	Form Form `json:"form"`
}
type ConfirmEmailPage struct {
	Plain
	Next Action `json:"next"`
}

// DeadLinkPage — мёртвая ссылка-секрет (rest-1): одна страница на
// /invite/, /forgot/, /confirm-email/ и /p/ — те же слова, тот же код, те
// же заголовки и та же задержка, что бы это ни была за ссылка и была ли она
// вообще (C-11).
type DeadLinkPage struct {
	Plain
	SignIn Action `json:"signIn"` // «Sign in» — единственная дверь
}

// ForgotPage — /forgot: логин или почта (s-auth-5), варианты (s-auth-6),
// помочь нечем (s-auth-7), письмо отправлено (s-auth-8), администратор
// спрошен (s-auth-9).
type ForgotPage struct {
	Plain
	Form    Form           `json:"form"`    // логин или почта; в остальных состояниях пуста
	Options []ForgotOption `json:"options"` // разрушительная очистка и — при депонированном ключе — запрос администратору
	HasMail bool           `json:"hasMail"` // подтверждённого адреса нет — помочь нельзя, и так и сказано
	Sent    bool           `json:"sent"`
	Asked   bool           `json:"asked"`
	Back    Action         `json:"back"` // «Back to sign in», во всех состояниях
}

// ForgotOption — карточка варианта (s-auth-6): что будет, и кнопка — форма
// из одной кнопки, которая несёт логин.
type ForgotOption struct {
	Title Text `json:"title"`
	Body  Text `json:"body"`
	Form  Form `json:"form"`
}

type WipeStep string

const (
	WipeConfirm  WipeStep = "confirm"  // очистка ещё не сделана, предупреждение на экране
	WipePassword WipeStep = "password" // очищено, тем же токеном задают новый пароль
)

// ForgotWipePage — /forgot/{token}: подтверждение очистки (s-auth-10),
// потом новый пароль (s-auth-11; предупреждение о пароле — в Notices).
type ForgotWipePage struct {
	Plain
	Step   WipeStep `json:"step"`
	Login  Text     `json:"login"`  // логин остаётся прежним и показывается, а не спрашивается
	Form   Form     `json:"form"`   // на первом шаге — одна кнопка очистки, на втором — пароль и повтор
	Cancel *Action  `json:"cancel"` // «Cancel» первого шага — на вход; nil на втором
}

// AboutPage — «о программе» (rest-3): Title «Carrel», версия и коммит, теглайн
// (Lede и Tagline), строка лицензии, ссылки на исходники и лицензии, кнопка
// назад.
type AboutPage struct {
	Plain
	Tagline Text   `json:"tagline"` // вторая строка теглайна, после Lede
	Version Text   `json:"version"`
	Commit  Text   `json:"commit"`
	Licence Text   `json:"licence"` // AGPL: изменённую версию для других — с исходниками
	Source  Action `json:"source"`  // видимая ссылка на исходники запущенной версии
	Third   Action `json:"third"`
	// Back — без сеанса «Back to sign in»; из оболочки — туда, где были
	Back Action `json:"back"`
}

// PublishedPage — страница ссылки (U-6.15). Lede — строка под заголовком
// готовой фразой: у календаря диапазон дней, у книги счёт, у записи — её
// время, у файла — тип, размер и день правки; у папки её нет (итог — в
// крошках, Folder.Total). Foot — подвал: до какого дня ссылка действует
// (Пс2). Владельца страница не называет (S-25.1).
type PublishedPage struct {
	Plain
	Kind     Kind             `json:"kind"`
	ReadOnly bool             `json:"readOnly"` // метка read-only рядом с логотипом
	Range    *Range           `json:"range"`
	Contacts []ContactGroup   `json:"contacts"` // по букве, как в приложении (s-publish-7)
	Days     []AgendaDay      `json:"days"`
	Tasks    []TaskRow        `json:"tasks"`  // опубликованный список задач
	Notes    []NoteRow        `json:"notes"`  // опубликованная тетрадь
	Fields   []Field          `json:"fields"` // одна запись: контакт, событие, задача
	Body     Text             `json:"body"`   // описание события или задачи
	Note     *NoteDoc         `json:"note"`
	File     *PublishedFile   `json:"file"`
	Folder   *PublishedFolder `json:"folder"`
	Actions  []Action         `json:"actions"` // Download · Download folder
	Foot     Runs             `json:"foot"`
}
type PublishedFile struct {
	Name    Text        `json:"name"`
	Size    Size        `json:"size"`
	Type    Text        `json:"type"`
	Preview PreviewKind `json:"preview"` // C-8: изображение, PDF, текст, разметка, ничего
	Changed When        `json:"changed"` // день последней правки: «2026-08-10» после слова экрана «changed» (s-publish-9)
	Body    Text        `json:"body"`
	Blocks  []Block     `json:"blocks"`
	Href    string      `json:"href"`
}
type PublishedFolder struct {
	Crumbs []Crumb   `json:"crumbs"`
	Rows   []FileRow `json:"rows"`
	DAV    Text      `json:"dav"` // тот же секрет монтируется как диск
	// Total — итог после текущей крошки: «4 files · 6.6 MB» (s-publish-12)
	Total Text `json:"total"`
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

// PasswordHistoryPage — история пароля учётной записи по её постоянной
// ссылке (решение владельца 16.10.2026, Ад40): без входа, и только то, что
// делали с паролем, — открытие ключа мастер-паролем, сброс администратором,
// смена пароля, сброс учётной записи по ссылке «Forgot password»: когда и
// кем. Title — «Password history of anna»; Lede — с какого дня история
// ведётся. Empty — слова пустой истории; у непустой пусто
type PasswordHistoryPage struct {
	Plain
	Rows  []PasswordHistoryRow `json:"rows"`
	Empty Text                 `json:"empty"`
}

// PasswordHistoryRow — одно событие: момент числами, что сделано, кем.
type PasswordHistoryRow struct {
	When When `json:"when"`
	What Text `json:"what"`
	Who  Text `json:"who"`
}
