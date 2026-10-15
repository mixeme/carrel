package view

type SearchPage struct {
	Shell
	Query   Text      `json:"query"`
	Bar     ListBar   `json:"bar"` // сегменты All / Contacts / Events / Tasks / Notes / Files
	Poll    PollBar   `json:"poll"`
	Results []FindRow `json:"results"`
	Note    Text      `json:"note"` // «содержимое файлов не ищется» — говорится в выдаче
}

type FindRow struct {
	Href   string `json:"href"`
	Kind   Kind   `json:"kind"`
	Colour Colour `json:"colour"`
	Stamp  Stamp  `json:"stamp"`
	Title  Text   `json:"title"`
	Detail Text   `json:"detail"`
	When   Text   `json:"when"`
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
	// у всех разделов (states-3, -8, -13, -19, -26)
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
	// строка на каждый источник с Answer, его Retry; пусто — не ответили
	// все или полоса этих слов не пишет (поиск, s-find-1). Summary тогда
	// отказов не называет
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
	GroupID string          `json:"groupID"`
	Target  DuplicateMember `json:"target"`
	Rows    []MergeRow      `json:"rows"`
	Warning Notice          `json:"warning"` // что будет записано, что удалено
	Actions []Action        `json:"actions"`
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
	Failure *Notice      `json:"failure"` // отказ сервера: метод, адрес, код, тело (S-12.2); nil — отказа не было
	Actions []Action     `json:"actions"`
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
	Cancels Text     `json:"cancels"`
	Warning Runs     `json:"warning"` // завести там, перенести записи, удалить здесь — до начала; число записей жирным (s-profile-8)
	Actions []Action `json:"actions"`
}

type CollectionDeleteSheet struct {
	Kind       Kind     `json:"kind"`
	Title      Text     `json:"title"` // «Delete on the server» (s-profile-9, -19)
	Subject    Text     `json:"subject"`
	Warning    Runs     `json:"warning"`    // что именно исчезнет и что копии у Carrel нет; число жирным (s-profile-9, -19)
	References []Text   `json:"references"` // что на неё ссылается: ссылки и устройства счётом, задания по имени
	Archive    Runs     `json:"archive"`    // когда попадала в архив (день моноширинно) и чего восстановление не сделает («new» жирным; s-profile-9, -19)
	Export     *Action  `json:"export"`     // «сначала скачать»; nil — экспортировать нечем
	ExportNote Text     `json:"exportNote"`
	Form       Form     `json:"form"` // подтверждение набором имени
	Actions    []Action `json:"actions"`
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
}

// RewriteSheet — «переписать адрес в записях?» после сохранения карточки, где
// адрес заменён (s-contacts-15, S-15.4). Вопрос называет последствия до
// нажатия: рубрики «The address», «What rewriting can set off» и «What will
// not change» — постоянные подписи клиента, строки под ними — данные.
type RewriteSheet struct {
	Title   Text          `json:"title"`   // «Rewrite the address in their records?»
	Subject Text          `json:"subject"` // «Анна Ковалёва · saved»
	Pairs   []RewritePair `json:"pairs"`   // какой адрес на какой и сколько записей называют старый
	Effects []Field       `json:"effects"` // Label — «9 records on servers that schedule», Value — что это может вызвать
	Kept    []Field       `json:"kept"`    // Label — «2 records in Shared», Value — «read-only; they keep the old address»
	Note    Text          `json:"note"`    // строка под блоками: «Collections that are not ticked were not looked in.» (s-contacts-19)
	Form    Form          `json:"form"`    // план в адресе и CSRF
	Actions []Action      `json:"actions"` // Rewrite N records · Leave as is
}
type RewritePair struct {
	From Text `json:"from"`
	To   Text `json:"to"`
	Note Text `json:"note"` // «14 events and tasks name the old one; …»
}

// ExportSheet — «из какой коллекции отдать»: вход в экспорт из сведённого
// вида, где коллекция не одна.
type ExportSheet struct {
	Kind    Kind         `json:"kind"`
	Target  TargetPicker `json:"target"` // варианты — адреса выдачи каждой коллекции
	Actions []Action     `json:"actions"`
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
	// with errors (Error), UID collisions, skipped — когда не ноль
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
// коллекции. Note — «‹38› of ‹40› records of this file are already in
// Work.»; Options — три ссылки на этот же предпросмотр (GET, existing=):
// «Skip them» — без ответа выбрана она, «Replace them with the file's
// version», «Import as copies»; выбранная — On. Строки предпросмотра и
// кнопка импорта говорят, что сделает нажатие
type ImportExisting struct {
	Note    Text     `json:"note"`
	Options []Action `json:"options"`
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
	// Note — сообщение шага (tc-msg): на первом — что ничего не пишется до
	// предпросмотра (s-io-3), у заметок — front matter и ‹X-›свойства
	// (s-io-4); на предпросмотре — что импорт делает с записями этого рода
	// (s-io-5…8)
	Note Runs `json:"note"`
}

// ImportCount — один счёт предпросмотра: «‹122› ready» (s-io-5…8)
type ImportCount struct {
	Count int  `json:"count"`
	Label Text `json:"label"` // «ready», «with errors», «UID collisions», «skipped»
	Error bool `json:"error,omitempty"`
}
type ImportTable struct {
	Rubric  Text     `json:"rubric"`
	Columns []Text   `json:"columns"`
	Rows    [][]Text `json:"rows"`
	Count   int      `json:"count"`
}
type ImportReport struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Renamed int `json:"renamed"`
	// Replaced — записи коллекции, заменённые версией файла (выбор
	// ImportExisting «Replace them…»); в Created не входят
	Replaced int `json:"replaced"`
	// Failed — не записано: запись файла не прочлась или сервер отказал
	// (s-io-9, «Failed»); Failures — почему, строка на запись
	Failed   int    `json:"failed"`
	Failures []Text `json:"failures"`
	// Renames — UID, что сменились: «1a77-40de-bb02 → 9c02-4411-7ee1»
	// (s-io-9, «UID collisions»)
	Renames []Text        `json:"renames"`
	Rows    []ImportTable `json:"rows"`
	Note    Text          `json:"note"`
}

type PublishSheet struct {
	Kind    Kind   `json:"kind"`
	Target  Text   `json:"target"` // что публикуется — написано в шапке, не переспрашивается
	Local   bool   `json:"local"`  // значок «local» в шапке (s-publish-2): SignedIn.Local, у листа оболочки нет
	Range   *Range `json:"range"`  // только у календаря
	Form    Form   `json:"form"`   // сколько показывать, разрешить ли .ics, показывать ли описания
	Expires Text   `json:"expires"`
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
	From    Text        `json:"from"`    // с какого адреса уйдёт — видно до отправки
	ReplyTo Text        `json:"replyTo"` // пусто — в письме будет строка «отвечать не нужно»
	// ReplyNote — строка подвала: чем подписано и куда придёт ответ, имя
	// жирным — «Signed **Михаил Енученко** · replies go to mix@example.org»
	// (s-mail-1); без подтверждённого адреса — что ответа не будет
	ReplyNote Runs            `json:"replyNote"`
	Attach    []MailAttachRow `json:"attach"`
	NotAsked  []Stamp         `json:"notAsked"` // книги, которые не ответили, названы прямо
	Limits    Runs            `json:"limits"`   // «Published links» — жирным (s-mail-1)
	Actions   []Action        `json:"actions"`
}
type Recipient struct {
	Address Text `json:"address"`
	Label   Text `json:"label"`
	Name    Text `json:"name"`
}
type MailAttachRow struct {
	Kind   Kind   `json:"kind"`
	Title  Text   `json:"title"`
	Format Text   `json:"format"`
	Size   Size   `json:"size"`
	AsLink bool   `json:"asLink"` // уйдёт ссылкой, а не вложением — видно до отправки
	Remove Action `json:"remove"`
}
type MailAttachSheet struct {
	Rail    Rail            `json:"rail"`
	Rows    []FindRow       `json:"rows"`
	Preview []MailAttachRow `json:"preview"` // что окажется в списке вложений — до нажатия
	Range   *Form           `json:"range"`   // «Calendar»: весь или даты, с и по — только у дат; nil у остальных
	Format  *Form           `json:"format"`  // «Note»: Markdown или исходный .ics; nil у остальных
	Actions []Action        `json:"actions"`
}
