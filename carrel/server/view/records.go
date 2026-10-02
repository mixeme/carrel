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
	// ForeignShow — «Show 3», кнопка, раскрывающая Foreign (свёрнуты): без
	// сценариев — ссылка на ту же карточку с ?foreign=1; nil — их нет или
	// они раскрыты
	ForeignShow *Action       `json:"foreignShow"`
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
	UID      string  `json:"uid"`
	Name     Text    `json:"name"`
	Subtitle Text    `json:"subtitle"` // где и кем, как у карточки
	Linked   *Badge  `json:"linked"`   // «in 2 books»; nil — человек в одной книге
	Card     Details `json:"card"`     // карточка стоит в панели как контекст экрана, а не как выделение
	// CardClosed — панель закрыта крестиком без сценариев (Card.Close,
	// ?card=0): Card пуст, панели нет; прочие ссылки экрана открывают её снова
	CardClosed bool            `json:"cardClosed"`
	Bar        ListBar         `json:"bar"`
	Range      Range           `json:"range"` // окно на полосе: From, To и Show в Form; «Widen the range» отвечают им
	Poll       PollBar         `json:"poll"`  // хронология — поиск по одному человеку (S-15.4)
	Groups     []TimelineGroup `json:"groups"`
	Note       Text            `json:"note"` // «Nothing was looked for before …»
	Actions    []Action        `json:"actions"`
}

// TimelineGroup — месяц хронологии; записи без даты — последней группой.
type TimelineGroup struct {
	Rubric Text          `json:"rubric"`
	Count  int           `json:"count"` // сколько в ней записей
	Rows   []TimelineRow `json:"rows"`
}

// TimelineRow — хронология человека: события, задачи и заметки, где он
// участник или организатор или с которыми связана его карточка
// (RELATED-TO), и файлы с его именем в названии (S-15.4). Упоминание —
// другая функция («Упоминания», раздел 7.23 макета), сюда не входит.
type TimelineRow struct {
	Href string `json:"href"`
	// Target — details у строки хронологии: карточка записи в панели
	// (s-contacts-7); у связанной записи карточки и панели пусто — переход
	Target string `json:"target,omitempty"`
	Colour Colour `json:"colour"` // метка коллекции записи
	Kind   Kind   `json:"kind"`
	When   Text   `json:"when"`
	Title  Text   `json:"title"`
	Stamp  Stamp  `json:"stamp"`
	Why    Text   `json:"why"` // «attendee · accepted», «organiser», «related to this person», «name in the file name» — колонка обязательна
}

type ContactEditPage struct {
	Shell
	UID      string      `json:"uid"`
	Name     Text        `json:"name"`     // заголовок: имя человека, у новой — «New contact»
	Subtitle Text        `json:"subtitle"` // книга: «nextcloud · Контакты»
	Form     Form        `json:"form"`
	Photo    PhotoBlock  `json:"photo"`
	Loss     *LossReport `json:"loss"` // C-9
	ReadOnly bool        `json:"readOnly"`
}

// PhotoBlock — фотография формы контакта. У новой формы (s-contacts-27) — как
// у карточки без фото: Add и строка форматов; когда кадр выбран
// (…/new/photo-preview, C-5.4) — он сам, Replace, Crop, Remove.
type PhotoBlock struct {
	Href     string   `json:"href"`
	Initials Text     `json:"initials"`
	Editable bool     `json:"editable"`
	Note     Text     `json:"note"`
	Actions  []Action `json:"actions"`
}

type PhotoPreviewPage struct {
	Shell
	Href    string `json:"href"` // предпросмотр текущего кадра
	Zoom    int    `json:"zoom"`
	OffsetX int    `json:"offsetX"`
	OffsetY int    `json:"offsetY"`
	Rotate  int    `json:"rotate"`
	// Saved — что будет записано в карточку: «512 × 512 · JPEG · about 90
	// KB» (s-contacts-5)
	Saved   Text     `json:"saved"`
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
	// Attendees — люди события: организатор первым (Status «organiser»),
	// за ним участники; только чтение, без согласования
	Attendees []AttendeeRow `json:"attendees"`
	Attach    []AttachRow   `json:"attach"`
	Related   []TimelineRow `json:"related"`
	Linked    []Field       `json:"linked"` // та же встреча в других календарях (S-16)
	Source    SourceBlock   `json:"source"`
	Loss      *LossReport   `json:"loss"`
	Actions   []Action      `json:"actions"`  // на виду: правка, вложение, новая задача, удаление
	Menu      []MenuItem    `json:"menu"`     // «⋯»: Move, Copy, Export .ics, Publish a link
	ReadOnly  bool          `json:"readOnly"` // чужая коллекция: правки нет вовсе, а не заблокированная форма
}

// EventEditPage — правка: та же запись формой.
type EventEditPage struct {
	Shell
	UID      string `json:"uid"`
	Name     Text   `json:"name"`     // заголовок: название события, у нового — «New event» (s-agenda-3)
	Subtitle Text   `json:"subtitle"` // календарь: «nextcloud · Личный»
	// Form — поля, напоминания строками reminder.N, повторение (Advanced
	// RRULE пуст, правило — его образец)
	Form      Form          `json:"form"`
	Attendees []AttendeeRow `json:"attendees"` // организатор первым; только чтение (U-6.2)
	Attach    []AttachRow   `json:"attach"`
	// Foreign — «Kept as the server sent them»: свойства, которых Carrel не
	// знает, имя и значение как есть, моноширинно (s-agenda-3); пусто — нет
	Foreign    []Field     `json:"foreign"`
	Loss       *LossReport `json:"loss"`
	Delete     *Action     `json:"delete"`     // внизу формы; nil — у новой записи
	DeleteNote Text        `json:"deleteNote"` // строка под удалением: «Series is edited and deleted whole.»
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

// AttendeeRow — участник: адрес (ATTENDEE, ORGANIZER) с именем и статусом;
// книги только говорят, кто это (задача 77, s-agenda-1, -2).
type AttendeeRow struct {
	Name   Text `json:"name"`
	Email  Text `json:"email"`
	Status Text `json:"status"`
	// Href — карточка человека, найденная по адресу в отмеченных книгах,
	// чьи списки сеанс читал; пусто — незнакомый: экран рисует адрес
	Href string `json:"href"`
	// Colour — метка книги этой карточки; пусто у незнакомого
	Colour Colour `json:"colour"`
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
