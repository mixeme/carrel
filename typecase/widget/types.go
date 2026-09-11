package widget

// Action is a labeled control used in bars, forms, and menus.
type Action struct {
	Label   string
	Icon    string
	Href    string
	Method  string
	Kind    ButtonKind
	Size    ButtonSize
	Enabled bool
	Reason  string
}

// Render returns the tc-btn markup for this action.
func (a Action) Render() string {
	return Button(a).Render()
}

// MenuItem is one entry in a dropdown menu.
type MenuItem struct {
	Label     string
	Icon      string
	Href      string
	Separator bool
	Disabled  bool
	Reason    string
}

// ButtonKind is the visual role of a button.
type ButtonKind string

const (
	ButtonNormal  ButtonKind = "normal"
	ButtonPrimary ButtonKind = "primary"
	ButtonDanger  ButtonKind = "danger"
)

// ButtonSize is the height tier of a button.
type ButtonSize string

const (
	ButtonSizeForm  ButtonSize = "form"
	ButtonSizeBar   ButtonSize = "bar"
	ButtonSizeTable ButtonSize = "table"
)

// InputKind is the field content role.
type InputKind string

const (
	InputText     InputKind = "text"
	InputNumber   InputKind = "number"
	InputDate     InputKind = "date"
	InputTime     InputKind = "time"
	InputEmail    InputKind = "email"
	InputTel      InputKind = "tel"
	InputURL      InputKind = "url"
	InputPassword InputKind = "password"
	InputSelect   InputKind = "select"
	InputTextarea InputKind = "textarea"
)

// CheckKind is checkbox or radio.
type CheckKind string

const (
	CheckCheckbox CheckKind = "checkbox"
	CheckRadio    CheckKind = "radio"
)

// MarkColour is an external colour token for tc-mark.
type MarkColour string

// BadgeRole is the semantic role of a badge.
type BadgeRole string

const (
	BadgeNeutral BadgeRole = "neutral"
	BadgeLinked  BadgeRole = "linked"
	BadgeAlert   BadgeRole = "alert"
	BadgeLocal   BadgeRole = "local"
	BadgePlan    BadgeRole = "plan"
)

// MsgRole is the tone of a message strip.
type MsgRole string

const (
	MsgInfo   MsgRole = "info"
	MsgAccent MsgRole = "accent"
	MsgAlert  MsgRole = "alert"
)

// SkelShape is the placeholder geometry.
type SkelShape string

const (
	SkelLine   SkelShape = "line"
	SkelRect   SkelShape = "rect"
	SkelCircle SkelShape = "circle"
)

// Density is row density from the shell.
type Density string

const (
	DensityNormal Density = "normal"
	DensityTouch  Density = "touch"
)

// OverlayKind is the overlay presentation mode.
type OverlayKind string

const (
	OverlaySheet   OverlayKind = "sheet"
	OverlayDialog  OverlayKind = "dialog"
	OverlayPopover OverlayKind = "popover"
)

// MenuState is open/closed/disabled for menus.
type MenuState string

const (
	MenuClosed   MenuState = "closed"
	MenuOpen     MenuState = "open"
	MenuDisabled MenuState = "disabled"
)

// MenuAlign is horizontal alignment of the popover.
type MenuAlign string

const (
	MenuAlignStart MenuAlign = "start"
	MenuAlignEnd   MenuAlign = "end"
)

// RowState is the visual state of a list row.
type RowState string

const (
	RowNormal   RowState = "normal"
	RowSelected RowState = "selected"
	RowMuted    RowState = "muted"
	RowAlert    RowState = "alert"
)

// RowVariant selects the cell layout for tc-row.
type RowVariant string

const (
	RowDefault RowVariant = ""
	RowAvatar  RowVariant = "avatar"
	RowLead    RowVariant = "lead"
	RowTick    RowVariant = "tick"
	RowExcerpt RowVariant = "excerpt"
	RowFile    RowVariant = "file"
	RowFind    RowVariant = "find"
)

// TickState is checked/unchecked/indeterminate for tick rows.
type TickState string

const (
	TickOff      TickState = "off"
	TickOn       TickState = "on"
	TickPartial  TickState = "partial"
	TickDisabled TickState = "disabled"
)

// TickRowState is availability of a tick row.
type TickRowState string

const (
	TickRowOK          TickRowState = "ok"
	TickRowReadOnly    TickRowState = "read-only"
	TickRowUnavailable TickRowState = "unavailable"
	TickRowGone        TickRowState = "gone"
)

// ColumnKind is table column width role.
type ColumnKind string

const (
	ColText   ColumnKind = "text"
	ColNumber ColumnKind = "number"
	ColTime   ColumnKind = "time"
	ColAction ColumnKind = "action"
)

// TreeNode is one node in a tree.
type TreeNode struct {
	ID       string
	Label    string
	Children []TreeNode
	Open     bool
	Mark     Mark
	Badges   []Badge
}

// QueueRowState is the state of one queue row.
type QueueRowState string

const (
	QueueIdle      QueueRowState = "idle"
	QueueRunning   QueueRowState = "running"
	QueueDone      QueueRowState = "done"
	QueueFailed    QueueRowState = "failed"
	QueuePartial   QueueRowState = "partial"
	QueueCancelled QueueRowState = "cancelled"
)

// DocMode is reading or editing.
type DocMode string

const (
	DocReading DocMode = "reading"
	DocEditing DocMode = "editing"
)

// DocWidth is measure width for documents.
type DocWidth string

const (
	DocWidthFull    DocWidth = "full"
	DocWidthMeasure DocWidth = "measure"
)

// ProseBlock is one block in structured prose.
type ProseBlock struct {
	Kind  string
	Text  string
	Items []string
	Href  string
	Code  string
	Level int
}

// FieldRow is a label-value pair.
type FieldRow struct {
	Label string
	Value string
	Path  bool
}

// Build is version metadata for rail foot.
type Build struct {
	Name    string
	Version string
	Commit  string
	Href    string
}

// Crumb is one breadcrumb segment.
type Crumb struct {
	Label string
	Href  string
}

// NavItem is one navigation link.
type NavItem struct {
	Label string
	Icon  string
	Href  string
	Count *int
}

// SegOption is one segment in a seg control.
type SegOption struct {
	Label string
	Value string
}

// FieldError is a form-level error tied to a field name.
type FieldError struct {
	Name  string
	Error string
}

// SelectOption is one option in a select input.
type SelectOption struct {
	Label    string
	Value    string
	Selected bool
}

// DiffOption is one choice in a diff card.
type DiffOption struct {
	Label string
	Value string
}

// Step is one step in a diagnostic block.
type Step struct {
	Label  string
	Status string
}

// RunItem is one participant in a run bar.
type RunItem struct {
	Label  string
	State  string
	Action Action
}

// SearchRow is a recent search entry.
type SearchRow struct {
	Query string
	Href  string
}

// JumpRow is a jump-to entry in search.
type JumpRow struct {
	Label string
	Hint  string
	Href  string
}

// MetaSection is one section in doc meta.
type MetaSection struct {
	Title string
	Tags  []Tag
	Rows  []FieldRow
	Rows2 []Row
}

// SideBlock is one block in the side panel.
type SideBlock struct {
	Sec Sec
}

// ListGroup is a rubric and its rows.
type ListGroup struct {
	Rubric Rubric
	Rows   []Row
}

// RailSection is one group in the rail.
type RailSection struct {
	Title string
	State string
	Rows  []TickRow
}

// DangerRow is one destructive action row.
type DangerRow struct {
	Title  string
	Body   string
	Action Action
}

// TableRow is one data row.
type TableRow struct {
	Cells    []string
	Actions  []Action
	Selected bool
}

// Column is a table column definition.
type Column struct {
	Label string
	Kind  ColumnKind
}

// QueueRow is one row in the operation queue.
type QueueRow struct {
	Label    string
	State    QueueRowState
	Percent  *int
	Question string
	Choices  []Action
	Action   Action
}

// RowCells holds variant-specific cell data for tc-row.
type RowCells struct {
	Avatar   Avatar
	Lead     string
	Title    string
	Subtitle string
	Excerpt  string
	Icon     string
	Name     string
	Size     string
	Time     string
	Detail   string
	Caption  string
	Due      string
	Progress *int
}
