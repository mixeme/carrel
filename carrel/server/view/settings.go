package view

// Form — общая форма: поля, ошибки, кнопки.
type Form struct {
	Action  string       `json:"action"`
	Method  string       `json:"method"`
	Sets    []FieldSet   `json:"sets"`
	Errors  []FieldError `json:"errors"`
	Actions []Action     `json:"actions"`
}
type FieldSet struct {
	Legend Text `json:"legend"`
	// Folded — набор свёрнут: Legend — кнопка, раскрывающая строки («Show 6
	// empty fields», s-contacts-3)
	Folded bool `json:"folded,omitempty"`
	// Unfold — та кнопка без сценариев: GET на ту же форму с ?fields=all,
	// где каждое поле в своей рубрике; nil у несвёрнутого набора
	Unfold *Action   `json:"unfold"`
	Rows   []FormRow `json:"rows"`
}
type FormRow struct {
	Name  string `json:"name"`
	Label Text   `json:"label"`
	// Приглушённая часть подписи, после неё: «(optional)» у «Email»
	// (s-admin-1, s-auth-4), «SHA-256 · optional» у «Certificate
	// fingerprint» (s-profile-2); у прочих пусто
	LabelAside Text      `json:"labelAside,omitempty"`
	Kind       InputKind `json:"kind"` // C-8
	Value      Text      `json:"value"`
	Options    []Option  `json:"options"`
	Hint       Text      `json:"hint"`
	Error      Text      `json:"error"`
	Extra      *Action   `json:"extra"` // кнопка, принадлежащая полю
	ReadOnly   bool      `json:"readOnly"`
	Required   bool      `json:"required"`
	// Образец в пустом поле, как его рисует кадр: «DD.MM.YYYY» у дня
	// рождения (s-contacts-4); у прочих полей пусто
	Placeholder Text `json:"placeholder,omitempty"`
	// У отметки — Name поля, которое она выключает, пока стоит: «The server
	// asks no password» и Password (s-profile-14). ReadOnly того поля приходит
	// по текущему значению отметки, дальше клиент переключает его сам; у
	// прочих полей пусто
	Disables string `json:"disables,omitempty"`
	// У InputAdd — строки, которые добавляет одно нажатие, пустые; «{n}» в
	// их Name клиент заменяет наименьшим числом, под которым строки этого
	// поля в форме ещё нет (s-contacts-3 «Add phone», s-agenda-3 «Add
	// reminder»); у прочих пусто
	Template []FormRow `json:"template,omitempty"`
	// Поле, в котором стоит курсор, когда форма показана; одно на форму:
	// вернувшееся с причиной (s-auth-13), иначе то, где его рисует кадр —
	// Login у входа (s-auth-1), New password (account-3), Server URL
	// (s-profile-2)… — полный список в C-6; у прочих false
	Focus bool `json:"focus,omitempty"`
	// Моноширинное поле, как его рисует кадр (is-mono): то, что набирают и
	// копируют знак в знак, — логин у первого запуска (s-auth-4), адрес
	// сервера, номер телефона, дата и время, UID, временный пароль; у прочих
	// false
	Mono bool `json:"mono,omitempty"`
}
type Option struct {
	Value     string `json:"value"`
	Label     Text   `json:"label"`
	Selected  bool   `json:"selected"`
	Separator bool   `json:"separator,omitempty"` // черта перед вариантом, как у MenuItem
	Disabled  bool   `json:"disabled,omitempty"`  // виден, но не выбирается
	Reason    Text   `json:"reason,omitempty"`    // почему не выбирается; пусто у доступного
	Colour    Colour `json:"colour,omitempty"`    // метка варианта-коллекции (s-notes-15); у прочих пусто
	Group     Text   `json:"group,omitempty"`     // рубрика перед вариантом: «As sent» (s-agenda-8); у прочих пусто
	Meta      Text   `json:"meta,omitempty"`      // подпись справа: род и день записи, «event · 12 August» (s-tasks-10); у прочих пусто
	Typed     bool   `json:"typed,omitempty"`     // «By UID…» (s-tasks-10): выбор открывает строку, где значение набирают руками под тем же Name; Value пуст; образец строки — Placeholder поля
	Hint      Text   `json:"hint,omitempty"`      // у Typed — строка под строкой набора: «By UID: a record that is not among the loaded ones.» (s-tasks-11); у прочих пусто
}
type FieldError struct {
	Name    string `json:"name"`
	Message Text   `json:"message"`
}

type AccountPage struct {
	Shell
	Subtitle     Text         `json:"subtitle"`     // под заголовком: «mix · created 11 February 2026» (account-1)
	Profile      Form         `json:"profile"`      // рубрики Name и Email: Display name, Address с Change
	Email        EmailStatus  `json:"email"`        // под полем Address: значок или сообщение с кнопкой
	PasswordNote Text         `json:"passwordNote"` // строка у Change password: что перевязывается
	Escrow       EscrowStatus `json:"escrow"`
	Sessions     []SessionRow `json:"sessions"`
	Actions      []Action     `json:"actions"`
	Danger       []Action     `json:"danger"`
	DangerNote   Text         `json:"dangerNote"` // строка у Delete account: что удаляется и что остаётся
}

// EmailStatus — состояние адреса под полем Address (account-8, s-mail-9,
// account-9, account-1). Badge — у подтверждённого и у ожидающей смены,
// Message — у неподтверждённого: последствие и одна кнопка в Actions.
type EmailStatus struct {
	State       EmailState `json:"state"`       // C-8
	Badge       *Badge     `json:"badge"`       // «confirmed 14 February», «pending: new@example.org»; nil — значка нет
	ConfirmedAt When       `json:"confirmedAt"` // день подтверждения; пусто — подтверждён до v0.2.60, дня нет
	Pending     Text       `json:"pending"`     // новый адрес, ждущий ссылки
	SentTo      Text       `json:"sentTo"`      // куда ушло письмо со ссылкой, пока ссылка жива
	SentAt      When       `json:"sentAt"`      // когда оно ушло: «11:42»
	Message     *Notice    `json:"message"`     // последствие неподтверждённого адреса; nil — его нет
}
type SessionRow struct {
	Agent    Text   `json:"agent"`
	Address  Text   `json:"address"`
	Started  When   `json:"started"`
	LastSeen When   `json:"lastSeen"`
	Current  bool   `json:"current"`
	End      Action `json:"end"`
}
type EscrowStatus struct {
	Active       bool     `json:"active"`
	Covered      bool     `json:"covered"`
	By           Text     `json:"by"`
	Badge        *Badge   `json:"badge"`        // у рубрики: «deposited 2026-03-02» (account-7); nil — копии нет
	Deposited    When     `json:"deposited"`    // «2026-03-02»; пусто — копии нет
	LastRecovery When     `json:"lastRecovery"` // «never» или день последнего восстановления
	Note         Text     `json:"note"`         // честная формулировка последствия, не «резервная копия ключей»
	ActionNote   Text     `json:"actionNote"`   // строка у кнопки отзыва: чем он кончится (account-4)
	Actions      []Action `json:"actions"`
}

type EscrowPage struct {
	Shell
	Subtitle Text         `json:"subtitle"` // «A copy of your key on this server» (account-4), «No copy of your data key is deposited» (account-12)
	Rubric   Text         `json:"rubric"`   // над словами без копии: «What a deposit does» (account-12); с копией и при выключенной схеме пусто
	Status   EscrowStatus `json:"status"`
	Confirm  *UnlockSheet `json:"confirm"` // лист подтверждения перед депонированием
	History  []AuditRow   `json:"history"`
}

// EscrowWithdrawSheet — лист перед отзывом депонированного ключа: копия
// уничтожается, и восстановить учётную запись больше некому.
type EscrowWithdrawSheet struct {
	Title   Text     `json:"title"`   // «Withdraw my deposited key» — то, что нажимают (account-11)
	Warning Text     `json:"warning"` // что кончается и что остаётся
	Form    Form     `json:"form"`    // скрытое action=withdraw и CSRF
	Actions []Action `json:"actions"` // Withdraw my deposited key · Cancel
}

// PasswordPage — смена пароля (account-3), обычная и вынужденная.
type PasswordPage struct {
	Shell
	Form    Form    `json:"form"`
	Warning Notice  `json:"warning"` // вынужденная: вход по временному паролю
	Note    Text    `json:"note"`    // строка под полями: ключ тот же, меняется обёртка
	Back    *Action `json:"back"`    // «Back to settings»; nil при вынужденной — идти некуда
}

type UnlockSheet struct {
	Purpose Text  `json:"purpose"` // ради чего спрашивают пароль
	Warning Text  `json:"warning"`
	Form    Form  `json:"form"`
	Until   *When `json:"until"` // до какого времени будет открыто
}

type ConnectionsPage struct {
	Shell
	// Subtitle — строка под заголовком: «Signed in as mix · 3 servers · 12
	// collections · 2 subscriptions» (s-profile-1); та же и под диалогом;
	// логин — жирным (s-profile-10)
	Subtitle      Runs               `json:"subtitle"`
	Accounts      []ConnectionCard   `json:"accounts"`
	Subscriptions []SubscriptionCard `json:"subscriptions"`
	// Add — диалог подключения, его правка или подписка; nil — на списке:
	// диалог открывает Connect account в Actions (s-profile-1, s-profile-2)
	Add *Form `json:"add"`
	// Шапка диалога поверх страницы — страница под ним как есть: «Connect
	// account» (s-profile-2), «Edit connection» (s-profile-16) и строка «One
	// connection gives calendars and address books alike.»; пусто — диалога
	// нет или это подписка
	DialogTitle    Text     `json:"dialogTitle"`
	DialogSubtitle Text     `json:"dialogSubtitle"`
	Actions        []Action `json:"actions"` // Connect account · Subscribe to a feed
	// Note — сообщение страницы под карточками: с открытой панелью подписки
	// — что ленту читают на каждом визите и что делает **Check**
	// (s-profile-10); без панели пусто
	Note Runs `json:"note"`
}
type ConnectionCard struct {
	AccountID string       `json:"accountID"`
	Title     Text         `json:"title"`
	Server    Text         `json:"server"`
	Login     Text         `json:"login"`
	Password  *SecretField `json:"password"` // Show password; nil у сервера без пароля (s-profile-15)
	Access    Text         `json:"access"`   // «no password» в строке адреса (s-profile-15); у прочих пусто
	// Plain — «plain WebDAV, no principal» в строке адреса у хранилища, где
	// сервер не назвал принципала (s-profile-1); у прочих пусто
	Plain Text `json:"plain"`
	// Answered — когда сервер ответил сеансу последним: «5 min ago» после
	// слов экрана «last answer» (s-profile-1); пусто — сеанс ещё не читал ни
	// одной его коллекции
	Answered    When             `json:"answered"`
	State       CollectionState  `json:"state"`
	Collections []CollectionRow  `json:"collections"`
	Trace       *DiagnosticTrace `json:"trace"`   // след последней проверки
	Actions     []Action         `json:"actions"` // Test · Test again · Edit · New collection · Disconnect
}
type SecretField struct {
	Shown  bool   `json:"shown"`
	Value  Text   `json:"value"` // заполняется только после подтверждения пароля
	Reveal Action `json:"reveal"`
}
type CollectionRow struct {
	Label    Text       `json:"label"`
	Path     Text       `json:"path"`
	Colour   Colour     `json:"colour"`
	Kinds    []Kind     `json:"kinds"`
	ReadOnly bool       `json:"readOnly"`
	LastRead When       `json:"lastRead"`
	Changed  When       `json:"changed"`
	Objects  *int       `json:"objects"`
	Menu     []MenuItem `json:"menu"` // Rename · Delete · Move
}
type SubscriptionCard struct {
	Title   Text   `json:"title"`
	URL     Text   `json:"url"`
	Colour  Colour `json:"colour"`
	Kinds   []Kind `json:"kinds"`
	Stale   bool   `json:"stale"`
	Checked When   `json:"checked"`
	// Size и Took — сколько весил и сколько читался последний ответ ленты в
	// этом сеансе: «34 KB», «0.4 s» (s-profile-1, -10); не читалась — пусто
	Size    Text     `json:"size"`
	Took    Text     `json:"took"`
	Actions []Action `json:"actions"` // Check · Edit · Unsubscribe
}

// AttachmentsPage — Settings → Attachments (attachments-1): коллекция
// списком, папка в ней с Browse (дерево — FilePickerSheet ?for=attachments),
// подпапка по месяцам, имя файла; всё — поля Form
type AttachmentsPage struct {
	Shell
	Form Form `json:"form"`
	Note Text `json:"note"`
}

type DevicesPage struct {
	Shell
	Tab      DeviceTab       `json:"tab"` // C-8
	Accounts []DeviceAccount `json:"accounts"`
	Warning  Notice          `json:"warning"` // что содержит профиль и куда он попадает
	Actions  []Action        `json:"actions"`
	Note     Text            `json:"note"` // строка над Generate profile: как его выдадут; пусто вне Apple
}
type DeviceAccount struct {
	Title    Text        `json:"title"`
	Rows     []DeviceRow `json:"rows"`
	QR       string      `json:"qr"`       // Android: код этого аккаунта, адрес изображения; пусто — кода нет
	Login    Text        `json:"login"`    // Thunderbird, Anything else: логин в строке аккаунта
	Password SecretField `json:"password"` // пароль этого подключения; пусто у Apple и у аккаунта без кода
	Actions  []Action    `json:"actions"`  // Open in DAVx5 · Copy / Copy password
}
type DeviceRow struct {
	Label    Text   `json:"label"`
	Kind     Kind   `json:"kind"`
	Type     Text   `json:"type"` // CalDAV · CardDAV · WebDAV
	URL      Text   `json:"url"`
	Included bool   `json:"included"`
	Copy     Action `json:"copy"`
}

type DavloomPage struct {
	Shell
	// Subtitle — строка под заголовком (s-davloom-1): «One address for every
	// device instead of an account for every server», у локального адреса —
	// и « · ‹danger›a phone on mobile data will not reach a local address»
	Subtitle Runs          `json:"subtitle"`
	Enabled  bool          `json:"enabled"`
	Address  Text          `json:"address"`
	Devices  []ProxyDevice `json:"devices"`
	Add      Form          `json:"add"`
	Note     Runs          `json:"note"` // смена основного пароля не отключает устройства; «not» — жирным (s-davloom-1)
}
type ProxyDevice struct {
	Name     Text     `json:"name"`
	Created  When     `json:"created"`
	LastSeen When     `json:"lastSeen"`
	Scope    []Kind   `json:"scope"`
	ReadOnly bool     `json:"readOnly"` // право устройства: только чтение или чтение и запись
	Actions  []Action `json:"actions"`  // Edit scope · read only | read and write (On — действующее) · Revoke
}

type BackupPage struct {
	Shell
	// Subtitle — на странице задания: «Job · last copy 2 days ago»
	// (s-backup-2), назначение не только WebDAV — жирным: «Job · destination
	// **WebDAV and download** · last copy …» (s-backup-3); на списке — счёт
	// заданий, «Three jobs · their own sources, destinations and triggers»
	// (s-backup-1), без заданий — «No jobs yet» (s-backup-6); у нового
	// задания пусто
	Subtitle Runs `json:"subtitle"`
	// Actions — шапка списка (s-backup-1, s-backup-6): New job (основная,
	// ?job=new — страница нового задания) и Restore (?restore=1 — открытое
	// восстановление, RestoreBlock.Open); и без заданий тоже
	Actions []Action `json:"actions"`
	// Empty и EmptyNote — пустое состояние без заданий (s-backup-6): что
	// делает копия, ‹.ics› и ‹.vcf› моноширинно; и что для восстановления
	// задание не нужно, **Restore** жирным. С заданиями пусто
	Empty      Runs         `json:"empty"`
	EmptyNote  Runs         `json:"emptyNote"`
	Jobs       []BackupJob  `json:"jobs"`
	Add        Form         `json:"add"`        // новое задание; на странице задания (s-backup-2) — оно само
	Delete     *Action      `json:"delete"`     // внизу страницы задания; nil — на списке и у нового
	DeleteNote Text         `json:"deleteNote"` // строка у удаления: копии остаются, удаляется расписание
	Restore    RestoreBlock `json:"restore"`
}
type BackupJob struct {
	Name        Text    `json:"name"`
	Sources     []Stamp `json:"sources"`
	Destination Text    `json:"destination"`
	Keep        int     `json:"keep"`
	Encrypted   bool    `json:"encrypted"`
	LastRun     When    `json:"lastRun"`
	LastState   OpState `json:"lastState"`
	// Clock — отметка «On a clock of your own — cron, GoSentry, a job in
	// Gitea» в рубрике When (s-backup-2): On — внешний запуск включён;
	// снятая выключает его, отдельного выключателя нет
	Clock Action `json:"clock"`
	// ClockHint — подсказка под отметкой Clock, и у снятой тоже: цена
	// включения — ключ задания хранится под ключом сервера (s-backup-2,
	// S-24.4)
	ClockHint Text `json:"clockHint"`
	// Trigger — карточка строки внешнего запуска; nil, пока он выключен:
	// показывать и обновлять нечего (решение владельца, записка 19)
	Trigger *TriggerBlock `json:"trigger"`
	Runs    []BackupRun   `json:"runs"`
	Actions []Action      `json:"actions"` // Run · Edit (s-backup-1)
}
type TriggerBlock struct {
	Command  Text        `json:"command"` // готовая строка или скрипт, собранный под назначение
	Password SecretField `json:"password"`
	Note     Text        `json:"note"`
}
type BackupRun struct {
	ID       Text    `json:"id"`
	Started  When    `json:"started"`
	State    OpState `json:"state"`
	Size     Size    `json:"size"`
	Download *Action `json:"download"`
}
type RestoreBlock struct {
	// Open — восстановление открыто: Restore в шапке (?restore=1) или
	// архив уже открыт (шаг сопоставления)
	Open bool `json:"open"`
	// Subtitle — строка под заголовком открытого архива (s-backup-8):
	// задание, что в манифесте, и где архив лежит — моноширинно; «Job
	// «Личное» · manifest read · 4 collections · 1 214 records ·
	// ‹carrel-backup/2026-08-14T09-12.zip›»; архив не открыт — пусто
	Subtitle Runs           `json:"subtitle"`
	Source   Form           `json:"source"`
	Preview  []ImportTable  `json:"preview"`
	Targets  []TargetPicker `json:"targets"`
	Note     Runs           `json:"note"` // восстановление создаёт, а не откатывает; «creates» — жирным (s-backup-8)
	Actions  []Action       `json:"actions"`
}

type AppearancePage struct {
	Shell
	Browser  Form   `json:"browser"`  // тема, плотность, колонки — живут в этом браузере
	Account  Form   `json:"account"`  // первый день недели, 12/24 — на сервере
	Instance Form   `json:"instance"` // часовой пояс — ставит администратор
	Notes    []Text `json:"notes"`
}

type AdminUsersPage struct {
	Shell
	Bar  ListBar        `json:"bar"` // полоса s-admin-2: пока только Columns (s-admin-21)
	Rows []AdminUserRow `json:"rows"`
	Add  []Action       `json:"add"` // шапка (s-admin-2): Add user — на «Add users», Reset a password — лист PasswordResetSheet
}
type AdminUserRow struct {
	Login     Text       `json:"login"`
	Role      Text       `json:"role"` // «User», «Administrator» — слова списка s-admin-8
	Created   When       `json:"created"`
	LastLogin When       `json:"lastLogin"`
	Accounts  int        `json:"accounts"` // число, не сами подключения
	Sessions  int        `json:"sessions"`
	Disabled  bool       `json:"disabled"`
	Escrowed  bool       `json:"escrowed"`
	Address   Text       `json:"address"` // подтверждённый адрес; пусто — его нет (s-admin-22)
	Menu      []MenuItem `json:"menu"`    // только «⋯» (s-admin-12)
}

// PasswordResetSheet — «Reset a password» из шапки списка людей (s-admin-13):
// кого сбрасывают, что при этом уничтожается, временный пароль и путь без
// потерь через Key escrow — подсказкой у пароля.
type PasswordResetSheet struct {
	Title   Text            `json:"title"`   // «Reset a password»
	Form    Form            `json:"form"`    // target — Whose («anna · User»; сам администратор и отключённые — выключены с причиной), password — Temporary password, confirm — «Type the login to confirm», последней строкой (s-admin-13, s-admin-20); в ответе на сброс пуста
	Warning Text            `json:"warning"` // что уничтожается: сохранённые пароли DAV-серверов и всё, что ими открывалось; записи на серверах остаются; в ответе на сброс пусто
	Issued  *IssuedPassword `json:"issued"`  // временный пароль — один раз, в ответе на сброс (s-admin-14); nil — нет
	Actions []Action        `json:"actions"` // Reset password (danger) · Cancel; в ответе на сброс пусто — подвал листа держит Issued.Done
}

// AdminInvitesPage — «Add users» (s-admin-1): три способа завести человека
// карточками в ряд, равноправные, и действующие приглашения под ними.
type AdminInvitesPage struct {
	Shell
	Subtitle  Text        `json:"subtitle"`  // «Three ways, for three situations. There is no global mode: any of them works.»
	Invite    AddUserCard `json:"invite"`    // Invitation: Role; Create link · Or send to, Send invitation
	Temporary AddUserCard `json:"temporary"` // Temporary password: Role, Login, Email (optional), Temporary password; Create user
	SelfReg   AddUserCard `json:"selfReg"`   // Self-registration: Publish the registration form; Save
	Rows      []InviteRow `json:"rows"`
}

// AddUserCard — одна карточка «Add users» (s-admin-1).
type AddUserCard struct {
	Title   Text            `json:"title"`   // «Invitation», «Temporary password», «Self-registration»
	Aside   Text            `json:"aside"`   // обычным шрифтом за заголовком: «chooses their own password»; пусто — нет
	Badge   *Badge          `json:"badge"`   // «hand to hand» (alert), «open»; nil — значка нет
	Lede    Text            `json:"lede"`    // строка под заголовком; пусто — нет
	Warning *Notice         `json:"warning"` // «Until the user changes this password, you can sign in as them.»; nil — нет
	Form    Form            `json:"form"`
	Link    Text            `json:"link"`   // ссылка строкой с Copy: только что созданное приглашение, адрес открытой регистрации; пусто — строки нет
	Copy    *Action         `json:"copy"`   // Copy у Link; nil, когда Link пуст
	Issued  *IssuedPassword `json:"issued"` // временный пароль — один раз, в ответе на Create user; nil — нет
}

// IssuedPassword — пароль, показанный администратору один раз: ни в
// состоянии, ни в журнале его нет. Карточка «Temporary password · visible
// only now» (s-admin-14, s-admin-17): сообщение, логин и пароль строкой,
// Copy и строка рядом, Done.
type IssuedPassword struct {
	Login    Text   `json:"login"`
	Password Text   `json:"password"`
	Copy     Action `json:"copy"`
	Message  Runs   `json:"message"` // «The password of anna is reset.» (s-admin-14), «The user viktor is created. Hand this over; …» (s-admin-17); Login в нём — жирным
	Note     Text   `json:"note"`    // строка рядом с Copy: «Hand it over yourself; they choose their own at the next sign-in.»; пусто — нет
	Done     Action `json:"done"`    // Done: у листа сброса — в подвале листа, у карточки Create user — рядом с Copy
}

// InviteRow — ожидающее приглашение (s-admin-1). Логина у него нет: его
// выбирает приглашённый.
type InviteRow struct {
	Address  Text       `json:"address"`  // первая колонка: адрес письма; «—» у ссылки
	Link     Text       `json:"link"`     // видна всегда, независимо от отправки
	Delivery Text       `json:"delivery"` // «link» или «email»: как приглашение передано
	Status   Text       `json:"status"`   // «pending»: в списке только действующие
	Sent     Text       `json:"sent"`     // «sent», «failed — 550 mailbox unavailable» (ответ реле), «SMTP not configured»; «—» у ссылки
	Expires  When       `json:"expires"`
	Actions  []Action   `json:"actions"` // Resend (у приглашения письмом) · Copy link
	Menu     []MenuItem `json:"menu"`    // «⋯»: Extend, черта, Revoke
}

type AdminSettingsPage struct {
	Shell
	Instance Form            `json:"instance"`
	SMTP     Form            `json:"smtp"`
	Mail     Form            `json:"mail"` // форма адреса отправителя, отправка письмом
	Test     *MailTestResult `json:"test"`
	Limits   Form            `json:"limits"`
}

// MailTestResult — полный ответ сервера, без пароля реле.
type MailTestResult struct {
	OK    bool `json:"ok"`
	Reply Text `json:"reply"`
}

type EscrowRecoveryResult struct {
	Login    Text   `json:"login"`
	Password Text   `json:"password"` // shown once; not stored in state
	Copy     Action `json:"copy"`
	Mailed   Text   `json:"mailed"` // «отправлено», «SMTP не настроен»
}

type AdminEscrowPage struct {
	Shell
	Subtitle Text                  `json:"subtitle"` // «4 of 6 accounts open with the master password»; пусто, пока схема не включена
	Enabled  bool                  `json:"enabled"`
	Warning  Notice                `json:"warning"`
	Form     Form                  `json:"form"`   // Policy: New accounts are covered; до включения — мастер-пароль и Turn on key escrow
	Master   Form                  `json:"master"` // Current, New, Repeat; Change
	Opens    int                   `json:"opens"`  // число у «Whom the master password opens»: депонировавшие
	Covered  []EscrowCoverRow      `json:"covered"`
	Recover  Form                  `json:"recover"` // Account, Master password, Temporary password for the user; Recover
	Result   *EscrowRecoveryResult `json:"result"`
	History  []AuditRow            `json:"history"`
}

// EscrowCoverRow — строка «Whom the master password opens» (s-admin-6):
// каждая учётная запись списка пользователей, депонировала она или нет.
type EscrowCoverRow struct {
	Login        Text `json:"login"`
	Covered      bool `json:"covered"`      // копия депонирована
	Deposited    When `json:"deposited"`    // «2026-03-02» или «not deposited»
	LastRecovery When `json:"lastRecovery"` // «never», «2026-08-14 18:40» или «—» у недепонировавшей
}

type AdminAuditPage struct {
	Shell
	Bar    ListBar    `json:"bar"`
	Filter Form       `json:"filter"` // выбор действия (s-admin-10): GET, уходит при выборе
	Rows   []AuditRow `json:"rows"`
}
type AuditRow struct {
	When   When `json:"when"`
	Who    Text `json:"who"`
	What   Text `json:"what"`
	Whom   Text `json:"whom"`
	Detail Text `json:"detail"`
}

type AdminDavPage struct {
	Shell
	Form    Form             `json:"form"`
	Trace   *DiagnosticTrace `json:"trace"`
	Full    *ExerciseReport  `json:"full"` // второй, изменяющий этап — только по согласию
	Actions []Action         `json:"actions"`
}
type ExerciseReport struct {
	Rows []ExerciseRow `json:"rows"`
	OK   bool          `json:"ok"`
}
type ExerciseRow struct {
	Operation Text `json:"operation"`
	Result    Text `json:"result"`
	Detail    Text `json:"detail"`
}

type AdminInstallPage struct {
	Shell
	PublicURL Text         `json:"publicURL"`
	Rows      []InstallRow `json:"rows"`
	Actions   []Action     `json:"actions"`
}
type InstallRow struct {
	Check  Text       `json:"check"`
	State  CheckState `json:"state"` // C-8; серый — то, что снаружи не измерить
	Value  Text       `json:"value"`
	Advice Text       `json:"advice"`
}
