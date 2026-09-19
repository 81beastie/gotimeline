package hayabusa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_JoinsBinaryAndDir(t *testing.T) {
	sc := Scanner{Binary: "/opt/hayabusa", Rules: "/rules"}
	got := sc.Command("/cases/demo")
	if !strings.HasPrefix(got, "/opt/hayabusa dfir-timeline -d /cases/demo") {
		t.Errorf("Command = %q, ожидала начало с бинарника и -d", got)
	}
}

func TestBuild_MissingBinaryIsError(t *testing.T) {
	sc := Scanner{Binary: "нет-такого-хаябуса"}
	if _, err := sc.Build(t.TempDir(), t.TempDir()); err == nil {
		t.Error("ожидала ошибку для несуществующего бинарника")
	}
}

func TestBuild_FakeScannerProducesCSV(t *testing.T) {
	dir := t.TempDir()
	work := t.TempDir()

	fake := filepath.Join(dir, "fake-hayabusa")
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    -o) out=\"$2\"; shift 2;;\n    *) shift;;\n  esac\ndone\nprintf 'Timestamp,Level\\n2026-01-01T01:00:00Z,info\\n' > \"$out\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	sc := Scanner{Binary: fake, Rules: "/rules"}
	csv, err := sc.Build(dir, work)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(csv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Timestamp") {
		t.Errorf("фальшивый сканер должен был создать CSV, получено: %q", b)
	}
}

func TestBuild_FailsWhenOutputMissing(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-hayabusa")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	sc := Scanner{Binary: fake}
	if _, err := sc.Build(dir, t.TempDir()); err == nil {
		t.Error("ожидала ошибку: сканер отработал, но CSV не создал")
	}
}
