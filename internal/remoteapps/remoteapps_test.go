package remoteapps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// anydeskTrace — реальный формат connection_trace.txt AnyDesk в UTF-16LE.
func anydeskTrace(t *testing.T) string {
	t.Helper()
	// "Incoming 2025-10-09, 05:30 User 640693301 640693301\r\n" и т.п.
	lines := []string{
		"Incoming  2025-10-09, 05:30    User 640693301 640693301",
		"Outgoing  2025-10-09, 05:37    User 640693301 640693301",
		"Incoming  2025-10-14, 02:42    Passwd 1708936107 1708936107",
		"Outgoing  2025-11-07, 06:50    REJECTED 499428485 499428485",
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "anydesk"), 0o755)
	// AnyDesk пишет UTF-16LE с BOM
	b := []byte{0xFF, 0xFE}
	for _, l := range lines {
		for _, r := range l + "\r\n" {
			b = append(b, byte(r), 0)
		}
	}
	os.WriteFile(filepath.Join(dir, "anydesk", "connection_trace.txt"), b, 0o644)
	return dir
}

func TestLoadAnyDesk_TraceBecomesFacts(t *testing.T) {
	dir := anydeskTrace(t)

	facts, err := Load(dir, "HOST1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 4 {
		t.Fatalf("фактов: %d, ожидала 4", len(facts))
	}

	in, out, rejected := 0, 0, 0
	for _, f := range facts {
		if f.App != "anydesk" {
			t.Errorf("app = %q, ожидала anydesk", f.App)
		}
		switch {
		case f.Dir == "in":
			in++
		case f.Dir == "out":
			out++
		}
		if f.Host != "HOST1" {
			t.Errorf("host = %q", f.Host)
		}
	}
	if in != 2 || out != 2 {
		t.Errorf("in=%d out=%d, ожидала 2/2", in, out)
	}
	_ = rejected

	// время: трейс в локальном времени (Мск, UTC+3) → на таймлайне UTC
	first := facts[0]
	if first.T != "2025-10-09T02:30" {
		t.Errorf("t = %q, ожидала 2025-10-09T02:30 (05:30 Мск → 02:30 UTC)", first.T)
	}
}

func TestLoadAnyDesk_AuthTypeInDetails(t *testing.T) {
	dir := anydeskTrace(t)

	facts, _ := Load(dir, "HOST1", 3)
	if !contains(facts, "Passwd") {
		t.Error("в details должен быть тип авторизации Passwd")
	}
	if !contains(facts, "REJECTED") {
		t.Error("отклонённое подключение должно сохраниться (REJECTED)")
	}
}

func TestLoadAnyDesk_IDsInDetails(t *testing.T) {
	dir := anydeskTrace(t)

	facts, _ := Load(dir, "HOST1", 3)
	if !contains(facts, "1708936107") {
		t.Error("в details должен быть ID подключения")
	}
}

func TestLoad_EmptyDirNoFactsNoError(t *testing.T) {
	facts, err := Load(t.TempDir(), "HOST1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Errorf("пустой каталог — 0 фактов, получено %d", len(facts))
	}
}

func contains(facts []Fact, sub string) bool {
	for _, f := range facts {
		if f.Label != "" && (containsStr(f.Label, sub) || containsStr(f.Details, sub)) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// anydeskTraces — ad.trace и ad_svc.trace с реальными строками сессий.
func anydeskTraces(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ad := filepath.Join(dir, "anydesk")
	os.MkdirAll(ad, 0o755)
	adTrace := strings.Join([]string{
		"   info 2026-04-21 02:57:05.509      front  48620  36660                  app.frontend_session - Requesting session.",
		"   info 2026-04-21 02:57:42.829      front  48620  28024                  app.frontend_session - The user has requested a connection quit.",
		"   info 2026-09-18 03:02:15.444      front  20408  11072                  app.frontend_session - Requesting session.",
		"   info 2026-09-18 03:02:40.110      front  20408  20768                  app.frontend_session - The user has requested a connection quit.",
		"   info 2026-09-18 03:05:00.000      ctrl   1  1                  base.monitor_info - Monitors found: 1",
	}, "\n")
	svcTrace := strings.Join([]string{
		"   info 2026-09-07 08:01:34.665       gsvc   4428   7084    9            anynet.connection_mgr - Main relay connection established.",
		"   info 2026-09-07 08:01:34.665       gsvc   4428   7084    9            anynet.connection_mgr - New user data. Client-ID: 1432074739.",
		"   info 2026-09-10 12:46:38.244       gsvc   4500   6648    9            anynet.connection_mgr - Main relay connection established.",
		"   info 2026-09-10 12:46:38.244       gsvc   4500   6648    9            anynet.connection_mgr - New user data. Client-ID: 1432074739.",
	}, "\n")
	os.WriteFile(filepath.Join(ad, "ad.trace"), []byte(adTrace), 0o644)
	os.WriteFile(filepath.Join(ad, "ad_svc.trace"), []byte(svcTrace), 0o644)
	return dir
}

func TestLoadAnyDesk_TraceSessionsBecomeFacts(t *testing.T) {
	dir := anydeskTraces(t)

	facts, err := Load(dir, "HOST1", 3)
	if err != nil {
		t.Fatal(err)
	}
	// 2 сессии из ad.trace (Requesting) + 2 коннекта релея из ad_svc.trace
	sessions := 0
	relays := 0
	for _, f := range facts {
		if !containsStr(f.Details, "сессия") && !containsStr(f.Details, "релей") {
			continue
		}
		if containsStr(f.Details, "релей") {
			relays++
		} else {
			sessions++
		}
	}
	if sessions != 2 {
		t.Errorf("сессий из ad.trace: %d, ожидала 2 (21.04 и 18.09)", sessions)
	}
	if relays != 2 {
		t.Errorf("коннектов релея из ad_svc.trace: %d, ожидала 2 (07.09 и 10.09)", relays)
	}
}

func TestLoadAnyDesk_SessionTimeIsUTC(t *testing.T) {
	dir := anydeskTraces(t)

	facts, _ := Load(dir, "HOST1", 3)
	// 2026-04-21 02:57 Мск (UTC+3) → 2026-04-20T23:57 UTC
	found := false
	for _, f := range facts {
		if f.T == "2026-04-20T23:57" {
			found = true
		}
	}
	if !found {
		t.Errorf("сессия 02:57 Мск не сконвертирована в 23:57 UTC предыдущего дня")
	}
}

func TestLoadAnyDesk_TechnicalNoiseSkipped(t *testing.T) {
	dir := anydeskTraces(t)

	facts, _ := Load(dir, "HOST1", 3)
	for _, f := range facts {
		if containsStr(f.Details, "Monitors found") {
			t.Error("технический шум (monitor_info) не должен попадать в факты")
		}
	}
}
