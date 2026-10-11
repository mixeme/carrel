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
	// Compact — «Compact rows» (s-contacts-1, components-38): форма GET на
	// адрес списка — скрытые поля запроса списка, скрытое compact=0 и
	// отметка compact с одним вариантом «1» (Selected — строки плотные);
	// выбор помнит кука раздела (S-9.6); nil — переключателя нет
	Compact  *Form    `json:"compact"`
	Segments []Action `json:"segments"` // Open / Done / All; All / Contacts / Events / …
	Range    *Range   `json:"range"`    // диапазон дат там, где он есть
	Actions  []Action `json:"actions"`  // операции над выделенным
	Selected int      `json:"selected"`
	// Size и SelectedSize — у папки файлов: объём всех её файлов («14 items ·
	// 38.2 MB») и выделенных («412 KB» перед чертой полосы выделения,
	// s-files-1, -2); у прочих списков пусто
	Size         Size `json:"size,omitempty"`
	SelectedSize Size `json:"selectedSize,omitempty"`
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
	// to, «Show» её отправляет (Range.Action, s-contacts-7, s-agenda-1); без
	// _csrf — поля GET уходят в адрес (C-11); nil — его нет
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
	Href string `json:"href"`
	// Target — details: строка открывает запись в панели подробностей —
	// фрагмент по адресу записи (C-5.4, C-7.2); пусто — строка без адреса
	Target string `json:"target,omitempty"`
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
	// Subtitle — под заголовком: единый вид — «5 of 7 calendars · 3 accounts ·
	// 10 – 16 August 2026 · ‹Europe/Moscow›», один календарь — «nextcloud ·
	// 10 – 16 August 2026 · ‹Europe/Moscow›»: окно и пояс экземпляра
	// моноширинно (s-agenda-1)
	Subtitle Runs        `json:"subtitle"`
	Bar      ListBar     `json:"bar"`
	Days     []AgendaDay `json:"days"`
	Poll     *PollBar    `json:"poll"`  // единый вид; nil — один календарь или ничего не отмечено
	Empty    *Notice     `json:"empty"` // ничего не отмечено: почему и кнопка «Tick every calendar» (U-6.0); nil — есть что показать
}
type AgendaDay struct {
	Rubric Text        `json:"rubric"` // «Monday · 10 August» (C-10)
	Count  int         `json:"count"`  // сколько строк в дне
	Rows   []AgendaRow `json:"rows"`
}
type AgendaRow struct {
	Kind Kind   `json:"kind"` // event: календарь показывает только события (Typecase Р-56)
	Href string `json:"href"`
	// Target — details: строка открывает запись в панели подробностей —
	// фрагмент по адресу записи (C-5.4, C-7.2); пусто — строка без адреса
	Target   string `json:"target,omitempty"`
	UID      string `json:"uid"`
	Colour   Colour `json:"colour"`
	Stamp    Stamp  `json:"stamp"`
	Time     Text   `json:"time"` // «10:00» или «all day»
	Summary  Text   `json:"summary"`
	Location Text   `json:"location"`
	// Badges — по важности: «Due» задачи, «Cancelled» отменённого
	// вхождения, «tentative» (тревога) и «cancelled» статуса, «linked ×2»,
	// «4 attendees» (участники с организатором), «repeats weekly»; строка
	// кадра рисует первый (s-agenda-1)
	Badges  []Badge `json:"badges"`
	Sources []Stamp `json:"sources"`
	// Tags и Changed — столбцы, которые добавляет меню Columns агенды
	// (s-agenda-18): категории события и когда оно менялось («2 Aug»,
	// C-10); пусто — у события их нет или сервер не сказал
	Tags    []Tag       `json:"tags"`
	Changed Text        `json:"changed"`
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
	On      bool        `json:"on"` // строка открыта в панели подробностей (is-on, s-agenda-1)
}

type TasksPage struct {
	Shell
	// Subtitle — под заголовком: единый вид — «2 of 3 lists · nextcloud · 30
	// tasks» (одно подключение — его имя, несколько — «2 accounts»; число
	// задач — когда опрос закончен), один список — «nextcloud · 21 tasks»
	// (s-tasks-1)
	Subtitle Text        `json:"subtitle"`
	Bar      ListBar     `json:"bar"` // сегменты Open / Done / All со счётом
	Groups   []TaskGroup `json:"groups"`
	Poll     *PollBar    `json:"poll"`  // единый вид; nil — один список или ничего не отмечено
	Empty    *Notice     `json:"empty"` // ничего не отмечено: почему и кнопка «Tick every list» (U-6.0); nil — есть что показать
}
type TaskGroup struct {
	Rubric Text `json:"rubric"`
	Count  int  `json:"count"` // сколько задач в группе
	// Overdue — группа просроченного: счёт тревогой (is-alert, s-tasks-1)
	Overdue bool      `json:"overdue"`
	Rows    []TaskRow `json:"rows"`
}
type TaskRow struct {
	Href string `json:"href"`
	// Target — details: строка открывает запись в панели подробностей —
	// фрагмент по адресу записи (C-5.4, C-7.2); пусто — строка без адреса
	Target  string `json:"target,omitempty"`
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
	Priority Text    `json:"priority"`
	Status   Text    `json:"status"`
	Changed  Text    `json:"changed"`
	Badges   []Badge `json:"badges"` // «linked ×2» у связанной задачи
	// Sources — источники связанной группы (подсказка столбца источника);
	// Stamp такой строки в едином виде — их счёт, «2 sources» (s-tasks-1)
	Sources []Stamp     `json:"sources"`
	Tick    *Action     `json:"tick"` // отметка выполнения в строке; nil — только чтение
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
	On      bool        `json:"on"` // строка открыта в панели подробностей (is-on, s-tasks-1)
}

type NotesPage struct {
	Shell
	// Subtitle — под заголовком: единый вид — «2 of 3 notebooks · 86 notes
	// · VJOURNAL» (число заметок — когда опрос закончен), одна тетрадь —
	// «nextcloud · 2 notes · VJOURNAL»: компонент, которым заметки лежат на
	// сервере (s-notes-1)
	Subtitle Text        `json:"subtitle"`
	Bar      ListBar     `json:"bar"`
	Groups   []NoteGroup `json:"groups"`
	Poll     *PollBar    `json:"poll"`  // единый вид; nil — одна тетрадь или ничего не отмечено
	Empty    *Notice     `json:"empty"` // ничего не отмечено: почему и кнопка «Tick every notebook» (U-6.0); nil — есть что показать
}

// NoteGroup — месяц тетради, «August 2026», по дате заметки; без даты —
// последней группой «No date»; по названию — одна группа без рубрики
// (s-notes-1).
type NoteGroup struct {
	Rubric Text      `json:"rubric"`
	Count  int       `json:"count"` // сколько заметок в группе
	Rows   []NoteRow `json:"rows"`
}
type NoteRow struct {
	Href string `json:"href"`
	// Target — details: строка открывает запись в панели подробностей —
	// фрагмент по адресу записи (C-5.4, C-7.2); пусто — строка без адреса
	Target  string  `json:"target,omitempty"`
	UID     string  `json:"uid"`
	Colour  Colour  `json:"colour"`
	Stamp   Stamp   `json:"stamp"`
	Title   Text    `json:"title"`
	Excerpt Text    `json:"excerpt"`
	Date    Text    `json:"date"`
	Tags    []Tag   `json:"tags"`
	Changed Text    `json:"changed"` // последняя правка коротко с временем, «10 Aug 11:05» (s-notes-24, C-10)
	Badges  []Badge `json:"badges"`  // «linked ×2» у связанной заметки
	// Sources — источники связанной группы (подсказка столбца источника);
	// Stamp такой строки в едином виде — их счёт, «2 sources» (s-notes-1)
	Sources []Stamp     `json:"sources"`
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
	On      bool        `json:"on"` // строка открыта в панели подробностей (is-on, s-notes-1)
}
