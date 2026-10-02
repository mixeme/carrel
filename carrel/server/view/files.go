package view

type FilesRootPage struct {
	Shell
	Collections []FileCollectionRow `json:"collections"`
	Empty       *Notice             `json:"empty"` // ни одной папки: почему пусто и что сделать
}
type FileCollectionRow struct {
	Href     string          `json:"href"`
	Label    Text            `json:"label"`
	Stamp    Stamp           `json:"stamp"`
	Colour   Colour          `json:"colour"`
	ReadOnly bool            `json:"readOnly"`
	Purpose  Text            `json:"purpose"` // вложения, публикации
	State    CollectionState `json:"state"`
}

type FilesPage struct {
	Shell
	// Subtitle — строка под заголовком папки: подключение хранилища, у
	// хранилища только для чтения с тем, почему Upload и New folder
	// выключены: «sftpgo · vps · read-only: nothing can be uploaded or made
	// here» (s-files-15)
	Subtitle Text `json:"subtitle"`
	// Attachments — эта папка — папка вложений (S-20): метка «attachment
	// folder» в строке под заголовком (s-files-1); слово — экрана
	Attachments bool           `json:"attachments"`
	Crumbs      []Crumb        `json:"crumbs"`
	Bar         ListBar        `json:"bar"`
	View        FileView       `json:"view"` // список или плитки
	Rows        []FileRow      `json:"rows"`
	Queue       *TransferQueue `json:"queue"`
	Tree        *FileTree      `json:"tree"` // дерево выбора; на странице раздела nil
	// Select — форма выделения без сценариев (U-8, как RestoreMatch.Form):
	// GET на адрес папки, скрытые поля её запроса (view, sort, filter) и
	// select каждого выделенного. Отметка строки — кнопка отправки этой
	// формы: поле toggle, значение — имя строки; отметка в шапке таблицы —
	// поле all со значением 1 (выделить все показанные, а все выделенные —
	// снять). Сервер отвечает редиректом на папку с новым select. Actions —
	// «Clear selection» (files.select.clear, папка без select), пока выделено
	Select *Form `json:"select"`
	// Name — строка имени новой папки, которую открывает New folder шапки и
	// рейла (POST mkcol без поля name): поле name «Name of the new folder»,
	// Create (Form.Action — адрес mkcol) и Cancel — та же папка. После отказа
	// строка приходит с набранным, отказ — в Notices; nil — строку не
	// открывали
	Name *Form `json:"name"`
}
type Crumb struct {
	Label Text   `json:"label"`
	Href  string `json:"href"`
}
type FileRow struct {
	Href     string     `json:"href"`
	Name     Text       `json:"name"`
	IsDir    bool       `json:"isDir"`
	Type     Text       `json:"type"`
	Size     Size       `json:"size"`
	Modified When       `json:"modified"`
	Thumb    string     `json:"thumb"` // адрес миниатюры; пусто — значок типа
	Selected bool       `json:"selected"`
	Menu     []MenuItem `json:"menu"`
	// Upload — у строки папки: загрузка в неё (бросок на строку, U-15.3;
	// s-files-1), POST …/upload/{путь папки}/ с полем файла File; у
	// хранилища только для чтения выключена с причиной; у файла nil
	Upload *Action `json:"upload"`
}
type TransferQueue struct {
	Rows []TransferRow `json:"rows"`
	Note Text          `json:"note"`
}
type TransferRow struct {
	Name     Text       `json:"name"`
	Size     Size       `json:"size"`
	Percent  int        `json:"percent"`
	State    OpState    `json:"state"`    // C-8
	Question *NameClash `json:"question"` // занятое имя — вопрос на месте, а не настройка
	Actions  []Action   `json:"actions"`
}

// NameClash — Skip · Keep both · Overwrite this one.
type NameClash struct {
	Name    Text     `json:"name"`
	Options []Action `json:"options"`
}
type FileTree struct {
	Nodes  []FileTreeNode `json:"nodes"`
	Chosen string         `json:"chosen"`
	// New — «New folder» в рубрике дерева (s-files-4, s-notes-11): папка
	// заводится в выбранной и сразу выбирается; nil — дерево без неё
	New *Action `json:"new"`
	// Name — строка имени над деревом, которую открывает New (s-files-13):
	// поле name «Name of the new folder», Create и Cancel; Action — New.Href.
	// После отказа строка приходит с набранным, отказ — под ней. nil — у
	// дерева нет New или она выключена
	Name *Form `json:"name"`
}
type FileTreeNode struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Stamp    Stamp          `json:"stamp"`
	ReadOnly bool           `json:"readOnly"`
	Children []FileTreeNode `json:"children"`
	Expanded bool           `json:"expanded"`
	// File — узел-файл дерева, из которого выбирают файл (From Files у
	// Attach); у дерева папок таких узлов нет
	File bool `json:"file,omitempty"`
	// Size и Lands — у узла-файла дерева From Files (s-files-16): размер и
	// «inside» — ляжет в запись — или «link» — запись сошлётся на него, по
	// правилу вложения (S-20); у прочих узлов пусто
	Size  Size `json:"size,omitempty"`
	Lands Text `json:"lands,omitempty"`
	// Href — тот же лист, где выбран этот узел (GET; у листа операции —
	// адрес op без go, с to=ID): без сценариев узел выбирает ссылкой;
	// недоступный узел — пусто
	Href string `json:"href,omitempty"`
}

type FilePickerSheet struct {
	Title   Text     `json:"title"`   // «Upload files» (s-notes-11), «Attach from Files» (s-files-16)
	Subject Text     `json:"subject"` // для чего: «to «Собрать чемодан»»; пусто — строки нет
	Purpose Text     `json:"purpose"` // зачем выбирают: папка вложений, назначение копии, файл для вложения; остаётся и при отказе
	Tree    FileTree `json:"tree"`
	// Failure — отказ: New folder не завёл папку (под строкой имени, как
	// FileOpSheet.Failure, s-files-19) или папку нельзя выбрать; nil — отказа
	// не было
	Failure *Notice  `json:"failure"`
	Actions []Action `json:"actions"`
}

type FileOpSheet struct {
	Op      FileOp    `json:"op"`      // C-8
	Title   Text      `json:"title"`   // «Move 3 items» (s-files-4)
	Subject Text      `json:"subject"` // откуда: «from Documents / carrel / attachments»
	Items   []Text    `json:"items"`
	Target  *FileTree `json:"target"`
	Warning Runs      `json:"warning"` // что сделает перенос: на одном сервере MOVE (моноширинно, s-files-4), между хранилищами — скачивание и загрузка
	Failure *Notice   `json:"failure"` // New folder не завёл папку; nil — отказа не было
	// Form — у переименования: POST на адрес подтверждения (go=1), скрытый
	// _csrf и поле name — новое имя, сначала нынешнее, курсор в нём; Rename
	// листа отправляет её. У прочих операций nil
	Form    *Form    `json:"form"`
	Actions []Action `json:"actions"`
}

type PublishedLinksPage struct {
	Shell
	Rows []PublishedRow `json:"rows"`
}
type PublishedRow struct {
	Title    Text     `json:"title"`
	Kind     Kind     `json:"kind"`
	Stamp    Stamp    `json:"stamp"`
	Link     Text     `json:"link"`
	Created  When     `json:"created"`
	Expires  Text     `json:"expires"`
	LastSeen When     `json:"lastSeen"`
	Opens    int      `json:"opens"`
	Actions  []Action `json:"actions"` // Copy · Send (с почтой) · Revoke (s-publish-1)
}
