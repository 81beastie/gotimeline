// Package domain — модели данных таймлайна.
package domain

// Fact — ключевой маркер на таймлайне (находка расследования).
type Fact struct {
	T       string `json:"t"`     // "2026-09-11T09:29"
	Host    string `json:"host"`  // имя компьютера как в EVTX
	Kind    string `json:"kind"`  // ключ палитры шаблона (ksos, anydesk, ...)
	Label   string `json:"label"` // короткий заголовок
	Details string `json:"details"`
}

// Sample — образец записи журнала внутри одного часа.
type Sample struct {
	Ts      string `json:"ts"`
	Rule    string `json:"rule"`
	EID     string `json:"eid"`
	Ch      string `json:"ch"`
	Details string `json:"details"`
	Extra   string `json:"extra,omitempty"`
}

// RuleCount — правило Hayabusa с числом срабатываний за час.
type RuleCount struct {
	Rule string `json:"rule"`
	N    int    `json:"n"`
}

// HourPoint — агрегат событий одного хоста за один час.
type HourPoint struct {
	T       string      `json:"t"` // "2026-09-14T01"
	N       int         `json:"n"`
	Rules   []RuleCount `json:"rules"`
	Samples []Sample    `json:"samples"`
}

// Incident — окно инцидента (вертикальная зона на таймлайне).
type Incident struct {
	From string `json:"from"` // "2026-09-03T00:00"
	To   string `json:"to"`
}

// Data — полный набор данных для встраивания в HTML.
type Data struct {
	Facts    []Fact                 `json:"facts"`
	Hourly   map[string][]HourPoint `json:"hourly"`
	Hosts    []string               `json:"hosts"`
	Incident *Incident              `json:"incident,omitempty"`
}

// Level — уровень серьёзности Hayabusa с рангом для порога.
type Level int

// Уровни Hayabusa по возрастанию веса.
const (
	LevelInfo Level = iota + 1
	LevelLow
	LevelMed
	LevelHigh
	LevelCritical
)

// ParseLevel — строка уровня из CSV Hayabusa → Level; ok=false для неизвестных.
func ParseLevel(s string) (Level, bool) {
	switch s {
	case "critical":
		return LevelCritical, true
	case "high":
		return LevelHigh, true
	case "med", "medium":
		return LevelMed, true
	case "low":
		return LevelLow, true
	case "informational", "info":
		return LevelInfo, true
	}
	return 0, false
}
