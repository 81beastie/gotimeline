// Package render — встраивание данных в HTML-шаблон (итог автономен, без сервера).
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/81beastie/gotimeline/internal/domain"
)

//go:embed template.html
var templateFS embed.FS

const (
	dataPlaceholder  = "/*__DATA__*/null"
	titlePlaceholder = "__TITLE__"
)

// Write — сериализует Data и встраивает в шаблон; пишет итоговый файл.
func Write(path, title string, data *domain.Data) error {
	tpl, err := templateFS.ReadFile("template.html")
	if err != nil {
		return err
	}
	if !bytes.Contains(tpl, []byte(dataPlaceholder)) {
		return errors.New("шаблон не содержит плейсхолдер данных")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	html := strings.Replace(string(tpl), dataPlaceholder, string(payload), 1)
	html = strings.ReplaceAll(html, titlePlaceholder, escapeHTML(title))
	return os.WriteFile(path, []byte(html), 0o644)
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
