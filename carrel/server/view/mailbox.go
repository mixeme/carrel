package view

// MailboxPage — диалог своего почтового ящика (carrel-mailbox.md M-3): его
// подключение (s-mailbox-22, -23), итог подключения, когда чтение работает,
// а отправка нет (-25), и правка (-27). Диалог стоит в главной зоне один,
// без страницы под ним.
type MailboxPage struct {
	Shell
	// Shell.Title — «Connect a mailbox» (-22, -25); у правки — подпись
	// ящика, «home» (-27)
	//
	// Subtitle — «Read it and write from it over IMAP and SMTP. Letters stay
	// on your mail server.» (-22); у итога — «office ·
	// m.yenuchenko@office.example» (-25); у правки — «‹mix@home.example› ·
	// mailbox», адрес моноширинно (-27)
	Subtitle Runs `json:"subtitle"`
	// Form — поля: Email address, Label, Colour, App password или Password,
	// узлы и порты руками (?servers=manual); у правки — Label, Colour, Name
	// in From, Signature, автосохранение черновика, Password. nil — итог
	// подключения: формы нет
	Form *Form `json:"form"`
	// Provider — что узнанный по домену провайдер требует, до поля пароля:
	// «Yandex lets mail programs in with an **app password**…» (-22); пусто
	// — провайдер не узнан. Closed — вход по паролю закрыт: Provider —
	// «**This mailbox cannot be connected.** …» тревогой после адреса,
	// пароля нет, Connect выключен (-23)
	Provider Runs `json:"provider"`
	Closed   bool `json:"closed"`
	// Servers — рубрика Servers: найденные или хранимые серверы и «Enter
	// servers by hand»; nil — их ещё не искали или они набираются руками
	Servers *MailServers `json:"servers"`
	// Escrow — «**Key escrow is on:** the administrator can open this
	// password too — and through a mailbox, reset your other passwords.»
	// тревогой в конце формы, пока депонирование действует (S-29.2); пусто —
	// не действует
	Escrow Runs `json:"escrow"`
	// Saved — итог подключения: «**Saved. Reading works, sending does
	// not:** the server refused the password for sending.» (-25); пусто — диалог
	Saved Runs `json:"saved"`
	// Trace — «What was tried»: след подключения (-25) или Test only; nil —
	// проверки не было
	Trace *DiagnosticTrace `json:"trace"`
	// TraceNote — строка под следом: что делать (-25); пусто — нечего
	TraceNote Text `json:"traceNote"`
	// Actions — Connect · Test only · Cancel (-22); у правки Save · Test
	// only · Cancel (-27); у итога Done · Enter servers by hand · Change the
	// password (-25)
	Actions []Action `json:"actions"`
}

// MailServers — серверы ящика в рубрике Servers (s-mailbox-22, -27).
type MailServers struct {
	// Badges — «found» у найденных при подключении (-22); у хранимых пусто
	Badges []Badge          `json:"badges"`
	Lines  []MailServerLine `json:"lines"`
	// From — откуда они: «from autoconfig.yandex.ru» (-22), «found by SRV»
	// (-27), «typed by hand»
	From Text `json:"from"`
	// Manual — «Enter servers by hand»: тот же диалог с узлами и портами
	// (?servers=manual)
	Manual Action `json:"manual"`
}

// MailServerLine — сервер одной строкой: «IMAP imap.yandex.ru:993 TLS».
type MailServerLine struct {
	Role     Text `json:"role"`     // IMAP · SMTP
	Address  Text `json:"address"`  // узел:порт
	Security Text `json:"security"` // TLS · STARTTLS
}

// MailboxFoldersPage — вопрос о специальных папках, которых сервер не
// назвал (s-mailbox-24): спрашивается один раз после подключения, как папка
// вложений; потом меняется кнопкой Change в таблице карточки ящика.
type MailboxFoldersPage struct {
	Shell
	// Shell.Title — «Which folders are these?»
	Subtitle Text `json:"subtitle"` // «home did not say where it keeps its archive and spam.»
	// Form — выбор папки на роль (archive, spam…; «None» среди вариантов),
	// под каждым «Suggested by the name.», если вариант предложен по имени;
	// отметка «The server puts a copy of sent letters into Sent itself»
	// (sentCopy); Save · Skip
	Form Form `json:"form"`
}

// MailboxCard — свой ящик на экране подключений, рубрикой Mailboxes между
// DAV-серверами и подписками (s-mailbox-26); с выключенной на экземпляре
// почтой — только Remove (-28).
type MailboxCard struct {
	ID     string `json:"id"` // «home» — {mailbox} его адресов
	Label  Text   `json:"label"`
	Colour Colour `json:"colour"`
	// Address · IMAP · SMTP · Limit — строка адреса: «mix@home.example ·
	// IMAP mail.home.example:993 · SMTP mail.home.example:465 · up to 25
	// MB»; Limit пусто — сервер SIZE не назвал
	Address Text `json:"address"`
	IMAP    Text `json:"imap"`
	SMTP    Text `json:"smtp"`
	Limit   Text `json:"limit"`
	// Answered — когда ящик ответил сеансу последним, после слов экрана
	// «last answer»; пусто — сеанс его ещё не спрашивал
	Answered When `json:"answered"`
	// SendRefused — отправка с ящика не работает: бейдж «sending does not
	// work», карточка тревожная, «reading works» в строке адреса, Test again
	// (-26, office)
	SendRefused bool `json:"sendRefused"`
	// Trace — ответ SMTP при последней проверке у отказавшей отправки; nil у
	// прочих
	Trace *DiagnosticTrace `json:"trace"`
	// Note — следствие отказа для листа письма: «In «From» it stays off
	// until Test passes.»; у прочих пусто
	Note Text `json:"note"`
	// Off — почта ящиков выключена на экземпляре: «not used while mail is
	// off» в строке адреса (-28); пусто — включена
	Off      Text         `json:"off"`
	Password *SecretField `json:"password"` // Show password; nil, пока почта выключена
	// Folders — роли специальных папок: Sent, Drafts, Archive, Spam, Trash;
	// пусто, пока почта выключена
	Folders []MailFolderRow `json:"folders"`
	// KeepsSent — стоит отметка в подвале карточки «The server puts a copy
	// of sent letters into Sent itself»; SentToggle — её нажатие (POST
	// …/{mailbox}/sentcopy), подпись — слова отметки; nil, пока почта
	// выключена
	KeepsSent  bool    `json:"keepsSent"`
	SentToggle *Action `json:"sentToggle"`
	// Actions — Test · Test again · Edit · Disconnect; с выключенной почтой
	// — Remove
	Actions []Action `json:"actions"`
}

// MailFolderRow — роль специальной папки ящика строкой таблицы (-26).
type MailFolderRow struct {
	Role   MailRole `json:"role"`
	Label  Text     `json:"label"`  // «Sent», «Spam»
	Folder Text     `json:"folder"` // папка на сервере, «Архив»; пусто — нет
	How    Text     `json:"how"`    // «announced by the server», «you chose it», «suggested by the name»
	Change Action   `json:"change"`
}

// MailRole — роль специальной папки (RFC 6154).
type MailRole string

const (
	MailSent    MailRole = "sent"
	MailDrafts  MailRole = "drafts"
	MailArchive MailRole = "archive"
	MailJunk    MailRole = "junk"
	MailTrash   MailRole = "trash"
)
