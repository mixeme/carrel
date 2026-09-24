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
}
type Option struct {
	Value    string `json:"value"`
	Label    Text   `json:"label"`
	Selected bool   `json:"selected"`
}
type FieldError struct {
	Name    string `json:"name"`
	Message Text   `json:"message"`
}

type AccountPage struct {
	Shell
	Profile  Form         `json:"profile"` // логин, отображаемое имя, адрес и его подтверждение
	Mail     Form         `json:"mail"`    // обратный адрес и что уйдёт в письме
	Escrow   EscrowStatus `json:"escrow"`
	Sessions []SessionRow `json:"sessions"`
	Actions  []Action     `json:"actions"`
	Danger   []Action     `json:"danger"`
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
	Active   bool     `json:"active"`
	Covered  bool     `json:"covered"`
	By       Text     `json:"by"`
	LastUsed When     `json:"lastUsed"`
	Note     Text     `json:"note"` // честная формулировка последствия, не «резервная копия ключей»
	Actions  []Action `json:"actions"`
}

type EscrowPage struct {
	Shell
	Status  EscrowStatus `json:"status"`
	Confirm *UnlockSheet `json:"confirm"` // лист подтверждения перед депонированием
	History []AuditRow   `json:"history"`
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
	Accounts      []ConnectionCard   `json:"accounts"`
	Subscriptions []SubscriptionCard `json:"subscriptions"`
	Add           Form               `json:"add"`
	Actions       []Action           `json:"actions"`
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

type AttachmentsPage struct {
	Shell
	Form   Form     `json:"form"`
	Folder FileTree `json:"folder"`
	Note   Text     `json:"note"`
}

type DevicesPage struct {
	Shell
	Tab      DeviceTab       `json:"tab"` // C-8
	Accounts []DeviceAccount `json:"accounts"`
	Warning  Notice          `json:"warning"` // что содержит профиль и куда он попадает
	Actions  []Action        `json:"actions"`
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
	Jobs    []BackupJob  `json:"jobs"`
	Add     Form         `json:"add"`
	Restore RestoreBlock `json:"restore"`
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
	Actions     []Action     `json:"actions"`
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
	Role      Text       `json:"role"`
	Created   When       `json:"created"`
	LastLogin When       `json:"lastLogin"`
	Accounts  int        `json:"accounts"` // число, не сами подключения
	Sessions  int        `json:"sessions"`
	Disabled  bool       `json:"disabled"`
	Escrowed  bool       `json:"escrowed"`
	Menu      []MenuItem `json:"menu"`
}

type AdminInvitesPage struct {
	Shell
	Rows []InviteRow `json:"rows"`
	Add  Form        `json:"add"`
}
type InviteRow struct {
	Login   Text     `json:"login"`
	Address Text     `json:"address"`
	Link    Text     `json:"link"` // видна всегда, независимо от отправки
	Sent    Text     `json:"sent"` // «отправлено», «ошибка», «SMTP не настроен»
	Expires When     `json:"expires"`
	Actions []Action `json:"actions"`
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
	Bar  ListBar    `json:"bar"`
	Rows []AuditRow `json:"rows"`
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
