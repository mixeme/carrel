# Ручная приёмка v1

Осталось незакрытое. Автоматические проверки — [tests.md](tests.md). Критерии — [carrel-spec.md](carrel-spec.md) §21, §23.6–§23.10. Учётные данные: `dev/credentials/` ([dev-credentials.md](dev-credentials.md)).

**v1** закрывается, когда отмечены **P1, P2 и P5** (P3 и P4 уже пройдены). Остальное — по возможности. После v1 — [roadmap.md](roadmap.md).

## Блокируют v1

- [ ] **P1** — заметка Carrel ↔ **jtx Board** (создание, правка, `X-`поля на месте после обеих правок). §23.9
- [ ] **P2** — заметка Carrel ↔ **Evolution** (чтение и правка)
- [ ] **P5** — вложение скриншота в заметку: вставка за пару секунд после настройки каталога. §23.10

## Каркас

- [ ] **A9** — в `THIRD_PARTY.md` текст лицензии каждой зависимости; версии совпадают с `go.mod` и `internal/web/static/`

## Транспорт и discovery

- [ ] Radicale или Nextcloud — URL с `.well-known` и без
- [ ] Переподключение после `docker compose restart`
- [ ] Смена пароля пользователем сохраняет аккаунты; сброс админом делает их недоступными

## Контакты

- [ ] Export `.vcf` пачкой
- [ ] Фото: EXIF orientation 6 на реальном снимке; GPS не остаётся; DAVx5 / iOS после sync
- [ ] Книга 500+ контактов — приемлемая скорость списка
- [ ] Печать: список и карточка, ч/б, колонтитул, разрывы не внутри записи

## Календарь

- [ ] Import/export `.ics`: TZ, all-day
- [ ] Агенда за неделю: читаемость, TZ
- [ ] RRULE: правка серии — сверка с DAVx5 / Apple Calendar
- [ ] Печать агенды: разрывы страниц, колонтитул с диапазоном

## Задачи, заметки, единый вид, поиск

- [ ] Единый вид: 3 календаря из 2 аккаунтов, метки источника на каждой строке
- [ ] Fanout: прогресс «N из M»; SSE через **Apache** с конфигом из [README](../README.md) (§21)
- [ ] Fallback poll на нестабильной сети (переключение вышек)
- [ ] Markdown import 50+ файлов — предпросмотр и отчёт
- [ ] Заметка к прошедшему событию из агенды
- [ ] «New note» с телефона за ~2 секунды

## Дубликаты

- [ ] Ложные срабатывания и пропуски на реальной книге — это суждение, а не утверждение
- [ ] Экран «Дубликаты»: раскрытие групп, сравнение полей
- [ ] Объединение на сервере: частичный отказ DELETE описан честно, ничего не откатывается

## WebDAV и вложения

`CARREL_TEST_WEBDAV_*`, отдельно от Baikal.

- [ ] Скачивание 10+ MB без роста памяти процесса
- [ ] Nextcloud против Baikal WebDAV — оба листаются и отдают файлы
- [ ] `ATTACH` URI открывается (или честно не открывается) в jtx / Thunderbird — §23.10 заранее называет это свойством подхода
- [ ] Unicode в именах файлов и каталогов
- [ ] Detach не удаляет файл на сервере
- [ ] Сквозной проход: Baikal (заметка) + отдельный WebDAV (файл) — единственное, что не покрыто ни handler-, ни integration-тестами

## Desktop (после сборки `carrel-desktop`)

План: [plans/desktop-wrapper.md](plans/desktop-wrapper.md).

- [ ] **P-desktop-1** — Remote: первый запуск, URL, login, навигация
- [ ] **P-desktop-2** — Local: lazy download sidecar, setup/login, данные в профиле OS-user
- [ ] **P-desktop-3** — Tray on: close → фон; tray off: close → sidecar остановлен
- [ ] **P-desktop-4** — Sign out → onboarding → другой режим (Remote ↔ Local)
- [ ] **P-desktop-5** — Fan-out на Win и Linux: SSE или fallback polling
- [ ] Admin install + server checkbox; user install; два OS-users на одной машине

## Перед приёмкой

```bash
go test ./...
CGO_ENABLED=1 go test -race ./...
go test -tags=integration ./...   # dev/credentials: baikal.env + webdav.env
```

1. Отметить **P1, P2, P5** и пройти остальное по возможности.
2. Тег v1, `CHANGELOG.md`.
3. Оставшееся после v1 — в [roadmap.md](roadmap.md).
