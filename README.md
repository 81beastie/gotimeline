# gotimeline

Интерактивный HTML-таймлайн событий Windows по каталогу с EVTX-журналами.
Один бинарник на Go: сам запускает [Hayabusa](https://github.com/Yamato-Security/hayabusa), агрегирует события и выдаёт **автономный HTML-файл** — его можно открыть двойным кликом, положить на флешку или опубликовать на любом статическом хостинге (nginx, GitHub Pages) и обсуждать с коллегами.

```
каталог с EVTX ──► Hayabusa dfir-timeline ──► агрегация по часам/хостам ──► timeline.html
```

## Возможности

- **Скан каталога** с EVTX (рекурсивно, симлинки тоже) с предварительным запуском Hayabusa;
- **Дорожки по хостам** — по одному ряду на каждое имя компьютера из журналов;
- **Почасовые столбики плотности событий** с настраиваемым порогом «всплеска»;
- **Клик по столбику** — топ правил Hayabusa за час + до 12 исходных записей журнала (время, канал, EventID, Details);
- **Маркеры-находки** — ваши ключевые события расследования поверх таймлайна (`facts.json`);
- **Окно инцидента** — вертикальная зона на всём периоде;
- **Масштабирование**: колесо мыши (зум к курсору), протянуть мышью (выделить интервал), двойной клик (весь период), горизонтальная прокрутка (Shift+колесо / полоса);
- **Фильтры**: скрыть хост или тип находки кликом по легенде;
- **Ноль зависимостей на фронтенде** — чистый HTML/JS/CSS в одном файле, без CDN и сервера.

## Установка

Требуется Go 1.27+. Для скана каталогов — установленный [Hayabusa](https://github.com/Yamato-Security/hayabusa) (v4+); для режима `-skip-scan` по готовому CSV Hayabusa не нужен.

```bash
go install github.com/81beastie/gotimeline/cmd/gotimeline@latest
```

Бинарник кладётся в `$(go env GOBIN)` или `$(go env GOPATH)/bin` (обычно `~/go/bin`).
Если бинарник не находится в shell — добавь каталог в `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Обновление и проверка версии

```bash
go install github.com/81beastie/gotimeline/cmd/gotimeline@latest
gotimeline -version
```

Команды `go update` в Go не существует. Проверить установленную версию: `go version -m $(which gotimeline)`.
Удаление: `rm $(which gotimeline)`.

### Сборка из репозитория

```bash
git clone https://github.com/81beastie/gotimeline
cd gotimeline && go build -o gotimeline ./cmd/gotimeline
```

### Релизы

Каждый merge в `main` автоматически публикует patch-релиз (CI-воркфлоу
`.github/workflows/release.yml`: тесты → кросс-сборка → следующий semver-тег).
Поэтому `@latest` всегда соответствует актуальному `main` — ручное тегирование
не нужно. Minor/major теги (`v0.3.0`, `v1.0.0`) ставятся вручную при смене
API или флагов.

Версия в `gotimeline -version` берётся из build info (тега), а не из кода, поэтому
врать она не может: `go install @latest` покажет номер установленного релиза,
локальная сборка — `dev`.

Чтобы пропустить публикацию релиза для конкретного merge, добавь
`[skip release]` в сообщение коммита.

## Использование

### Полный цикл: скан каталога с EVTX

```bash
./gotimeline \
  -dir /cases/incident01 \
  -hayabusa ~/tools/hayabusa \
  -rules ~/tools/rules \
  -min-level info \
  -facts facts.json \
  -incident-from "2026-01-01T00:00" -incident-to "2026-01-07T23:59" \
  -out timeline.html \
  -title "Инцидент 01: весь парк"
```

### По готовому CSV Hayabusa (без пересканирования)

```bash
./gotimeline -skip-scan -csv timeline_full_sorted.csv -out timeline.html
```

### Флаги

| Флаг | По умолчанию | Описание |
|---|---|---|
| `-dir` | `.` | каталог с EVTX (рекурсивно) |
| `-hayabusa` | `hayabusa` | путь к бинарнику Hayabusa |
| `-rules` | — | каталог правил Hayabusa (пусто = встроенные) |
| `-out` | `timeline.html` | итоговый HTML-файл |
| `-work` | temp | каталог для промежуточного CSV |
| `-min-level` | `info` | минимальный уровень событий: `info` \| `low` \| `med` \| `high` \| `critical` |
| `-facts` | — | JSON с маркерами-находками |
| `-incident-from`, `-incident-to` | — | границы окна инцидента, `2026-01-01T00:00` |
| `-title` | — | заголовок страницы |
| `-skip-scan` + `-csv` | — | не запускать Hayabusa, взять готовый CSV |
| `-version` | — | версия (из тега релиза, локальная сборка — `dev`) |

### facts.json — маркеры-находки

```json
[
  {
    "t": "2026-01-01T09:29",
    "host": "HOST1",
    "kind": "av-off",
    "label": "Антивирус переведён в паузу",
    "details": "Пауза защиты за 3 минуты до установки нелицензионного ПО."
  }
]
```

Доступные `kind` (цвета в легенде): `av-off`, `remote`, `auth`, `admin`, `msi`, `reboot`, `journal`, `mesh`, `anon`, `anom`, `rdp`. Незнакомый `kind` рисуется красным с собственным именем — можно задавать свои категории.

Времена — **UTC** в формате ISO (`2026-01-01T09:29`).

## Как читать таймлайн

- **Столбики** — количество событий за час (высота ∝ √N, всплески ≥ порога подсвечены). Клик — что именно происходило в этот час;
- **Круглые маркеры** — маркеры из `facts.json`. Клик — полное описание;
- **Красная зона** — окно инцидента;
- Горячие клавиши мыши: колесо — зум, протянуть — интервал, двойной клик — сброс, Shift+колесо — прокрутка.

## Архитектура

Clean Architecture, слои разделены, без внешних зависимостей:

```
cmd/gotimeline/          CLI-оркестрация: флаги → run()
internal/domain/         модели: Fact, HourPoint, Data, Level
internal/hayabusa/       запуск внешнего сканера (dfir-timeline)
internal/parser/         агрегация CSV → domain.Data, LoadFacts
internal/render/         встраивание данных в шаблон (go:embed)
```

- `parser` не знает про Hayabusa, `render` не знает про CSV — общение только через модели `domain`;
- Итоговый HTML автономен: JSON встроен прямо в `<script>` (с HTML-safe экранированием), поэтому открывается по `file://` без CORS-проблем и без сервера.

## Тесты

```bash
go test ./... -cover
```

| Пакет | Покрытие |
|---|---|
| `internal/domain` | 100% |
| `internal/parser` | 94% |
| `internal/render` | 77% |

## Лицензия

[MIT](LICENSE) — [81beastie](https://github.com/81beastie)

## AI-assistance

Этот проект разрабатывался в паре с AI-ассистентом [Koda](https://kodacode.ru) (команда NLP-Core-Team).

- значительная часть кода написана в диалоге с Koda
- все фичи прошли цикл TDD: сначала падающие тесты, затем реализация
- архитектурные решения (слои Clean Architecture, встраивание JSON в автономный HTML вместо fetch, HTML-safe сериализация) выведены через эксперименты и обсуждение с Koda

Автор проекта ревьюил и принимает ответственность за весь код.
