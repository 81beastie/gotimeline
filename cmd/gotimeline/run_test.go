package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const okCSV = `"Timestamp","RuleTitle","Level","Computer","Channel","EventID","RecordID","Details","ExtraFieldInfo","RuleID"
"2026-01-01T01:00:00Z","R","info","H","Sec","1","1","d","e","r"
"2026-01-01T02:00:00Z","R2","low","H","Sec","2","2","d2","e2","r2"
`

const okFacts = `[{"t":"2026-01-01T01:00","host":"H","kind":"msi","label":"Установка"}]`

func tmpFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun_SkipScanEndToEnd(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	out := filepath.Join(t.TempDir(), "timeline.html")
	cfg := config{
		skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV),
		minLevel: "info", out: out, title: "Тест",
		factsPath:    tmpFile(t, "facts.json", okFacts),
		incidentFrom: "2026-01-01T00:00", incidentTo: "2026-01-01T23:59",
	}
	if err := run(cfg); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{"<h1>Тест</h1>", "Установка", "окно инцидента"} {
		if !strings.Contains(s, want) {
			t.Errorf("в HTML нет %q", want)
		}
	}
}

func TestRun_SkipScanWithoutCSVIsError(t *testing.T) {
	if err := run(config{skipScan: true, minLevel: "info"}); err == nil {
		t.Error("ожидала ошибку: -skip-scan без -csv")
	}
}

func TestRun_UnknownMinLevelIsError(t *testing.T) {
	cfg := config{skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV), minLevel: "nope"}
	if err := run(cfg); err == nil {
		t.Error("ожидала ошибку для неизвестного -min-level")
	}
}

func TestRun_MissingCSVIsError(t *testing.T) {
	cfg := config{skipScan: true, csvPath: filepath.Join(t.TempDir(), "нет.csv"), minLevel: "info"}
	if err := run(cfg); err == nil {
		t.Error("ожидала ошибку для отсутствующего CSV")
	}
}

func TestTimelineCSV_ScanMissingBinaryIsError(t *testing.T) {
	cfg := config{dir: t.TempDir(), hayabusaBin: "нет-такого-хаябуса"}
	if _, err := timelineCSV(cfg); err == nil {
		t.Error("ожидала ошибку запуска несуществующего бинарника")
	}
}

func TestMainWithArgs_BadFlagReturnsTwo(t *testing.T) {
	if code := mainWithArgs([]string{"--нет-такого-флага"}); code != 2 {
		t.Errorf("код возврата = %d, ожидала 2 (ошибка флагов)", code)
	}
}

func TestMainWithArgs_FullPipelineExitZero(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	out := filepath.Join(t.TempDir(), "timeline.html")
	args := []string{"-skip-scan", "-csv", tmpFile(t, "in.csv", okCSV),
		"-min-level", "info", "-out", out, "-title", "E2E"}
	if code := mainWithArgs(args); code != 0 {
		t.Errorf("код возврата = %d, ожидала 0", code)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("итоговый HTML не создан: %v", err)
	}
}

// anydeskBundle — мини-бандл logcollector: HOST1/anydesk/connection_trace.txt (UTF-16LE).
func anydeskBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ad := filepath.Join(dir, "HOST1", "anydesk")
	os.MkdirAll(ad, 0o755)
	b := []byte{0xFF, 0xFE}
	for _, r := range "Incoming  2026-09-14, 05:30    User 640693301 640693301\r\n" {
		b = append(b, byte(r), 0)
	}
	os.WriteFile(filepath.Join(ad, "connection_trace.txt"), b, 0o644)
	return dir
}

func TestRun_RemoteDirAddsAppFacts(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	out := filepath.Join(t.TempDir(), "timeline.html")
	cfg := config{
		skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV),
		minLevel: "info", out: out, title: "Remote",
		remoteDir: anydeskBundle(t), remoteTZ: 3,
	}
	if err := run(cfg); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		`"app":"anydesk"`,
		`"dir":"in"`,
		"AnyDesk",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("в HTML нет %q", want)
		}
	}
	// 05:30 Мск (UTC+3) → 02:30 UTC
	if !strings.Contains(s, "2026-09-14T02:30") {
		t.Error("время коннекта не сконвертировано в UTC (05:30 Мск → 02:30 UTC)")
	}
}

func TestRun_RemoteDirMissingIsNotFatal(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	out := filepath.Join(t.TempDir(), "timeline.html")
	cfg := config{
		skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV),
		minLevel: "info", out: out,
		remoteDir: filepath.Join(t.TempDir(), "несуществующий"),
	}
	if err := run(cfg); err != nil {
		t.Fatalf("отсутствующий -remote-dir не должен валить сбор: %v", err)
	}
}

func TestRun_RemoteHostGetsOwnLane(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	out := filepath.Join(t.TempDir(), "timeline.html")
	cfg := config{
		skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV), // хост H, без HOST1
		minLevel: "info", out: out, title: "Lane",
		remoteDir: anydeskBundle(t), remoteTZ: 3,
	}
	if err := run(cfg); err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(out)
	s := string(html)
	// HOST1 нет в CSV — дорожка должна быть создана, иначе факт не отрисуется
	// (проверяем именно массив hosts, а не JSON факта)
	if !strings.Contains(s, `"hosts":["H","HOST1"]`) {
		t.Errorf("массив hosts не содержит дорожку HOST1")
	}
}

func TestRun_ScanDirAutoDetectsRemoteApps(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	// один каталог: и CSV-хост H, и подкаталог HOST1 с anydesk-трейсом —
	// remote-факты должны подхватиться без отдельного флага
	scanDir := anydeskBundle(t)
	out := filepath.Join(t.TempDir(), "timeline.html")
	cfg := config{
		skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV),
		minLevel: "info", out: out, title: "Auto",
		dir: scanDir, remoteTZ: 3,
	}
	if err := run(cfg); err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(out)
	s := string(html)
	if !strings.Contains(s, `"app":"anydesk"`) {
		t.Error("коннекты AnyDesk не подхвачены из -dir без -remote-dir")
	}
	if !strings.Contains(s, `"hosts":["H","HOST1"]`) {
		t.Error("дорожка HOST1 не создана")
	}
}

func TestRun_AutoDetectSkipsOtherRemoteDirs(t *testing.T) {
	// -remote-dir задан явно — авто-детект не должен дублировать факты
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	scanDir := anydeskBundle(t)
	out := filepath.Join(t.TempDir(), "timeline.html")
	cfg := config{
		skipScan: true, csvPath: tmpFile(t, "in.csv", okCSV),
		minLevel: "info", out: out, title: "Dedup",
		dir: scanDir, remoteDir: scanDir, remoteTZ: 3,
	}
	if err := run(cfg); err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(out)
	s := string(html)
	if strings.Count(s, `"app":"anydesk"`) != 1 {
		t.Errorf("дубль remote-фактов: app встречается %d раз, ожидала 1",
			strings.Count(s, `"app":"anydesk"`))
	}
}
