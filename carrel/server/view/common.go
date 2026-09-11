package view

// Готовые к показу значения. Собираются сервером по C-10;
// клиент выводит их как есть и никогда не разбирает обратно.
type (
	Text   string // строка интерфейса или данных пользователя
	When   string // дата и время: «10 August, 10:00», «весь день», «2 min ago»
	Size   string // объём: «1.4 MB»
	Colour string // цвет метки коллекции: готовое значение, C-10
	Stamp  string // происхождение записи: «nextcloud · Личный»
)

// Action — кнопка, пункт меню и строка панели — одно и то же.
type Action struct {
	ID      string `json:"id"` // устойчивое имя действия, C-7
	Label   Text   `json:"label"`
	Icon    string `json:"icon,omitempty"`   // имя из набора U-4; пусто — без значка
	Href    string `json:"href,omitempty"`   // пусто, если действие отправляет форму
	Method  string `json:"method"`           // "GET" | "POST"
	Danger  bool   `json:"danger,omitempty"` // разрушающее: удаление, сброс, объединение
	Enabled bool   `json:"enabled"`
	Reason  Text   `json:"reason,omitempty"` // почему выключено; пусто, если включено
}

type MenuItem struct {
	Action
	Separator bool `json:"separator,omitempty"` // черта перед пунктом
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
	Actions []Action    `json:"actions"`
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
	Stamp    Stamp  `json:"stamp"`
	Colour   Colour `json:"colour"`
	ReadOnly bool   `json:"readOnly"`
	LastRead When   `json:"lastRead"`
	Changed  When   `json:"changed"`
	Objects  *int   `json:"objects"`
}
