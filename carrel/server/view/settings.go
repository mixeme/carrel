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
	Legend Text      `json:"legend"`
	Rows   []FormRow `json:"rows"`
}
type FormRow struct {
	Name     string    `json:"name"`
	Label    Text      `json:"label"`
	Kind     InputKind `json:"kind"` // C-8
	Value    Text      `json:"value"`
	Options  []Option  `json:"options"`
	Hint     Text      `json:"hint"`
	Error    Text      `json:"error"`
	Extra    *Action   `json:"extra"` // кнопка, принадлежащая полю
	ReadOnly bool      `json:"readOnly"`
	Required bool      `json:"required"`
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
	Subtitle Text         `json:"subtitle"` // «A copy of your key on this server» (account-4)
	Status   EscrowStatus `json:"status"`
	Confirm  *UnlockSheet `json:"confirm"` // лист подтверждения перед депонированием
	History  []AuditRow   `json:"history"`
}

// EscrowWithdrawSheet — лист перед отзывом депонированного ключа: копия
// уничтожается, и восстановить учётную запись больше некому.
type EscrowWithdrawSheet struct {
	Warning Text     `json:"warning"`
	Form    Form     `json:"form"`    // скрытое action=withdraw и CSRF
	Actions []Action `json:"actions"` // Withdraw my deposited key · Cancel
}

type PasswordPage struct {
	Shell
	Form    Form   `json:"form"`
	Warning Notice `json:"warning"`
}

type UnlockSheet struct {
	Purpose Text  `json:"purpose"` // ради чего спрашивают пароль
	Warning Text  `json:"warning"`
	Form    Form  `json:"form"`
	Until   *When `json:"until"` // до какого времени будет открыто
}

type ConnectionsPage struct {
	Shell
	// Subtitle — строка под заголовком: на списке «Signed in as mix · 3
	// servers · 12 collections · 2 subscriptions» (s-profile-1), в диалоге
	// подключения — его строка (s-profile-2)
	Subtitle      Text               `json:"subtitle"`
	Accounts      []ConnectionCard   `json:"accounts"`
	Subscriptions []SubscriptionCard `json:"subscriptions"`
	// Add — диалог подключения, его правка или подписка; nil — на списке:
	// диалог открывает Connect account в Actions (s-profile-1, s-profile-2)
	Add     *Form    `json:"add"`
	Actions []Action `json:"actions"` // Connect account · Subscribe to a feed
}
type ConnectionCard struct {
	AccountID   string           `json:"accountID"`
	Title       Text             `json:"title"`
	Server      Text             `json:"server"`
	Login       Text             `json:"login"`
	Password    SecretField      `json:"password"`
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
	Title   Text     `json:"title"`
	URL     Text     `json:"url"`
	Colour  Colour   `json:"colour"`
	Kinds   []Kind   `json:"kinds"`
	Stale   bool     `json:"stale"`
	Checked When     `json:"checked"`
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
	Enabled bool          `json:"enabled"`
	Address Text          `json:"address"`
	Devices []ProxyDevice `json:"devices"`
	Add     Form          `json:"add"`
	Note    Text          `json:"note"` // смена основного пароля не отключает устройства
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
	Subtitle   Text         `json:"subtitle"` // на странице задания: «Job · last copy 2 days ago»; пусто на списке
	Jobs       []BackupJob  `json:"jobs"`
	Add        Form         `json:"add"`        // новое задание; на странице задания (s-backup-2) — оно само
	Delete     *Action      `json:"delete"`     // внизу страницы задания; nil — на списке и у нового
	DeleteNote Text         `json:"deleteNote"` // строка у удаления: копии остаются, удаляется расписание
	Restore    RestoreBlock `json:"restore"`
}
type BackupJob struct {
	Name        Text         `json:"name"`
	Sources     []Stamp      `json:"sources"`
	Destination Text         `json:"destination"`
	Keep        int          `json:"keep"`
	Encrypted   bool         `json:"encrypted"`
	LastRun     When         `json:"lastRun"`
	LastState   OpState      `json:"lastState"`
	Trigger     TriggerBlock `json:"trigger"`
	Runs        []BackupRun  `json:"runs"`
	Actions     []Action     `json:"actions"` // Run · Edit (s-backup-1)
}
type TriggerBlock struct {
	Command  Text        `json:"command"` // готовая строка или скрипт, собранный под назначение
	Password SecretField `json:"password"`
	Note     Text        `json:"note"`
	Off      *Action     `json:"off"` // выключить внешний запуск; nil — он не включён
}
type BackupRun struct {
	ID       Text    `json:"id"`
	Started  When    `json:"started"`
	State    OpState `json:"state"`
	Size     Size    `json:"size"`
	Download *Action `json:"download"`
}
type RestoreBlock struct {
	Source  Form           `json:"source"`
	Preview []ImportTable  `json:"preview"`
	Targets []TargetPicker `json:"targets"`
	Note    Text           `json:"note"` // восстановление создаёт, а не откатывает
	Actions []Action       `json:"actions"`
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
	Rows    []AdminUserRow `json:"rows"`
	Add     []Action       `json:"add"`
	SelfReg Form           `json:"selfReg"`
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
	Menu      []MenuItem `json:"menu"` // только «⋯» (s-admin-12)
}

// UserRoleSheet — «Set role» строки пользователя: список ролей s-admin-8.
type UserRoleSheet struct {
	Login Text `json:"login"`
	Form  Form `json:"form"` // role: User, Administrator; Save · Cancel
}

type AdminInvitesPage struct {
	Shell
	Rows []InviteRow `json:"rows"`
	Add  Form        `json:"add"`
}
type InviteRow struct {
	Login    Text       `json:"login"`
	Address  Text       `json:"address"`
	Link     Text       `json:"link"`     // видна всегда, независимо от отправки
	Delivery Text       `json:"delivery"` // «link» или «email»: как приглашение передано
	Status   Text       `json:"status"`   // «pending»: в списке только действующие
	Sent     Text       `json:"sent"`     // «sent», «failed — 550 mailbox unavailable» (ответ реле), «SMTP not configured»
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
	Enabled bool                  `json:"enabled"`
	Warning Notice                `json:"warning"`
	Form    Form                  `json:"form"`
	Master  Form                  `json:"master"`
	Covered []AdminUserRow        `json:"covered"`
	Recover Form                  `json:"recover"`
	Result  *EscrowRecoveryResult `json:"result"`
	History []AuditRow            `json:"history"`
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
