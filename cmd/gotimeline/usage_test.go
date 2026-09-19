package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainWithArgs_HelpPrintsUsage(t *testing.T) {
	out := runGotimeline(t, "-h")
	for _, want := range []string{"Использование:", "Флаги:", "-skip-scan", "Примеры:"} {
		if !strings.Contains(out, want) {
			t.Errorf("справка не содержит %q", want)
		}
	}
}

func TestMainWithArgs_PositionalArgOverridesDir(t *testing.T) {
	old := osStdout
	osStdout = os.NewFile(0, os.DevNull)
	t.Cleanup(func() { osStdout = old })

	dir := t.TempDir()
	args := []string{"-skip-scan", "-csv", tmpFile(t, "in.csv", okCSV), "-out", filepath.Join(dir, "t.html"), dir}
	if code := mainWithArgs(args); code != 0 {
		t.Fatalf("код возврата = %d, ожидала 0", code)
	}
}

func TestMainWithArgs_BadFlagPrintsUsageNotLoop(t *testing.T) {
	out := captureStdout(t, func() int { return mainWithArgs([]string{"--нет-флага"}) })
	if !strings.Contains(out, "Использование:") {
		t.Errorf("кривой флаг должен печатать справку, получено: %q", out)
	}
	if strings.Contains(out, "см. gotimeline -help") {
		t.Error("отсылка к самой себе (зацикленный -help) должна быть убрана")
	}
}

func captureStdout(t *testing.T, fn func() int) string {
	t.Helper()
	old := osStdout
	path := filepath.Join(t.TempDir(), "out.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	osStdout = f
	code := fn()
	f.Close()
	osStdout = old
	if code != 2 {
		t.Errorf("код возврата = %d, ожидала 2", code)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
