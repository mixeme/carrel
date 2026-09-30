package view

// У записи два адреса, и первый из них читается. Панель подробностей адреса
// не имеет и заменой чтению не служит.

type ContactPage struct {
	Shell
	UID       string  `json:"uid"`
	Name      Text    `json:"name"`
	Subtitle  Text    `json:"subtitle"` // «Издательство «Слово» · Выпускающий редактор» — где и кем
	Avatar    Avatar  `json:"avatar"`
	PhotoHref string  `json:"photoHref"` // пусто — фотографии нет
	PhotoNote Text    `json:"photoNote"` // «внешняя ссылка, правка недоступна»
	Fields    []Field `json:"fields"`    // категории — строкой «Tags» между Birthday и Note
	Foreign   []Field `json:"foreign"`   // чужие свойства, показанные как есть
	// ForeignShow — «Show 3», кнопка, раскрывающая Foreign; пусто — их нет
	ForeignShow Text          `json:"foreignShow"`
	Books       []Field       `json:"books"`    // тот же человек в других книгах: книга, чем разнится, ссылка, цвет
	Timeline    []TimelineRow `json:"timeline"` // врезка: несколько последних, не вся хронология
	// TimelineCount — сколько всего нашла хронология этой сессии; nil — её не открывали
	TimelineCount *int        `json:"timelineCount"`
	More          Action      `json:"more"` // «Open the timeline» — ведёт на /timeline
	Source        SourceBlock `json:"source"`
	Actions       []Action    `json:"actions"` // кнопки на виду; удаление — Danger, клиент ставит его внизу
	Menu          []MenuItem  `json:"menu"`    // «⋯»: Move, Copy, Export .vcf, Publish a link
	ReadOnly      bool        `json:"readOnly"`
}

// ContactTimelinePage — лента человека: свой экран, потому что это список
// записей, а не карточка.
type ContactTimelinePage struct {
	Shell
	UID      string          `json:"uid"`
	Name     Text            `json:"name"`
	Subtitle Text            `json:"subtitle"` // где и кем, как у карточки
	Linked   *Badge          `json:"linked"`   // «in 2 books»; nil — человек в одной книге
	Card     Details         `json:"card"`     // карточка стоит в панели как контекст экрана, а не как выделение
	Bar      ListBar         `json:"bar"`
	Range    Range           `json:"range"` // окно на полосе: From, To и Show в Form; «Widen the range» отвечают им
	Poll     PollBar         `json:"poll"`  // хронология — поиск по одному человеку (S-15.4)
	Groups   []TimelineGroup `json:"groups"`
	Note     Text            `json:"note"` // «Nothing was looked for before …»
	Actions  []Action        `json:"actions"`
}

// TimelineGroup — месяц хронологии; записи без даты — последней группой.
type TimelineGroup struct {
	Rubric Text          `json:"rubric"`
	Count  int           `json:"count"` // сколько в ней записей
	Rows   []TimelineRow `json:"rows"`
}

// TimelineRow — хронология человека: события, задачи, заметки и файлы, где
// он упомянут.
type TimelineRow struct {
	Href   string `json:"href"`
	Colour Colour `json:"colour"` // метка коллекции записи
	Kind   Kind   `json:"kind"`
	When   Text   `json:"when"`
	Title  Text   `json:"title"`
	Stamp  Stamp  `json:"stamp"`
	Why    Text   `json:"why"` // «участник · приняла», «упомянута в тексте» — колонка обязательна
}

type ContactEditPage struct {
	Shell
	UID      string      `json:"uid"`
	Form     Form        `json:"form"`
	Photo    PhotoBlock  `json:"photo"`
	Loss     *LossReport `json:"loss"` // C-9
	ReadOnly bool        `json:"readOnly"`
}

type PhotoBlock struct {
	Href     string   `json:"href"`
	Initials Text     `json:"initials"`
	Editable bool     `json:"editable"`
	Note     Text     `json:"note"`
	Actions  []Action `json:"actions"`
}

type PhotoPreviewPage struct {
	Shell
	Href    string   `json:"href"` // предпросмотр текущего кадра
	Zoom    int      `json:"zoom"`
	OffsetX int      `json:"offsetX"`
	OffsetY int      `json:"offsetY"`
	Rotate  int      `json:"rotate"`
	Actions []Action `json:"actions"`
}

// EventPage — чтение: то, что открывается по ссылке и печатается.
type EventPage struct {
	Shell
	UID        string          `json:"uid"`
	Summary    Text            `json:"summary"` // заголовок записи; документа — Shell.Title
	Fields     []Field         `json:"fields"`
	Body       Text            `json:"body"`
	Recurrence RecurrenceBlock `json:"recurrence"`
	Reminders  []ReminderRow   `json:"reminders"`
	Attendees  []AttendeeRow   `json:"attendees"` // только чтение, без согласования
	Attach     []AttachRow     `json:"attach"`
	Related    []TimelineRow   `json:"related"`
	Linked     []Field         `json:"linked"` // та же встреча в других календарях (S-16)
	Source     SourceBlock     `json:"source"`
	Loss       *LossReport     `json:"loss"`
	Actions    []Action        `json:"actions"`  // на виду: правка, вложение, новая задача, удаление
	Menu       []MenuItem      `json:"menu"`     // «⋯»: Move, Copy, Export .ics, Publish a link
	ReadOnly   bool            `json:"readOnly"` // чужая коллекция: правки нет вовсе, а не заблокированная форма
}

// EventEditPage — правка: та же запись формой.
type EventEditPage struct {
	Shell
	UID        string          `json:"uid"`
	Form       Form            `json:"form"`
	Recurrence RecurrenceBlock `json:"recurrence"`
	Reminders  []ReminderRow   `json:"reminders"`
	Attendees  []AttendeeRow   `json:"attendees"`
	Attach     []AttachRow     `json:"attach"`
	Loss       *LossReport     `json:"loss"`
	Delete     *Action         `json:"delete"`     // внизу формы; nil — у новой записи
	DeleteNote Text            `json:"deleteNote"` // строка под удалением: «Series is edited and deleted whole.»
}

type RecurrenceBlock struct {
	Summary Text    `json:"summary"` // человеческое описание правила
	Raw     Text    `json:"raw"`     // сырое правило для тех, кто его читает
	Fields  []Field `json:"fields"`
	Note    Text    `json:"note"` // «Series is edited and deleted whole.»; на правке — пусто, строка в DeleteNote
}

type ReminderRow struct {
	Label  Text    `json:"label"`
	Fields []Field `json:"fields"`
	Remove Action  `json:"remove"`
}
type AttendeeRow struct {
	Name   Text `json:"name"`
	Email  Text `json:"email"`
	Status Text `json:"status"`
}
type AttachRow struct {
	Href    string      `json:"href"`
	Name    Text        `json:"name"`
	Size    Size        `json:"size"`
	Type    Text        `json:"type"`
	Inline  bool        `json:"inline"` // данные внутри записи, а не файл в папке (S-20)
	State   RecordState `json:"state"`
	Actions []Action    `json:"actions"`
}

// TaskPage — чтение.
type TaskPage struct {
	Shell
	UID       string        `json:"uid"`
	Summary   Text          `json:"summary"` // заголовок записи; документа — Shell.Title
	Fields    []Field       `json:"fields"`
	Body      Text          `json:"body"`
	Done      bool          `json:"done"`
	Percent   *int          `json:"percent"`
	Reminders []ReminderRow `json:"reminders"`
	Attach    []AttachRow   `json:"attach"`
	Related   []TimelineRow `json:"related"`
	Linked    []Field       `json:"linked"` // та же задача в других списках (S-16)
	Source    SourceBlock   `json:"source"`
	Loss      *LossReport   `json:"loss"`
	Actions   []Action      `json:"actions"` // на виду: правка, отметка, вложение, удаление
	Menu      []MenuItem    `json:"menu"`    // «⋯»: Move, Copy, Export .ics, Publish a link
	ReadOnly  bool          `json:"readOnly"`
}

// TaskEditPage — правка.
type TaskEditPage struct {
	Shell
	UID       string        `json:"uid"`
	Form      Form          `json:"form"`
	Done      bool          `json:"done"`
	Percent   *int          `json:"percent"`
	Reminders []ReminderRow `json:"reminders"`
	Attach    []AttachRow   `json:"attach"`
	Loss      *LossReport   `json:"loss"`
	Delete    *Action       `json:"delete"` // внизу формы, одна кнопка без строки под ней; nil — у новой записи
}

type NotePage struct {
	Shell
	UID    string      `json:"uid"`
	Doc    NoteDoc     `json:"doc"`
	Meta   NoteMeta    `json:"meta"`
	Source SourceBlock `json:"source"`
	Loss   *LossReport `json:"loss"`
}

// NoteDoc — фрагмент: чтение или правка.
type NoteDoc struct {
	Mode     NoteMode `json:"mode"` // C-8
	Title    Text     `json:"title"`
	Body     Text     `json:"body"`     // исходный текст, не HTML
	Rendered []Block  `json:"rendered"` // разобранный Markdown, C-6.9
	Bar      []Action `json:"bar"`      // панель правки; отступает по рангу (L-4)
	Dirty    bool     `json:"dirty"`
	ReadOnly bool     `json:"readOnly"`
	// Fields — поля правки вне текста: тетрадь, дата и время, теги, связь
	// (s-notes-20). Только в режиме правки; уходят тем же сохранением, что
	// заголовок и тело.
	Fields []FormRow `json:"fields"`
	// Foreign — строка под текстом правки о свойствах, которых Carrel не
	// знает: первое по имени и сколько ещё, готовой фразой — «Properties
	// Carrel does not know — X-JTX-COLOR and two more — are kept unchanged
	// and not shown here.» (s-notes-20). Хранятся они как есть. Пусто —
	// таких нет или режим чтения.
	Foreign Text `json:"foreign"`
}

type NoteMeta struct {
	Date    Text          `json:"date"` // день и время, когда оно у заметки есть (s-notes-2)
	Tags    []Tag         `json:"tags"`
	Related []TimelineRow `json:"related"`
	Linked  []Field       `json:"linked"` // та же заметка в других тетрадях (S-16)
	Attach  []AttachRow   `json:"attach"`
	Actions []Action      `json:"actions"` // колонка: вложение, удаление
	Menu    []MenuItem    `json:"menu"`    // «⋯» над текстом: Move, Copy, Publish a link
}

type ConflictPage struct {
	Shell
	Kind     Kind          `json:"kind"`
	UID      string        `json:"uid"`
	ReadAt   When          `json:"readAt"`
	ServerAt When          `json:"serverAt"`
	Rows     []ConflictRow `json:"rows"`
	Actions  []Action      `json:"actions"` // Write this combination · Use only my edit · Use only the server version · Cancel
}

// ConflictRow — карточка расхождения, варианты столбиком.
type ConflictRow struct {
	Property Text           `json:"property"`
	Mine     Text           `json:"mine"`
	Theirs   Text           `json:"theirs"`
	Choice   ConflictChoice `json:"choice"` // C-8
}
