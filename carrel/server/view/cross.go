package view

type SearchPage struct {
	Shell
	Query Text `json:"query"`
	// Subtitle — под заголовком: что спрошено, сколько найдено, кто не
	// ответил — «14 of 14 collections · 7 records · 1 source timed out»
	// (s-find-1, Пк5); пусто без запроса
	Subtitle Text      `json:"subtitle"`
	Bar      ListBar   `json:"bar"` // сегменты All / Contacts / Events / Tasks / Notes / Files
	Poll     PollBar   `json:"poll"`
	Results  []FindRow `json:"results"`
	// Note — «содержимое файлов не ищется», кусками: PROPFIND моноширинно
	// (s-find-1, Пк14); пусто, когда файлы не спрошены
	Note Runs `json:"note"`
}

type FindRow struct {
	Href   string `json:"href"`
	Kind   Kind   `json:"kind"`
	Colour Colour `json:"colour"`
	Stamp  Stamp  `json:"stamp"`
	Title  Text   `json:"title"`
	// Marked — Title кусками в выдаче поиска: совпавшее с запросом — strong,
	// без учёта регистра (s-find-1: «**Ковал**ёва Анна»; Пк13); пусто —
	// найдено не по названию
	Marked Runs `json:"marked,omitempty"`
	Detail Text `json:"detail"`
	When   Text `json:"when"`
	// Строка по роду (s-find-1, Пк12). Контакт: Avatar — инициалы и цвет,
	// Subtitle — телефон (моноширинно), Detail — почта, Caption —
	// организация. Файл: Icon — значок по типу, Size, Changed — изменён
	// числами «2026-08-04 11:20», Access — «rw» или «ro». У прочих пусто
	Avatar   *Avatar `json:"avatar"`
	Subtitle Text    `json:"subtitle,omitempty"`
	Caption  Text    `json:"caption,omitempty"`
	Icon     string  `json:"icon,omitempty"`
	Size     Size    `json:"size,omitempty"`
	Changed  Text    `json:"changed,omitempty"`
	Access   Text    `json:"access,omitempty"`
}

type SearchHistoryRow struct {
	Query   Text   `json:"query"`
	Outcome Text   `json:"outcome"` // «12 found» или «nothing found»
	When    When   `json:"when"`
	Href    string `json:"href"`
}
type JumpRow struct {
	Href  string `json:"href"`
	Kind  Kind   `json:"kind"`
	Title Text   `json:"title"`
	Stamp Stamp  `json:"stamp"`
}

type PollBar struct {
	TaskID string `json:"taskID"`
	// Stream, Poll, Every — адреса хода задачи (C-5.3, C-7.4) с запросом
	// страницы, на которой стоит полоса: поток событий (пусто при
	// CARREL_PROGRESS_MODE=poll), запасной опрос (фрагмент poll) и его
	// интервал в миллисекундах (CARREL_PROGRESS_POLL_MILLIS); у полосы без
	// задачи пусты
	Stream     string `json:"stream"`
	Poll       string `json:"poll"`
	Every      int    `json:"every"`
	Summary    Runs   `json:"summary"`    // «Polled 5 of 7 · 12 records · 1 refused» кусками (s-contacts-7)
	AnsweredAt When   `json:"answeredAt"` // когда собрана показанная выдача
	// Aside — справа на полосе моноширинно: когда собрана выдача, «2 min ago»,
	// и «from cache · » перед этим, когда источник ответил тем, что сеанс
	// уже держал (s-agenda-1); пока опрос идёт — сколько он длится, «0.8 s»,
	// у всех разделов (states-3, -8, -13, -19, -26); у законченного поиска —
	// сколько шёл обход, «1.8 s», а AnsweredAt сегодня — одни часы (s-find-1)
	Aside Text `json:"aside"`
	// Note — под строками, когда источник не ответил: что показано и чего
	// нет — «What arrived in time is shown. Records from the unavailable
	// source are missing — …» (states-6, states-9); пусто — ответили все
	Note    Text             `json:"note"`
	Running bool             `json:"running"`
	Percent int              `json:"percent"`
	Sources []SourceProgress `json:"sources"`
	Actions []Action         `json:"actions"` // Stop · Search again
	// Failed — красная кнопка полосы над списком и хронологией, «1 failed»
	// (решение владельца 02.10.2026, states-16): закрытое меню отказов —
	// строка на каждый источник с Answer, его Retry; у поиска так же
	// (s-find-1, с v0.2.262); пусто — ответили все. Summary отказов не
	// называет
	Failed Text `json:"failed,omitempty"`
	// RetryAll — «Retry all» внизу того же меню, когда отказов больше
	// одного: каждый неответивший источник снова; nil — отказ один или нет
	RetryAll *Action `json:"retryAll"`
}

type SourceProgress struct {
	Stamp     Stamp       `json:"stamp"`
	Colour    Colour      `json:"colour"`
	State     SourceState `json:"state"` // C-8
	Count     *int        `json:"count"`
	FromCache bool        `json:"fromCache"`
	Message   Text        `json:"message"` // краткая причина отказа
	// Answer — не ответил: что спросили и что ответил сервер, целиком, как
	// пишет меню отказов (states-16) — «REPORT /dav/calendars/duty/ → 503
	// Service Unavailable», «PROPFIND /dav.php/calendars/mix/ → 401
	// Unauthorized: the saved password was refused. Open Connections to
	// update it.»; без ответа — где спросили и что ответа не было; у
	// ответившего и остановленного пусто
	Answer Text    `json:"answer,omitempty"`
	Retry  *Action `json:"retry"` // повтор только этого источника
}

type DuplicatesPage struct {
	Shell
	// Subtitle — под заголовком: «27 records compared · 2 to look at»
	// (s-dup-1), «0 records compared» (s-dup-6); групп ждать нечего —
	// только сравнённое
	Subtitle  Text             `json:"subtitle"`
	Groups    []DuplicateGroup `json:"groups"`
	Threshold ThresholdControl `json:"threshold"`
	Bar       ListBar          `json:"bar"`
}

type DuplicateGroup struct {
	ID       string            `json:"id"`
	Kind     Kind              `json:"kind"`
	Score    int               `json:"score"`
	Reasons  []Text            `json:"reasons"` // «тот же адрес», «тот же телефон» — причина рядом со счётом
	Rows     []DuplicateMember `json:"rows"`
	Actions  []Action          `json:"actions"`  // Link · Not duplicates · Merge on the server
	Decision DuplicateDecision `json:"decision"` // C-8
	// Hint — под строками группы: что значит такая пара (s-dup-1, Дб6) —
	// у событий с разными UID «The same event in two calendars, different
	// UIDs. …», у заметок «Same title and the same date: a note copied into
	// a second notebook. …»; пусто — сказать нечего
	Hint Text `json:"hint,omitempty"`
	// Расхождения в однозначных полях: ответ уходит с Link как
	// MergeRow.Name=<номер варианта> и запоминается при связанной группе
	Choices []MergeRow `json:"choices"`
	// Form — карточка группы формой, когда выбирать есть что (U-8, s-dup-1):
	// POST /app/duplicates/decide, скрытые _csrf и group; радио Choices
	// стоят в ней, а решения группы (Actions, POST) — её кнопки на свой
	// адрес (Action.Submits), каждая с do и group в адресе. nil — выбирать
	// нечего: решения уходят каждое своей формой
	Form *Form `json:"form"`
}

type DuplicateMember struct {
	Href     string `json:"href"`
	Stamp    Stamp  `json:"stamp"`
	Colour   Colour `json:"colour"`
	Title    Text   `json:"title"`
	Detail   Text   `json:"detail"`
	ReadOnly bool   `json:"readOnly"`
}

type ThresholdControl struct {
	Value   int             `json:"value"`
	Default int             `json:"default"`
	Note    Text            `json:"note"`
	Weights []WeightControl `json:"weights"` // очки каждого признака; своё у человека, иначе экземпляра
	// Form — поле «Match threshold» без сценариев (U-8, s-dup-1): POST
	// /app/duplicates/decide, скрытые _csrf и do=threshold, строка value
	// (число, моноширинно); уходит по Enter, кнопки у него нет
	Form Form `json:"form"`
	// WeightForm — меню Weights формой (s-dup-3, -7): POST
	// /app/duplicates/decide?do=weights, скрытый _csrf; поля — Weights по
	// их Name. Actions — Save (отправка формы) и Back to defaults — та же
	// форма на свой адрес (Action.Submits, do=defaults): веса экземпляра
	// всем; своих весов нет — выключена с причиной
	WeightForm Form `json:"weightForm"`
}

// WeightControl — очки одного признака дубликата (S-16); 0 выключает
// признак.
type WeightControl struct {
	Signal string `json:"signal"`
	// Name — поле признака в WeightForm: weight.<Signal>; пустое значение —
	// вес экземпляра
	Name    string `json:"name"`
	Label   Text   `json:"label"`
	Value   int    `json:"value"`
	Default int    `json:"default"`
	Own     bool   `json:"own"` // значение человека, а не экземпляра: пометка «changed»
}

type DuplicatesMergePage struct {
	Shell
	// Subtitle — под заголовком: кто и сколько записей — «Вера Лебедева ·
	// 2 records» (s-dup-2, -4); пусто, когда оставляемой записи нет
	Subtitle Text            `json:"subtitle"`
	GroupID  string          `json:"groupID"`
	Target   DuplicateMember `json:"target"`
	Rows     []MergeRow      `json:"rows"`
	Warning  Notice          `json:"warning"` // что будет записано, что удалено
	// План слияния полями (s-dup-2, -4; Дб14, Дб15) — рубрики экрана:
	// Stays — «Where the merged record stays»: источник каждой записи
	// группы, оставляемая — On; другая — этот же экран с ней оставляемой
	// (GET); только для чтения — выключена с причиной
	Stays []Action `json:"stays"`
	// StaysNote — под ним: «Вера Лебедева there is rewritten with the
	// values you chose.»
	StaysNote Text `json:"staysNote"`
	// Deleted — «Deleted afterwards»: название и приглушённо источник
	Deleted []DuplicateMember `json:"deleted"`
	// Gains — «The record that stays gains»: «EMAIL;TYPE=work:
	// v.lebedeva@example.org», «PHOTO (from nextcloud · Контакты)», «2
	// reminders from the others»
	Gains []Text `json:"gains"`
	// NotCarried — «Not carried over» (s-dup-4): переписанные экземпляры
	// удаляемых, время, от которого оставляемая отказывается
	NotCarried []Text `json:"notCarried"`
	// Happened — «What happened» у слияния, не кончившегося одной записью
	// (s-dup-5): «Kept and rewritten: …», «Deleted: …», «Still there: …»;
	// что ответил сервер — Warning.Details
	Happened []Text   `json:"happened"`
	Actions  []Action `json:"actions"`
}

// MergeRow — свойство, о котором спрашивают (s-dup-1: поле с радио), или
// объединяемое множеством (Union). На экране слияния события и задачи с
// разным временем — ещё строка «When» (choice.WHEN): время каждого участника
// словами, выбранное берётся целиком (S-16).
type MergeRow struct {
	// Property — подпись поля по-человечески: «Display name», «Organisation»,
	// «Photo», «Title», «Location»
	Property Text `json:"property"`
	// Name — поле формы вопроса, choice.<свойство>: «choice.FN»; вариант
	// уходит своим номером в Options (0, 1, …). У Union пусто
	Name    string `json:"name"`
	Options []Text `json:"options"`
	Chosen  int    `json:"chosen"`
	Union   bool   `json:"union"`
}

// TargetPicker — «куда» в шапке листа.
type TargetPicker struct {
	Options []TargetOption `json:"options"`
	Chosen  string         `json:"chosen"`
	// Field — поле формы листа, под которым уходит выбранный вариант (его
	// Value): «to» у листа переноса (s-contacts-2); пусто — выбор ссылками
	Field string  `json:"field,omitempty"`
	New   *Action `json:"new"` // строка под чертой: новая коллекция прямо отсюда (s-io-10…13)
}
type TargetOption struct {
	ID string `json:"id"`
	// Href — пункт меню «куда» ссылкой (U-8, s-io-10…13, s-profile-8): тот
	// же экран с этим вариантом выбранным (GET); пусто, когда вариант —
	// значение поля формы (сервер листа новой коллекции) или недоступен
	Href     string `json:"href,omitempty"`
	Label    Text   `json:"label"`
	Stamp    Stamp  `json:"stamp"`
	Colour   Colour `json:"colour"`
	ReadOnly bool   `json:"readOnly"` // показывается и недоступна
	Reason   Text   `json:"reason"`   // почему недоступна: «уже здесь», «пароль · 401», «только файлы»
	Last     bool   `json:"last"`     // последний использованный
	// Value — значение варианта под Field формы листа: «подключение/
	// коллекция» сегментами адреса у листа переноса; пусто, где выбор —
	// ссылкой (Href)
	Value string `json:"value,omitempty"`
	// Here — вариант недоступен потому, что запись уже там («already
	// there»), а не потому, что коллекция только для чтения (s-contacts-2:
	// одно — словами, другое — плашкой)
	Here bool `json:"here,omitempty"`
}

type NewSheet struct {
	Kind   Kind         `json:"kind"`
	Target TargetPicker `json:"target"`
	// Form holds the few fields without which the record does not exist
	// (U-6.16). Empty when the sheet only asks where the record goes.
	Form Form `json:"form"`
	// Строка под полями: что лист не спрашивает и почему — «No time means
	// the whole day…» у задачи (s-notes-10), у контакта и события —
	// s-notes-8, s-notes-9; пусто, когда полей нет
	Note    Text     `json:"note,omitempty"`
	Actions []Action `json:"actions"`
	// Under — экран, над которым открыт лист, пришедший страницей: откуда
	// нажали New (back, иначе Referer), иначе список рода или коллекции
	// (s-notes-5, -8, -9, -10); nil у фрагмента
	Under *Under `json:"under"`
}

// CopiesSheet — лист «какую копию» у записи связанной группы (записка 57,
// Р-58 Typecase): Edit записи из коллекции — «Which copy to edit?»
// (s-contacts-32, один лист у всех родов) — и галка связанной задачи в
// списке — «Which copy to mark done?» (s-tasks-16). Лист — форма Form с
// одной строкой copy: у правки InputRadio (одна копия; GET на адрес правки
// открытой, сервер ведёт на правку выбранной), у галки InputCheck (сколько
// угодно; POST на …/{uid}/done). Варианты — копии: метка коллекции
// (Colour), «nextcloud · Архив» (Label), чем копия разнится (Meta: «the
// copy you opened», «Priority differ»); копия только для чтения — Disabled
// с Reason. Отмечена открытая копия.
type CopiesSheet struct {
	Kind    Kind `json:"kind"`
	Title   Text `json:"title"`   // «Which copy to edit?», «Which copy to mark done?»
	Subject Text `json:"subject"` // что и сколько копий: «Вера Лебедева · 3 copies»
	// Note — строка под шапкой: «This person stands in three books. Saving
	// changes one copy only; the others stay as they are.» (s-contacts-32),
	// «This task stands in two lists. Tick the copy that is done; a copy that
	// is not ticked stays as it is.» (s-tasks-16)
	Note    Text     `json:"note"`
	Form    Form     `json:"form"`
	Actions []Action `json:"actions"` // главная — кнопка формы (Submits): «Edit this copy», «Mark done»; Cancel — назад
	// Under — экран под листом, открытым переходом (Under); nil у фрагмента
	Under *Under `json:"under"`
}

// AskSheet — вопрос перед записью (записка Typecase 56, s-agenda-22…-24;
// решение владельца 11.10.2026, А1): там, где из правки не видно, чего
// человек хотел, а неверная догадка теряет данные, Save сначала спрашивает
// — одним листом на всё сохранение, поверх правки. Первый случай — серия,
// у которой встречи, изменённые вручную, остались без своего дня. Лист —
// форма: POST на адрес правки со всем, что прислала правка, скрытым;
// ответы — кнопки этой формы (Submits) со своими адресами (?left=keep,
// remove, cancel). Отказ после ответа — тот же лист: что записано, что нет
// и почему (s-agenda-24), ответы — «Try again» и «Leave it in the series».
type AskSheet struct {
	Kind Kind `json:"kind"`
	// Icon — значок шапки, «i-refresh» (s-agenda-22, и у листа отказа,
	// s-agenda-24); пусто — без значка
	Icon    string `json:"icon"`
	Title   Text   `json:"title"`   // вопрос: «Keep the meetings that were moved by hand?»
	Subject Text   `json:"subject"` // что и в каком состоянии: «Дежурство: первая линия · not saved yet» («· saved in part»)
	// Blocks — блоки листа: рубрика и строки — «What you changed», «Left
	// without their day · 3» (s-agenda-22); «Written», «Not written · 1»
	// (s-agenda-24)
	Blocks []AskBlock `json:"blocks"`
	Alert  Text       `json:"alert"` // сообщение тревоги под блоками (s-agenda-24); пусто — нет
	Note   Text       `json:"note"`  // строка под блоками: что сделает каждый ответ
	Form   Form       `json:"form"`
	// Actions — ответы: главный первым (по умолчанию), последним — отказ
	// (Cancel, «Leave it in the series»)
	Actions []Action `json:"actions"`
	// Under — правка под листом (Under.EventEdit, .TaskEdit) с тем, что
	// набрано; nil у фрагмента
	Under *Under `json:"under"`
}

// AskBlock — блок листа вопроса: рубрика со счётом, где он есть, и строки.
type AskBlock struct {
	Title Text      `json:"title"` // «Left without their day · 3»
	Lines []AskLine `json:"lines"`
}

// AskLine — строка блока: жирное и, приглушённо после тире, пояснение.
type AskLine struct {
	Text  Text `json:"text"`  // «26 January, 12:00 — Duty moved»
	Aside Text `json:"aside"` // «— it was the meeting of 26 January, 09:00»
}

// Under — экран, над которым открыт лист, пришедший страницей: переходом
// без сценариев (решение владельца 15.10.2026; Overlay Typecase — «под
// затемнением то же тело оболочки», кадр s-contacts-2). Это страница, откуда
// лист открыли (back листа; без него — страница записи или списка, к которой
// лист относится), как её отдаёт свой адрес; ровно одно поле не nil. Лист,
// пришедший фрагментом (C-7.2), экрана под собой не несёт — он остаётся у
// клиента: Under nil, как и у страницы, которую сервер построить не смог.
type Under struct {
	Contacts *ContactsListPage `json:"contacts"`
	Contact  *ContactPage      `json:"contact"`
	Agenda   *AgendaPage       `json:"agenda"`
	Event    *EventPage        `json:"event"`
	Tasks    *TasksPage        `json:"tasks"`
	Task     *TaskPage         `json:"task"`
	Notes    *NotesPage        `json:"notes"`
	Note     *NotePage         `json:"note"`
	// правка под листом вопроса перед записью (AskSheet) — с набранным
	EventEdit *EventEditPage `json:"eventEdit"`
	TaskEdit  *TaskEditPage  `json:"taskEdit"`
	// страница подключений под листами коллекций (Кл1; s-profile-7…-9),
	// пароля (s-profile-3) и отключения
	Connections *ConnectionsPage `json:"connections"`
	// страницы настроек и администрирования под листами U-6.17 (Лс29):
	// Account — депонирование и отзыв (account-2, -11), Devices — пароль
	// у «Set up a new device» (s-plan-3), AdminUsers — сброс пароля
	// (s-admin-13)
	Account    *AccountPage    `json:"account"`
	Devices    *DevicesPage    `json:"devices"`
	AdminUsers *AdminUsersPage `json:"adminUsers"`
	// Davloom — Add device с набранным под листом пароля (s-davloom-5),
	// Backup — страница задания под ним же: внешний запуск (193 п. 3)
	Davloom *DavloomPage `json:"davloom"`
	Backup  *BackupPage  `json:"backup"`
}

type MoveSheet struct {
	Kind    Kind         `json:"kind"`
	Copy    bool         `json:"copy"`
	Title   Text         `json:"title"`   // «Move contact», «Copy event» (s-contacts-2)
	Subject Text         `json:"subject"` // что и откуда: «Анна Ковалёва · from nextcloud · Контакты»
	Target  TargetPicker `json:"target"`
	// Cancels — строка листа о людях записи (решение владельца 12.10.2026):
	// перенос уносит событие с участниками с сервера, который планирует
	// (RFC 6638), и тот может сообщить им об отмене — «nextcloud may tell
	// its attendees that the event is cancelled.»; пусто — сказать нечего:
	// копия, участников нет, сервер не планирует или неизвестно, перенос
	// внутри подключения. О приглашениях лист не говорит
	Cancels Text `json:"cancels"`
	Warning Runs `json:"warning"` // что сделает перенос: на одном сервере MOVE (моноширинно, s-contacts-2), на другой аккаунт — копия и удаление исходника
	// Form — лист как форма (s-contacts-2): POST на адрес переноса, скрытые
	// _csrf, copy у копии, warned — о каком переносе сказал Warning;
	// выбранная коллекция уходит под Target.Field. Основная кнопка —
	// Actions[0] (Submits)
	Form    Form     `json:"form"`
	Actions []Action `json:"actions"`
	// Under — экран под листом, открытым переходом (Under; s-contacts-2 —
	// объединённая книга); nil у фрагмента
	Under *Under `json:"under"`
}

// Четыре листа коллекции (C-5.6). Родов записей они не различают: заголовок
// берётся из Kind, а состав полей у всех четырёх один и тот же.
type CollectionNewSheet struct {
	Kind Kind `json:"kind"`
	// Title — заголовок листа по роду: «New calendar», «New address book»,
	// «New task list», «New notebook», «New folder» (s-profile-5, -20, -21)
	Title   Text         `json:"title"`
	Target  TargetPicker `json:"target"`  // «на каком сервере»; неотвечающий виден с причиной
	Note    Text         `json:"note"`    // почему в списке не все подключения
	Form    Form         `json:"form"`    // имя, цвет, набор компонентов, адрес с подсказкой
	Failure *Notice      `json:"failure"` // отказ сервера: одна строка и блок C-9.1 — шаг, метод, адрес, код, тело, что код значит (S-12.2, s-profile-6); nil — отказа не было
	// Stamp — сервер в шапке листа, когда отказ занял его место (s-profile-6,
	// Кл6): форма тогда скрыта и едет с кнопками; пусто — без отказа
	Stamp Stamp `json:"stamp"`
	// Explain — что отказ значит для человека, строкой под блоком
	// (s-profile-6): «This account may sign in and read, but not create. …»;
	// пусто — сказать нечего
	Explain Text     `json:"explain"`
	Actions []Action `json:"actions"`
	// Under — экран под листом, пришедшим страницей (Кл1): откуда открыт
	// (back, иначе Referer), иначе Connections (s-profile-5…-9); nil у
	// фрагмента
	Under *Under `json:"under"`
}

type CollectionRenameSheet struct {
	Kind Kind `json:"kind"`
	// Title — «Rename and colour» у календаря (s-profile-7), «Rename» у
	// книги, подписки без цвета и папки (s-profile-18)
	Title   Text     `json:"title"`
	Stamp   Stamp    `json:"stamp"`   // имя сервера; у папки хранилища — сервер и папка, «sftpgo · vps · Documents» (s-profile-18); пусто у подписки — у неё имя местное
	Form    Form     `json:"form"`    // имя и цвет одним листом: оба уходят одним запросом
	Address Text     `json:"address"` // показан и не правится (S-12.2); пусто у папки хранилища: её имя и есть адрес
	Note    Text     `json:"note"`    // «переименование — не перенос»: адрес остаётся прежним
	Failure *Notice  `json:"failure"` // отказ сервера; форма хранит введённое
	Actions []Action `json:"actions"`
	// Under — экран под листом, пришедшим страницей (Кл1): откуда открыт
	// (back, иначе Referer), иначе Connections (s-profile-5…-9); nil у
	// фрагмента
	Under *Under `json:"under"`
}

type CollectionMoveSheet struct {
	Kind    Kind         `json:"kind"`
	Copy    bool         `json:"copy"`    // тот же лист с другой кнопкой; исходная остаётся
	Title   Text         `json:"title"`   // «Move calendar», «Copy address book», «Move folder» (s-profile-8)
	Subject Text         `json:"subject"` // что переносят: «Дни рождения · nextcloud»
	Target  TargetPicker `json:"target"`  // цель — подключение, а не коллекция (S-12.3)
	// Cancels — как у MoveSheet: календарь уходит с сервера, который
	// планирует, и участникам его встреч тот может сообщить об отмене;
	// пусто — копия, не календарь, сервер не планирует или неизвестно
	Cancels Text `json:"cancels"`
	Warning Runs `json:"warning"` // завести там, перенести записи, удалить здесь — до начала; число записей жирным (s-profile-8)
	// Form — лист как форма (s-profile-8, Кл11): POST на адрес переноса
	// (a, c, back в адресе), скрытые _csrf и copy у копии; выбранный сервер
	// уходит под Target.Field («to», TargetOption.Value — подключение).
	// Основная кнопка — Actions[0] (Submits)
	Form    Form     `json:"form"`
	Actions []Action `json:"actions"`
	// Under — экран под листом, пришедшим страницей (Кл1): откуда открыт
	// (back, иначе Referer), иначе Connections (s-profile-5…-9); nil у
	// фрагмента
	Under *Under `json:"under"`
}

// Reference — то, что ссылается на коллекцию, и что с ним станет
// (s-profile-9: «1 published link» жирным, «— answers “not found” from that
// moment» приглушённо; тире — экрана)
type Reference struct {
	What Text `json:"what"`
	Then Text `json:"then"`
}

// RecordDeleteSheet — лист удаления записи (s-agenda-26, s-tasks-18;
// решения владельца А8, А3, А12; задача Typecase 166): ответ POST
// …/{uid}/delete, когда есть что терять; простая запись удаляется сразу.
// Каждая часть — только когда ей есть что назвать.
type RecordDeleteSheet struct {
	Kind    Kind `json:"kind"`
	Title   Text `json:"title"`   // «Delete event», «Delete task»
	Subject Text `json:"subject"` // название записи
	// Warning — где удаляется и что копии у Carrel нет: «The event is
	// deleted on nextcloud · Личный. Carrel holds no copy — …»
	Warning Text `json:"warning"`
	// References — «What is pointing at it now»: опубликованные ссылки и
	// записи, связанные с ней, среди загруженных; What жирным, Then — что с
	// ними станет
	References []Reference `json:"references"`
	// Cancels — участникам сервер может сообщить об отмене («may», как у
	// листов переноса); пусто — сказать нечего
	Cancels Text `json:"cancels"`
	// Attachments — имя жирным; записанное в запись уходит с ней, ссылка в
	// папку вложений остаётся (её файл можно удалить отметкой формы)
	Attachments []Reference `json:"attachments"`
	// Form — CSRF, etag, sheet=1; отметки attachment (удалить файл ссылки
	// тоже, сняты) и copy (копии связанной записи, открытая отмечена)
	Form    Form     `json:"form"`
	Actions []Action `json:"actions"` // Delete (тревожная, кнопка формы) · Cancel
	// Under — экран под листом, пришедшим страницей: карточка; nil у
	// фрагмента
	Under *Under `json:"under"`
}

type CollectionDeleteSheet struct {
	Kind    Kind `json:"kind"`
	Title   Text `json:"title"` // «Delete on the server» (s-profile-9, -19)
	Subject Text `json:"subject"`
	Warning Runs `json:"warning"` // что именно исчезнет и что копии у Carrel нет; число жирным (s-profile-9, -19)
	// References — что на неё ссылается и что с ним станет (s-profile-9,
	// -19; Кл16): ссылки и устройства счётом, задания по имени
	References []Reference `json:"references"`
	// Cancels — как у CollectionMoveSheet: календарь уходит с сервера,
	// который планирует, и участникам его встреч тот может сообщить об
	// отмене (решение владельца А12, Кл18); пусто — сказать нечего
	Cancels    Text     `json:"cancels"`
	Archive    Runs     `json:"archive"` // когда попадала в архив (день моноширинно) и чего восстановление не сделает («new» жирным; s-profile-9, -19)
	Export     *Action  `json:"export"`  // «сначала скачать»; nil — экспортировать нечем
	ExportNote Text     `json:"exportNote"`
	Form       Form     `json:"form"` // подтверждение набором имени
	Actions    []Action `json:"actions"`
	// Under — экран под листом, пришедшим страницей (Кл1): откуда открыт
	// (back, иначе Referer), иначе Connections (s-profile-5…-9); nil у
	// фрагмента
	Under *Under `json:"under"`
	// Failure — отказ сервера удалить (Кл4): лист остаётся, набранное имя в
	// поле; nil — отказа не было
	Failure *Notice `json:"failure"`
}

// ConnectionRemoveSheet — лист «Disconnect»: подключение уходит из Carrel, на
// сервере не трогается ничего. Брат CollectionDeleteSheet, без экспорта и
// архива — удалять нечего.
type ConnectionRemoveSheet struct {
	Subject    Text     `json:"subject"`    // подпись подключения и его адрес
	Warning    Text     `json:"warning"`    // что уходит вместе с ним и что на сервере всё остаётся
	References []Text   `json:"references"` // коллекции, ссылки, задания копии, устройства davloom
	Form       Form     `json:"form"`       // подтверждение: скрытое подключение и CSRF
	Actions    []Action `json:"actions"`    // Disconnect · Cancel
	// Under — экран под листом, пришедшим страницей (Лс29): Connections; nil у фрагмента
	Under *Under `json:"under"`
}

// RewriteSheet — «переписать адрес в записях?» после сохранения карточки, где
// адрес заменён (s-contacts-15, S-15.4). Вопрос называет последствия до
// нажатия: рубрики «The address», «What rewriting can set off» и «What will
// not change» — постоянные подписи клиента, строки под ними — данные.
type RewriteSheet struct {
	Title   Text          `json:"title"`   // «Rewrite the address in their records?», у нескольких замен — «… addresses …» (s-contacts-29)
	Subject Text          `json:"subject"` // «Анна Ковалёва · saved»
	Pairs   []RewritePair `json:"pairs"`   // какой адрес на какой и сколько записей называют старый
	// Effects — «What rewriting can set off» строками, как одну замену
	// рисуют s-contacts-15, -19, -20: Label — «9 records on servers that
	// schedule», Value — что это может вызвать
	Effects []Field `json:"effects"`
	// Groups — те же группы отметками, как s-contacts-29 (решения владельца
	// 12.10.2026, А5): снятая исключает группу; форма шлёт отметки всегда
	Groups []RewriteGroup `json:"groups"`
	// Both — строка о записи, что стоит в двух группах: переписывается, только
	// когда отмечены обе, — с её названием (s-contacts-29); пусто — таких нет
	Both Text    `json:"both"`
	Kept []Field `json:"kept"` // Label — «2 records in Shared», Value — «read-only; they keep the old address»
	Note Text    `json:"note"` // строка под блоками: «Collections that are not ticked were not looked in.» (s-contacts-19)
	Form Form    `json:"form"` // план в адресе, CSRF и groups=1 — форма несёт отметки групп
	// Tally — по строке на запись, которую можно переписать: условия, при
	// которых она переписывается, через пробел (записка 65): «to.1» — выбор
	// адреса не пуст, «to.1|to.2» — любой из двух, «group=others» — группа
	// отмечена; клиент считает выполненные при каждом изменении формы
	Tally []string `json:"tally"`
	// Count, CountOne — слова основной кнопки по счёту: «Rewrite {n}
	// records», «Rewrite 1 record»; без сценария — Label кнопки, счёт сервера
	Count    Text     `json:"count"`
	CountOne Text     `json:"countOne"`
	Actions  []Action `json:"actions"` // Rewrite N records · Leave as is
}
type RewritePair struct {
	From Text `json:"from"`
	To   Text `json:"to"`   // выбранный по умолчанию; пусто — «Do not rewrite»
	Note Text `json:"note"` // «22 records name it», «3 records name it; they keep it»
	// Choice — выбор, на какой адрес переписать (s-contacts-29): поле
	// «to.N», варианты — нынешние адреса карточки и «Do not rewrite»
	// (пустое значение); по умолчанию — адрес, набранный в той же строке
	// формы, иначе «Do not rewrite» (решение владельца 12.10.2026, А4)
	Choice FormRow `json:"choice"`
}

// RewriteGroup — группа риска листа «переписать адрес» (s-contacts-29):
// отметка «group» со своим значением, снятая исключает записи группы.
type RewriteGroup struct {
	Name  string `json:"name"`  // «group»
	Value string `json:"value"` // scheduling, unknown.N (сервер не сказал), others, past
	Label Text   `json:"label"` // «On servers that schedule», «On baikal», «Meetings organised by others», «Past meetings»
	On    bool   `json:"on"`    // отмечена
	Count Text   `json:"count"` // «9 records» — кнопка со списком записей
	// Records — записи группы: Label — название, Value — день (моноширинно),
	// Colour — метка коллекции, Href — запись
	Records []Field `json:"records"`
	Note    Text    `json:"note"` // что это может вызвать
}

// ExportSheet — «из какой коллекции отдать»: вход в экспорт из сведённого
// вида, где коллекция не одна.
type ExportSheet struct {
	Kind    Kind         `json:"kind"`
	Target  TargetPicker `json:"target"` // варианты — адреса выдачи каждой коллекции
	Actions []Action     `json:"actions"`
	// Under — экран под листом, пришедшим страницей (Лс29): откуда открыт (back, иначе Referer), иначе сведённый список рода; nil у фрагмента
	Under *Under `json:"under"`
}

type ImportPage struct {
	Shell
	Kind Kind       `json:"kind"`
	Step ImportStep `json:"step"` // C-8
	// Steps — строка шагов мастера (s-io-3…9): «1 · Source», «2 · Preview»,
	// «3 · Report»; текущий — по Step, в этом же порядке
	Steps []Text `json:"steps"`
	// Subtitle — строка под заголовком: на предпросмотре файл моноширинно,
	// размер и формат — «‹Разное (mix).vcf› · 84 KB · vCard 3.0»
	// (s-io-5…8), у папки — её имя, размер и число файлов; в отчёте — куда
	// легло, «nextcloud · Контакты» (s-io-9); на первом шаге пусто
	Subtitle Runs         `json:"subtitle"`
	Source   ImportSource `json:"source"`
	Into     TargetPicker `json:"into"`
	// Counts — счёт над таблицей предпросмотра (s-io-5…8): ready всегда,
	// with errors (Error), already here, skipped — когда не ноль
	Counts []ImportCount `json:"counts"`
	// Existing — вопрос предпросмотра о записях файла, которые коллекция уже
	// держит под тем же UID (решение владельца 12.10.2026): сколько их и что
	// с ними сделать; nil — таких нет или коллекция не выбрана
	Existing *ImportExisting `json:"existing"`
	Preview  []ImportTable   `json:"preview"`
	Report   *ImportReport   `json:"report"`
	Actions  []Action        `json:"actions"`
}

// ImportExisting — что сделать с записями файла, которые уже есть в
// коллекции (записка 52, s-io-5…8): вопрос стоит у поля Into. Note — подпись
// поля, сам вопрос: «5 records of this file are already in Контакты» («1
// note of this file is already in Записи»); Options — три ссылки на этот же
// предпросмотр (GET, existing=): «Skip them» — без ответа выбрана она,
// «Replace them», «Import as copies» («Skip it», «Replace it», «Import as a
// copy»); выбранная — On. Hint — подсказка о выбранном под ссылками.
// Строки предпросмотра и кнопка импорта говорят, что сделает нажатие
type ImportExisting struct {
	Note    Text     `json:"note"`
	Options []Action `json:"options"`
	Hint    Text     `json:"hint"` // «They stay as they are here; the file's version is not written.»
}
type ImportSource struct {
	Kinds []Text `json:"kinds"`
	// Hint — строка под полем файла: что берётся, расширение другого рода
	// моноширинно (s-io-3, s-io-4)
	Hint Runs `json:"hint"`
	// Folder — второй источник заметок, дерево папок WebDAV (s-io-4):
	// хранилища и папки открытого вглубь до выбранной, узел — GET, приходит
	// с ним выбранным (Chosen); берёт её Preview
	Folder *FileTree `json:"folder"`
	// Preview — «Preview folder» под деревом (s-io-4): выключена с причиной,
	// пока папка не выбрана; nil — папок нет
	Preview *Action `json:"preview"`
	// FolderHint — строка под деревом папок (s-io-4): берутся ‹.md› прямо в
	// папке, вглубь не идут, на сервере ничего не трогается
	FolderHint Runs `json:"folderHint"`
	// Taken — файл, уже взятый броском на сведённый список (подпись s-io-1,
	// Им4): имя моноширинно, размер и формат; выбор Into ведёт в его
	// предпросмотр. Пусто — файла нет
	Taken Runs `json:"taken"`
	// Note — сообщение шага (tc-msg): на первом — что ничего не пишется до
	// предпросмотра (s-io-3), у заметок — front matter и ‹X-›свойства
	// (s-io-4); на предпросмотре — что импорт делает с записями этого рода
	// (s-io-5…8)
	Note Runs `json:"note"`
}

// ImportCount — один счёт предпросмотра: «‹122› ready» (s-io-5…8)
type ImportCount struct {
	Count int  `json:"count"`
	Label Text `json:"label"` // «ready», «with errors», «already here», «skipped»
	Error bool `json:"error,omitempty"`
}
type ImportTable struct {
	Rubric  Text         `json:"rubric"`
	Columns []Text       `json:"columns"`
	Lines   []ImportLine `json:"lines"`
	Count   int          `json:"count"`
}

// ImportLine — строка таблицы импорта (s-io-5…8, Им8): ячейки по Columns
// и то, как кадр их метит. Result — что будет с записью: ready — ляжет,
// failed — не прочтена (итог тревогой), skipped — пропущена (итог
// приглушённо); Untitled — у записи нет названия, во второй ячейке заглушка
// «— (no FN)», «— (no SUMMARY)», «—» (блёкло); Component — чужой род
// записи бейджем у названия, «VTODO»; пустая ячейка — «—» приглушённо.
// В таблицах отчёта — только Cells
type ImportLine struct {
	Cells     []Text       `json:"cells"`
	Result    ImportResult `json:"result,omitempty"`
	Untitled  bool         `json:"untitled,omitempty"`
	Component Text         `json:"component,omitempty"`
}
type ImportReport struct {
	Created int `json:"created"`
	// Here — записи, что уже были в коллекции и пропущены по ответу «Skip»
	// (записка 52, s-io-9: «Already here, skipped»); Heres — какие: «Артём
	// Белов · 1a77-40de-bb02». В Skipped не входят
	Here    int    `json:"here"`
	Heres   []Text `json:"heres"`
	Skipped int    `json:"skipped"`
	// Skips — какие пропущены и почему, строка на запись (s-io-14): «Купить
	// билеты · VTODO: a calendar takes events — import it into a list of
	// tasks»
	Skips   []Text `json:"skips"`
	Renamed int    `json:"renamed"`
	// Replaced — записи коллекции, заменённые версией файла (выбор
	// ImportExisting «Replace them…»); в Created не входят
	Replaced int `json:"replaced"`
	// Replaces — какие заменены: «Анна Ковалёва · 4f2b-9c31-a0e7»
	Replaces []Text `json:"replaces"`
	// Failed — не записано: запись файла не прочлась или сервер отказал
	// (s-io-9, «Failed»); Failures — почему, строка на запись
	Failed   int    `json:"failed"`
	Failures []Text `json:"failures"`
	// Renames — UID, что сменились: «1a77-40de-bb02 → 9c02-4411-7ee1»
	// (s-io-9, «Imported as copies»)
	Renames []Text        `json:"renames"`
	Rows    []ImportTable `json:"rows"`
	Note    Text          `json:"note"`
}

type PublishSheet struct {
	Kind Kind `json:"kind"`
	// Title — заголовок шапки: «Publish a calendar», «Publish an event»,
	// «Publish a folder» (s-publish-2…5, -19)
	Title Text `json:"title"`
	// Target — что именно, в конце шапки, не переспрашивается: у коллекции —
	// где она («nextcloud · Личный»), у записи и файла — название, у папки —
	// и сколько в ней: «Материалы к релизу · 12 files»
	Target  Text `json:"target"`
	Local   bool `json:"local"` // значок «local» в шапке (s-publish-2): SignedIn.Local, у листа оболочки нет
	Form    Form `json:"form"`  // сколько показывать, разрешить ли .ics, показывать ли описания
	Expires Text `json:"expires"`
	// Note — сообщение, которое стоит на листе всегда, до ссылки и после: у
	// папки — что получает гость («The visitor gets the folder with
	// everything inside it, read-only, and the same address mounts as a
	// drive in Windows, macOS and Linux.», s-publish-5); у прочих пусто
	Note    Text     `json:"note"`
	Link    Text     `json:"link"` // готовая ссылка после создания
	Actions []Action `json:"actions"`
}

type MailSheet struct {
	To      []Recipient `json:"to"`
	Suggest []Recipient `json:"suggest"`
	Subject Text        `json:"subject"`
	Body    Text        `json:"body"`
	// From — с какого адреса уйдёт, видно до отправки: в шапке «from
	// mix@carrel.example» (s-mail-1); FromName — имя, каким его увидит
	// получатель («Михаил Енученко», через общий релей — «… (via Carrel)»)
	From     Text `json:"from"`
	FromName Text `json:"fromName"`
	ReplyTo  Text `json:"replyTo"` // пусто — в письме будет строка «отвечать не нужно»
	// ReplyNote — строка подвала: чем подписано и куда придёт ответ, имя
	// жирным — «Signed **Михаил Енученко** · replies go to mix@example.org»
	// (s-mail-1); без подтверждённого адреса — что ответа не будет
	ReplyNote Runs            `json:"replyNote"`
	Attach    []MailAttachRow `json:"attach"`
	NotAsked  []Stamp         `json:"notAsked"` // книги, которые не ответили, названы прямо
	Limits    Runs            `json:"limits"`   // «Published links» — жирным (s-mail-1)
	// Links — строка под вложениями, когда что-то уходит ссылкой: сколько и
	// почему, и где их отозвать, «Published links» жирным (s-mail-1); пусто
	// — ссылок нет
	Links   Runs     `json:"links,omitempty"`
	Actions []Action `json:"actions"`
}
type Recipient struct {
	Address Text   `json:"address"`
	Label   Text   `json:"label"`
	Name    Text   `json:"name"`
	Colour  Colour `json:"colour"` // метка книги, откуда человек (s-mail-1, -3); набранный рукой — без неё
}
type MailAttachRow struct {
	Kind   Kind `json:"kind"`
	Title  Text `json:"title"`
	Format Text `json:"format"`
	Size   Size `json:"size"`
	AsLink bool `json:"asLink"` // уйдёт ссылкой, а не вложением — видно до отправки
	// Terms — условия ссылки словами, как их говорит список ссылок: «30
	// days», «No expiry»; Change — «Change…», лист публикации поверх письма
	// с ними, главная кнопка «Apply» (s-mail-1, записка Typecase 70). У
	// вложения и у ссылки, сделанной раньше письма, пусто и nil
	Terms  Text    `json:"terms,omitempty"`
	Change *Action `json:"change"`
	Remove Action  `json:"remove"`
}
type MailAttachSheet struct {
	Rail    Rail            `json:"rail"`
	Rows    []FindRow       `json:"rows"`
	Preview []MailAttachRow `json:"preview"` // что окажется в списке вложений — до нажатия
	Range   *Form           `json:"range"`   // «Calendar»: весь или даты, с и по — только у дат; nil у остальных
	Format  *Form           `json:"format"`  // «Note»: Markdown или исходный .ics; nil у остальных
	Actions []Action        `json:"actions"`
}
