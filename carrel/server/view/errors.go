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
}
type TraceStep struct {
	Name   Text       `json:"name"` // «.well-known», «принципал», «home-set», «коллекции»
	State  CheckState `json:"state"`
	Detail Text       `json:"detail"` // что именно ответил сервер
}

// LossReport — сервер принял запись, но что-то потерял.
type LossReport struct {
	Properties []Text   `json:"properties"`
	Note       Text     `json:"note"`       // уведомление, не блокировка: запись уже произошла
	Aggregated bool     `json:"aggregated"` // повторная потеря на этом сервере: сказано один раз
	Actions    []Action `json:"actions"`    // что можно сделать с потерей; пусто — нечего
}
