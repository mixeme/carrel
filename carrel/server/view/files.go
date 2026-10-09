package view

type FilesRootPage struct {
	Shell
	// Subtitle — под заголовком: «2 of 3 storages on 2 servers» (s-files-6):
	// отмечено из живых хранилищ и на скольких подключениях они все
	Subtitle    Text                `json:"subtitle"`
	Collections []FileCollectionRow `json:"collections"`
	// Note — сообщение под таблицей (s-files-6): что такое корень и, когда
	// есть снятые хранилища, кто не отмечен — их имена жирным, «‹Public› is
	// not ticked, so it is not here.»; пусто — показать нечего
	Note  Runs    `json:"note"`
	Empty *Notice `json:"empty"` // ни одной папки: почему пусто и что сделать
}
type FileCollectionRow struct {
	Href     string          `json:"href"`
	Label    Text            `json:"label"`
	Stamp    Stamp           `json:"stamp"`
	Server   Text            `json:"server"` // подключение — столбец Server моноширинно, «sftpgo · vps» (s-files-6)
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
	// Count — сколько в папке, «5 items», «1 item», «0 items», после метки
	// папки вложений (s-files-1, states-32): всё, что в ней лежит, а не то,
	// что оставил фильтр; пусто — папка не прочитана (states-33)
	Count Text `json:"count"`
	// Access — у хранилища только для чтения, после счёта: «read-only:
	// nothing can be uploaded or made here» (states-34); у прочих пусто
	Access Text `json:"access"`
	// Attachments — эта папка — папка вложений (S-20): метка «attachment
	// folder» в строке под заголовком (s-files-1); слово — экрана
	Attachments bool      `json:"attachments"`
	Crumbs      []Crumb   `json:"crumbs"`
	Bar         ListBar   `json:"bar"`
	View        FileView  `json:"view"` // список или плитки
	Rows        []FileRow `json:"rows"`
	// Empty — в папке ничего нет (states-32): «This folder is empty.», что с
	// ней сделать и кнопки шапки — Upload со стрелкой и New folder; у
	// хранилища только для чтения — один заголовок. Полосы и таблицы у такой
	// папки нет. nil — в папке что-то лежит (и когда всё скрыл фильтр)
	Empty *Notice `json:"empty"`
	// Unread — сервер хранилища не ответил (states-33): на экране шапка без
	// счёта и сообщение из Notices; строк, полосы и очереди нет
	Unread bool           `json:"unread"`
	Queue  *TransferQueue `json:"queue"`
	Tree   *FileTree      `json:"tree"` // дерево выбора; на странице раздела nil
	// Select — форма выделения (s-files-5; решение владельца 09.10.2026):
	// POST на адрес операций папки (…/op/…) со скрытым _csrf. Отметка
	// строки и плитки — поле этой формы SelectField со значением — именем
	// строки, отмеченное у выделенных (FileRow.Selected); операции полосы
	// выделения — Rename, Move, Copy, Delete — её кнопки (Action.Submits,
	// операция — в адресе кнопки). Без сценариев полоса выделения стоит
	// всегда, поэтому операции приходят и когда ничего не выделено. Actions
	// — «Clear selection» (files.select.clear, папка без select), пока
	// выделено адресом
	Select *Form `json:"select"`
	// SelectField — имя поля отметки формы Select: «item»
	SelectField string `json:"selectField"`
}
type Crumb struct {
	Label Text   `json:"label"`
	Href  string `json:"href"`
}
type FileRow struct {
	Href string `json:"href"`
	// Open — адрес имени строки (s-files-1): у папки — она сама (как Href),
	// у файла — его папка с ним выделенным; Href файла — скачивание
	Open     string `json:"open"`
	Name     Text   `json:"name"`
	IsDir    bool   `json:"isDir"`
	Type     Text   `json:"type"`
	Size     Size   `json:"size"`
	Modified When   `json:"modified"` // числами: «2026-08-10 11:07» (s-files-1)
	Thumb    string `json:"thumb"`    // адрес миниатюры; пусто — значок типа
	// Icon — значок строки по типу (s-files-1): i-file у папки, i-img у
	// картинки, i-doc у прочего
	Icon string `json:"icon"`
	// Note — почему у картинки нет миниатюры: «too large» — больше потолка
	// миниатюры (s-files-5); у прочих пусто
	Note Text `json:"note,omitempty"`
	// Items — у папки: сколько в ней, «34 items», «1 item», «0 items»
	// (s-files-1: Type «folder · 34 items»; s-files-5: плитка «12 items»).
	// Число — из того же PROPFIND папки, где сервер его называет (Nextcloud),
	// иначе — из папки, которую эта сессия уже читала; пусто — неизвестно
	// (не «0 items»). У файла пусто
	Items Text `json:"items,omitempty"`
	// Dimensions — у картинки: ширина и высота, «4032×3024» (s-files-5:
	// «2.8 MB · 4032×3024»), — у одной выбранной, из начала файла (S-17);
	// у прочих пусто
	Dimensions Text       `json:"dimensions,omitempty"`
	Selected   bool       `json:"selected"`
	Menu       []MenuItem `json:"menu"`
	// Upload — у строки папки: загрузка в неё (бросок на строку, U-15.3;
	// s-files-1), POST …/upload/{путь папки}/ с полем файла File; у
	// хранилища только для чтения выключена с причиной; у файла nil
	Upload *Action `json:"upload"`
}
type TransferQueue struct {
	// Title — шапка очереди (s-files-2): «Uploading 2 of 3» — пока строка
	// ждёт ответа, которая она по счёту; всё решено — «Uploaded 2 of 3»,
	// сколько сделано; глагол — по роду строк (Moving, Copying, Renaming,
	// Transferring у разных)
	Title Text          `json:"title"`
	Total Size          `json:"total"` // объём файлов очереди моноширинно: «12.4 MB»
	Rows  []TransferRow `json:"rows"`
	// Note — почему: отказы строк и отказ загрузки целиком, через « · »;
	// пусто — сказать нечего
	Note Text `json:"note"`
}
type TransferRow struct {
	Name     Text       `json:"name"`
	Size     Size       `json:"size"`
	Percent  int        `json:"percent"`
	State    OpState    `json:"state"`    // C-8
	Question *NameClash `json:"question"` // занятое имя — вопрос на месте, а не настройка
	Actions  []Action   `json:"actions"`
}

// NameClash — Keep both · Replace · Skip (s-files-2): Replace перезаписывает
// осознанно, единственная запись без предусловия (S-17.1).
type NameClash struct {
	Name    Text     `json:"name"`
	Options []Action `json:"options"`
}

// FileServer — сервер в выборе шапки листа с деревом (s-files-4,
// s-notes-11, s-files-16: «Server»): подключение, у которого есть
// хранилища файлов. On — выбранный: дерево листа показывает его хранилища;
// Href — тот же лист с этим сервером (GET), у выбранного пусто
type FileServer struct {
	Label Text   `json:"label"`
	Href  string `json:"href"`
	On    bool   `json:"on"`
}

type FileTree struct {
	Nodes  []FileTreeNode `json:"nodes"`
	Chosen string         `json:"chosen"`
	// Field — у дерева листа переноса и копии (s-files-4): имя поля формы
	// листа, которым узел отмечают, — «to», значение — ID узла; у узла
	// только для чтения отметка выключена. Пусто — дерево выбирает адресом
	// узла (Href): листы выбора
	Field string `json:"field"`
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
	ID    string `json:"id"`
	Label string `json:"label"`
	// Stamp — пусто: сервер называет шапка листа (FileServer), корень
	// хранилища его не повторяет
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
	// Attachments — это папка вложений (бейдж «attachment folder», s-notes-11);
	// Fresh — папка только что заведена New folder этого листа («just
	// created»); слова — экрана
	Attachments bool `json:"attachments,omitempty"`
	Fresh       bool `json:"fresh,omitempty"`
	// Href — тот же лист, где выбран этот узел (GET; у листа операции —
	// адрес op без go, с to=ID): без сценариев узел выбирает ссылкой;
	// недоступный узел — пусто
	Href string `json:"href,omitempty"`
}

type FilePickerSheet struct {
	Title   Text `json:"title"`   // «Upload files» (s-notes-11), «Attach from Files» (s-files-16)
	Subject Text `json:"subject"` // для чего: «to «Собрать чемодан»»; пусто — строки нет
	Purpose Text `json:"purpose"` // зачем выбирают: папка вложений, назначение копии, файл для вложения; остаётся и при отказе
	// Where — выбранная папка путём, «Documents / carrel / 2026-08» (выбор в
	// шапке листа загрузки, s-notes-11, s-files-19); пусто — не выбрана или
	// у листа шапки выбора нет
	Where Text `json:"where"`
	// Servers — выбор сервера в шапке листа; Tree — хранилища выбранного
	Servers []FileServer `json:"servers"`
	Tree    FileTree     `json:"tree"`
	// Failure — отказ: New folder не завёл папку (под строкой имени, как
	// FileOpSheet.Failure, s-files-19) или папку нельзя выбрать; nil — отказа
	// не было
	Failure *Notice  `json:"failure"`
	Actions []Action `json:"actions"`
}

type FileOpSheet struct {
	Op    FileOp `json:"op"`    // C-8
	Title Text   `json:"title"` // «Move 3 items» (s-files-4)
	// Subject — где: у переноса и копии «from Documents / carrel /
	// attachments» (s-files-4), у переименования и удаления — путь без
	// «from» (s-files-21, -22)
	Subject Text   `json:"subject"`
	Items   []Text `json:"items"`
	// Servers — выбор сервера в шапке листа переноса и копии; Target —
	// хранилища выбранного. У прочих операций пусто
	Servers []FileServer `json:"servers"`
	Target  *FileTree    `json:"target"`
	// Alert — у удаления, тревогой перед Warning (s-files-22): сколько
	// исчезнет, какого объёма и на каком сервере, и что копии нет — «‹3›
	// files, 4.4 MB, are deleted on sftpgo · vps. …», число жирным; у папки
	// — «The folder and everything in it …». У прочих операций пусто
	Alert Runs `json:"alert"`
	// Warning — что сделает операция: перенос на одном сервере — MOVE
	// (моноширинно, s-files-4), между хранилищами — скачивание и загрузка;
	// переименование — кого заденет новый адрес (s-files-21); удаление —
	// что ссылок на файлы никто не считал (s-files-22)
	Warning Runs    `json:"warning"`
	Failure *Notice `json:"failure"` // New folder не завёл папку; nil — отказа не было
	// У листа New folder (Op new, s-files-23): Title «New folder», Subject «in
	// Documents / carrel / attachments», Form — POST на адрес mkcol с полем
	// name «Name», Failure — отказ под полем (имя занято: та же фраза, что в
	// листе переноса), Actions — Create (кнопка формы) и Cancel
	//
	// Form — форма листа. У переименования: POST на адрес подтверждения
	// (go=1), скрытый _csrf и поле name — новое имя, сначала нынешнее,
	// курсор в нём; Rename листа отправляет её. У переноса и копии
	// (s-files-4): POST на адрес подтверждения со скрытым _csrf, цель — поле
	// Target.Field, отметка узла дерева; «Move» и «Copy» — кнопка этой
	// формы (Submits). Пока выбранная папка целью быть не может, кнопка
	// выключена с причиной, а форма стоит: адрес узла (Href) — лист с ним
	// выбранным и кнопкой, какая она для него. У удаления nil
	Form    *Form    `json:"form"`
	Actions []Action `json:"actions"`
}

type PublishedLinksPage struct {
	Shell
	// Subtitle — под заголовком (s-publish-1): «Read-only, one object or one
	// collection per link»; у локального экземпляра — и «this instance is
	// local: the link opens only from your network» тревогой
	Subtitle Runs           `json:"subtitle"`
	Rows     []PublishedRow `json:"rows"`
}
type PublishedRow struct {
	Title Text `json:"title"`
	// Kind — что за ссылка словом (s-publish-1): у коллекции — её род,
	// «calendar», «address book», «task list», «notebook»; у записи — род
	// записи, «event», «contact», «task», «note»; «file», «folder»
	Kind  Text  `json:"kind"`
	Stamp Stamp `json:"stamp"`
	// Colour — метка коллекции у ссылки на коллекцию и запись; Icon — значок
	// у файла (i-doc) и папки (i-file); у прочих пусто
	Colour Colour `json:"colour,omitempty"`
	Icon   string `json:"icon,omitempty"`
	Link   Text   `json:"link"`
	// Address — ссылка целиком, со схемой и узлом: её берёт Copy
	// (s-publish-1: «https://carrel.example/p/9c1f4e»); Link — путь
	Address Text `json:"address"`
	// Created, Expires, LastSeen — числами, моноширинно в таблице
	// (s-publish-1; слово владельца 09.10.2026 — пока как в кадре):
	// «2026-08-10», «2026-09-08» или «No expiry», «2026-08-10 08:41» или
	// «never opened»
	Created  When `json:"created"`
	Expires  Text `json:"expires"`
	LastSeen When `json:"lastSeen"`
	// Unopened — ссылку ни разу не открывали: LastSeen приглушён
	Unopened bool     `json:"unopened"`
	Opens    int      `json:"opens"`
	Actions  []Action `json:"actions"` // Copy · Send (с почтой) · Revoke (s-publish-1)
}
