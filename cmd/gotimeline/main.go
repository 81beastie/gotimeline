// gotimeline — сборка интерактивного HTML-таймлайна по каталогу с EVTX:
// запуск Hayabusa (dfir-timeline) → агрегация CSV → автономный HTML.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/81beastie/gotimeline/internal/domain"
	"github.com/81beastie/gotimeline/internal/hayabusa"
	"github.com/81beastie/gotimeline/internal/parser"
	"github.com/81beastie/gotimeline/internal/render"
)

// osStdout — точка подмены в тестах.
var osStdout io.Writer = os.Stdout

func main() {
	os.Exit(mainWithArgs(os.Args[1:]))
}

// mainWithArgs — парсинг флагов и запуск; код возврата для тестов.
func mainWithArgs(args []string) int {
	fs := flag.NewFlagSet("gotimeline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cfg config
	registerFlags(fs, &cfg)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(osStdout, "см. gotimeline -help")
		return 2
	}
	if cfg.showVersion {
		fmt.Fprintf(osStdout, "gotimeline %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	if err := run(cfg); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}

type config struct {
	dir, hayabusaBin, rules, out, work string
	minLevel, factsPath                string
	incidentFrom, incidentTo, title    string
	skipScan                           bool
	csvPath                            string
	showVersion                        bool
}

// registerFlags — регистрирует флаги в cfg; Parse пишет прямо в эту структуру.
func registerFlags(fs *flag.FlagSet, cfg *config) {
	fs.StringVar(&cfg.dir, "dir", ".", "каталог с EVTX (рекурсивно)")
	fs.StringVar(&cfg.hayabusaBin, "hayabusa", "hayabusa", "путь к бинарнику Hayabusa")
	fs.StringVar(&cfg.rules, "rules", "", "каталог правил Hayabusa (пусто = встроенные)")
	fs.StringVar(&cfg.out, "out", "timeline.html", "итоговый HTML-файл")
	fs.StringVar(&cfg.work, "work", "", "каталог для промежуточного CSV (пусто = temp)")
	fs.StringVar(&cfg.minLevel, "min-level", "info", "минимальный уровень: info|low|med|high|critical")
	fs.StringVar(&cfg.factsPath, "facts", "", "facts.json с ключевыми маркерами")
	fs.StringVar(&cfg.incidentFrom, "incident-from", "", "начало окна инцидента, 2026-09-03T00:00")
	fs.StringVar(&cfg.incidentTo, "incident-to", "", "конец окна инцидента")
	fs.StringVar(&cfg.title, "title", "Интерактивный таймлайн", "заголовок страницы")
	fs.BoolVar(&cfg.skipScan, "skip-scan", false, "не запускать Hayabusa, взять готовый CSV (-csv)")
	fs.StringVar(&cfg.csvPath, "csv", "", "готовый CSV таймлайна Hayabusa")
	fs.BoolVar(&cfg.showVersion, "version", false, "показать версию и выйти")
}

func run(cfg config) error {
	if cfg.skipScan && cfg.csvPath == "" {
		return fmt.Errorf("-skip-scan требует -csv <файл.csv>")
	}
	level, ok := domain.ParseLevel(cfg.minLevel)
	if !ok {
		return fmt.Errorf("неизвестный -min-level %q (info|low|med|high|critical)", cfg.minLevel)
	}

	csvFile, err := timelineCSV(cfg)
	if err != nil {
		return err
	}

	data, err := parser.Parse(csvFile, parser.Options{MinLevel: level})
	if err != nil {
		return err
	}
	if err := enrich(data, cfg); err != nil {
		return err
	}
	if err := render.Write(cfg.out, cfg.title, data); err != nil {
		return err
	}
	reportStats(cfg.out, data)
	return nil
}

// timelineCSV — источник CSV: запуск Hayabusa или готовый файл.
func timelineCSV(cfg config) (string, error) {
	if cfg.skipScan {
		if _, err := os.Stat(cfg.csvPath); err != nil {
			return "", fmt.Errorf("csv недоступен: %w", err)
		}
		return cfg.csvPath, nil
	}
	work := cfg.work
	if work == "" {
		td, err := os.MkdirTemp("", "gotimeline-*")
		if err != nil {
			return "", err
		}
		work = td
	}
	sc := hayabusa.Scanner{Binary: cfg.hayabusaBin, Rules: cfg.rules}
	log.Printf("запуск: %s", sc.Command(cfg.dir))
	return sc.Build(cfg.dir, work)
}

// enrich — маркеры-находки и окно инцидента поверх данных.
func enrich(data *domain.Data, cfg config) error {
	if cfg.factsPath != "" {
		facts, err := parser.LoadFacts(cfg.factsPath)
		if err != nil {
			return err
		}
		data.Facts = facts
	}
	if cfg.incidentFrom != "" && cfg.incidentTo != "" {
		data.Incident = &domain.Incident{From: cfg.incidentFrom, To: cfg.incidentTo}
	}
	return nil
}

func reportStats(out string, data *domain.Data) {
	total := 0
	for _, arr := range data.Hourly {
		total += len(arr)
	}
	fmt.Fprintf(osStdout, "готово: %s\n  хостов: %d\n  часовых точек: %d\n  маркеров: %d\n",
		filepath.Base(out), len(data.Hosts), total, len(data.Facts))
}
