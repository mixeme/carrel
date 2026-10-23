package view

// У записи два адреса, и первый из них читается. Панель подробностей адреса
// не имеет и заменой чтению не служит.

type ContactPage struct {
	Shell
	// LinkedRecord — карточка связанной записи (/app/{kind}/linked/{uid},
	// записка Typecase 117 п. 5): выбранное, не одна копия; её места — в
	// Books, а источника, хронологии и удаления у неё нет
	LinkedRecord bool    `json:"linkedRecord,omitempty"`
	UID          string  `json:"uid"`
	Name         Text    `json:"name"`
	Subtitle     Text    `json:"subtitle"` // «Издательство «Слово» · Выпускающий редактор» — где и кем
	Avatar       Avatar  `json:"avatar"`
	PhotoHref    string  `json:"photoHref"` // пусто — фотографии нет
	PhotoNote    Text    `json:"photoNote"` // «внешняя ссылка, правка недоступна»
	Fields       []Field `json:"fields"`    // категории — строкой «Tags» между Birthday и Note
	Foreign      []Field `json:"foreign"`   // чужие свойства, показанные как есть
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
// (…/new/photo-preview, C-5.4) — он сам, Replace, Crop, Remove. Кнопки фото и
// новой, и существующей карточки шлют форму, в которой стоят (Action.Submits).
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
	// LinkedRecord — карточка связанной записи (/app/{kind}/linked/{uid},
	// записка Typecase 117 п. 5): выбранное, не одна копия; её места — в
	// Linked, а источника, хронологии и удаления у неё нет
	LinkedRecord bool   `json:"linkedRecord,omitempty"`
	UID          string `json:"uid"`
	Summary      Text   `json:"summary"` // заголовок записи; документа — Shell.Title
	// Subtitle — у связанного события, открытого из объединённого вида: где
	// оно, «Combined calendar · in 3 calendars» (s-agenda-25); пусто — у
	// события одного календаря
	Subtitle   Text            `json:"subtitle,omitempty"`
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
	Form Form `json:"form"`
	// Attendees — поле участников (s-agenda-3, -14, -15): клиент ставит его
	// набором «Attendees» формы, между Reminders и Attachments
	Attendees AttendeeField `json:"attendees"`
	Attach    []AttachRow   `json:"attach"`
	// Foreign — «Kept as the server sent them»: свойства, которых Carrel не
	// знает, имя и значение как есть, моноширинно (s-agenda-3); пусто — нет
	Foreign    []Field     `json:"foreign"`
	Loss       *LossReport `json:"loss"`
	Delete     *Action     `json:"delete"`     // внизу формы; nil — у новой записи
	DeleteNote Text        `json:"deleteNote"` // строка под удалением: «Series is edited and deleted whole.»
	// Source — рубрика Source формы, как у задачи (s-tasks-3): Path, Read и
	// сравнение с сервером; у новой — без пути (Р18 списка равенства родов)
	Source SourceBlock `json:"source"`
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
	// Value — значение плашки в форме правки, «mailto:…»; на чтении пусто
	Value string `json:"value,omitempty"`
	// Fixed — организатор в своей правке: плашка без крестика, уходит
	// скрытым полем (s-agenda-3)
	Fixed bool `json:"fixed,omitempty"`
}

// AttendeeField — участники в правке события (задача 77, s-agenda-3):
// плашки колонками и строка набора. Форма шлёт организатора скрытым полем
// Name, каждого другого — отмеченным флажком того же Name, и набранное в
// строке (Add.Name): сервер берёт человека, найденного по имени в отмеченных
// книгах, или адрес, иначе ошибка у строки. Форма без строки набора
// участников не меняет. Приглашённый (ReadOnly) видит плашки без крестиков
// и без строки, и почему — Hint (s-agenda-14).
type AttendeeField struct {
	Name string        `json:"name"` // «attendee»
	Rows []AttendeeRow `json:"rows"` // организатор первым; Value и Fixed
	// Add — строка набора: Name «attendee.add», Label «Add attendee»,
	// Placeholder «Name or address», Value — набранное, Error — почему не
	// взято; nil — список меняет только организатор
	Add *FormRow `json:"add"`
	// Suggest — адрес подсказок (C-5, /app/contacts/suggest): набранное
	// уходит в ?q=, ответ — SuggestList (s-agenda-12, -13)
	Suggest string `json:"suggest"`
	// Hint — под полем: где ищется и кто шлёт приглашения — «A name or an
	// address: the ticked address books are searched. On save, nextcloud
	// sends the invitations and their changes — Carrel sends no mail
	// itself.»; у приглашённого — «You are invited by Мария Лебедева: only
	// the organiser changes the list.»
	Hint     Text `json:"hint"`
	ReadOnly bool `json:"readOnly"` // приглашённый или календарь только для чтения
}

// SuggestList — подсказки поля участников (s-agenda-12, -13): ответ на
// /app/contacts/suggest?q=. Ищутся отмеченные книги, чьи списки сеанс
// читал, по имени и по адресу; строка на каждый адрес человека.
type SuggestList struct {
	Items []SuggestItem `json:"items"`
	// Note — под чертой: набранный адрес не из книг — «Not in the ticked
	// address books.»; иначе пусто
	Note Text `json:"note"`
	// Searched, NotAsked — у подсказки письма, когда ответили не все книги
	// (s-mail-3): «Searched 4 of 5 address books» и те, что не ответили
	// («baikal · Офис»); иначе пусто
	Searched Text    `json:"searched,omitempty"`
	NotAsked []Stamp `json:"notAsked,omitempty"`
}

// SuggestItem — одна подсказка: человек одним адресом, или «Add» адреса,
// которого в книгах нет (Add).
type SuggestItem struct {
	Name   Runs   `json:"name"`   // имя, найденное — RunStrong: «‹Ве›ра Лебедева»; у Add пусто
	Email  Text   `json:"email"`  // адрес, на который придёт приглашение
	Book   Text   `json:"book"`   // книга справа: «Архив»; у Add пусто
	Colour Colour `json:"colour"` // метка книги; у Add пусто
	Value  string `json:"value"`  // значение плашки, которую подсказка ставит: «mailto:vera@example.net»; у записи — её UID
	Add    bool   `json:"add"`    // «Add ‹адрес›» (s-agenda-13)
	// Meta — у подсказки Related to: род и день записи справа, «event · 12
	// Aug» (s-tasks-10); у людей пусто
	Meta Text `json:"meta,omitempty"`
	// Label — у подсказки письма: метка адреса из карточки, «work»,
	// «home» (s-mail-3); у прочих пусто
	Label Text `json:"label,omitempty"`
}
type AttachRow struct {
	Href   string `json:"href"`
	Name   Text   `json:"name"`
	Size   Size   `json:"size"`
	Type   Text   `json:"type"`
	Inline bool   `json:"inline"` // данные внутри записи, а не файл в папке (S-20)
	// Place — где лежит файл ссылки (s-notes-1, -2, -20): в папке вложений —
	// где лежит она, как её называет рейл файлов, «Documents / carrel»;
	// иначе в Files — хранилище и папки; у данных внутри записи и у ссылки
	// вне каталога пусто
	Place   Text        `json:"place,omitempty"`
	State   RecordState `json:"state"`
	Actions []Action    `json:"actions"`
}

// TaskPage — чтение.
type TaskPage struct {
	Shell
	// LinkedRecord — карточка связанной записи (/app/{kind}/linked/{uid},
	// записка Typecase 117 п. 5): выбранное, не одна копия; её места — в
	// Linked, а источника, хронологии и удаления у неё нет
	LinkedRecord bool   `json:"linkedRecord,omitempty"`
	UID          string `json:"uid"`
	Summary      Text   `json:"summary"` // заголовок записи; документа — Shell.Title
	// Subtitle — под заголовком: где задача и, когда она просрочена, на
	// сколько — тревогой, «nextcloud · Дела · ‹overdue by 3 days›»
	// (s-tasks-2)
	Subtitle Runs `json:"subtitle"`
	// Fields — в порядке кадра (s-tasks-2): Status (без STATUS — «Needs
	// action»), Due (Runs: срок моноширинно, просрочка тревогой), Starts,
	// Complete («0%», моноширинно), Priority (моноширинно), Reminders, Where,
	// Link, Tags
	Fields []Field `json:"fields"`
	Body   Text    `json:"body"`
	Done   bool    `json:"done"`
	// Attendees — люди задачи, как у события: организатор первым (Status
	// «organiser»), за ним участники; только чтение (Р3 списка равенства
	// родов; кадра нет). Организатор — не строка Fields
	Attendees []AttendeeRow `json:"attendees"`
	Attach    []AttachRow   `json:"attach"`
	Related   []TimelineRow `json:"related"`
	Linked    []Field       `json:"linked"` // та же задача в других списках (S-16)
	Source    SourceBlock   `json:"source"`
	Loss      *LossReport   `json:"loss"`
	// Actions — на виду: Edit (i-pencil), Mark done, Attach, Print
	// (record.print); удаление — Danger, внизу
	Actions  []Action   `json:"actions"`
	Menu     []MenuItem `json:"menu"` // «⋯»: Move, Copy, Export .ics, Publish a link
	ReadOnly bool       `json:"readOnly"`
}

// TaskEditPage — правка.
type TaskEditPage struct {
	Shell
	UID  string `json:"uid"`
	Name Text   `json:"name"` // заголовок: название задачи, у новой — «New task» (s-tasks-3)
	// Subtitle — список и просрочка тревогой: «nextcloud · Дела ·
	// ‹overdue by 3 days›» (s-tasks-3)
	Subtitle Runs `json:"subtitle"`
	// Form — поля в рубриках кадра: выполнение (Done, Percent complete) и
	// напоминания (reminder.N) — его строки
	Form Form `json:"form"`
	// Attendees — поле участников, как у правки события (Р3 списка равенства
	// родов; кадра нет): форма без строки набора участников не меняет
	Attendees AttendeeField `json:"attendees"`
	Attach    []AttachRow   `json:"attach"`
	// Source — рубрика Source формы (s-tasks-3): Path, Read и сравнение с
	// сервером; у новой — без пути
	Source SourceBlock `json:"source"`
	Loss   *LossReport `json:"loss"`
	Delete *Action     `json:"delete"` // внизу формы, одна кнопка без строки под ней; nil — у новой записи
	// Foreign — «Kept as the server sent them», как у события (s-agenda-3):
	// свойства задачи, которых Carrel не знает, имя и значение как есть,
	// моноширинно; пусто — нет (Р17 списка равенства родов)
	Foreign []Field `json:"foreign"`
}

type NotePage struct {
	Shell
	// LinkedRecord — карточка связанной записи (/app/{kind}/linked/{uid},
	// записка Typecase 117 п. 5): выбранное, не одна копия; её места — в
	// Meta.Linked, а источника, хронологии и удаления у неё нет
	LinkedRecord bool   `json:"linkedRecord,omitempty"`
	UID          string `json:"uid"`
	// Subtitle — у связанной заметки, открытой из объединённого вида: где
	// она, «All notebooks · in 2 notebooks», и — когда текст выбирали —
	// «· text from», а за ним коллекция текста TextFrom, меткой и именем
	// (s-notes-32); пусто — у заметки одной тетради
	Subtitle Text        `json:"subtitle,omitempty"`
	TextFrom *FieldFrom  `json:"textFrom"`
	Doc      NoteDoc     `json:"doc"`
	Meta     NoteMeta    `json:"meta"`
	Source   SourceBlock `json:"source"`
	Loss     *LossReport `json:"loss"`
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
	// Form — правка без сценариев (U-8, s-notes-20): документ — форма. Action
	// и Method — сохранение (POST на адрес правки, у новой — /new); строки —
	// скрытые _csrf, etag, readat, затем заголовок (summary, «Title») и текст
	// (description, «Body», Focus) с теми же значениями, что Title и Body;
	// Fields уходят с этой же формой; Save — в Bar (Submits). nil — чтение
	Form *Form `json:"form"`
	// Fields — поля правки вне текста: тетрадь, дата и время, теги, связь
	// (s-notes-20). Только в режиме правки; уходят тем же сохранением, что
	// заголовок и тело.
	Fields []FormRow `json:"fields"`
	// Foreign — строка под текстом правки о свойствах, которых Carrel не
	// знает: первое по имени и сколько ещё, готовой фразой — «Properties
	// Carrel does not know — X-JTX-COLOR and two more — are kept unchanged
	// and not shown here.» (s-notes-20), имя свойства — моноширинным
	// куском. Хранятся они как есть. Пусто — таких нет или режим чтения.
	Foreign Runs `json:"foreign"`
}

type NoteMeta struct {
	Date Text  `json:"date"` // день и время, когда оно у заметки есть (s-notes-2)
	Tags []Tag `json:"tags"`
	// TagsBadge — бейдж у рубрики Tags: «linked», когда другая копия
	// связанной заметки держит другие теги — здесь мог быть выбор (s-notes-2,
	// -3, -27; Р-58 Typecase); nil — бейджа нет
	TagsBadge *Badge `json:"tagsBadge"`
	// TagsFrom — у связанной заметки: коллекция, откуда взят выбранный тег
	// (s-notes-32, «Дневник»); nil — выбора не было
	TagsFrom *FieldFrom `json:"tagsFrom"`
	// Link — ссылка заметки (URL): «Link», значение и Href, как у карточки
	// события и задачи (Р8 списка равенства родов; кадра нет); nil — нет
	Link *Field `json:"link"`
	// Attendees — люди заметки, как у события и задачи: организатор первым,
	// за ним участники; только чтение (Р3 списка равенства родов; кадра нет)
	Attendees []AttendeeRow `json:"attendees"`
	Related   []TimelineRow `json:"related"`
	Linked    []Field       `json:"linked"` // та же заметка в других тетрадях (S-16)
	Attach    []AttachRow   `json:"attach"`
	Actions   []Action      `json:"actions"` // колонка: вложение, удаление
	Menu      []MenuItem    `json:"menu"`    // «⋯» над текстом: Move, Copy, Publish a link
}

type ConflictPage struct {
	Shell
	Kind Kind   `json:"kind"`
	UID  string `json:"uid"`
	// Subtitle — что случилось, строкой под заголовком (s-conflict-1):
	// «Another client changed this contact while you were editing it.
	// Nothing has been overwritten.»; у удалённой — «Another client deleted
	// this contact while you were editing it. Nothing has been written.»
	// (s-conflict-7); пусто — сервер не ответил (s-conflict-10)
	Subtitle Text `json:"subtitle"`
	// Ask — что сделать, второй строкой под заголовком, когда заранее не
	// выбрано ничего: «Choose a side for every property.» (s-conflict-6),
	// «Choose a side.» (-7); пусто — выбор сделан заранее
	Ask Text `json:"ask"`
	// Deleted — запись удалили на сервере, пока её правили (Р-67,
	// s-conflict-7): одна карточка — имя записи; mine пишет её заново с
	// правкой, theirs оставляет удалённой; варианты «Your edit · write it
	// back» и «Server · leave it deleted» — слова экрана
	Deleted  bool          `json:"deleted"`
	ReadAt   When          `json:"readAt"`
	ServerAt When          `json:"serverAt"`
	Rows     []ConflictRow `json:"rows"`
	// Agree — свойства, что человек изменил так же, как сервер: «‹DTSTART›
	// agrees — kept as it is.» над карточками (s-conflict-3, -4, -5; слова —
	// экрана)
	Agree []Text `json:"agree"`
	// Form — выбор по строкам без сценариев (U-8, s-conflict-1): POST
	// …/conflict, скрытые _csrf, etag показанной версии и op=combine;
	// строки шлют Name своей ConflictRow со значением mine или theirs (C-8
	// ConflictChoice), а Actions — «Write this combination» («Write this
	// choice» у удалённой), её отправка; ждёт (Action.Waits), пока у
	// карточки без выбора заранее не выбрана сторона. nil — писать нечего:
	// запись не прочлась
	Form *Form `json:"form"`
	// Actions — Use only my edit (Use only my form, когда чья правка не
	// известна, s-conflict-6) · Use only the server version · Cancel:
	// каждая своей формой (POST с op и etag в адресе); «Write this
	// combination» — кнопка Form, рисуется первой (s-conflict-1). У
	// удалённой и непрочтённой — только Cancel. Расходиться нечему (Р-69) —
	// экрана нет: черновик сброшен, GET ведёт на карточку
	Actions []Action `json:"actions"`
}

// ConflictRow — карточка расхождения, варианты столбиком.
type ConflictRow struct {
	Property Text `json:"property"`
	Mine     Text `json:"mine"`
	Theirs   Text `json:"theirs"`
	// Choice — что выбрано заранее (радио отмечено, C-8); его же сервер
	// берёт, когда поле строки не пришло
	Choice ConflictChoice `json:"choice"`
	// Name — поле формы строки, choice.<номер строки>: радио «Your edit» —
	// значение mine, «Server» — theirs (ConflictPage.Form)
	Name string `json:"name"`
	// Diff — вид расхождения (C-8): both — изменили обе стороны, mine —
	// только правка (сервер держит то, что было открыто); пусто — версии,
	// с которой открыли форму, уже нет (s-conflict-1…-5: «both changed»,
	// «only in your edit»; s-conflict-6: вида нет, вариант — «Your form»).
	// Карточка без выбора заранее (Choice пуст) обязательна: форму без
	// её стороны браузер не отправит (Check.Required)
	Diff ConflictDiff `json:"diff"`
}
