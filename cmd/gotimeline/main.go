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
	"github.com/81beastie/gotimeline/internal/remoteapps"
	"github.com/81beastie/gotimeline/internal/render"
)

// osStdout — точка подмены в тестах.
var osStdout io.Writer = os.Stdout

func main() {
	os.Exit(mainWithArgs(os.Args[1:]))
}

// mainWithArgs — парсинг флагов и запуск; код возврата для тестов.
// Первый позиционный аргумент (без флага) — каталог с EVTX (аналог -dir).
func mainWithArgs(args []string) int {
	fs := flag.NewFlagSet("gotimeline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cfg config
	registerFlags(fs, &cfg)
	fs.Usage = func() { printUsage(osStdout) }
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		printUsage(osStdout)
		return 2
	}
	if cfg.showVersion {
		fmt.Fprintf(osStdout, "gotimeline %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return 0
	}
	if rest := fs.Args(); len(rest) > 0 {
		cfg.dir = rest[0]
	}
	if err := run(cfg); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}

// printUsage — справка на русском.
func printUsage(w io.Writer) {
	fmt.Fprint(w, `gotimeline `+version+` — интерактивный HTML-таймлайн по EVTX через Hayabusa.

Использование:
  gotimeline [флаги] [каталог с EVTX]     скан каталога (по умолчанию .)
  gotimeline -skip-scan -csv file.csv     по готовому CSV Hayabusa

Флаги:
  -dir каталог        каталог с EVTX (рекурсивно); можно задать позиционным аргументом
  -hayabusa путь      путь к бинарнику Hayabusa (по умолчанию hayabusa из PATH)
  -rules каталог      каталог правил Hayabusa (пусто = встроенные)
  -out файл           итоговый HTML-файл (по умолчанию timeline.html)
  -work каталог       каталог для промежуточного CSV (пусто = temp)
  -min-level уровень  минимальный уровень событий: info|low|med|high|critical
  -facts файл         facts.json с ключевыми маркерами-находками
  -remote-dir каталог бандлы logcollector: коннекты AnyDesk/удалёнки на таймлайн
  -remote-tz часы     часовой пояс машин бандлов (Мск = 3, по умолчанию 3)
  -incident-from T    начало окна инцидента, напр. 2026-01-01T00:00
  -incident-to T      конец окна инцидента
  -title текст        заголовок страницы
  -skip-scan          не запускать Hayabusa, взять готовый CSV (-csv)
  -csv файл           готовый CSV таймлайна Hayabusa
  -version            версия и выход
  -h, -help           эта справка

Примеры:
  gotimeline ~/cases/demo -hayabusa ~/tools/hayabusa -rules ~/tools/rules
  gotimeline -skip-scan -csv timeline.csv -out report.html -title "Кейс 42"
`)
}

type config struct {
	dir, hayabusaBin, rules, out, work string
	minLevel, factsPath                string
	incidentFrom, incidentTo, title    string
	remoteDir                          string
	remoteTZ                           int
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
	fs.StringVar(&cfg.incidentFrom, "incident-from", "", "начало окна инцидента, 2026-01-01T00:00")
	fs.StringVar(&cfg.incidentTo, "incident-to", "", "конец окна инцидента")
	fs.StringVar(&cfg.title, "title", "Интерактивный таймлайн", "заголовок страницы")
	fs.StringVar(&cfg.remoteDir, "remote-dir", "", "каталог с бандлами logcollector (коннекты AnyDesk и прочей удалёнки)")
	fs.IntVar(&cfg.remoteTZ, "remote-tz", 3, "часовой пояс машин в бандлах, часов от UTC (Мск = 3)")
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

// enrich — маркеры-находки, коннекты удалёнки и окно инцидента поверх данных.
func enrich(data *domain.Data, cfg config) error {
	if cfg.factsPath != "" {
		facts, err := parser.LoadFacts(cfg.factsPath)
		if err != nil {
			return err
		}
		data.Facts = facts
	}
	// источники бандлов удалёнки: явный -remote-dir плюс сам сканируемый каталог
	// (если там лежат следы anydesk/прочей удалёнки — подхватываем без лишних ключей)
	remoteDirs := []string{}
	if cfg.remoteDir != "" {
		remoteDirs = append(remoteDirs, cfg.remoteDir)
	}
	if cfg.dir != "" && cfg.dir != cfg.remoteDir {
		remoteDirs = append(remoteDirs, cfg.dir)
	}
	seenRemote := map[string]bool{}
	for _, rd := range remoteDirs {
		if seenRemote[rd] {
			continue
		}
		seenRemote[rd] = true
		facts, err := remoteFacts(rd, cfg.remoteTZ)
		if err != nil {
			return err
		}
		data.Facts = append(data.Facts, facts...)
		ensureHostLanes(data, facts)
	}
	if cfg.incidentFrom != "" && cfg.incidentTo != "" {
		data.Incident = &domain.Incident{From: cfg.incidentFrom, To: cfg.incidentTo}
	}
	return nil
}

// ensureHostLanes — хост бандла может отсутствовать в EVTX-данных:
// без дорожки его факты не отрисуются, поэтому добавляем пустую дорожку.
func ensureHostLanes(data *domain.Data, facts []domain.Fact) {
	known := make(map[string]bool, len(data.Hosts))
	for _, h := range data.Hosts {
		known[h] = true
	}
	for _, f := range facts {
		if !known[f.Host] {
			known[f.Host] = true
			data.Hosts = append(data.Hosts, f.Host)
		}
	}
}

// remoteFacts — коннекты удалёнки из бандлов logcollector.
// Два случая: dir — каталог с бандлами-подкаталогами (каждый = хост)
// или dir — сам бандл (есть manifest.json или следы удалёнки напрямую).
func remoteFacts(dir string, tz int) ([]domain.Fact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // каталога нет — тишина
		}
		return nil, fmt.Errorf("чтение %s: %w", dir, err)
	}

	// сам бандл: manifest.json или следы удалёнки лежат прямо здесь
	if isBundle(entries) {
		f, err := remoteapps.Load(dir, filepath.Base(dir), tz)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	// каталог с бандлами: каждый подкаталог = хост
	var facts []domain.Fact
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		f, err := remoteapps.Load(filepath.Join(dir, e.Name()), e.Name(), tz)
		if err != nil {
			return nil, err
		}
		facts = append(facts, f...)
	}
	return facts, nil
}

// isBundle — каталог сам является бандлом logcollector.
func isBundle(entries []os.DirEntry) bool {
	for _, e := range entries {
		if e.Name() == "manifest.json" {
			return true
		}
		if e.IsDir() && remoteapps.HasTraces(e.Name()) {
			return true
		}
	}
	return false
}

func reportStats(out string, data *domain.Data) {
	total := 0
	for _, arr := range data.Hourly {
		total += len(arr)
	}
	fmt.Fprintf(osStdout, "готово: %s\n  хостов: %d\n  часовых точек: %d\n  маркеров: %d\n",
		filepath.Base(out), len(data.Hosts), total, len(data.Facts))
}
