package view

// Shell — общая часть каждой страницы внутри входа. Встраивается в каждую
// страничную вью-модель и заполняется одинаково всегда.
type Shell struct {
	Title   Text     `json:"title"` // заголовок документа
	Top     TopBar   `json:"top"`
	Rail    Rail     `json:"rail"`
	Notices []Notice `json:"notices"` // над содержимым, не поверх него
	Details *Details `json:"details"` // панель подробностей; nil — панели на экране нет
	Print   Print    `json:"print"`
}

type TopBar struct {
	Search SearchBox `json:"search"`
	// New — «New note», основная кнопка со значком i-quill, и её стрелка
	// (s-notes-7): Note, повторяющий кнопку, за чертой Contact, Event, Task,
	// за чертой Upload file, за чертой Letter — когда письмо можно отправить
	New    Action     `json:"new"`
	Signed SignedIn   `json:"signed"` // кто вошёл и куда — первой строкой меню учётной записи
	User   []MenuItem `json:"user"`   // меню учётной записи
	// UserHref — куда ведёт кнопка учётной записи без сценариев: страница
	// учётной записи
	UserHref string `json:"userHref"`
	Unlocked *When  `json:"unlocked"` // до какого времени открыта выдача учётных данных
	// Lock — Lock now рядом с меткой «credentials unlocked · 4:12»
	// (s-profile-4, s-plan-1, -2, -4, -5): запирает выдачу этого сеанса до
	// срока, вход остаётся (S-10.7); nil — выдача закрыта
	Lock    *Action `json:"lock"`
	Refresh Refresh `json:"refresh"`
}

// SignedIn — кто вошёл и на каком экземпляре. Экземпляров у человека
// бывает несколько — домашний, рабочий, местный, — и на экране они
// ничем больше не различаются. Это сведения, а не действие: нажать не на что.
type SignedIn struct {
	Login    Text `json:"login"`    // имя, под которым вошли
	Instance Text `json:"instance"` // имя экземпляра, S-26
	// Local — экземпляр локальный: адреса, которые он выдаёт, отвечают только
	// изнутри своей сети (значок «local» у davloom, выданных ссылок, ссылки
	// устройства и отметки «On a clock of your own» — s-davloom-1,
	// s-publish-1, s-plan-1, s-backup-2). Объявленного адреса снаружи
	// (CARREL_PUBLIC_URL) нет или он — имя либо адрес частной сети
	Local bool `json:"local"`
}

type Refresh struct {
	Action   Action `json:"action"`
	Answered When   `json:"answered"` // когда закончился последний веерный опрос этого раздела
	Running  bool   `json:"running"`  // опрос идёт: подпись говорит это, кнопка не повторяет его
}

type SearchBox struct {
	Query   Text               `json:"query"`
	Action  string             `json:"action"`
	Name    string             `json:"name"`    // имя поля запроса формы GET на Action: «q»
	History []SearchHistoryRow `json:"history"` // C-6.6
	Jump    []JumpRow          `json:"jump"`    // подсказки из памяти сеанса: ни одного запроса
	Whole   *Action            `json:"whole"`   // «Timeline · имя»; nil — контакт не совпал
}

type Rail struct {
	Nav      []NavRow      `json:"nav"` // разделы со значками; первым списком, над коллекциями
	Sections []RailSection `json:"sections"`
	// Neighbours — соседние заметки тетради на странице заметки, чтение и
	// правка (s-notes-2, s-notes-20), в порядке списка тетради; On — открытая.
	// Берутся из списка, уже загруженного сеансом: не загружен — пусто.
	Neighbours []Action `json:"neighbours"`
	Tags       []Tag    `json:"tags"` // фильтр по категориям; у файлов пуст
	// Shortcuts — рубрика «Shortcuts» рейла файлов (s-files-1, s-files-6):
	// Attachments и Published на каждом экране файлов; у прочих разделов пусто
	Shortcuts []Shortcut `json:"shortcuts"`
	Foot      []Action   `json:"foot"` // New collection, Connect account, Subscribe to a feed
	// Ticks — форма отметок (U-8): POST /app/sources/{kind}, скрытые _csrf и
	// back — экран, куда вернуться. Галка строки — кнопка отправки этой
	// формы: поле toggle, значение — SourceRow.Href. Сервер переключает одну
	// эту коллекцию; строка единого вида отмечает все, а все отмеченные —
	// снимает. nil — отметки рейла ничего не шлют: рейл возврата, настройки
	Ticks *Form     `json:"ticks"`
	About RailAbout `json:"about"` // название и сборка — последней строкой (C-13)
	// Davloom — подпись знака davloom в подвале (components-36): «davloom ·
	// running · 3 devices»; пусто — режим выключен
	Davloom Text `json:"davloom"`
	Note    Text `json:"note"` // строка состояния под списком
	// Status — блок состояния в подвале рейла (components-35): у рейла
	// настроек — davloom и копии, у администрирования — почта и проверка
	// установки; у прочих рейлов пусто и в ответе нет
	Status []RailStatus `json:"status,omitempty"`
}

// RailStatus — одна строка блока состояния: точка тоном, заголовок и строка
// под ним — «davloom · running · 3 devices» / «Last response
// ‹2026-08-16 09:41›», «Backups are running» / «Last backup ‹…› ·
// verified», «Mail is going out» / «Last sent ‹…›», «Checks passed» /
// «Last check ‹…›»; время — моноширинно
type RailStatus struct {
	Tone  StatusTone `json:"tone"` // C-8
	Title Text       `json:"title"`
	Line  Runs       `json:"line"`
}

// RailAbout — последняя строка рейла: название и сборка порознь, сборка
// моноширинно (s-contacts-1). Ведёт на /about.
type RailAbout struct {
	Action      // Label — название, «Carrel»
	Build  Text `json:"build"` // «0.0.1 · 786949a»; пусто у сборки из исходников
}

type NavRow struct {
	Action      // On — открытый раздел; на выдаче поиска не отмечен ни один
	Count  *int `json:"count"` // nil — счёт этому разделу не считается: прочерк, не ноль
}

// Shortcut — строка рубрики Shortcuts: место и справа то, что о нём сказано
// в кадре — где лежит папка вложений или сколько выдано ссылок.
type Shortcut struct {
	Action      // On — это место открыто
	Place  Text `json:"place"` // где лежит: «Documents / carrel», без папки вложений «not set» (s-files-18); у Published пусто
	Count  *int `json:"count"` // у Published — выданные ссылки; nil — счёта у строки нет
}

type RailSection struct {
	Account   Text            `json:"account"`
	AccountID string          `json:"accountID"`
	State     CollectionState `json:"state"` // недоступен — помечается здесь, C-8
	// Feeds — секция подписок: ленты, прочитанные снаружи; кадр ставит у
	// рубрики бейдж «external» (s-agenda-1, s-contacts-7)
	Feeds bool        `json:"feeds,omitempty"`
	Rows  []SourceRow `json:"rows"`
}

type SourceRow struct {
	Href     string          `json:"href"`
	Label    Text            `json:"label"`
	Colour   Colour          `json:"colour"`
	Count    *int            `json:"count"` // nil — коллекция снята или ещё не отвечала: прочерк, не ноль
	Tick     TickState       `json:"tick"`  // C-8
	ReadOnly bool            `json:"readOnly"`
	On       bool            `json:"on"`   // открытая: её список на экране или запись из неё (is-on)
	Feed     bool            `json:"feed"` // подписка — лента, прочитанная снаружи (is-ext)
	State    CollectionState `json:"state"`
	// Detail — не ответила: почему, полностью — подсказка значка ошибки
	// строки, «401 Unauthorized — the saved password was refused. Open
	// Connections to update it.»; ни слов рядом с именем, ни Retry в рейле
	// нет (решение владельца 02.10.2026: отказ целиком и его Retry — в меню
	// «N failed» полосы хода, PollBar.Failed, states-16)
	Detail Text       `json:"detail"`
	Menu   []MenuItem `json:"menu"`
}

// Details — панель у списка: запись для чтения, как её рисуют кадры панелей
// (s-contacts-1, s-agenda-1, s-tasks-1, s-notes-1). Типы блоков — те же, что
// у страницы записи.
type Details struct {
	Title    Text `json:"title"`
	Subtitle Text `json:"subtitle"` // под заголовком: откуда запись, у заметки — и когда
	// Date — у заметки день и время, когда они у неё есть, моноширинно после
	// Subtitle (s-notes-1), как NoteMeta.Date страницы; у прочих пусто
	Date      Text          `json:"date"`
	Avatar    *Avatar       `json:"avatar"`    // контакт: фото или инициалы; у других родов nil
	Fields    []Field       `json:"fields"`    // категории — строкой «Tags» с Field.Tags
	Body      Text          `json:"body"`      // описание события и задачи, как есть
	Rendered  []Block       `json:"rendered"`  // текст заметки, разобранный (C-6.9)
	Attendees []AttendeeRow `json:"attendees"` // участники события, только чтение
	Attach    []AttachRow   `json:"attach"`
	// AttachHint — строка под вложениями задачи (s-tasks-1): до какого
	// объёма файл пишется в саму запись, а больше — в папку вложений, «…up
	// to 2 MB the file is written into the task itself…» (порог — настройка
	// экземпляра, S-20); у прочих родов пусто
	AttachHint Text          `json:"attachHint,omitempty"`
	Related    []TimelineRow `json:"related"`
	Foreign    []Field       `json:"foreign"` // контакт: чужие свойства, как их прислал сервер
	// ForeignShow — «Show 3», кнопка, раскрывающая Foreign: без сценариев —
	// ссылка на тот же список с этой панелью и ?foreign=1; nil — их нет или
	// они раскрыты
	ForeignShow *Action     `json:"foreignShow"`
	Books       []Field     `json:"books"` // контакт: тот же человек в других книгах (s-contacts-7)
	Source      SourceBlock `json:"source"`
	Actions     []Action    `json:"actions"`    // кнопки на виду; удаление — Danger, внизу панели
	Menu        []MenuItem  `json:"menu"`       // «⋯» панели, в порядке кадра
	DeleteNote  Text        `json:"deleteNote"` // строка у удаления: у серии «Series is edited and deleted whole.»
	// Close — крестик без сценариев: тот же список без панели; пусто —
	// панель пришла фрагментом (C-7.2)
	Close string `json:"close"`
}

type Field struct {
	Label Text `json:"label"`
	Value Text `json:"value"`
	// Href — значение ведёт (FieldsItem.Href, s-contacts-6): телефон — tel:
	// из цифр и плюса, почта — mailto:, другая книга — карточка там
	Href string `json:"href"`
	Mono bool   `json:"mono"` // путь, идентификатор, диагностика; у контакта — телефоны и день рождения
	// Aside — приглушённо после значения: «in 7 months» у дня рождения
	Aside Text `json:"aside,omitempty"`
	// Tags — строка категорий метками; Value пуст
	Tags []Tag `json:"tags,omitempty"`
	// Colour — метка коллекции у строки другой книги или календаря (Books, Linked)
	Colour Colour `json:"colour,omitempty"`
	// Runs — значение кусками, когда кадр выделяет часть (s-agenda-2: When —
	// «‹10:00 – 11:00›, Monday 10 August»); Value тогда пуст
	Runs Runs `json:"runs,omitempty"`
	// Badge — бейдж после значения: «created 12:00 Asia/Yekaterinburg» у When
	// события из другого пояса (подсказка «Created in another time zone» —
	// слово экрана), «until 2026-12-28» у Repeats (s-agenda-1, -2)
	Badge Badge `json:"badge,omitzero"`
}

// Print — колонтитул распечатки, C-10.
type Print struct {
	Title Text `json:"title"`
	Range Text `json:"range"`
	Taken When `json:"taken"`
}
