package view

// Строка списка своя у каждого рода — это разные наборы данных, и общей
// строки «на всё» не заводится.

type ListBar struct {
	Filter   Text     `json:"filter"` // фильтр по уже загруженному, без второго опроса
	Loaded   int      `json:"loaded"`
	Total    *int     `json:"total"`
	Compact  bool     `json:"compact"`
	Segments []Action `json:"segments"` // Open / Done / All; All / Contacts / Events / …
	Range    *Range   `json:"range"`    // диапазон дат там, где он есть
	Actions  []Action `json:"actions"`  // операции над выделенным
	Selected int      `json:"selected"`
}

type Range struct {
	From    Text     `json:"from"`
	To      Text     `json:"to"`
	Presets []Action `json:"presets"`
}

type ContactsListPage struct {
	Shell
	Bar    ListBar        `json:"bar"`
	Groups []ContactGroup `json:"groups"`
	Poll   *PollBar       `json:"poll"` // единый вид; nil — одна коллекция
}
type ContactGroup struct {
	Rubric Text         `json:"rubric"`
	Rows   []ContactRow `json:"rows"`
}
type ContactRow struct {
	Href    string      `json:"href"`
	UID     string      `json:"uid"`
	Colour  Colour      `json:"colour"`
	Stamp   Stamp       `json:"stamp"`
	Avatar  Avatar      `json:"avatar"`
	Name    Text        `json:"name"`
	Phone   Text        `json:"phone"`
	Email   Text        `json:"email"`
	Org     Text        `json:"org"`
	Badges  []Badge     `json:"badges"`  // «2 источника» у свёрнутого дубликата
	Sources []Stamp     `json:"sources"` // раскрытые источники свёрнутой группы
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
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
	Href    string      `json:"href"`
	UID     string      `json:"uid"`
	Colour  Colour      `json:"colour"`
	Stamp   Stamp       `json:"stamp"`
	Done    bool        `json:"done"`
	Summary Text        `json:"summary"`
	Due     Text        `json:"due"`
	Overdue bool        `json:"overdue"`
	Percent *int        `json:"percent"`
	Tags    []Tag       `json:"tags"`
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
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
	Menu    []MenuItem  `json:"menu"`
	State   RecordState `json:"state"`
}
