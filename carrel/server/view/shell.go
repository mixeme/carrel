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
	Search   SearchBox  `json:"search"`
	NewMenu  []MenuItem `json:"newMenu"`  // «New note» и стрелка: остальные роды, файл, письмо
	User     []MenuItem `json:"user"`     // меню учётной записи
	Unlocked *When      `json:"unlocked"` // до какого времени открыта выдача учётных данных
	Refresh  Refresh    `json:"refresh"`
}

type Refresh struct {
	Action   Action `json:"action"`
	Answered When   `json:"answered"` // когда закончился последний веерный опрос этого раздела
	Running  bool   `json:"running"`  // опрос идёт: подпись говорит это, кнопка не повторяет его
}

type SearchBox struct {
	Query   Text               `json:"query"`
	Action  string             `json:"action"`
	History []SearchHistoryRow `json:"history"` // C-6.6
	Jump    []JumpRow          `json:"jump"`    // подсказки из памяти сеанса: ни одного запроса
	Whole   *Action            `json:"whole"`   // «Timeline · имя»; nil — контакт не совпал
}

type Rail struct {
	Nav      []NavRow      `json:"nav"` // разделы; первым списком, над коллекциями
	Sections []RailSection `json:"sections"`
	Tags     []Tag         `json:"tags"` // фильтр по категориям; у файлов пуст
	Foot     []Action      `json:"foot"` // New collection, Connect account, Subscribe to a feed;
	// сюда же встаёт Apply там, где выбор применяется не сразу
	// (без сценариев и на узком экране — U-8, U-9)
	About Action `json:"about"` // название, версия и коммит сборки — последней строкой (C-13)
	Note  Text   `json:"note"`  // строка состояния под списком
}

type NavRow struct {
	Action
	Count *int `json:"count"` // nil — счёт этому разделу не считается: прочерк, не ноль
	On    bool `json:"on"`    // открытый раздел; на выдаче поиска не отмечен ни один
}

type RailSection struct {
	Account   Text            `json:"account"`
	AccountID string          `json:"accountID"`
	State     CollectionState `json:"state"` // недоступен — помечается здесь, C-8
	Rows      []SourceRow     `json:"rows"`
}

type SourceRow struct {
	Href     string          `json:"href"`
	Label    Text            `json:"label"`
	Colour   Colour          `json:"colour"`
	Count    *int            `json:"count"` // nil — коллекция снята или ещё не отвечала: прочерк, не ноль
	Tick     TickState       `json:"tick"`  // C-8
	ReadOnly bool            `json:"readOnly"`
	State    CollectionState `json:"state"`
	Menu     []MenuItem      `json:"menu"`
}

type Details struct {
	Title   Text        `json:"title"`
	Fields  []Field     `json:"fields"`
	Source  SourceBlock `json:"source"`
	Actions []Action    `json:"actions"` // повтор того, что есть в панели над списком, не замена
}

type Field struct {
	Label Text   `json:"label"`
	Value Text   `json:"value"`
	Href  string `json:"href"`
	Mono  bool   `json:"mono"` // путь, идентификатор, диагностика
}

// Print — колонтитул распечатки, C-10.
type Print struct {
	Title Text `json:"title"`
	Range Text `json:"range"`
	Taken When `json:"taken"`
}
