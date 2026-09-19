package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/81beastie/gotimeline/internal/domain"
)

func TestLoadFacts_ReadsMarkers(t *testing.T) {
	content := `[
	 {"t":"2026-01-01T09:29","host":"HOST1","kind":"av-off","label":"Антивирус на паузе","details":"защита отключена"},
	 {"t":"2026-01-02T09:22","host":"HOST1","kind":"remote","label":"Удалённый доступ"}
	]`
	path := filepath.Join(t.TempDir(), "facts.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	facts, err := LoadFacts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 {
		t.Fatalf("маркеров: %d, ожидала 2", len(facts))
	}
	if facts[0].Host != "HOST1" || facts[0].Kind != "av-off" || facts[0].Label != "Антивирус на паузе" {
		t.Errorf("первый маркер: %+v", facts[0])
	}
	if facts[1].T != "2026-01-02T09:22" {
		t.Errorf("второй маркер: %+v", facts[1])
	}
}

func TestLoadFacts_MissingFileIsError(t *testing.T) {
	if _, err := LoadFacts(filepath.Join(t.TempDir(), "нет.json")); err == nil {
		t.Error("ожидала ошибку для отсутствующего файла")
	}
}

func TestLoadFacts_InvalidJSONIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.json")
	if err := os.WriteFile(path, []byte("{не json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFacts(path); err == nil {
		t.Error("ожидала ошибку для битого JSON")
	}
}

func TestParse_EmptyCSVGivesEmptyData(t *testing.T) {
	data, err := Parse(writeTemp(t, csvHeader+"\n"), Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Hosts) != 0 || len(data.Hourly) != 0 {
		t.Errorf("пустой CSV: ожидала пустые Hosts/Hourly, получили %+v", data.Hosts)
	}
	if data.Facts == nil {
		t.Error("Facts должен быть пустым слайсом, не nil (для JSON [])")
	}
}
