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
	Crumbs []Crumb        `json:"crumbs"`
	Bar    ListBar        `json:"bar"`
	View   FileView       `json:"view"` // список или плитки
	Rows   []FileRow      `json:"rows"`
	Queue  *TransferQueue `json:"queue"`
	Tree   *FileTree      `json:"tree"` // дерево выбора; на странице раздела nil
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
}
type FileTreeNode struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Stamp    Stamp          `json:"stamp"`
	ReadOnly bool           `json:"readOnly"`
	Children []FileTreeNode `json:"children"`
	Expanded bool           `json:"expanded"`
}

type FilePickerSheet struct {
	Purpose Text     `json:"purpose"` // зачем выбирают: папка вложений, назначение копии
	Tree    FileTree `json:"tree"`
	Actions []Action `json:"actions"`
}

type FileOpSheet struct {
	Op      FileOp    `json:"op"`      // C-8
	Title   Text      `json:"title"`   // «Move 3 items» (s-files-4)
	Subject Text      `json:"subject"` // откуда: «from Documents / carrel / attachments»
	Items   []Text    `json:"items"`
	Target  *FileTree `json:"target"`
	Warning Text      `json:"warning"` // что сделает перенос: на одном сервере MOVE, между хранилищами — скачивание и загрузка
	Failure *Notice   `json:"failure"` // New folder не завёл папку; nil — отказа не было
	Actions []Action  `json:"actions"`
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
