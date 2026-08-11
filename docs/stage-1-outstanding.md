# Этап 1: открытые пункты (v0.1.0)

Этап 1 **реализован в коде** и выпущен как **v0.1.0**. Ниже — всё, что по факту ещё не закрыто: формальная приёмка, прогоны в окружении и намеренно отложенное на этап 2+.

Источник требований: [carrel-spec.md](carrel-spec.md) §21 (критерии приёмки), §19 (граница этапа 1).

---

## Статус

| Область | Статус |
|---------|--------|
| Реализация по плану (шаги 0–14) | готово |
| `go test ./...` | проходит |
| `go test -race ./...` | **не прогонялся** (нужен CGO и C-компилятор) |
| Ручная приёмка в контейнере (A1–A9) | **не пройдена** |
| GitHub Actions / CI | вне scope этапа 1 |

---

## 1. Автоматическая проверка с детектором гонок

Обязательно по §21 и бывшему [stage-1-acceptance.md](stage-1-acceptance.md):

```bash
CGO_ENABLED=1 go test -race ./...
```

Требует установленный C-компилятор (`gcc` на Linux, Xcode CLT на macOS). На Windows без toolchain прогон не выполнялся.

**Закрытие:** один успешный прогон без падений; при обнаружении гонок — исправить код.

---

## 2. Ручная приёмка (Docker)

Сборка с метаданными сборки:

```bash
docker compose build \
  --build-arg VERSION=0.1.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD)
```

### A1. Пустой том → экран создания администратора

```bash
docker volume rm carrel_carrel-data   # если остался от прошлого прогона
docker compose up -d
```

- [ ] `http://127.0.0.1:8080/` перенаправляет на `/setup`
- [ ] Форма создаёт первого администратора и сразу открывает панель
- [ ] Повторное открытие `/setup` ведёт на `/login`, второго администратора завести нельзя
- [ ] `docker compose ps` показывает состояние `healthy` (healthcheck без shell, через `carrel healthcheck`)

### A2. Версия и коммит

- [ ] `/about` открывается без входа
- [ ] Версия и коммит на странице совпадают с аргументами сборки из команды выше
- [ ] Ссылка на исходники ведёт на репозиторий, лицензия названа AGPL-3.0-or-later

### A3. `read_only: true` и непривилегированный пользователь

При работающем контейнере:

```bash
docker compose exec carrel /carrel healthcheck   # distroless: без shell, только бинарь
docker inspect --format '{{.HostConfig.ReadonlyRootfs}} {{.Config.User}}' <container>
```

- [ ] `ReadonlyRootfs` = `true`, пользователь непривилегированный (`nonroot`, uid 65532)
- [ ] Создание администратора и правка настроек проходят при read-only корне: пишется только том `/var/lib/carrel`
- [ ] `docker inspect --format '{{.HostConfig.CapDrop}}'` показывает `[ALL]`
- [ ] Порт слушается только на петле: `curl` с адреса хоста в сети работает, снаружи — нет

### A4. SIGTERM не рвёт активные запросы

```bash
docker compose stop -t 20 carrel
```

- [ ] Запрос, выполнявшийся в момент остановки, завершается штатно, а не обрывом соединения
- [ ] В логах есть запись о завершении, процесс уходит до истечения 15 секунд, если запросов нет
- [ ] После перезапуска все сессии сброшены — вход требуется заново (ключи живут только в памяти)

### A5. Логи не содержат секретов

Добавить в `compose.yaml` на время проверки `CARREL_LOG_LEVEL: debug` рядом с
`CARREL_DATA_DIR` и перезапустить:

```bash
docker compose up -d
docker compose logs carrel > /tmp/carrel.log
```

Пройти вход, создание приглашения, его приём, смену пароля и тестовое письмо, затем:

- [ ] В `/tmp/carrel.log` нет паролей, токенов приглашений, мастер-пароля и содержимого писем
- [ ] Логи структурированные (JSON), идут в stdout

### A6. Диагностика SMTP

- [ ] С заведомо неверным хостом «Send test email» показывает полный ответ (отказ соединения), а не «ошибка»
- [ ] С неверным паролем показан ответ сервера на аутентификацию
- [ ] В выводе диагностики нет пароля реле
- [ ] С верными настройками письмо доходит, в диагностике виден весь диалог

### A7. Письмо о восстановлении

- [ ] Включить депонирование, создать пользователя, восстановить его — пользователю уходит письмо
- [ ] Письмо нельзя отключить из панели
- [ ] При ненастроенном SMTP или отсутствии адреса панель сообщает, что уведомить надо самому

### A8. Приглашение без SMTP (сквозной проход руками)

- [ ] SMTP не настроен: приглашение создаётся, ссылка показана с кнопкой копирования
- [ ] Статус доставки — `not configured`, приглашение при этом действительно
- [ ] По ссылке пользователь задаёт пароль и попадает внутрь
- [ ] Повторный переход по той же ссылке отдаёт «недействительно»

### A9. `THIRD_PARTY.md`

- [ ] Перечислены все зависимости из `go.mod` и вендоренный htmx с его SSE-расширением
- [ ] Для каждой приведён текст лицензии, версии совпадают с `go.mod` и с файлами в `internal/web/static/`

---

## 3. Автотесты: что уже закрыто

Покрыто `go test ./...` (имена тестов для справки):

| Критерий §21 | Тест |
|---|---|
| Админ не видит данных других пользователей | `TestAdminCannotReadAnotherAccountsData` |
| Приглашённый заводит пароль сам | `TestStageOneAcceptanceFlow` |
| Invite без SMTP | `TestStageOneAcceptanceFlow`, `TestInviteWorksWithoutSMTP` |
| Токен одноразовый, в сторе только хэш | `TestStageOneAcceptanceFlow`, `TestInviteStoresOnlyDigest`, `TestHashToken` |
| SMTP-диагностика | `TestSendPassesTheServersRefusalThrough`, `TestSendReportsAnUnreachableRelay` |
| Деструктивный сброс пароля | `TestResetPasswordIsAnnouncedAsDestructive`, `TestResetPasswordReplacesDEK` |
| Смена пароля сохраняет данные | `TestChangePasswordKeepsDEK`, `TestProfilePasswordChange` |
| Disable убивает сессии сразу | `TestDisableEndsActiveSessionsAtOnce` |
| Последний админ защищён | `TestLastAdminSurvivesThePanel`, `TestLastAdminGuard` |
| Нет ложного «forgot password» | `TestForgotOffersNoReset` |
| Escrow выключен по умолчанию | `TestEscrowOffByDefault`, `TestAdminCannotReadAnotherAccountsData` |
| Escrow не ретроактивен | `TestEscrowAppliesOnlyToLaterUsers`, `TestRecoveryRefusedWithoutADeposit` |
| Восстановление только с мастер-паролем | `TestRecoveryNeedsTheMasterPassword`, `TestRecoveryIsThrottled` |
| Восстановление в журнале и письме | `TestRecoveryThroughThePanel`, `TestEscrowActionsAreAudited` |
| Статус escrow в профиле | `TestEscrowCoversNewAccountsAndSaysSo`, `TestForbiddenOptOutIsVisible` |
| `/about` публичная | `TestAboutPublic`, `TestAboutNoSessionRequired` |

---

## 4. Вне scope этапа 1 (этап 2+, не баги)

Следующее **намеренно не реализовано** в v0.1.0:

- DAV transport, discovery, SSRF guard (§6, §24.2)
- `internal/provider/*`, cache, fanout
- Публичная форма self-registration (флаг в настройках есть, UI: «no public form in this stage»)
- Счётчик DAV-аккаунтов в админке всегда `0`
- Заглушка после входа: «No accounts connected yet»
- GitHub Actions / публикация в ghcr.io
- WebDAV files, duplicates, unified view

---

## 5. Критерий закрытия этапа 1

Этап 1 считается **полностью принятым**, когда:

1. `CGO_ENABLED=1 go test -race ./...` проходит без падений.
2. Чеклист A1–A9 отмечен пройденным (можно обновить этот файл, заменив `[ ]` на `[x]`).
3. После этого файл [stage-1-outstanding.md](stage-1-outstanding.md) можно удалить или свести к пустому «всё закрыто».
