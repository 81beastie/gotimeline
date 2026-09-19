package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const okCSV = `"Timestamp","RuleTitle","Level","Computer","Channel","EventID","RecordID","Details","ExtraFieldInfo","RuleID"
"2026-09-14T01:00:00Z","R","info","H","Sec","1","1","d","e","r"
"2026-09-14T02:00:00Z","R2","low","H","Sec","2","2","d2","e2","r2"
`

const okFacts = `[{"t":"2026-09-14T01:00","host":"H","kind":"msi","label":"Установка"}]`

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
		incidentFrom: "2026-09-14T00:00", incidentTo: "2026-09-14T23:59",
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
