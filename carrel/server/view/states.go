package view

// Перечисления. Везде, где значений конечное число, во вью-модели стоит
// один из этих типов, а не свободная строка.

type Kind string

const (
	KindContact Kind = "contact"
	KindEvent   Kind = "event"
	KindTask    Kind = "task"
	KindNote    Kind = "note"
	KindFile    Kind = "file"
)

// SourceState — источник в веерном опросе.
type SourceState string

const (
	SourceWaiting     SourceState = "waiting" // очередь дошла не до всех
	SourceQuerying    SourceState = "querying"
	SourceDone        SourceState = "done"        // ответил, число в Count
	SourceEmpty       SourceState = "empty"       // ответил, находок нет
	SourceError       SourceState = "error"       // ответил отказом
	SourceTimeout     SourceState = "timeout"     // не успел
	SourceCancelled   SourceState = "cancelled"   // опрос прерван
	SourceUnavailable SourceState = "unavailable" // не отвечает вовсе
)

type CollectionState string

const (
	CollectionOK          CollectionState = "ok"
	CollectionReadOnly    CollectionState = "read-only" // права сервера
	CollectionUnavailable CollectionState = "unavailable"
	CollectionGone        CollectionState = "gone" // на сервере её больше нет
)

type RecordState string

const (
	RecordOK          RecordState = "ok"
	RecordConflict    RecordState = "conflict" // разошлась с сервером
	RecordUnavailable RecordState = "unavailable"
	RecordLinked      RecordState = "linked" // связана как дубликат
)

type TickState string

const (
	TickOff  TickState = "off"
	TickOn   TickState = "on"
	TickPart TickState = "part"
)

type OpState string

const (
	OpIdle      OpState = "idle"
	OpRunning   OpState = "running"
	OpDone      OpState = "done"
	OpFailed    OpState = "failed"
	OpPartial   OpState = "partial"
	OpCancelled OpState = "cancelled"
)

type FileOp string

const (
	FileRename FileOp = "rename"
	FileMove   FileOp = "move"
	FileCopy   FileOp = "copy"
	FileDelete FileOp = "delete"
	// FileNewFolder — лист New folder шапки папки и подвала рейла (s-files-23)
	FileNewFolder FileOp = "new"
)

type ImportStep string

const (
	ImportStepSource  ImportStep = "source"
	ImportStepPreview ImportStep = "preview"
	ImportStepReport  ImportStep = "report"
)

type ConflictChoice string

const (
	ChoiceMine   ConflictChoice = "mine"
	ChoiceTheirs ConflictChoice = "theirs"
	ChoiceUnset  ConflictChoice = ""
)

// ConflictDiff — вид расхождения у свойства (s-conflict-1…-5): изменили обе
// стороны, только правка, только сервер; пусто — не известно (версии, с
// которой открыли форму, уже нет).
type ConflictDiff string

const (
	DiffBoth    ConflictDiff = "both"
	DiffMine    ConflictDiff = "mine"
	DiffTheirs  ConflictDiff = "theirs"
	DiffUnknown ConflictDiff = ""
)

type DuplicateDecision string

const (
	DecisionNone    DuplicateDecision = ""
	DecisionLinked  DuplicateDecision = "linked" // ничего не пишется на серверы
	DecisionIgnored DuplicateDecision = "ignored"
	DecisionMerged  DuplicateDecision = "merged"
)

type NoteMode string

const (
	NoteReading NoteMode = "reading"
	NoteEditing NoteMode = "editing"
)

type FileView string

const (
	FileList  FileView = "list"
	FileTiles FileView = "tiles"
)

type EmailState string

const (
	EmailNone        EmailState = "none"
	EmailConfirmed   EmailState = "confirmed"
	EmailPending     EmailState = "pending"
	EmailUnconfirmed EmailState = "unconfirmed"
	EmailSent        EmailState = "sent"
)

type NoticeRole string

const (
	NoticeInfo   NoticeRole = "info"
	NoticeAccent NoticeRole = "accent"
	NoticeAlert  NoticeRole = "alert"
)

type BadgeRole string

const (
	BadgeNeutral BadgeRole = "neutral"
	BadgeLinked  BadgeRole = "linked"
	BadgeAlert   BadgeRole = "alert"
	BadgeLocal   BadgeRole = "local"
	BadgePlan    BadgeRole = "plan"
)

type InputKind string

const (
	InputText     InputKind = "text"
	InputPassword InputKind = "password"
	InputEmail    InputKind = "email"
	InputURL      InputKind = "url"
	InputDate     InputKind = "date"
	InputTime     InputKind = "time"
	InputNumber   InputKind = "number"
	InputSelect   InputKind = "select"
	InputCheck    InputKind = "check"
	InputRadio    InputKind = "radio"
	InputArea     InputKind = "area"
	// InputHidden is a value the form carries and nobody fills in: the
	// token, the version a form was read at, the step it stands for. It is
	// never shown and has no label.
	InputHidden InputKind = "hidden"
	// InputAdd is a button that adds a row to a repeating field: «Add
	// phone» (s-contacts-3), «Add reminder» (s-agenda-3). Template is the
	// row it adds; without scripts it posts the form with Name=Value, and
	// the form comes back with one more row, nothing written.
	InputAdd InputKind = "add"
	// InputRemove is Remove at the end of one row of a repeating field: a
	// phone, an e-mail, an address (s-contacts-3). Value names the row
	// («tel.1»); without scripts it posts the form with Name=Value, and the
	// form comes back without the row, nothing written.
	InputRemove InputKind = "remove"
)

type DeviceTab string

const (
	TabApple       DeviceTab = "apple"
	TabAndroid     DeviceTab = "android"
	TabThunderbird DeviceTab = "thunderbird"
	TabOther       DeviceTab = "other"
)

type CheckState string

const (
	CheckOK      CheckState = "ok"
	CheckBad     CheckState = "bad"
	CheckUnknown CheckState = "unknown" // серый флажок: то, что снаружи не измерить
)

// StatusTone — точка строки состояния рейла (RailStatus, components-35).
type StatusTone string

const (
	StatusOn    StatusTone = "on"    // идёт как надо — точка акцента
	StatusOff   StatusTone = "off"   // выключено или не проверялось — приглушённо
	StatusAlert StatusTone = "alert" // требует внимания — тревога
)

type PreviewKind string

const (
	PreviewNone     PreviewKind = "none"
	PreviewImage    PreviewKind = "image"
	PreviewPDF      PreviewKind = "pdf"
	PreviewText     PreviewKind = "text"
	PreviewMarkdown PreviewKind = "markdown"
)

type BlockKind string

const (
	BlockPara    BlockKind = "para"
	BlockHeading BlockKind = "heading"
	BlockList    BlockKind = "list"
	BlockItem    BlockKind = "item"
	BlockTask    BlockKind = "task" // пункт-задача «- [ ]»; отметка — Block.Checked
	BlockCode    BlockKind = "code"
	BlockQuote   BlockKind = "quote"
	BlockLink    BlockKind = "link"
	BlockEmph    BlockKind = "emph"
	BlockStrong  BlockKind = "strong"
	BlockTable   BlockKind = "table" // таблица; Children — строки
	BlockRow     BlockKind = "row"   // строка таблицы; Children — ячейки, Header — строка заголовка
	BlockCell    BlockKind = "cell"  // ячейка; текст — как у абзаца, Text или Children
)

// CellAlign — выравнивание столбца таблицы в разметке («:--», «:-:», «--:»).
// Пусто — не задано.
type CellAlign string

const (
	AlignLeft   CellAlign = "left"
	AlignCenter CellAlign = "center"
	AlignRight  CellAlign = "right"
)

// RunMark — выделение куска фразы сервера (Run). Пусто — обычный текст.
// Слова те же, что у Field.Mono и BlockStrong: моноширинным кадр пишет то,
// что набирают и копируют как есть (логин, адрес), жирным — имя в фразе.
type RunMark string

const (
	RunPlain  RunMark = ""
	RunMono   RunMark = "mono"   // tc-mono: логин, адрес, путь
	RunStrong RunMark = "strong" // <b>: имя в фразе
	RunDanger RunMark = "danger" // тревога: источник, который не ответил (s-contacts-7)
)
