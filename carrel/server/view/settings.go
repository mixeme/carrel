package view

// Form — общая форма: поля, ошибки, кнопки.
type Form struct {
	Action string `json:"action"`
	Method string `json:"method"`
	// Target — зона C-7.2, которую меняет ответ-фрагмент на эту форму, как
	// у Action: sheet у формы листа, rail у формы рейла; пусто — переход
	Target string `json:"target,omitempty"`
	// Clear — у формы фильтра со словом: тот же адрес без слова, со скрытыми
	// полями формы — крестик в конце поля (записка Typecase 129 п. 2); пусто
	// — у пустого фильтра и у прочих форм
	Clear   string       `json:"clear,omitempty"`
	Sets    []FieldSet   `json:"sets"`
	Errors  []FieldError `json:"errors"`
	Actions []Action     `json:"actions"`
}
type FieldSet struct {
	Legend Text `json:"legend"`
	// LegendAside — приглушённый хвост подписи набора: «— changed later
	// too, without re-issuing the password» у «What the device will see»
	// (s-davloom-2); пусто — хвоста нет
	LegendAside Text `json:"legendAside,omitempty"`
	// Folded — набор свёрнут: Legend — кнопка, раскрывающая строки («Show 6
	// empty fields», s-contacts-3)
	Folded bool `json:"folded,omitempty"`
	// Unfold — та кнопка без сценариев: GET на ту же форму с ?fields=all,
	// где каждое поле в своей рубрике; nil у несвёрнутого набора
	Unfold *Action   `json:"unfold"`
	Rows   []FormRow `json:"rows"`
	// Facts — строки набора только для чтения после полей, не поля формы:
	// «Time zone · ‹Europe/Moscow› · this instance» у Dates and times
	// (appearance-1, записка Typecase 104); пусто — строк нет
	Facts []Fact `json:"facts,omitempty"`
	// Diag — след одной строкой под строками набора: «PROPFIND
	// /dav/Documents/carrel/attachments/ → 401 Unauthorized» у недоступной
	// папки вложений (attachments-6, записка Typecase 93); пусто — следа нет
	Diag Text `json:"diag,omitempty"`
	// Badges — плашки рубрики после Legend: «not configured» тревогой у
	// Mail, «off» у Letters from users (s-admin-3); пусто — без плашек
	Badges []Badge `json:"badges,omitempty"`
	// Hint — строка под набором: «Without the title an event shows as
	// “Busy”.» у «What the visitor sees» (s-publish-2, -19); у прочих пусто
	Hint Text `json:"hint,omitempty"`
	// HintRuns — Hint кусками, когда кадр выделяет часть: ‹.ics› и ‹.vcf›
	// моноширинно у «What to copy», «Also started by» жирно и строки
	// перечня у «When» задания копии (s-backup-2); Hint тогда пуст
	HintRuns Runs `json:"hintRuns,omitempty"`
}

// Fact — строка только для чтения: имя, значение моноширинно, приглушённый
// хвост после него («this instance»)
type Fact struct {
	Label Text `json:"label"`
	Value Text `json:"value"`
	Aside Text `json:"aside,omitempty"`
}
type FormRow struct {
	Name  string `json:"name"`
	Label Text   `json:"label"`
	// Приглушённая часть подписи, после неё: «(optional)» у «Email»
	// (s-admin-1, s-auth-4), «SHA-256 · optional» у «Certificate
	// fingerprint» (s-profile-2); у прочих пусто
	LabelAside Text `json:"labelAside,omitempty"`
	// Приглушённое после выбранного значения: «as sent» у пояса, пока выбран
	// тот, с которым запись пришла (s-agenda-3); у прочих пусто
	ValueAside Text      `json:"valueAside,omitempty"`
	Kind       InputKind `json:"kind"` // C-8
	Value      Text      `json:"value"`
	Options    []Option  `json:"options"`
	// Multiple — у InputSelect: выбрано может быть несколько вариантов, и
	// каждый выбранный уходит значением под Name (Related to — все связи
	// записи, s-tasks-3); Value тогда — первый из них
	Multiple bool `json:"multiple,omitempty"`
	// Add — у такого поля: строка, которой набирают ещё одно значение, со
	// своим именем («related.add»), подсказкой («Title or UID»), доступным
	// именем («Add a link») и адресом подсказок (Suggest); стоит в поле
	// после выбранных, есть ли они или нет (s-tasks-3, -14). nil — строки нет
	Add  *FormRow `json:"add"`
	Hint Text     `json:"hint"`
	// From — имя строки, из которой значение выводится на лету: адрес новой
	// коллекции — из Name, по правилу architecture.md (транслитерация,
	// дефисы, [a-z0-9-], пустое — col); клиент ставит выведенное образцом
	// поля и в путь подсказки, набранное руками главнее (Кл5); пусто — нет
	From string `json:"from,omitempty"`
	// HintRuns — Hint кусками, когда кадр выделяет часть: имя сервера и
	// адрес моноширинно, «Tasks» жирно (s-profile-5, Кл3); Hint тогда пуст
	HintRuns Runs `json:"hintRuns,omitempty"`
	// LabelRuns — Label кусками, когда кадр выделяет часть: «Allow ‹.ics›
	// download and subscription» (s-publish-2, Пб5); Label тогда — те же
	// слова одной строкой, для подписи без кусков
	LabelRuns Runs `json:"labelRuns,omitempty"`
	Error     Text `json:"error"`
	// Invalid — поле в тревоге без своих слов: причина сказана над полями
	// — пароль, на который сервер ответил отказом (s-profile-24); у прочих
	// false
	Invalid bool `json:"invalid,omitempty"`
	// Reason — строка выключена сейчас и почему: «Let users send letters»
	// без настроенной почты — «Mail is not configured: there is nothing to
	// send through yet.» (s-admin-3, записка Typecase 88); у прочих пусто
	Reason   Text    `json:"reason,omitempty"`
	Extra    *Action `json:"extra"` // кнопка, принадлежащая полю
	ReadOnly bool    `json:"readOnly"`
	Required bool    `json:"required"`
	// Образец в пустом поле, как его рисует кадр: «DD.MM.YYYY» у дня
	// рождения (s-contacts-4); у прочих полей пусто
	Placeholder Text `json:"placeholder,omitempty"`
	// Accept — у InputFile: какие файлы поле берёт, «.zip» у архива
	// восстановления (s-backup-15); у прочих пусто
	Accept string `json:"accept,omitempty"`
	// У отметки — Name полей, которые она выключает, пока стоит, через
	// пробел (имя поля пробела не держит): «The server asks no password» и
	// Password (s-profile-14), All day задачи — «start.time due.time»
	// (s-tasks-3). ReadOnly тех полей приходит по текущему значению отметки;
	// клиент рисует их выключенными и дальше переключает сам: выключенное
	// поле с формой не уходит (П5); у прочих полей пусто
	Disables string `json:"disables,omitempty"`
	// У конца — Name поля той же формы, за которым он идёт: конец события
	// сдвигается на столько же, на сколько сдвинули начало, длина остаётся
	// (решение владельца 11.10.2026, s-agenda-3, s-notes-9); Day — у времени
	// конца Name поля его даты: конец, перешедший полночь, берёт её с
	// собой. У прочих пусто. Клиент отдаёт обе строки библиотеке как есть
	Follows string `json:"follows,omitempty"`
	Day     string `json:"day,omitempty"`
	// Поле, которого нет на экране, пока вариант выбора той же формы, что
	// его называет (Option.Shows), не выбран: «How long», «Unit» и «At»
	// строки напоминания, пока выбран готовый шаг (s-agenda-19). Клиент
	// отдаёт его библиотеке как есть: со сценарием скрытое поле не
	// отправляется, без него сервер не читает его, если вариант не выбран;
	// у прочих false
	Hidden bool `json:"hidden,omitempty"`
	// Changed — Name полей той же формы, правка которых показывает скрытое
	// поле (Typecase Field.Changed): со сценарием оно видно, пока одно из
	// них отличается от пришедшего, без сценария скрыто и не читается;
	// «Published links» правки подключения со ссылками — server, login
	// (s-profile-22). У прочих пусто
	Changed []string `json:"changed,omitempty"`
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
	// Адрес подсказок поля по мере набора (C-5): набранное уходит в ?q=,
	// ответ — SuggestList; его ставит в меню поля живой слой. Related to
	// задачи и заметки (s-tasks-10, s-notes-20); у прочих пусто
	Suggest string `json:"suggest,omitempty"`
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
	// Columns — когда варианты стоят строками таблицы: значения столбцов
	// после Label; у копии задания — день, размер, счёт коллекций
	// (s-backup-7); у прочих пусто
	Columns []Text `json:"columns,omitempty"`
	Typed   bool   `json:"typed,omitempty"` // «By UID…» (s-tasks-10): выбор открывает строку, где значение набирают руками под тем же Name; Value пуст; образец строки — Placeholder поля
	Hint    Text   `json:"hint,omitempty"`  // у Typed — строка под строкой набора: «By UID: a record that is not among the loaded ones.» (s-tasks-11); у прочих пусто
	// Shows — Name полей той же формы через пробел, которые видны, только
	// пока выбран этот вариант (FormRow.Hidden у них, пока не выбран):
	// «Custom…» напоминания — «reminder.0.count reminder.0.unit», у записи
	// на весь день ещё «reminder.0.at» (s-agenda-19); у прочих пусто
	Shows string `json:"shows,omitempty"`
	// Disables — Name полей той же формы через пробел, выключенных, пока
	// выбран этот вариант: «The whole calendar» — «from to» (s-publish-2,
	// записка Typecase 105); поля стоят всегда, без сценария уходят и при
	// нём, и сервер их тогда не читает; у прочих пусто
	Disables string `json:"disables,omitempty"`
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
	Badge       *Badge     `json:"badge"`       // в рубрике Email (записка Typecase 90): «confirmed», «pending», «not confirmed» (тревогой); nil — значка нет
	ConfirmedAt When       `json:"confirmedAt"` // день подтверждения; пусто — подтверждён до v0.2.60, дня нет
	Pending     Text       `json:"pending"`     // новый адрес, ждущий ссылки
	SentTo      Text       `json:"sentTo"`      // куда ушло письмо со ссылкой, пока ссылка жива
	SentAt      When       `json:"sentAt"`      // когда оно ушло: «11:42»
	// Aside — слова у значка (tc-plain): «new@example.org», «new@example.org
	// · link not sent», «link sent 12 February 11:42»; пусто — нет
	Aside Text `json:"aside"`
	// Send — Send confirmation или Send again после Change в строке поля
	// (s-mail-9, account-9, -10); nil — ссылку слать некуда или она ушла
	Send *Action `json:"send"`
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
	Active       bool   `json:"active"`
	Covered      bool   `json:"covered"`
	By           Text   `json:"by"`
	Badge        *Badge `json:"badge"`        // у рубрики: «deposited 2026-03-02» (account-7); nil — копии нет
	Deposited    When   `json:"deposited"`    // «2026-03-02»; пусто — копии нет
	LastRecovery When   `json:"lastRecovery"` // «never» или день последнего восстановления; пусто, когда есть Recoveries
	// Recoveries — строка «Recovered» на каждое восстановление вместо Last
	// recovery (account-16, записка Typecase 99): когда, моноширинно, и кто,
	// что: «olga, administrator: the password was reset»
	Recoveries []EscrowRecovery `json:"recoveries,omitempty"`
	Note       Text             `json:"note"`       // честная формулировка последствия, не «резервная копия ключей»
	ActionNote Text             `json:"actionNote"` // строка у кнопки отзыва: чем он кончится (account-4)
	Actions    []Action         `json:"actions"`
}

// EscrowRecovery — одно восстановление учётной записи по депонированному
// ключу (account-16).
type EscrowRecovery struct {
	At  When `json:"at"`  // «2026-07-11 14:02»
	Who Text `json:"who"` // «olga, administrator: the password was reset»
}

type EscrowPage struct {
	Shell
	Subtitle Text         `json:"subtitle"` // «A copy of your key on this server» (account-4), «No copy of your data key is deposited» (account-12)
	Rubric   Text         `json:"rubric"`   // над словами без копии: «What a deposit does» (account-12); с копией и при выключенной схеме пусто
	Status   EscrowStatus `json:"status"`
	Confirm  *UnlockSheet `json:"confirm"` // лист депонирования — только в ответе с отказом без сценариев (неверный пароль); открытый лист — свой ответ, …/escrow/deposit (account-2); nil — его нет
	// PasswordLink — постоянная ссылка на историю пароля этой учётной записи
	// (решение владельца 16.10.2026, Ад40): выдана при депонировании, видна
	// всегда, не меняется и не гасится; nil — ключ не депонировали
	PasswordLink *PasswordLink `json:"passwordLink"`
}

// PasswordLink — ссылка на историю пароля: адрес моноширинно, Copy и
// слова, зачем её хранить.
type PasswordLink struct {
	Address Text   `json:"address"`
	Copy    Action `json:"copy"`
	Note    Text   `json:"note"`
}

// EscrowWithdrawSheet — лист перед отзывом депонированного ключа: копия
// уничтожается, и восстановить учётную запись больше некому.
type EscrowWithdrawSheet struct {
	Title   Text     `json:"title"`   // «Withdraw my deposited key» — то, что нажимают (account-11)
	Warning Text     `json:"warning"` // что кончается и что остаётся
	Form    Form     `json:"form"`    // скрытое action=withdraw и CSRF
	Actions []Action `json:"actions"` // Withdraw my deposited key · Cancel
	// Under — экран под листом, пришедшим страницей (Лс29): Account (account-11); nil у фрагмента
	Under *Under `json:"under"`
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
	// Purpose — заголовок: ради чего спрашивают пароль — «Show password»
	// (s-profile-3), «Generate profile» (s-plan-3), «Add device»
	// (s-davloom-5), «Delete account» (account-15), «External trigger»
	// (193 п. 3); главная кнопка формы — само действие (записка 86)
	Purpose Text `json:"purpose"`
	// For — ради чего (C-8): вид листа — удаление и депонирование рисуются
	// своими кадрами (account-15, account-2)
	For UnlockFor `json:"for"`
	// Subject — справа приглушённо: чьё — «nextcloud» (s-profile-3, Нс18),
	// имя устройства, задание копии; пусто — ничьё (s-plan-3)
	Subject Text `json:"subject"`
	// Warning — строка под полем: «Unlocks stored credentials for five
	// minutes.»; у удаления пусто — пароль там подтверждение (account-15)
	Warning Text `json:"warning"`
	// Alert — что уходит, тревогой до поля: у удаления учётной записи
	// (account-15); у прочих пусто
	Alert Text `json:"alert"`
	Form  Form `json:"form"`
	// Under — экран под листом, пришедшим страницей (Лс29): откуда открыт (back, иначе Referer) — «Set up a new device» (s-plan-3), Connections (s-profile-3); у депонирования — Account (account-2); nil у фрагмента
	Under *Under `json:"under"`
}

type ConnectionsPage struct {
	Shell
	// Subtitle — строка под заголовком: «Signed in as mix · 3 servers · 12
	// collections · 2 subscriptions» (s-profile-1); та же и под диалогом;
	// логин — жирным (s-profile-10)
	Subtitle      Runs               `json:"subtitle"`
	Accounts      []ConnectionCard   `json:"accounts"`
	Subscriptions []SubscriptionCard `json:"subscriptions"`
	// Empty — ни одного подключения и подписки: «**Nothing is connected
	// yet.**» (s-profile-25); иначе пусто
	Empty Runs `json:"empty"`
	// Add — диалог подключения, его правка или подписка; nil — на списке:
	// диалог открывает Connect account в Actions (s-profile-1, s-profile-2)
	Add *Form `json:"add"`
	// Шапка диалога поверх страницы — страница под ним как есть: «Connect
	// account» (s-profile-2), «Edit connection» (s-profile-16) и строка «One
	// connection gives calendars and address books alike.»; пусто — диалога
	// нет или это подписка
	DialogTitle    Text `json:"dialogTitle"`
	DialogSubtitle Text `json:"dialogSubtitle"`
	// Trace — след «Test only» под диалогом: та же проверка, что у
	// администратора (AdminDavPage.Trace), без записи; диалог — с набранным,
	// пароль не возвращается. nil — проверки не было
	Trace   *DiagnosticTrace `json:"trace"`
	Actions []Action         `json:"actions"` // Connect account · Subscribe to a feed
	// Note — сообщение страницы под карточками: с открытой панелью подписки
	// — что ленту читают на каждом визите и что делает **Check**
	// (s-profile-10); без панели пусто
	Note Runs `json:"note"`
	// AddNote — тревожное сообщение под формой панели подписки: ленту
	// читает сервер Carrel, не браузер (s-profile-10, Нс27); без панели
	// пусто
	AddNote Text `json:"addNote"`
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
	Answered When `json:"answered"`
	// Refused — последний ответ сервера сеансу — отказ: «last answer
	// refused» вместо времени, карточка тревожная (s-profile-1, baikal;
	// Нс23); Answered тогда пуст
	Refused bool `json:"refused"`
	// Note — строка под следом у отказавшего: что значит отказ, «The server
	// refused the password. It may have been changed on the server: the one
	// Carrel saved no longer fits.» (s-profile-1); у прочих пусто
	Note        Text             `json:"note"`
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
	Label    Text   `json:"label"`
	Path     Text   `json:"path"`
	Colour   Colour `json:"colour"`
	Kinds    []Kind `json:"kinds"`
	ReadOnly bool   `json:"readOnly"`
	LastRead When   `json:"lastRead"`
	Changed  When   `json:"changed"`
	Objects  *int   `json:"objects"`
	// Tick — опрашивается ли коллекция: on — во всех её родах, part — в
	// части, off — ни в одном (s-profile-1: снятая — снятым флажком, её не
	// опрашивают; Нс21)
	Tick TickState `json:"tick"`
	// Toggle — флажок строки: отмечает коллекцию во всех её родах, а
	// отмеченную всюду снимает всюду, и страница возвращается; подпись — имя
	// коллекции. Это кнопка общей формы страницы (POST ?op=pick, записка
	// Typecase 103), Pick — её значение под именем pick: «подключение|путь»
	Toggle Action `json:"toggle"`
	Pick   string `json:"pick"`
	// New — коллекция только что найдена Update collection list: бейдж
	// «new» в колонке вида, отметка снята (s-profile-27)
	New  bool       `json:"new,omitempty"`
	Menu []MenuItem `json:"menu"` // Rename · Delete · Move
}
type SubscriptionCard struct {
	ID      string `json:"id"` // «f-…», {account} её адресов
	Title   Text   `json:"title"`
	URL     Text   `json:"url"`
	Colour  Colour `json:"colour"`
	Kinds   []Kind `json:"kinds"`
	Stale   bool   `json:"stale"`
	Checked When   `json:"checked"`
	// Size и Took — сколько весил и сколько читался последний ответ ленты в
	// этом сеансе: «34 KB», «0.4 s» (s-profile-1, -10); не читалась — пусто
	Size Text `json:"size"`
	Took Text `json:"took"`
	// Access — «no password» или «password saved» в строке адреса
	// (s-profile-1; Нс25)
	Access Text `json:"access"`
	// Password — Show password у ленты с сохранённым паролем, тем же окном,
	// что у подключения; nil — пароля нет
	Password *SecretField `json:"password"`
	// Lands — таблица «Lands in»: строка на род ленты (Нс26); Changed —
	// «Changed on the server», одна на все строки («yesterday»); пусто —
	// не читалась
	Lands   []FeedKindRow `json:"lands"`
	Changed When          `json:"changed"`
	// State — unavailable, когда последнее чтение ленты вернулось отказом;
	// Note — строка под карточкой, что значит отказ (s-profile-1:
	// «The feed asks for a password and the saved one no longer fits. …»);
	// Check тогда — «Check again», Checked — когда лента читалась последний
	// раз
	State CollectionState `json:"state"`
	Note  Text            `json:"note"`
	// Trace — след последней Check в сеансе (s-profile-1: у отказавшей —
	// над Note); nil — Check не было
	Trace   *DiagnosticTrace `json:"trace"`
	Actions []Action         `json:"actions"` // Check · Edit · Unsubscribe
}

// FeedKindRow — строка «Lands in» карточки подписки (s-profile-1): род
// ленты словом экрана («Calendar», «Tasks», «Notes»), отмечен ли он в своём
// разделе, сколько записей рода лента держала при последнем чтении (nil —
// не читалась) и флажок строки — кнопка общей формы страницы (POST
// ?op=pick), Pick — её значение: «подписка|раздел» (записка Typecase 103)
type FeedKindRow struct {
	Kind    Kind      `json:"kind"`
	Label   Text      `json:"label"`
	Tick    TickState `json:"tick"`
	Objects *int      `json:"objects"`
	Toggle  Action    `json:"toggle"`
	Pick    string    `json:"pick"`
}

// AttachmentsPage — Settings → Attachments (attachments-1): коллекция
// списком, папка в ней с Browse (дерево — FilePickerSheet ?for=attachments),
// подпапка по месяцам, имя файла; всё — поля Form
type AttachmentsPage struct {
	Shell
	// Subtitle — «Where a file attached to a note, an event or a task is
	// put» (attachments-1)
	Subtitle Text `json:"subtitle"`
	Form     Form `json:"form"`
	Note     Text `json:"note"`
}

type DevicesPage struct {
	Shell
	// Subtitle — «Everything discovery found — as one file or one code»
	// (s-plan-1); за ним — метка открытой выдачи из TopBar
	Subtitle Text            `json:"subtitle"`
	Tab      DeviceTab       `json:"tab"` // C-8
	Accounts []DeviceAccount `json:"accounts"`
	Warning  Notice          `json:"warning"` // что содержит профиль и куда он попадает
	Actions  []Action        `json:"actions"`
	Note     Text            `json:"note"` // строка над Generate profile: как его выдадут; пусто вне Apple
	// UnlockNote — под строками «Anything else», пока выдача открыта
	// (s-plan-5): срок словами, что всё закроется по сроку или по **Lock
	// now**, и журнал; иначе пусто
	UnlockNote Runs `json:"unlockNote"`
	// Link — выданная ссылка на профиль, пока она жива и не открыта
	// (s-plan-1, Нс40): держится и после перезагрузки страницы; только на
	// вкладке Apple; nil — ссылки нет
	Link *DeviceLink `json:"link"`
}

// DeviceLink — карточка выданной ссылки (s-plan-1): «Link ready until
// 09:46», значок «local» у локального экземпляра, подсказка, Download
// profile и Revoke (тревожная)
type DeviceLink struct {
	Title   Text     `json:"title"`
	Local   bool     `json:"local"`
	Hint    Text     `json:"hint"`
	Actions []Action `json:"actions"`
}
type DeviceAccount struct {
	// Title — подпись и логин, «nextcloud · mix» (Apple, Android); Label —
	// подпись одна, «nextcloud», жирным перед логином в строке таблицы
	// (Thunderbird, Anything else; s-plan-4, -5; Нс42)
	Title Text        `json:"title"`
	Label Text        `json:"label"`
	Rows  []DeviceRow `json:"rows"`
	// Included — аккаунт идёт в профиль и в коды (отметка s-plan-1);
	// Toggle — её переключатель «Leave out …» / «Include …», POST; профиль
	// и коды берут аккаунт целиком, потому отметка — у аккаунта (Нс39);
	// на вкладках без отметки (Thunderbird, Anything else) nil
	Included bool    `json:"included"`
	Toggle   *Action `json:"toggle"`
	// Reason — аккаунт никуда не входит и почему: «The server refused the
	// saved password.» (s-plan-1, -2); Refused — коротко справа красным:
	// «password refused», в строке Thunderbird — «the server refused the
	// saved password» (s-plan-4); у прочих пусто
	Reason   Text        `json:"reason"`
	Refused  Text        `json:"refused"`
	QR       string      `json:"qr"`       // Android: код этого аккаунта, адрес изображения; пусто — кода нет
	Login    Text        `json:"login"`    // Thunderbird, Anything else: логин в строке аккаунта
	Password SecretField `json:"password"` // пароль этого подключения; пусто у Apple и у аккаунта без кода
	Actions  []Action    `json:"actions"`  // Open in DAVx5 · Copy / Copy password
}
type DeviceRow struct {
	Label Text `json:"label"`
	// Colour — метка коллекции (s-plan-4, -5; Нс41); пусто — цвета нет
	Colour   Colour `json:"colour"`
	Kind     Kind   `json:"kind"`
	Type     Text   `json:"type"` // CalDAV · CardDAV · WebDAV
	URL      Text   `json:"url"`
	Included bool   `json:"included"`
	ReadOnly bool   `json:"readOnly,omitempty"` // коллекция только для чтения: «ro» у протокола (s-plan-5)
	Copy     Action `json:"copy"`
}

type DavloomPage struct {
	Shell
	// Subtitle — строка под заголовком (s-davloom-1): «One address for every
	// device instead of an account for every server», у локального адреса —
	// и « · ‹danger›a phone on mobile data will not reach a local address»
	Subtitle Runs `json:"subtitle"`
	Enabled  bool `json:"enabled"`
	// Address — адрес для клиента, AddressHint — подсказка под ним: как его
	// вводят в DAVx5 и чей пароль (s-davloom-1)
	Address     Text `json:"address"`
	AddressHint Text `json:"addressHint"`
	// Sees — «What the client sees» рядом с Copy у адреса: GET на лист
	// дерева /app/settings/davloom/tree (s-davloom-1, -9; решение владельца
	// 19.10, записка Typecase 96)
	Sees Action `json:"sees"`
	// Actions — шапка (s-davloom-1): Add device (основная, ?add=1 — диалог
	// добавления, s-davloom-2); у выключенного davloom — выключена с причиной
	// (s-davloom-8)
	Actions []Action      `json:"actions"`
	Devices []ProxyDevice `json:"devices"`
	// Acts — общая форма страницы davloom-acts (POST /app/settings/davloom с
	// csrf), которую шлют кнопки прав устройства, каждая на свой адрес
	// (записка Typecase 102)
	Acts Form `json:"acts"`
	// Empty — под рубрикой Devices без счёта, когда устройств нет: «No
	// devices yet.» жирно (s-davloom-6); у списка пусто
	Empty Runs `json:"empty,omitempty"`
	// Dialog — диалог поверх списка: добавление (?add=1, s-davloom-2),
	// выданный пароль (s-davloom-3, один раз); nil — список
	Dialog *DeviceDialog `json:"dialog"`
	Note   Runs          `json:"note"` // смена основного пароля не отключает устройства; «not» — жирным (s-davloom-1)
}
type ProxyDevice struct {
	Name     Text   `json:"name"`
	Created  When   `json:"created"`
	LastSeen When   `json:"lastSeen"`
	Scope    []Kind `json:"scope"`
	ReadOnly bool   `json:"readOnly"` // право устройства: только чтение или чтение и запись
	// Marks — метки коллекций, которые устройство видит, каждая один раз
	// (s-davloom-1: полоски в строке); у устройства всех родов кадр рисует
	// одну составную — клиент по Scope
	Marks []Colour `json:"marks"`
	// Actions — «5 collections · Change» (GET на лист области
	// /app/settings/davloom/scope?device=, s-davloom-7) · read only | read
	// and write (POST кнопками общей формы страницы, On — действующее;
	// записка Typecase 102) · Revoke
	Actions []Action `json:"actions"`
}

// DavloomScopeSheet — «What it sees» поверх davloom (s-davloom-7, записка
// Typecase 96): коллекции устройства отметками по подключениям, сеткой, как
// источники задания копии; права — в строке устройства, не здесь.
type DavloomScopeSheet struct {
	Subject Text `json:"subject"` // имя устройства, справа в заголовке
	// Form — POST на /app/settings/davloom/scope?device= с csrf; строка col
	// — отметки (InputCheck, Multiple): Option.Group у первой коллекции
	// подключения, Meta — что это («cal.», «book», «tasks»); Actions — Save
	// (основная) и Cancel (GET, davloom)
	Form Form `json:"form"`
	// Under — экран под листом, пришедшим страницей (Лс29): davloom; nil у
	// фрагмента
	Under *Under `json:"under"`
}

// DavloomTreeSheet — «What the client sees» листом (s-davloom-9; решение
// владельца 19.10): адрес справа в заголовке, дерево, Close.
type DavloomTreeSheet struct {
	Address Text `json:"address"`
	// Tree — синтетические уровни davloom — /dav/, principal/, cal/, card/,
	// files/ — и коллекции под ними адресами, как их увидит клиент
	Tree    []ProxyTreeRow `json:"tree"`
	Actions []Action       `json:"actions"` // Close (GET, davloom)
	Under   *Under         `json:"under"`
}

// ProxyTreeRow — строка дерева «What the client sees»: адрес уровнем ниже
// предыдущего (Depth), моноширинно; справа — что это (Label: «root»,
// «Calendars · events, tasks, notes», имя коллекции); у коллекции — её
// метка
type ProxyTreeRow struct {
	Path   Text   `json:"path"`
	Label  Text   `json:"label"`
	Colour Colour `json:"colour,omitempty"`
	Depth  int    `json:"depth"`
}

// DeviceDialog — диалог устройства (s-davloom-2, -3): заголовок и
// подзаголовок; у выданного — карточка пароля (Password); форма — у
// добавления имя, право, «What the device will see» (роды) и «Collections»,
// Create password и Cancel; у правки — роды и коллекции, Save и Cancel; у
// выданного — адрес и логин только для чтения и Done. Warning — тревогой под
// формой добавления: пароль показывают один раз
type DeviceDialog struct {
	Title    Text            `json:"title"`
	Subtitle Text            `json:"subtitle"`
	Password *DevicePassword `json:"password"`
	Form     Form            `json:"form"`
	Warning  Text            `json:"warning"`
}

// DevicePassword — выданный пароль устройства (s-davloom-3): подпись,
// значение, Copy и что его больше не покажут
type DevicePassword struct {
	Label   Text   `json:"label"`
	Value   Text   `json:"value"`
	Warning Text   `json:"warning"`
	Copy    Action `json:"copy"`
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
	Empty     Runs        `json:"empty"`
	EmptyNote Runs        `json:"emptyNote"`
	Jobs      []BackupJob `json:"jobs"`
	// JobsHint — строка под таблицей заданий: что значит verified
	// (s-backup-1); без заданий пусто
	JobsHint Text `json:"jobsHint"`
	// Job — открытое задание на своей странице (s-backup-2, Нс61): его
	// форма; на списке и у нового nil, и тогда Jobs — строки списка
	Job *BackupJob `json:"job"`
	// History — «History, all jobs» (s-backup-1): прогоны всех заданий,
	// новые первыми; на странице задания и у нового пусто
	History    []BackupHistoryRow `json:"history"`
	Add        Form               `json:"add"`        // новое задание; на странице задания (s-backup-2) — оно само
	Delete     *Action            `json:"delete"`     // внизу страницы задания; nil — на списке и у нового
	DeleteNote Text               `json:"deleteNote"` // строка у удаления: копии остаются, удаляется расписание
	Restore    RestoreBlock       `json:"restore"`
}

// BackupJob — строка задания (s-backup-1, Нс55).
type BackupJob struct {
	Name Text `json:"name"`
	// On — «Job is on»: снятое задание идёт только с экрана, строка
	// приглушена; Toggle — отметка строки, POST ?op=switch
	On     bool    `json:"on"`
	Toggle *Action `json:"toggle"`
	// Marks — метки коллекций, что копирует, по одной на коллекцию;
	// Copies — итог: «3 collections», «2 collections + files», «1 book»
	Marks  []Colour `json:"marks"`
	Copies Text     `json:"copies"`
	// Where и WhereNote — «Where to» двумя строками: «sftpgo · vps ·
	// Documents» / «/backup/work»; у скачивания — «download in browser» /
	// «nothing is written»
	Where     Text `json:"where"`
	WhereNote Text `json:"whereNote"`
	// When и WhenNote — только настроенное (решение владельца 19.10,
	// записка Typecase 95): правило входа «on sign-in» / «if older than 7
	// days»; пусто — «—». Расписания внешнего запуска Carrel не знает
	When     Text `json:"when"`
	WhenNote Text `json:"whenNote"`
	// Triggered — выдан пароль внешнего запуска: бейдж «trigger» в When
	Triggered bool `json:"triggered,omitempty"`
	// Keep — сколько копий держать; у скачивания 0 — держать нечего («—»)
	Keep      int  `json:"keep"`
	Encrypted bool `json:"encrypted"`
	// LastRun — последняя копия числами, «2026-08-16 03:10»; пусто — копий
	// не было («never»); LastNote — строкой ниже размер и проверка:
	// «18.4 MB · verified», непроверенная — тревогой
	LastRun   When     `json:"lastRun"`
	LastNote  Runs     `json:"lastNote"`
	LastState OpState  `json:"lastState"`
	Actions   []Action `json:"actions"` // Run · Edit (s-backup-1)
	// External — «External trigger» в «⋯» строки после двойников Run и
	// Edit: ссылка на лист внешнего запуска задания (s-backup-1, записка
	// Typecase 91)
	External Action `json:"external"`
}

// BackupHistoryRow — прогон в «History, all jobs» (s-backup-1, Нс56).
type BackupHistoryRow struct {
	ID   Text `json:"id"`
	At   When `json:"at"` // когда начат: «2026-08-16 03:10»
	Job  Text `json:"job"`
	Size Size `json:"size"`
	// Collections — «3 / 3»: сколько вошло целыми из скольких; Whole —
	// вошли все, иначе ячейка тревожная
	Collections Text `json:"collections"`
	Whole       bool `json:"whole"`
	// Verified — «read back», у неполной — и «‹danger›1 not copied» с
	// подсказкой, что не вошло и почему; не прочитанная обратно — тревогой
	Verified Runs `json:"verified"`
	// Trigger — чем запущен: «external», «on sign-in», «by hand»
	Trigger Text `json:"trigger"`
	// Via — у внешнего запуска второй строкой: пароль, которым пришли, и
	// адрес — «cbk_…lamp · 10.0.0.5»; у отказа — пароль, которым
	// пытались (s-backup-1, записка 91); у прочих пусто
	Via     Text     `json:"via"`
	Actions []Action `json:"actions"` // Restore · Download; нечего — без них
}

// TriggerBlock — карточка внешнего запуска (s-backup-2…5): вкладки crontab ·
// GoSentry · Gitea · curl несут данные — адрес, имя заголовка секрета,
// задание, сам секрет; неизменные слова карточки — экрана (U-1.3). Вкладки
// полны, только пока секрет только что выдан (Shown).
// BackupTriggerSheet — лист внешнего запуска задания поверх списка заданий
// (/app/settings/backup/trigger?job=, записка Typecase 91): выключен — цена
// включения тревогой и Turn on (s-backup-14); включён — пароль строкой и
// строка для вставки с паролем скрытым (s-backup-12); только что выдан —
// пароль и строка целиком, один раз (s-backup-13).
type BackupTriggerSheet struct {
	Title   Text `json:"title"`   // «External trigger»
	Subject Text `json:"subject"` // справа: задание, «Рабочее, ежедневно»
	Local   bool `json:"local"`   // значок «local» у строки: экземпляр локальный
	// Warning — у выключенного: цена включения (S-24.4); у прочих пусто
	Warning  Text             `json:"warning"`
	Password *TriggerPassword `json:"password"` // nil у выключенного
	// Line — «The line to paste» (вкладки ?tab=, команда, Note); у
	// включённого пароль в строке скрыт (cbk_••••); nil у выключенного
	Line *TriggerBlock `json:"line"`
	Form Form          `json:"form"` // скрытое csrf; Actions — Turn on · Cancel либо Close
	// Under — список заданий под листом, пришедшим страницей (Лс29); nil у
	// фрагмента
	Under *Under `json:"under"`
}

// TriggerPassword — пароль внешнего запуска строкой, как сеанс (s-backup-12):
// подсказка, когда выдан, последний запуск и откуда, сколько запусков;
// только что выданный — целиком, один раз (s-backup-13).
type TriggerPassword struct {
	Masked Text `json:"masked"` // «cbk_…lamp»
	// Use — «issued 2 August · last run 16 August 03:10 from ‹10.0.0.5› ·
	// 14 runs»; у только что выданного пусто
	Use     Runs     `json:"use"`
	Value   Text     `json:"value"`   // только что выданный — целиком; иначе пусто
	Actions []Action `json:"actions"` // New password · Revoke (danger); у выданного пусто
}

type TriggerBlock struct {
	Shown bool `json:"shown"` // только что выдан: «visible only now»
	// Script — назначению нужен скрипт (Download, Both): «The script to
	// paste», «Copy the script»; иначе строка
	Script     bool `json:"script"`
	Command    Text `json:"command"`    // crontab: строка запуска или скрипт
	ScriptName Text `json:"scriptName"` // «carrel-personal.sh» — Save as; у строки пусто
	// CronLine — у скрипта строка crontab, которая его запускает: «0 3 * *
	// * /usr/local/bin/carrel-personal.sh»; у строки пусто
	CronLine Text        `json:"cronLine"`
	At       Text        `json:"at"`     // «03:00» — «Runs at 03:00»
	Header   Text        `json:"header"` // имя заголовка секрета: «Authorization»
	GoSentry GoSentryJob `json:"goSentry"`
	// Status — запрос второго задания GoSentry, следящего за итогом: «GET
	// /api/backup/status?job=…»
	Status Text `json:"status"`
	// Gitea — workflow Gitea Actions: расписание и тот же запуск, секрет по
	// имени GiteaSecret; GiteaFile — где он лежит в репозитории
	Gitea       Text `json:"gitea"`
	GiteaFile   Text `json:"giteaFile"`
	GiteaSecret Text `json:"giteaSecret"`
	Curl        Text `json:"curl"` // один запрос, без расписания
	// Endpoints — «What it opens» (s-backup-5): метод и адрес каждого входа
	Endpoints []Text      `json:"endpoints"`
	Password  SecretField `json:"password"`
	// Note — что делает пароль: «Every use is logged, naming this
	// password. **New password** replaces it on the spot — and every line
	// already handed out stops working with it.» (s-backup-5)
	Note Runs `json:"note"`
	// Destination — у скрипта, почему не строка: «Destination: **WebDAV and
	// download** — two steps, so this is a file, not a line» (s-backup-3);
	// у строки пусто
	Destination Runs `json:"destination"`
	// Tabs — вкладки crontab · GoSentry · Gitea · curl ссылками на лист
	// внешнего запуска с ?tab= (вид, не хранится), открытая — On (s-backup-12)
	Tabs []Action `json:"tabs"`
}

// GoSentryJob — поля задания GoSentry (s-backup-4), как они названы в его
// диалоге; скрипту — путь к нему в Command, Arguments пусты.
type GoSentryJob struct {
	Name      Text   `json:"name"`
	Folder    Text   `json:"folder"`
	Schedule  Text   `json:"schedule"`
	Command   Text   `json:"command"`
	Arguments []Text `json:"arguments"` // по одному в строке, без кавычек
}
type RestoreBlock struct {
	// Open — восстановление открыто: Restore в шапке (?restore=1) или
	// архив уже открыт (шаг сопоставления)
	Open bool `json:"open"`
	// Title — заголовок диалога: «Restore» (s-backup-7), у открытого архива
	// — с его днём, «Restore from 2026-08-14» (s-backup-8; Нс64, Нс65)
	Title Text `json:"title"`
	// Subtitle — до архива: «Nothing is written until you have seen what
	// goes where» (s-backup-7); строка под заголовком открытого архива (s-backup-8):
	// задание, что в манифесте, и где архив лежит — моноширинно; «Job
	// «Личное» · manifest read · 4 collections · 1 214 records ·
	// ‹carrel-backup/2026-08-14T09-12.zip›»; архив не открыт — пусто
	Subtitle Runs `json:"subtitle"`
	// From — «Where the archive is»: From a job · From a server · From this
	// computer, ссылки на тот же шаг (?restore=1&from=…), выбранная On;
	// FromHint — подсказка под ними (s-backup-7, Нс74)
	From     []Action `json:"from"`
	FromHint Text     `json:"fromHint"`
	// Source — форма шага: набор выбранного источника — его Legend рубрика
	// карточки (From a job · From a server · From this computer) — и набор
	// «Archive password»; у копий задания — строка на задание (имя, день,
	// размер, счёт коллекций в Columns); у сервера — Collection и «Archive
	// in it» с Browse; у этого компьютера — Archive (поле файла), Storage
	// for a large archive и Folder in it (s-backup-7, -15, -17)
	Source Form `json:"source"`
	// Refusal — отказ шага тревогой наверху карточки источника: «The
	// archive is larger than 16 MB. Choose a folder of your storage for it,
	// or put it there and choose **From a server**.» (s-backup-16); пусто —
	// отказа нет
	Refusal Runs `json:"refusal,omitempty"`
	// Ranges — адрес, который живой слой спрашивает, какие куски архива
	// с этого компьютера слать вместо файла (FileField.Ranges шага 1,
	// s-backup-15; записки Typecase 98, 109): строки «смещение длина» —
	// начало и конец архива; у других источников пусто
	Ranges string `json:"ranges,omitempty"`
	// Match — шаг сопоставления (s-backup-8); nil — архив не открыт
	Match *RestoreMatch `json:"match"`
	// Report — шаг «3 · Report» (s-backup-18,
	// /app/settings/backup/restore/report): что сделало восстановление;
	// nil — не было
	Report  *RestoreReport `json:"report"`
	Note    Runs           `json:"note"`    // «Restoring is an import: records made since the archive stay, nothing is deleted.» (s-backup-8)
	Actions []Action       `json:"actions"` // Cancel открытого архива
}

// RestoreReport — отчёт восстановления (s-backup-18, записка Typecase 99):
// «Restored from 2026-08-14», итог строкой, строка на коллекцию, Done.
type RestoreReport struct {
	Title Text `json:"title"`
	// Subtitle — «3 collections · 141 records created · 5 already here,
	// skipped · 1 not restored» (у ответов Replace и копий — «replaced»,
	// «imported as copies»)
	Subtitle Text               `json:"subtitle"`
	Rows     []RestoreReportRow `json:"rows"`
	Done     Action             `json:"done"` // GET /app/settings/backup
}

// RestoreReportRow — коллекция архива: имя и метка, куда восстановлена,
// сколько создано, что случилось — «4 missing records created · 3 already
// here — skipped», «no differences» (Same — приглушённо), «2 already here —
// skipped · ‹danger›1 not restored: the server refused it (413, too large)».
type RestoreReportRow struct {
	Name     Text   `json:"name"`
	Colour   Colour `json:"colour"`
	Into     Text   `json:"into"`
	Created  int    `json:"created"`
	Happened Runs   `json:"happened"`
	Same     bool   `json:"same,omitempty"`
}

// RestoreMatch — сопоставление (s-backup-8): строка на коллекцию архива,
// отметка — что восстанавливать, «Restore into» — куда. Восстановление —
// импорт (S-24.3): недостающее создаётся, такое же не повторяется, ничего
// не удаляется; что уже есть и отличается — как ответил вопрос Existing.
// Без сценариев — одна форма: строки шлют pick.N и into.N, кнопки полосы —
// та же форма на свой адрес.
type RestoreMatch struct {
	// Form — POST ?op=restore-run, скрытый _csrf; кнопки — «Restore 3
	// collections» (основная) и полосы: Restore into, Skip, Select all,
	// Clear selection (restore.none: отметки сняты, куда — как было)
	Form     Form `json:"form"`
	Selected int  `json:"selected"` // «3 selected»
	// Existing — вопрос над полосой (s-backup-8, -20; владелец, 26.10):
	// «5 records of the archive are already here and differ», у архива,
	// который сравнят лишь при восстановлении, — «Records of the archive
	// that are already here and differ»; ответы — ссылки GET
	// ?existing=skip|replace|copy, выбранный On (без ответа — Skip them),
	// Hint — что он сделает. nil — нечего спрашивать
	Existing *ImportExisting `json:"existing"`
	// Into — список «Restore into» полосы: цель всем отмеченным строкам
	Into FormRow      `json:"into"`
	Rows []RestoreRow `json:"rows"`
	// Ranges, RangesOf — у архива, пришедшего с этого компьютера кусками
	// (s-backup-20, записка Typecase 109): форма по Restore спрашивает
	// Ranges со своими полями — какие куски слать (члены выбранных
	// коллекций) — и шлёт их из файла, который страница держит под именем
	// поля шага 1 RangesOf («file»); у прочих архивов пусто
	Ranges   string `json:"ranges,omitempty"`
	RangesOf string `json:"rangesOf,omitempty"`
}

// RestoreRow — коллекция архива в сопоставлении.
type RestoreRow struct {
	Name   Text   `json:"name"`   // «nextcloud / personal»: подключение и коллекция в архиве
	Colour Colour `json:"colour"` // метка совпавшей коллекции, иначе выведенная из пути
	Kind   Kind   `json:"kind"`
	Count  int    `json:"count"` // записей; у файловой коллекции — файлов
	// Pick — отметка строки (pick.N): восстанавливать
	Pick FormRow `json:"pick"`
	// Into — «Restore into» (into.N): коллекции рода, куда можно писать,
	// черта, New collection, Skip this one; заранее — совпавшая по пути
	Into FormRow `json:"into"`
	// Will — «What will happen»: «matched by path — 4 records missing, will
	// be created», «that server is gone — another collection chosen», «skip»
	Will   Text `json:"will"`
	Danger bool `json:"danger"` // Will тревогой: сервер исчез, архив не читается
	// Here — сколько записей архива цель держит иными; за Will, не тревогой:
	// « · 3 already here» (s-backup-8). 0 — таких нет или не сравнивали
	Here int `json:"here,omitempty"`
}

type AppearancePage struct {
	Shell
	// Subtitle — «Theme in this browser · week start and clock in the
	// account» (appearance-1)
	Subtitle Text `json:"subtitle"`
	// Form — одна форма кадра (appearance-1): рубрики Theme (theme: auto,
	// light, dark — радио сегментом), Density (density: comfortable,
	// compact), Lists (отметки panel, source, wide), Dates and times
	// (weekStart, clock) — первые три живут в куке этого браузера, последняя
	// — в аккаунте; Actions — Discard (GET, эта же страница) и Save, основная
	// (Р-57)
	Form Form `json:"form"`
	// Reset — «Reset remembered view settings» под Lists: POST, забывает
	// выбор столбцов каждого раздела и вид этого браузера
	Reset Action `json:"reset"`
}

type AdminUsersPage struct {
	Shell
	// Subtitle — «6 accounts · 2 administrators · 4 covered by escrow»
	// (s-admin-2, Ад5)
	Subtitle Text `json:"subtitle"`
	// Bar — полоса s-admin-2: «Filter by login» (FilterForm, GET списка),
	// сегменты All · Administrators · Disabled (?show=), Columns (s-admin-21)
	Bar  ListBar        `json:"bar"`
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
	// Under — экран под листом, пришедшим страницей (Лс29): Users (s-admin-13); nil у фрагмента
	Under *Under `json:"under"`
}

// AccountDeleteSheet — удаление учётной записи (s-admin-34, /admin/delete,
// поверх Users): «Delete anna», что Carrel забывает и что остаётся на
// серверах — тревогой до нажатия, поле «Type the login to confirm»
// (login, образец — логин), «Delete account» (danger) и Cancel. Форма —
// POST /admin/ со скрытыми action=delete и target; неверный логин —
// тот же лист с ошибкой у поля.
type AccountDeleteSheet struct {
	Title   Text     `json:"title"`
	Warning Text     `json:"warning"`
	Form    Form     `json:"form"`
	Actions []Action `json:"actions"`
	Under   *Under   `json:"under"` // Users под листом (Лс29); nil у фрагмента
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
	Address  Text `json:"address"`  // первая колонка: адрес письма; «by link» у ссылки (записка 156)
	Link     Text `json:"link"`     // видна всегда, независимо от отправки
	Delivery Text `json:"delivery"` // «link» или «email»: как приглашение передано
	Status   Text `json:"status"`   // «pending»: в списке только действующие
	Sent     Text `json:"sent"`     // «sent», «failed — 550 mailbox unavailable» (ответ реле), «SMTP not configured»; «—» у ссылки
	// SentFailed — письмо не ушло: Sent тревогой (s-admin-1, Ад14)
	SentFailed bool       `json:"sentFailed"`
	Expires    When       `json:"expires"`
	Actions    []Action   `json:"actions"` // Resend (у приглашения письмом) · Copy link
	Menu       []MenuItem `json:"menu"`    // «⋯»: Extend, черта, Revoke
}

type AdminSettingsPage struct {
	Shell
	// Subtitle — «Global settings and mail» (s-admin-3)
	Subtitle Text `json:"subtitle"`
	// Discard — «Discard» шапки: GET настроек, набранное не сохраняется
	Discard Action `json:"discard"`
	// Form — одна форма кадра (Ад17): рубрики Global, Mail, Letters from
	// users, Test; Actions — Save (в шапке, form=) и Send test (Submits:
	// шлёт набранное на свой Href, ничего не сохраняя, Ад37; выключена с
	// причиной, пока релея нет ни сохранённого, ни набранного)
	Form Form `json:"form"`
	// Test — ответ релея на пробу, полностью, без пароля; nil — пробы не было
	Test *MailTestResult `json:"test"`
}

// MailTestResult — полный ответ сервера, без пароля реле.
type MailTestResult struct {
	OK    bool `json:"ok"`
	Reply Text `json:"reply"`
}

type AdminEscrowPage struct {
	Shell
	Subtitle Text             `json:"subtitle"` // «4 of 6 accounts open with the master password»; пусто, пока схема не включена
	Enabled  bool             `json:"enabled"`
	Warning  Notice           `json:"warning"`
	Form     Form             `json:"form"`   // Policy: New accounts are covered; до включения — мастер-пароль и Turn on key escrow
	Master   Form             `json:"master"` // Current, New, Repeat; Change
	Opens    int              `json:"opens"`  // число у «Whom the master password opens»: депонировавшие
	Covered  []EscrowCoverRow `json:"covered"`
	Recover  Form             `json:"recover"` // Account, Master password, Temporary password for the user; Recover
	// Result — ответ на Recover: временный пароль, как его выдаёт сброс
	// (s-admin-14, Ад34); Note — ушло ли письмо человеку
	Result *IssuedPassword `json:"result"`
}

// EscrowCoverRow — строка «Whom the master password opens» (s-admin-6):
// каждая учётная запись списка пользователей, депонировала она или нет.
type EscrowCoverRow struct {
	Login        Text `json:"login"`
	Covered      bool `json:"covered"`      // копия депонирована
	Deposited    When `json:"deposited"`    // «2026-03-02» или «no» (записка 156)
	LastRecovery When `json:"lastRecovery"` // «never», «2026-08-14 18:40» или «—» у недепонировавшей
	// Recovered — LastRecovery — момент; false у «never» и «—»: слово
	// приглушено (s-admin-6, Ад35), как «no» у !Covered
	Recovered bool `json:"recovered"`
}

type AdminAuditPage struct {
	Shell
	Bar    ListBar    `json:"bar"`
	Filter Form       `json:"filter"` // выбор действия (s-admin-10) и день, с которого читать (since, s-admin-7): GET, уходит при выборе; без сценария — Actions: Show
	Rows   []AuditRow `json:"rows"`
}
type AuditRow struct {
	When   When `json:"when"`
	Who    Text `json:"who"`
	What   Text `json:"what"`
	Whom   Text `json:"whom"`
	Detail Text `json:"detail"`
	// Failure — строка о том, что не случилось (login.fail, …): действие
	// тревогой (s-admin-7, Ад25)
	Failure bool `json:"failure"`
}

type AdminDavPage struct {
	Shell
	Form  Form             `json:"form"`
	Trace *DiagnosticTrace `json:"trace"`
	// Agree — отметка согласия второго этапа (s-admin-5): стоит под следом,
	// вне полей формы, и уходит с нею; nil — обнаружение не прошло, второго
	// этапа нет (s-admin-27, -28)
	Agree   *FormRow        `json:"agree"`
	Full    *ExerciseReport `json:"full"` // второй, изменяющий этап — только по согласию
	Actions []Action        `json:"actions"`
}
type ExerciseReport struct {
	Rows []ExerciseRow `json:"rows"`
	OK   bool          `json:"ok"`
}

// ExerciseRow — род второго этапа (s-admin-29): Operation «Create, read
// back, delete · event», Result ok · failed · skipped, Detail — что
// спросили и что ответил сервер: «PUT · GET · DELETE /path → 201 · 200 ·
// 204», отказ — «PUT /path → 403 Forbidden»
type ExerciseRow struct {
	Operation Text `json:"operation"`
	Result    Text `json:"result"`
	Detail    Text `json:"detail"`
}

type AdminInstallPage struct {
	Shell
	// Subtitle — «Carrel asks itself from outside, at the address you
	// declared as the base one» (s-admin-4, -25, -26)
	Subtitle Text `json:"subtitle"`
	// Form — шапка s-admin-4: адрес, по которому проверять (url,
	// моноширинно; до прогона — объявленный адрес, иначе тот, по которому
	// пришли — s-admin-25), и «Run the check» (Ад29)
	Form Form         `json:"form"`
	Rows []InstallRow `json:"rows"`
	// Took — сколько шла проверка, «4.2 s»; пусто — её не было (Ад30)
	Took Text `json:"took"`
	// Note — что делает проверка (s-admin-4, -25, -26): снаружи внутрь,
	// ничего не меняет и не хранит, кроме строки журнала; под таблицей и
	// без неё
	Note Text `json:"note"`
}
type InstallRow struct {
	Check  Text       `json:"check"`
	State  CheckState `json:"state"` // C-8; серый — то, что снаружи не измерить
	Value  Text       `json:"value"`
	Advice Text       `json:"advice"`
}
