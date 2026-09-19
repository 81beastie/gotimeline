// Package hayabusa — запуск внешнего сканера Hayabusa и получение CSV-таймлайна.
package hayabusa

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Scanner — конфигурация запуска Hayabusa.
type Scanner struct {
	Binary string // путь к бинарнику
	Rules  string // каталог правил (пусто = встроенные)
}

// Build — запуск dfir-timeline по каталогу с EVTX; возвращает путь к CSV.
func (s Scanner) Build(dir, workdir string) (string, error) {
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(workdir, "timeline.csv")

	args := []string{"dfir-timeline", "--no-wizard", "-Q", "-K", "-p", "standard", "-O", "-U",
		"-d", dir, "-o", out}
	if s.Rules != "" {
		args = append(args, "-r", s.Rules)
	}
	cmd := exec.Command(s.Binary, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("hayabusa: %w", err)
	}
	if _, err := os.Stat(out); err != nil {
		return "", fmt.Errorf("hayabusa не создал %s", out)
	}
	return out, nil
}

// Command — строка запуска для логирования.
func (s Scanner) Command(dir string) string {
	return strings.Join([]string{s.Binary, "dfir-timeline", "-d", dir}, " ")
}
