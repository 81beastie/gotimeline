// Package remoteapps — коннекты инструментов удалённого доступа (AnyDesk, ...)
// из бандлов logcollector как факты таймлайна.
package remoteapps

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/81beastie/gotimeline/internal/domain"
)

// Fact — доменный факт (алиас для краткости).
type Fact = domain.Fact

// HasTraces — имя подкаталога с следами удалёнки (anydesk, rudesktop, ...).
func HasTraces(name string) bool {
	switch name {
	case "anydesk", "rudesktop", "teamviewer", "ammyy", "radmin", "meshagent", "supremo", "aeroadmin", "ultraviewer", "dwservice", "logmein":
		return true
	}
	return false
}

// Load — ищет следы удалёнки в бандле (каталог хоста из logcollector)
// и превращает коннекты в факты. tzOffset — часовой пояс машины в часах
// (Мск = 3): логи удалёнки пишутся в локальном времени, таймлайн — в UTC.
func Load(bundleDir, host string, tzOffset int) ([]Fact, error) {
	var facts []Fact

	anydeskFacts, err := loadAnyDesk(bundleDir, host, tzOffset)
	if err != nil {
		return nil, err
	}
	facts = append(facts, anydeskFacts...)

	sort.SliceStable(facts, func(i, j int) bool { return facts[i].T < facts[j].T })
	return facts, nil
}

// loadAnyDesk — все следы AnyDesk: connection_trace.txt (коннекты, UTF-16LE),
// ad.trace (сессии клиента), ad_svc.trace (коннекты релея сервиса).
func loadAnyDesk(bundleDir, host string, tzOffset int) ([]Fact, error) {
	adDir := filepath.Join(bundleDir, "anydesk")

	var facts []Fact

	connFacts, err := loadAnyDeskConnections(adDir, host, tzOffset)
	if err != nil {
		return nil, err
	}
	facts = append(facts, connFacts...)

	for _, trace := range []string{"ad.trace", "ad_svc.trace"} {
		traceFacts, err := loadAnyDeskTrace(filepath.Join(adDir, trace), host, tzOffset)
		if err != nil {
			return nil, err
		}
		facts = append(facts, traceFacts...)
	}
	return dedupSameMinute(facts), nil
}

// dedupSameMinute — релей-коннект и его Client-ID пишутся в одну секунду:
// на таймлайне достаточно одного факта с объединёнными деталями.
func dedupSameMinute(facts []Fact) []Fact {
	byKey := map[string]*Fact{}
	var order []string
	for i := range facts {
		f := facts[i]
		key := f.T + "|" + f.Host + "|" + f.App + "|" + f.Dir + "|" + f.Label
		if prev, ok := byKey[key]; ok {
			if !strings.Contains(prev.Details, f.Details) {
				prev.Details += " | " + f.Details
			}
			continue
		}
		cp := f
		byKey[key] = &cp
		order = append(order, key)
	}
	out := make([]Fact, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

// loadAnyDeskConnections — connection_trace.txt (UTF-16LE): "Incoming 2025-10-09, 05:30 User 640693301".
func loadAnyDeskConnections(adDir, host string, tzOffset int) ([]Fact, error) {
	path := filepath.Join(adDir, "connection_trace.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // файла нет — тишина
		}
		return nil, fmt.Errorf("чтение %s: %w", path, err)
	}

	var facts []Fact
	for _, line := range strings.Split(decodeUTF16LE(raw), "\n") {
		f, ok := parseAnyDeskLine(strings.TrimSpace(line), host, tzOffset)
		if ok {
			facts = append(facts, f)
		}
	}
	return facts, nil
}

// loadAnyDeskTrace — ad.trace/ad_svc.trace (UTF-8): сессии клиента и коннекты релея.
func loadAnyDeskTrace(path, host string, tzOffset int) ([]Fact, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("чтение %s: %w", path, err)
	}

	var facts []Fact
	for _, line := range strings.Split(string(raw), "\n") {
		f, ok := parseAnyDeskTraceLine(line, host, tzOffset)
		if ok {
			facts = append(facts, f)
		}
	}
	return facts, nil
}

// parseAnyDeskTraceLine — строка ad.trace/ad_svc.trace в факт.
// Формат: "   info 2026-04-21 02:57:05.509      front  ...  app.frontend_session - Requesting session."
func parseAnyDeskTraceLine(line, host string, tzOffset int) (Fact, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return Fact{}, false
	}
	local, err := time.ParseInLocation("2006-01-02 15:04:05", fields[1]+" "+fields[2], time.UTC)
	if err != nil {
		return Fact{}, false
	}
	utc := local.Add(time.Duration(-tzOffset) * time.Hour)
	msg := strings.Join(fields[3:], " ")

	switch {
	case strings.Contains(msg, "app.frontend_session - Requesting session."):
		return traceFact(utc, host, "anydesk", "out",
			"AnyDesk: старт сессии клиента",
			"AnyDesk: запрошена интерактивная сессия клиента (app.frontend_session). Техтрейс ad.trace."), true
	case strings.Contains(msg, "The user has requested a connection quit."):
		return traceFact(utc, host, "anydesk", "out",
			"AnyDesk: завершение сессии",
			"AnyDesk: пользователь завершил сессию (connection quit). Техтрейс ad.trace."), true
	case strings.Contains(msg, "Main relay connection established."):
		return traceFact(utc, host, "anydesk", "in",
			"AnyDesk: коннект релея",
			"AnyDesk: установлено соединение с релей-сервером AnyDesk (ad_svc.trace)."), true
	case strings.Contains(msg, "New user data. Client-ID:"):
		id := ""
		if i := strings.Index(msg, "Client-ID:"); i >= 0 {
			id = strings.TrimSpace(strings.TrimPrefix(msg[i+len("Client-ID:"):], " "))
			if j := strings.Index(id, "."); j >= 0 {
				id = id[:j]
			}
		}
		details := "AnyDesk: данные клиента через релей (ad_svc.trace)."
		if id != "" {
			details += " Client-ID: " + id
		}
		return traceFact(utc, host, "anydesk", "in", "AnyDesk: коннект релея", details), true
	}
	return Fact{}, false // технический шум — мимо
}

// traceFact — факт из техтрейса.
func traceFact(utc time.Time, host, app, dir, label, details string) Fact {
	return Fact{
		T:       utc.Format("2006-01-02T15:04"),
		Host:    host,
		Kind:    "remote",
		Label:   label,
		Details: details,
		App:     app,
		Dir:     dir,
	}
}

// parseAnyDeskLine — одна строка трейса в факт.
func parseAnyDeskLine(line, host string, tzOffset int) (Fact, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Fact{}, false
	}

	dir := ""
	switch strings.ToLower(fields[0]) {
	case "incoming":
		dir = "in"
	case "outgoing":
		dir = "out"
	default:
		return Fact{}, false
	}

	// fields[1]="2025-10-09," fields[2]="05:30" fields[3]="User" fields[4]="640693301"
	local, err := time.ParseInLocation("2006-01-02 15:04",
		strings.TrimSuffix(fields[1], ",")+" "+fields[2], time.UTC)
	if err != nil {
		return Fact{}, false
	}
	utc := local.Add(time.Duration(-tzOffset) * time.Hour)

	auth := fields[3]
	id := ""
	if len(fields) >= 5 {
		id = fields[4]
	}

	label := "AnyDesk: " + map[string]string{"in": "входящий", "out": "исходящий"}[dir]
	details := "AnyDesk " + map[string]string{"in": "входящее", "out": "исходящее"}[dir] +
		" подключение. Авторизация: " + auth
	if id != "" {
		details += ". ID: " + id
	}

	return Fact{
		T:       utc.Format("2006-01-02T15:04"),
		Host:    host,
		Kind:    "remote",
		Label:   label,
		Details: details,
		App:     "anydesk",
		Dir:     dir,
	}, true
}

// decodeUTF16LE — UTF-16LE с BOM → UTF-8 (формат файлов AnyDesk).
func decodeUTF16LE(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		b = b[2:]
	}
	u16 := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u16 = append(u16, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u16))
}
