package domain

import "testing"

func TestParseLevel_KnownLevels(t *testing.T) {
	cases := map[string]Level{
		"critical":      LevelCritical,
		"high":          LevelHigh,
		"med":           LevelMed,
		"medium":        LevelMed,
		"low":           LevelLow,
		"info":          LevelInfo,
		"informational": LevelInfo,
	}
	for in, want := range cases {
		got, ok := ParseLevel(in)
		if !ok {
			t.Errorf("ParseLevel(%q): ожидала ok=true", in)
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %d, ожидала %d", in, got, want)
		}
	}
}

func TestParseLevel_UnknownIsNotOk(t *testing.T) {
	for _, in := range []string{"", "noise", "warning", "INFO"} {
		if _, ok := ParseLevel(in); ok {
			t.Errorf("ParseLevel(%q): ожидала ok=false для неизвестного уровня", in)
		}
	}
}

func TestParseLevel_Ranking(t *testing.T) {
	if !(LevelInfo < LevelLow && LevelLow < LevelMed && LevelMed < LevelHigh && LevelHigh < LevelCritical) {
		t.Error("ранги уровней должны возрастать: info < low < med < high < critical")
	}
}
