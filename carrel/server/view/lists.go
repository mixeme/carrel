package view

// Строка списка своя у каждого рода — это разные наборы данных, и общей
// строки «на всё» не заводится.

type ListBar struct {
	Filter Text `json:"filter"` // фильтр по уже загруженному, без второго опроса
	// FilterForm — фильтр без сценариев: GET на адрес списка, поле filter и
	// скрытые поля запроса (task, sort, tag); nil — у списка фильтра нет
	FilterForm *Form `json:"filterForm"`
	Loaded     int   `json:"loaded"`
	Total      *int  `json:"total"`
	// Compact — «Compact rows» (s-contacts-1): On — строки плотные; ведёт на
	// тот же список с compact=1 или 0 (кука раздела, S-9.6); nil —
	// переключателя нет
	Compact  *Action  `json:"compact"`
	Segments []Action `json:"segments"` // Open / Done / All; All / Contacts / Events / …
	Range    *Range   `json:"range"`    // диапазон дат там, где он есть
	Actions  []Action `json:"actions"`  // операции над выделенным
	Selected int      `json:"selected"`
	// Columns — кнопка Columns полосы и её меню-форма (components-42 и
	// меню записки 25); nil — у списка выбора столбцов нет
	Columns *Columns `json:"columns"`
}

// Columns — выбор столбцов списка (components-21, components-42, S-9.6).
// Меню — форма: отмеченные пункты уходят полем col в том порядке, в каком
// стоят, и порядок пунктов — порядок столбцов. Выбор помнит этот браузер
// (U-12).
type Columns struct {
	Label Text   `json:"label"` // «Columns»: кнопка полосы и заголовок меню
	Icon  string `json:"icon"`  // значок кнопки, i-cols
	// Form — POST {адрес списка}/columns с запросом списка; скрытый _csrf;
	// Actions — Apply и «Reset to defaults», который отправляет col=default
	Form Form `json:"form"`
	// Items — пункты меню: показанные столбцы в их порядке, за ними снятые
	// в порядке по умолчанию; пока человек не выбирал — все в порядке по
	// умолчанию (components-21)
	Items []ColumnItem `json:"items"`
}
type ColumnItem struct {
	ID    string `json:"id"` // значение col и столбец строки (C-6.4): name, phone, email, org, birthday, tags, changed, source; title, due, complete, priority, status, excerpt, date, kind; size, type; login, role, created, last-login, dav, sessions, escrow, address; when, action, actor, target, detail
	Label Text   `json:"label"`
	On    bool   `json:"on"`              // отмечен: столбец показан
	Fixed bool   `json:"fixed"`           // стоит всегда первым, отметку не снять (Name, Title, Login, When)
	Aside Text   `json:"aside,omitempty"` // приглушённое после подписи: «always» у Fixed
}

type Range struct {
	From    Text     `json:"from"`
	To      Text     `json:"to"`
	Presets []Action `json:"presets"` // Day · Week · Month, «предыдущая» и «следующая»
	// Form — произвольный диапазон: форма GET на Form.Action, поля from и
	// to, «Show» её отправляет (Range.Action, s-contacts-7, s-agenda-1); nil —
	// его нет
	Form *Form `json:"form"`
}

type ContactsListPage struct {
	Shell
	// Subtitle — под заголовком: «3 of 4 books · 2 accounts · 312 cards»,
	// «nextcloud · 248 cards», «radicale · vps · the server granted no write
	// access» (s-contacts-1, s-io-1, states-4)
	Subtitle Text           `json:"subtitle"`
	ReadOnly bool           `json:"readOnly"` // книга только для чтения: метка у заголовка (states-4)
	Bar      ListBar        `json:"bar"`
	Groups   []ContactGroup `json:"groups"`
	Poll     *PollBar       `json:"poll"`  // единый вид; nil — одна коллекция или ничего не отмечено
	Empty    *Notice        `json:"empty"` // показать нечего: почему и что сделать, с кнопкой; nil — строки есть
}
type ContactGroup struct {
	Rubric Text         `json:"rubric"`
	Count  int          `json:"count"` // сколько карточек в группе
	Rows   []ContactRow `json:"rows"`
}
type ContactRow struct {
	Href   string `json:"href"`
	UID    string `json:"uid"`
	Colour Colour `json:"colour"`
	Stamp  Stamp  `json:"stamp"`
	Avatar Avatar `json:"avatar"`
	Name   Text   `json:"name"`
	Phone  Text   `json:"phone"`
	Email  Text   `json:"email"`
	Org    Text   `json:"org"`
	// Столбцы, которые человек может выбрать (s-contacts-23, -24):
	// день рождения «14.03.1987», без года «02.11»; категории карточки;
	// день последней правки коротко, «12 Aug» (C-10)
	Birthday Text        `json:"birthday"`
	Tags     []Tag       `json:"tags"`
	Changed  Text        `json:"changed"`
	Badges   []Badge     `json:"badges"`  // «2 источника» у свёрнутого дубликата
	Sources  []Stamp     `json:"sources"` // раскрытые источники свёрнутой группы
	Menu     []MenuItem  `json:"menu"`
	State    RecordState `json:"state"`
	On       bool        `json:"on"` // строка открыта в панели подробностей (is-on)
}

type AgendaPage struct {
	Shell
	Bar  ListBar     `json:"bar"`
	Days []AgendaDay `json:"days"`
	Poll *PollBar    `json:"poll"`
}
type AgendaDay struct {
	Rubric Text        `json:"rubric"`
	Rows   []AgendaRow `json:"rows"`
}
type AgendaRow struct {
	Kind     Kind        `json:"kind"` // event, task (срок) или note на одной ленте
	Href     string      `json:"href"`
	UID      string      `json:"uid"`
	Colour   Colour      `json:"colour"`
	Stamp    Stamp       `json:"stamp"`
	Time     Text        `json:"time"` // «10:00» или «весь день»
	Zone     Text        `json:"zone"` // исходный пояс, когда он отличается; иначе пусто
	Summary  Text        `json:"summary"`
	Location Text        `json:"location"`
	Badges   []Badge     `json:"badges"`
	Sources  []Stamp     `json:"sources"`
	Menu     []MenuItem  `json:"menu"`
	State    RecordState `json:"state"`
}

type TasksPage struct {
	Shell
	Bar    ListBar     `json:"bar"` // сегменты Open / Done / All со счётом
	Groups []TaskGroup `json:"groups"`
	Poll   *PollBar    `json:"poll"`
}
type TaskGroup struct {
	Rubric Text      `json:"rubric"`
	Rows   []TaskRow `json:"rows"`
}
type TaskRow struct {
	Href    string `json:"href"`
	UID     string `json:"uid"`
	Colour  Colour `json:"colour"`
	Stamp   Stamp  `json:"stamp"`
	Done    bool   `json:"done"`
	Summary Text   `json:"summary"`
	Due     Text   `json:"due"`
	Overdue bool   `json:"overdue"`
	Percent *int   `json:"percent"`
	Tags    []Tag  `json:"tags"`
	// Столбцы, которые человек может выбрать (s-tasks-12, -13): приоритет
	// числом, «5»; статус словом списка, «In process» (без STATUS — «Needs
	// action»); последняя правка коротко с временем, «10 Aug 11:02» (C-10)
	Priority Text        `json:"priority"`
	Status   Text        `json:"status"`
	Changed  Text        `json:"changed"`
	Badges   []Badge     `json:"badges"`  // «2 источника» у связанной задачи
	Sources  []Stamp     `json:"sources"` // источники связанной группы
	Tick     *Action     `json:"tick"`    // отметка выполнения в строке; nil — только чтение
	Menu     []MenuItem  `json:"menu"`
	State    RecordState `json:"state"`
}

type NotesPage struct {
	Shell
	Bar    ListBar     `json:"bar"`
	Groups []NoteGroup `json:"groups"`
	Poll   *PollBar    `json:"poll"`
}
type NoteGroup struct {
	Rubric Text      `json:"rubric"`
	Rows   []NoteRow `json:"rows"`
}
type NoteRow struct {
	Href    string      `json:"href"`
	UID     string      `json:"uid"`
	Colour  Colour      `json:"colour"`
	Stamp   Stamp       `json:"stamp"`
	Title   Text        `json:"title"`
	Excerpt Text        `json:"excerpt"`
	Date    Text        `json:"date"`
	Tags    []Tag       `json:"tags"`
	Changed Text        `json:"changed"` // последняя правка коротко с временем, «10 Aug 11:05» (s-notes-24, C-10)
	Badges  []Badge     `json:"badges"`  // «2 источника» у связанной заметки
	Sources []Stamp     `json:"sources"` // источники связанной группы
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
}
