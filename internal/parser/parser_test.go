package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/81beastie/gotimeline/internal/domain"
)

const csvHeader = `"Timestamp","RuleTitle","Level","Computer","Channel","EventID","RecordID","Details","ExtraFieldInfo","RuleID"`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "timeline.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParse_AggregatesHourlyByHost(t *testing.T) {
	path := writeTemp(t, csvHeader+"\n"+
		`"2026-01-01T01:01:00Z","Rule A","high","HOST1","Sec","5376","1","User: alice","x","id1"`+"\n"+
		`"2026-01-01T01:02:00Z","Rule A","high","HOST1","Sec","5376","2","User: alice","x","id1"`+"\n"+
		`"2026-01-01T01:03:00Z","Rule B","med","HOST1","Sec","4624","3","Type: 3","y","id2"`+"\n"+
		`"2026-01-02T05:00:00Z","Rule C","low","HOST2","Sys","7045","4","Svc: X","z","id3"`+"\n")

	data, err := Parse(path, Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Hosts) != 2 {
		t.Fatalf("хостов: %d, ожидала 2 (HOST1, HOST2)", len(data.Hosts))
	}
	h1 := data.Hourly["HOST1"]
	if len(h1) != 1 {
		t.Fatalf("часовых точек HOST1: %d, ожидала 1 (всё в час 01)", len(h1))
	}
	if h1[0].T != "2026-01-01T01" {
		t.Errorf("ключ часа = %q, ожидала 2026-01-01T01", h1[0].T)
	}
	if h1[0].N != 3 {
		t.Errorf("N = %d, ожидала 3 события в час", h1[0].N)
	}
	if got := data.Hourly["HOST2"][0].N; got != 1 {
		t.Errorf("N HOST2 = %d, ожидала 1", got)
	}
}

func TestParse_TopRulesSortedAndLimited(t *testing.T) {
	lines := csvHeader + "\n"
	for i := 0; i < 10; i++ {
		lines += `"2026-01-01T01:00:00Z","Rule A","info","H","Sec","1","` + itoa(i) + `","d","e","r"` + "\n"
	}
	for i := 0; i < 3; i++ {
		lines += `"2026-01-01T01:00:00Z","Rule B","info","H","Sec","1","` + itoa(100+i) + `","d","e","r"` + "\n"
	}
	for i := 0; i < 9; i++ {
		lines += `"2026-01-01T01:00:00Z","Rule C","info","H","Sec","1","` + itoa(200+i) + `","d","e","r"` + "\n"
	}

	data, err := Parse(writeTemp(t, lines), Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	rules := data.Hourly["H"][0].Rules
	if len(rules) != 3 {
		t.Fatalf("правил в топе: %d, ожидала 3", len(rules))
	}
	if rules[0].Rule != "Rule A" || rules[0].N != 10 {
		t.Errorf("первое правило = %v, ожидала Rule A x10 (сортировка по убыванию)", rules[0])
	}
	if rules[1].Rule != "Rule C" || rules[1].N != 9 {
		t.Errorf("второе правило = %v, ожидала Rule C x9", rules[1])
	}
}

func TestParse_MinLevelFilter(t *testing.T) {
	path := writeTemp(t, csvHeader+"\n"+
		`"2026-01-01T01:00:00Z","R info","info","H","Sec","1","1","d","e","r"`+"\n"+
		`"2026-01-01T02:00:00Z","R low","low","H","Sec","1","2","d","e","r"`+"\n"+
		`"2026-01-01T03:00:00Z","R high","high","H","Sec","1","3","d","e","r"`+"\n")

	data, err := Parse(path, Options{MinLevel: domain.LevelLow})
	if err != nil {
		t.Fatal(err)
	}
	pts := data.Hourly["H"]
	if len(pts) != 2 {
		t.Fatalf("точек: %d, ожидала 2 (low и high, info отфильтрован)", len(pts))
	}
	if pts[0].T != "2026-01-01T02" || pts[1].T != "2026-01-01T03" {
		t.Errorf("часы после фильтра: %q, %q — ожидала 02 и 03", pts[0].T, pts[1].T)
	}
}

func TestParse_SamplesCappedAtTwelve(t *testing.T) {
	lines := csvHeader + "\n"
	for i := 0; i < 20; i++ {
		lines += `"2026-01-01T01:00:00Z","R","info","H","Sec","1","` + itoa(i) + `","d","e","r"` + "\n"
	}
	data, err := Parse(writeTemp(t, lines), Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	s := data.Hourly["H"][0].Samples
	if len(s) != 12 {
		t.Errorf("образцов: %d, ожидала максимум 12", len(s))
	}
	if data.Hourly["H"][0].N != 20 {
		t.Errorf("N = %d, ожидала 20 (счётчик не режется лимитом образцов)", data.Hourly["H"][0].N)
	}
}

func TestParse_SkipsMalformedRows(t *testing.T) {
	path := writeTemp(t, csvHeader+"\n"+
		`"2026-01-01T01:00:00Z","R","info","H","Sec","1","1","d","e","r"`+"\n"+
		"кривая строка без полей\n"+
		`"short","row"`+"\n"+
		`"2026-01-01T02:00:00Z","R","unknown-level","H","Sec","1","2","d","e","r"`+"\n")

	data, err := Parse(path, Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	if pts := data.Hourly["H"]; len(pts) != 1 {
		t.Errorf("точек: %d, ожидала 1 (кривые строки и неизвестный уровень пропущены)", len(pts))
	}
}

func TestParse_HostsSorted(t *testing.T) {
	path := writeTemp(t, csvHeader+"\n"+
		`"2026-01-01T01:00:00Z","R","info","Zeta","Sec","1","1","d","e","r"`+"\n"+
		`"2026-01-01T01:00:00Z","R","info","Alpha","Sec","1","2","d","e","r"`+"\n"+
		`"2026-01-01T01:00:00Z","R","info","Mid","Sec","1","3","d","e","r"`+"\n")

	data, err := Parse(path, Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha", "Mid", "Zeta"}
	for i, h := range data.Hosts {
		if h != want[i] {
			t.Errorf("Hosts[%d] = %q, ожидала %q (сортировка A-Z)", i, h, want[i])
		}
	}
}

func TestParse_ClampsLongDetails(t *testing.T) {
	long := strings.Repeat("x", 250)
	path := writeTemp(t, csvHeader+"\n"+
		`"2026-01-01T01:00:00Z","R","info","H","Sec","1","1","`+long+`","`+long+`","r"`+"\n")

	data, err := Parse(path, Options{MinLevel: domain.LevelInfo})
	if err != nil {
		t.Fatal(err)
	}
	s := data.Hourly["H"][0].Samples[0]
	if len(s.Details) != 203 {
		t.Errorf("Details: %d байт, ожидала 200 + многоточие (3 байта UTF-8)", len(s.Details))
	}
	if !strings.HasSuffix(s.Details, "…") {
		t.Error("Details должен заканчиваться многоточием")
	}
	if len(s.Extra) != 153 {
		t.Errorf("Extra: %d байт, ожидала 150 + многоточие (3 байта UTF-8)", len(s.Extra))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
