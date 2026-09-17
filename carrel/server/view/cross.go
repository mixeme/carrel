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
	TaskID     string           `json:"taskID"`
	Summary    Text             `json:"summary"`    // «Опрошено 5 из 7 · 12 записей · 1 недоступен»
	AnsweredAt When             `json:"answeredAt"` // когда собрана показанная выдача
	Running    bool             `json:"running"`
	Percent    int              `json:"percent"`
	Sources    []SourceProgress `json:"sources"`
	Actions    []Action         `json:"actions"` // Stop · Search again
}

type SourceProgress struct {
	Stamp     Stamp       `json:"stamp"`
	Colour    Colour      `json:"colour"`
	State     SourceState `json:"state"` // C-8
	Count     *int        `json:"count"`
	FromCache bool        `json:"fromCache"`
	Message   Text        `json:"message"` // краткая причина отказа
	Retry     *Action     `json:"retry"`   // повтор только этого источника
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
	Value   int  `json:"value"`
	Default int  `json:"default"`
	Note    Text `json:"note"`
}

type DuplicatesMergePage struct {
	Shell
	GroupID string          `json:"groupID"`
	Target  DuplicateMember `json:"target"`
	Rows    []MergeRow      `json:"rows"`
	Warning Notice          `json:"warning"` // что будет записано, что удалено
	Actions []Action        `json:"actions"`
}
type MergeRow struct {
	Property Text   `json:"property"`
	Options  []Text `json:"options"`
	Chosen   int    `json:"chosen"`
	Union    bool   `json:"union"`
}

type QuickNoteSheet struct {
	Title    Text         `json:"title"`
	Target   TargetPicker `json:"target"`
	Body     Text         `json:"body"`
	Actions  []Action     `json:"actions"`
	NoTarget bool         `json:"noTarget"` // ни одной тетради: «No notebook» и Connect account
}

// TargetPicker — «куда» в шапке листа.
type TargetPicker struct {
	Options []TargetOption `json:"options"`
	Chosen  string         `json:"chosen"`
}
type TargetOption struct {
	ID       string `json:"id"`
	Label    Text   `json:"label"`
	Stamp    Stamp  `json:"stamp"`
	Colour   Colour `json:"colour"`
	ReadOnly bool   `json:"readOnly"` // показывается и недоступна
	Reason   Text   `json:"reason"`   // почему недоступна: «уже здесь», «пароль · 401», «только файлы»
	Last     bool   `json:"last"`     // последний использованный
}

type NewSheet struct {
	Kind   Kind         `json:"kind"`
	Target TargetPicker `json:"target"`
	// Form holds the few fields without which the record does not exist
	// (U-6.16). Empty when the sheet only asks where the record goes.
	Form    Form     `json:"form"`
	Actions []Action `json:"actions"`
}

type MoveSheet struct {
	Kind    Kind         `json:"kind"`
	Copy    bool         `json:"copy"`
	Target  TargetPicker `json:"target"`
	Warning Text         `json:"warning"` // на другой аккаунт — копия и удаление исходника, сказано до начала
	Actions []Action     `json:"actions"`
}

// Четыре листа коллекции (C-5.6). Родов записей они не различают: заголовок
// берётся из Kind, а состав полей у всех четырёх один и тот же.
type CollectionNewSheet struct {
	Kind    Kind         `json:"kind"`
	Target  TargetPicker `json:"target"`  // «на каком сервере»; неотвечающий виден с причиной
	Note    Text         `json:"note"`    // почему в списке не все подключения
	Form    Form         `json:"form"`    // имя, цвет, набор компонентов, адрес с подсказкой
	Failure *Notice      `json:"failure"` // отказ сервера: метод, адрес, код, тело (S-12.2); nil — отказа не было
	Actions []Action     `json:"actions"`
}

type CollectionRenameSheet struct {
	Kind    Kind     `json:"kind"`
	Stamp   Stamp    `json:"stamp"`   // имя сервера; пусто у подписки — у неё имя местное
	Form    Form     `json:"form"`    // имя и цвет одним листом: оба уходят одним запросом
	Address Text     `json:"address"` // показан и не правится (S-12.2)
	Note    Text     `json:"note"`    // «переименование — не перенос»: адрес остаётся прежним
	Failure *Notice  `json:"failure"` // отказ сервера; форма хранит введённое
	Actions []Action `json:"actions"`
}

type CollectionMoveSheet struct {
	Kind    Kind         `json:"kind"`
	Copy    bool         `json:"copy"`    // тот же лист с другой кнопкой; исходная остаётся
	Subject Text         `json:"subject"` // что переносят: «Дни рождения · nextcloud»
	Target  TargetPicker `json:"target"`  // цель — подключение, а не коллекция (S-12.3)
	Warning Text         `json:"warning"` // завести там, перенести записи, удалить здесь — до начала
	Actions []Action     `json:"actions"`
}

type CollectionDeleteSheet struct {
	Kind       Kind     `json:"kind"`
	Subject    Text     `json:"subject"`
	Warning    Text     `json:"warning"`    // что именно исчезнет и что копии у Carrel нет
	References []Text   `json:"references"` // что на неё ссылается: ссылки, задания, устройства
	Archive    Text     `json:"archive"`    // когда попадала в архив и чего восстановление не сделает
	Export     *Action  `json:"export"`     // «сначала скачать»; nil — экспортировать нечем
	ExportNote Text     `json:"exportNote"`
	Form       Form     `json:"form"` // подтверждение набором имени
	Actions    []Action `json:"actions"`
}

type ImportPage struct {
	Shell
	Kind    Kind          `json:"kind"`
	Step    ImportStep    `json:"step"` // C-8
	Source  ImportSource  `json:"source"`
	Into    TargetPicker  `json:"into"`
	Preview []ImportTable `json:"preview"`
	Report  *ImportReport `json:"report"`
	Actions []Action      `json:"actions"`
}
type ImportSource struct {
	Kinds  []Text        `json:"kinds"`
	Folder *TargetPicker `json:"folder"`
	Note   Text          `json:"note"`
}
type ImportTable struct {
	Rubric  Text     `json:"rubric"`
	Columns []Text   `json:"columns"`
	Rows    [][]Text `json:"rows"`
	Count   int      `json:"count"`
}
type ImportReport struct {
	Created int           `json:"created"`
	Skipped int           `json:"skipped"`
	Renamed int           `json:"renamed"`
	Rows    []ImportTable `json:"rows"`
	Note    Text          `json:"note"`
}

type PublishSheet struct {
	Kind    Kind     `json:"kind"`
	Target  Text     `json:"target"` // что публикуется — написано в шапке, не переспрашивается
	Range   *Range   `json:"range"`  // только у календаря
	Expires Text     `json:"expires"`
	Link    Text     `json:"link"` // готовая ссылка после создания
	Actions []Action `json:"actions"`
}

type MailSheet struct {
	To        []Recipient     `json:"to"`
	Suggest   []Recipient     `json:"suggest"`
	Subject   Text            `json:"subject"`
	Body      Text            `json:"body"`
	From      Text            `json:"from"`    // с какого адреса уйдёт — видно до отправки
	ReplyTo   Text            `json:"replyTo"` // пусто — в письме будет строка «отвечать не нужно»
	ReplyNote Text            `json:"replyNote"`
	Attach    []MailAttachRow `json:"attach"`
	NotAsked  []Stamp         `json:"notAsked"` // книги, которые не ответили, названы прямо
	Limits    Text            `json:"limits"`
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
	Actions []Action        `json:"actions"`
}
