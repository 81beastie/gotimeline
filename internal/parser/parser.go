// Package parser — агрегация CSV-таймлайна Hayabusa в доменные модели.
package parser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/81beastie/gotimeline/internal/domain"
)

const (
	maxSamplesPerHour = 12
	maxRulesPerHour   = 6
	maxDetailLen      = 200
	maxExtraLen       = 150
)

// Options — параметры агрегации.
type Options struct {
	MinLevel domain.Level
}

// Parse — читает CSV Hayabusa и строит domain.Data.
func Parse(path string, opts Options) (*domain.Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.ReuseRecord = true

	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	cols := columnIndex(header)

	agg := newAggregator()
	if err := agg.consume(r, cols, opts.MinLevel); err != nil {
		return nil, err
	}
	return agg.result(), nil
}

func columnIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.Trim(h, `"`)] = i
	}
	return m
}

// hourAgg — накопитель одного часа одного хоста.
type hourAgg struct {
	key     string
	n       int
	rules   map[string]int
	samples []domain.Sample
}

// aggregator — накопитель по всем хостам.
type aggregator struct {
	hours map[string]map[string]*hourAgg // host -> "2006-01-02T15" -> agg
	hosts map[string]bool
}

func newAggregator() *aggregator {
	return &aggregator{hours: map[string]map[string]*hourAgg{}, hosts: map[string]bool{}}
}

func (a *aggregator) consume(r *csv.Reader, cols map[string]int, minLevel domain.Level) error {
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			continue
		}
		if a.accept(rec, cols, minLevel) {
			continue
		}
	}
}

// accept — фильтрует и учитывает одну запись; true если принята.
func (a *aggregator) accept(rec []string, cols map[string]int, minLevel domain.Level) bool {
	ts, ok := field(rec, cols, "Timestamp")
	if !ok || len(ts) < 13 {
		return false
	}
	level, ok := field(rec, cols, "Level")
	if !ok {
		return false
	}
	lvl, ok := domain.ParseLevel(strings.ToLower(level))
	if !ok || lvl < minLevel {
		return false
	}
	host, ok := field(rec, cols, "Computer")
	if !ok || host == "" {
		return false
	}

	a.hosts[host] = true
	agg := a.hour(host, ts[:13])
	agg.n++
	rule, _ := field(rec, cols, "RuleTitle")
	agg.rules[rule]++
	if len(agg.samples) < maxSamplesPerHour {
		s := domain.Sample{Ts: ts, Rule: rule}
		s.Ch, _ = field(rec, cols, "Channel")
		s.EID, _ = field(rec, cols, "EventID")
		if d, ok := field(rec, cols, "Details"); ok {
			s.Details = clamp(d, maxDetailLen)
		}
		if e, ok := field(rec, cols, "ExtraFieldInfo"); ok {
			s.Extra = clamp(e, maxExtraLen)
		}
		agg.samples = append(agg.samples, s)
	}
	return true
}

func (a *aggregator) hour(host, hourKey string) *hourAgg {
	m := a.hours[host]
	if m == nil {
		m = map[string]*hourAgg{}
		a.hours[host] = m
	}
	agg := m[hourKey]
	if agg == nil {
		agg = &hourAgg{key: hourKey, rules: map[string]int{}}
		m[hourKey] = agg
	}
	return agg
}

// result — финальная сборка domain.Data с сортировкой.
func (a *aggregator) result() *domain.Data {
	d := &domain.Data{Hourly: map[string][]domain.HourPoint{}, Facts: []domain.Fact{}}
	for h := range a.hosts {
		d.Hosts = append(d.Hosts, h)
	}
	sort.Strings(d.Hosts)

	for host, m := range a.hours {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		points := make([]domain.HourPoint, 0, len(keys))
		for _, k := range keys {
			points = append(points, m[k].point())
		}
		d.Hourly[host] = points
	}
	return d
}

func (h *hourAgg) point() domain.HourPoint {
	p := domain.HourPoint{T: h.key, N: h.n, Samples: h.samples}
	for rule, n := range h.rules {
		p.Rules = append(p.Rules, domain.RuleCount{Rule: rule, N: n})
	}
	sort.Slice(p.Rules, func(i, j int) bool { return p.Rules[i].N > p.Rules[j].N })
	if len(p.Rules) > maxRulesPerHour {
		p.Rules = p.Rules[:maxRulesPerHour]
	}
	return p
}

func field(rec []string, cols map[string]int, name string) (string, bool) {
	i, ok := cols[name]
	if !ok || i >= len(rec) {
		return "", false
	}
	return rec[i], true
}

func clamp(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// LoadFacts — чтение внешнего facts.json.
func LoadFacts(path string) ([]domain.Fact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("facts: %w", err)
	}
	var facts []domain.Fact
	if err := json.Unmarshal(b, &facts); err != nil {
		return nil, err
	}
	return facts, nil
}
