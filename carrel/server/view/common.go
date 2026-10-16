package view

// Готовые к показу значения. Собираются сервером по C-10;
// клиент выводит их как есть и никогда не разбирает обратно.
type (
	Text   string // строка интерфейса или данных пользователя
	When   string // дата и время: «10 August, 10:00», «весь день», «2 min ago»
	Size   string // объём: «1.4 MB»
	Colour string // цвет метки коллекции: готовое значение, C-10
	Stamp  string // откуда запись: подпись подключения и имя коллекции, «nextcloud · Личный»
)

// Runs — фраза сервера, в которой кадр выделяет куски: адрес, куда ушло
// письмо, логин учётной записи (s-auth-3, s-auth-6, s-auth-10, s-auth-11).
// Клиент выводит куски подряд, каждый в своём выделении, и не режет и не
// склеивает их сам. Фраза без выделений — один кусок; пустая — ни одного.
type Runs []Run
type Run struct {
	Text Text    `json:"text"`
	Mark RunMark `json:"mark,omitempty"` // C-8; пусто — обычный текст
	// Hint — подсказка куска (title): «verified» подвала рейла — «The archive
	// was read back from the destination and the checksum matched»
	// (account-*, Нс5); пусто — подсказки нет
	Hint Text `json:"hint,omitempty"`
}

// Action — кнопка, пункт меню и строка панели — одно и то же.
type Action struct {
	ID     string `json:"id"` // устойчивое имя действия, C-7
	Label  Text   `json:"label"`
	Icon   string `json:"icon,omitempty"`   // имя из набора U-4; пусто — без значка
	Href   string `json:"href,omitempty"`   // пусто, если действие отправляет форму
	Method string `json:"method"`           // "GET" | "POST"
	Danger bool   `json:"danger,omitempty"` // разрушающее: удаление, сброс, объединение
	// Primary — основное действие, как его рисует кадр (is-primary): кнопка
	// формы, подтверждение листа, «New contact» в шапке списка
	Primary bool `json:"primary,omitempty"`
	Enabled bool `json:"enabled"`
	On      bool `json:"on,omitempty"`     // включённый переключатель: флажок ленты, сегмент «сейчас показано»
	Reason  Text `json:"reason,omitempty"` // почему выключено; пусто, если включено
	// Hint — подсказка у включённого действия (title кнопки): чем оно
	// обернётся — у New password внешнего запуска «The old password stops
	// working; …» (s-backup-2, -3, -10, -11); пусто — подсказки нет
	Hint Text `json:"hint,omitempty"`
	// Tally — счёт сегмента отдельно от подписи: «All» и «24» (SegOption.Count;
	// сложенный сегмент пишет их врозь, s-contacts-7); пусто — кадр счёта не
	// пишет. Не Count: у строк рейла (NavRow, Shortcut) свой Count числом
	Tally Text `json:"tally,omitempty"`
	// Arrow — стрелка кнопки (.m-split): пункты её меню; сама кнопка — то,
	// что делается без стрелки. Пусто — кнопка без стрелки
	Arrow []MenuItem `json:"arrow,omitempty"`
	// CSRF — токен сеанса у POST (C-11): без сценариев клиент шлёт действие
	// своей формой со скрытым _csrf этого значения; у GET и у страниц вне
	// входа пусто
	CSRF string `json:"csrf,omitempty"`
	// File — имя поля файла у POST, который несёт файл: без сценариев
	// клиент шлёт действие формой multipart/form-data с этим полем выбора
	// файла и _csrf — «file» у Attach и «From this computer», «photo» у Add
	// и Replace фотографии; пусто — действие без файла
	File string `json:"file,omitempty"`
	// Submits — кнопка отправляет форму, в которой стоит, на свой Href
	// (formaction); с File — multipart с этим полем файла, набранное едет
	// вместе с файлом: Add, Replace, Crop и Remove фотографии новой
	// карточки (s-contacts-27), Attach новой записи (s-agenda-15); без File
	// — сама форма: Save правки заметки (NoteDoc.Form, s-notes-20), From
	// Files и Remove вложений новой записи; false — действие уходит своей
	// формой
	Submits bool `json:"submits,omitempty"`
	// Target — зона C-7.2, которую меняет ответ-фрагмент на это действие
	// (C-7.1): sheet у адреса, отвечающего листом, rail у формы рейла, poll
	// у полосы хода, queue у загрузки, main у новой папки — по C-5; пусто —
	// переход. У кнопки формы (Href пуст) — пусто: цель у формы
	Target string `json:"target,omitempty"`
}

type MenuItem struct {
	Action
	Separator bool `json:"separator,omitempty"` // черта перед пунктом
	// Пункт «куда» стрелки New в едином виде (s-contacts-9): метка
	// коллекции, приглушённое справа («last used») и «только для чтения» —
	// такой пункт выключен с причиной, клиент ставит у него «ro»
	Colour   Colour `json:"colour,omitempty"`
	Aside    Text   `json:"aside,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

type Badge struct {
	Label Text      `json:"label"`
	Role  BadgeRole `json:"role"`
}

type Notice struct {
	Role    NoticeRole  `json:"role"`
	Title   Text        `json:"title"`
	Body    Text        `json:"body"`
	Details *Diagnostic `json:"details"` // технический блок; nil — его нет, C-9
	// Actions — кнопки уведомления: «New calendar» у календаря, в который
	// сервер взял не всё (s-agenda-11, SV-52)
	Actions []Action `json:"actions"`
}

type Avatar struct {
	Href     string `json:"href"` // адрес миниатюры; пусто — рисуются инициалы
	Initials Text   `json:"initials"`
	Colour   Colour `json:"colour"` // фон заглушки, выведен из UID
}

type Tag struct {
	Label Text   `json:"label"`
	Href  string `json:"href"`
	On    bool   `json:"on"`
}

// SourceBlock — «откуда запись и что с ней на сервере».
type SourceBlock struct {
	UID  Text `json:"uid"`  // идентификатор записи: строка UID источника в панели (s-contacts-1, -7)
	Path Text `json:"path"` // путь записи на её сервере: строка Path источника карточки (s-contacts-6)
	// Compared — после Read: «not changed on server» (версия на сервере та
	// же, что в прочитанном списке) или «changed on server since»; пусто —
	// списка не читали
	Compared Text   `json:"compared"`
	Stamp    Stamp  `json:"stamp"`
	Colour   Colour `json:"colour"`
	ReadOnly bool   `json:"readOnly"`
	LastRead When   `json:"lastRead"`
	Changed  When   `json:"changed"`
	Objects  *int   `json:"objects"`
}
