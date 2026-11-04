package view

// Diagnostic — технический блок под человеческим текстом.
type Diagnostic struct {
	Summary Text   `json:"summary"`
	Lines   []Text `json:"lines"` // моноширинные строки: метод, адрес, код, тело
}

// DiagnosticTrace — след пошаговой операции: подключение, проверка.
type DiagnosticTrace struct {
	Steps []TraceStep `json:"steps"`
	OK    bool        `json:"ok"`
	// Stopped — на каком шаге встало обнаружение, «Discovery stopped at
	// the principal.» (s-admin-28), над следом; прошло — пусто
	Stopped Text `json:"stopped,omitempty"`
}
type TraceStep struct {
	Name   Text       `json:"name"` // «.well-known», «принципал», «home-set», «коллекции»
	State  CheckState `json:"state"`
	Detail Text       `json:"detail"` // что именно ответил сервер
}

// LossReport — сервер принял запись, но что-то потерял; уведомление, не
// блокировка: запись уже произошла. Сообщение кадра (states-12, -17, -24,
// -31): Title жирным, за ним свойства через запятую, моноширинно, точкой в
// конце. Свойств нет — сообщения о свойствах нет.
type LossReport struct {
	Title      Text     `json:"title"` // «The change was saved, but the server did not keep these properties:»
	Properties []Text   `json:"properties"`
	Aggregated bool     `json:"aggregated"` // повторная потеря на этом сервере: сказано один раз
	Actions    []Action `json:"actions"`    // что можно сделать с потерей; пусто — нечего
	// Days — второе сообщение: дни серии, которых сервер не сохранил, словами
	// (states-36; Р-60 Typecase): Title — что потеряно («…did not keep two
	// days of the series.»), Body — каждый день и что с ним стало. Пропуск и
	// перенос тогда не стоят в Properties именами EXDATE и RECURRENCE-ID.
	// nil — дни серии целы
	Days *Notice `json:"days"`
}
