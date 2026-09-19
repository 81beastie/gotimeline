package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/81beastie/gotimeline/internal/domain"
)

func sampleData() *domain.Data {
	return &domain.Data{
		Facts: []domain.Fact{{T: "2026-09-11T09:29", Host: "BUH3", Kind: "ksos", Label: "KSOS SNOOZED"}},
		Hosts: []string{"BUH3"},
		Hourly: map[string][]domain.HourPoint{
			"BUH3": {{T: "2026-09-11T09", N: 5,
				Rules:   []domain.RuleCount{{Rule: "Svc Installed", N: 5}},
				Samples: []domain.Sample{{Ts: "2026-09-11T09:32:00Z", Rule: "Svc Installed", EID: "7045", Ch: "Sys", Details: "Svc: Acrobat"}}}},
		},
		Incident: &domain.Incident{From: "2026-09-03T00:00", To: "2026-09-17T23:59"},
	}
}

func writeOut(t *testing.T, data *domain.Data, title string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "timeline.html")
	if err := Write(path, title, data); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWrite_ProducesAutonomousHTML(t *testing.T) {
	html, err := os.ReadFile(writeOut(t, sampleData(), "Таймлайн БФК"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)

	if strings.Contains(s, dataPlaceholder) {
		t.Error("плейсхолдер данных не заменён")
	}
	if strings.Contains(s, titlePlaceholder) {
		t.Error("плейсхолдер заголовка не заменён")
	}
	for _, want := range []string{
		"<h1>Таймлайн БФК</h1>",
		`<title>Таймлайн БФК</title>`,
		"KSOS SNOOZED",
		"BUH3",
		"2026-09-11T09",
		"окно инцидента",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("в HTML нет %q", want)
		}
	}
	if !strings.Contains(s, "const DATA = {") {
		t.Error("данные не встроены как объект (остался null)")
	}
}

func TestWrite_EscapesScriptTagClosers(t *testing.T) {
	data := sampleData()
	data.Facts[0].Details = "закрытие тега </script> в данных"
	s := string(mustRead(t, writeOut(t, data, "x")))

	if strings.Contains(s, "</script> в данных") {
		t.Error("неэкранированный </script> внутри JSON — сломает страницу")
	}
	if !strings.Contains(s, `\u003c/script`) {
		t.Error("ожидала HTML-safe экранирование < через \u003c (json.Marshal по умолчанию)")
	}
}

func TestWrite_EmbeddedJSONIsValid(t *testing.T) {
	s := string(mustRead(t, writeOut(t, sampleData(), "x")))
	start := strings.Index(s, "const DATA = ") + len("const DATA = ")
	end := strings.Index(s[start:], ";\n")
	if end < 0 {
		t.Fatal("не нашла конец встроенного JSON")
	}
	var back domain.Data
	if err := json.Unmarshal([]byte(s[start:start+end]), &back); err != nil {
		t.Fatalf("встроенный JSON не парсится: %v", err)
	}
	if len(back.Hosts) != 1 || back.Hosts[0] != "BUH3" {
		t.Errorf("round-trip хостов: %+v", back.Hosts)
	}
	if back.Incident == nil || back.Incident.From != "2026-09-03T00:00" {
		t.Errorf("round-trip окна инцидента: %+v", back.Incident)
	}
}

func TestWrite_TitleHTMLEscaped(t *testing.T) {
	s := string(mustRead(t, writeOut(t, sampleData(), `Инцидент <"БФК"> & Co`)))
	if strings.Contains(s, "<h1>Инцидент <") {
		t.Error("заголовок с < не экранирован")
	}
	if !strings.Contains(s, "&lt;&#34;БФК&#34;&gt;") && !strings.Contains(s, "&lt;&quot;БФК&quot;&gt;") {
		t.Errorf("ожидала экранированный заголовок, найдено: %s", s[:200])
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
