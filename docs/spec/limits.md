# Лимиты Carrel

Сводная таблица всех числовых ограничений проекта: тайм-ауты, сроки жизни,
размеры, количества, частота запросов. Источники правды — код и спека; этот
файл — справочник для людей.

**Обновлять при изменении** `carrel/server/internal/conf/limits.go`,
`carrel/server/internal/conf/conf.go` (блок `defaults()`), жёстко зашитых
констант в пакетах и таблицы «Сроки и лимиты» в
[architecture.md](../plan/architecture.md).

## Как устроена конфигурация

| Уровень | Где | Как менять |
|---|---|---|
| **Продуктовые** | `internal/conf/limits.go` | Только изменением кода. Не `CARREL_*`, не экран «Лимиты» |
| **C-12** | `internal/conf/conf.go`, `{CARREL_DATA_DIR}/settings.env`, env `CARREL_*` | Файл настроек; env перекрывает файл. Полная таблица — [carrel-common.md](carrel-common.md) § C-12 |
| **Зашитые** | HTTP-транспорт, крипто, хранилище, rate limiter и т.д. | Только код |
| **Запланированные** | [architecture.md](../plan/architecture.md), [server.md](../plan/server.md) | Ещё не реализованы или не подключены |

**Легенда статуса:** ✅ работает в коде · ⚠️ объявлено, но обработчик ещё не использует · 📋 только в спеке/плане

---

## 1. Сроки жизни и тайм-ауты

### 1.1. Сессии и секретные ссылки (продуктовые)

| Что | Значение | Константа / место | Статус |
|---|---|---|---|
| Простой сессии | 12 ч | `SessionIdleTimeout` | ✅ |
| Абсолютный срок сессии | 7 суток | `SessionAbsoluteTimeout` | ✅ |
| Приглашение | 7 суток | `InviteTTL` | ✅ |
| Подтверждение адреса | 48 ч | `EmailConfirmTTL` | ✅ |
| Ссылка `/forgot` | 24 ч | `ForgotLinkTTL` | ✅ |
| Ссылка из консоли (wipe) | 15 мин, одноразовая | `ConsoleWipeLinkTTL` | ✅ |
| Скачивание профиля устройства | 5 мин, одноразовая | `DeviceProfileLinkTTL` | ⚠️ |
| Публичная ссылка по умолчанию | 30 суток | `PublicLinkDefaultTTL` | ⚠️ |
| Окно выдачи секретов (unlock) | 300 с (5 мин) | `CARREL_UNLOCK_TTL` | ✅ |

Простой сбрасывается любым запросом с живой SID. Абсолютный срок — с момента
входа (смены SID), не продлевается. Что наступит раньше — то и конец сессии.

### 1.2. C-12: DAV, кэш, прогресс

| Переменная | Умолчание | Назначение |
|---|---|---|
| `CARREL_DAV_CONNECT_TIMEOUT` | 10 с | Подключение к удалённому DAV |
| `CARREL_DAV_REQUEST_TIMEOUT` | 30 с | Запрос целиком (и `http.Client.Timeout`) |
| `CARREL_CACHE_COLLECTION_TTL` | 60 с | До перечитывания коллекции из кэша сессии |
| `CARREL_PROGRESS_SOURCE_TIMEOUT` | 10 с | Один источник в опросе/веере |
| `CARREL_PROGRESS_TOTAL_TIMEOUT` | 30 с | Весь опрос; потолок graceful shutdown |
| `CARREL_PROGRESS_POLL_MILLIS` | 700 мс | Интервал запасного poll (режим `poll`) |
| `CARREL_PROGRESS_MODE` | `sse` | `poll` — без SSE (для буферизующих прокси) |

### 1.3. HTTP-сервер и фоновые задачи

| Что | Значение | Место |
|---|---|---|
| Read header timeout | 20 с | `cmd/carrel/main.go` |
| Интервал purge сессий | 1 мин | `cmd/carrel/main.go` (`runSessionPurge`) |

### 1.4. Исходящий DAV HTTP-транспорт (зашито)

| Параметр | Значение | Место |
|---|---|---|
| Dial timeout | = `CARREL_DAV_CONNECT_TIMEOUT` | `internal/dav/guard.go` |
| Request timeout | = `CARREL_DAV_REQUEST_TIMEOUT` | `internal/dav/guard.go` |
| Idle connection timeout | 90 с | `internal/dav/guard.go` |
| TLS handshake timeout | 10 с | `internal/dav/guard.go` |
| Expect-Continue timeout | 1 с | `internal/dav/guard.go` |
| Минимальная версия TLS | TLS 1.2 | `internal/dav/guard.go`, `internal/dav/tls.go` |

### 1.5. Задержки безопасности (минимальное время ответа)

| Сценарий | Задержка | Место |
|---|---|---|
| Токен не найден | 200 мс | `internal/web/authz.go` |
| Неверный вход | 300 мс | `internal/web/handlers_public.go` |
| Неверная смена пароля | 300 мс | `internal/web/handlers_app.go` |
| Неверный unlock | 300 мс | `internal/web/handlers_app.go` |
| Forgot (равномерный ответ) | 300 мс | `internal/web/handlers_public.go` |

### 1.6. Запланировано (davloom)

| Что | Значение | Статус |
|---|---|---|
| Неуспех Basic auth, по учётке | 5 / мин | 📋 |
| Неуспех Basic auth, по IP | 20 / мин | 📋 |
| Успешный DAV, по устройству | 120 / мин | 📋 |

Отдельные вёдра, не веб. GET `/` и OPTIONS `/` — без лимита.

---

## 2. Частота запросов (rate limit)

Token bucket в памяти процесса (`internal/web/ratelimit.go`). При превышении —
429 без различимых тел.

### 2.1. Продуктовые пороги

| Ведро | Лимит | Константа | Маршруты |
|---|---|---|---|
| Вход / регистрация, учётка | 5 / мин | `LoginRatePerAccount` | POST входа, POST саморегистрации |
| Вход / регистрация, IP | 20 / мин | `LoginRatePerIP` | те же |
| Анонимные токены, IP | 30 / мин | `AnonTokenRatePerIP` | `/invite/{token}`, `/forgot`, подтверждение адреса, `/p/{token}`, … |
| После входа, сессия | 120 / мин | `SessionRateLimit` | все запросы с живой SID |

**Без вёдер:** `/healthz`, «О сервисе».

### 2.2. Внутренние параметры rate limiter

| Что | Значение | Место |
|---|---|---|
| Порог sweep карты вёдер | 4096 записей | `sweepAbove` |
| Минимальный интервал sweep | 1 с | `sweepInterval` |

---

## 3. Размеры (байты, пиксели, символы)

### 3.1. Продуктовые

| Что | Значение | Константа | Статус |
|---|---|---|---|
| Тело обычного POST (формы, JSON, CSRF) | 1 MiB | `MaxFormBodyBytes` | ✅ |
| Тело POST `photo-preview` (загрузка фото) | `CARREL_PHOTO_MAX_UPLOAD_BYTES` + 1 MiB | `bodyLimit` | ✅ |
| Ближайший кадр фото контакта | 400 % наибольшего квадрата | `PhotoMaxZoom` | ✅ |

Фото и импорт — отдельные C-12. Файлы на чужой WebDAV — **без потолка Carrel**
(C-12). Сверх 1 MiB — 413, тело дальше не читается.

### 3.2. C-12

| Переменная | Умолчание | Человекочитаемо | Назначение |
|---|---|---|---|
| `CARREL_DAV_MAX_RESPONSE_BYTES` | 10 485 760 | 10 MiB | Потолок одного DAV-ответа |
| `CARREL_CACHE_MAX_BODY_BYTES` | 67 108 864 | 64 MiB | Тела объектов на сессию |
| `CARREL_CACHE_MAX_PROCESS_BYTES` | 268 435 456 | 256 MiB | Общий кэш процесса |
| `CARREL_CACHE_MAX_THUMB_BYTES` | 16 777 216 | 16 MiB | Миниатюры на сессию |
| `CARREL_PHOTO_MAX_UPLOAD_BYTES` | 67 108 864 | 64 MiB | Загрузка фото контакта |
| `CARREL_PHOTO_MAX_PIXELS` | 100 000 000 | 100 Mpx | Отказ до декодирования |
| `CARREL_PHOTO_MAX_SIDE` | 512 px | — | Сторона после обрезки |
| `CARREL_PHOTO_THUMB_SIDE` | 96 px | — | Миниатюра в списках |
| `CARREL_PHOTO_JPEG_QUALITY` | 85 | — | Качество пересжатия |
| `CARREL_IMPORT_MAX_BYTES` | 16 777 216 | 16 MiB | Файл импорта |
| `CARREL_MAIL_MAX_ATTACH_BYTES` | 10 485 760 | 10 MiB | Одно вложение |
| `CARREL_MAIL_MAX_MESSAGE_BYTES` | 26 214 400 | 25 MiB | Письмо целиком |

### 3.3. Запланированные (не C-12)

| Что | Значение | Константа (план) | Статус |
|---|---|---|---|
| Inline-вложение | 2 MiB | `AttachInlineMax` | 📋 |
| Миниатюра из оригинала файла | 512 KiB | `FileThumbMaxBytes` | 📋 |

Коллекция может только **уменьшить** порог через `max-resource-size` (RFC 4791).

### 3.4. DAV и ошибки (зашито)

| Что | Значение | Место |
|---|---|---|
| Чтение тела HTTP-ошибки удалённого сервера | 256 байт | `internal/dav/status.go` |
| Санитизация текста ошибки | 256 символов | `internal/dav/guard.go` |

---

## 4. Количества и ёмкости

### 4.1. Продуктовые

| Что | Значение | Константа | Статус |
|---|---|---|---|
| Живых вееров на сессию | 4 | `MaxLiveFanouts` | ✅ |
| Параллельность источников в веере | 4 | `FanoutParallelism` | ⚠️ |
| Мин. длина пароля человека | 8 символов | `MinPasswordLen` | ✅ |
| Мин. длина мастер-пароля (escrow) | 16 символов | `MinMasterPasswordLen` | ✅ |

### 4.2. C-12

| Переменная | Умолчание | Назначение |
|---|---|---|
| `CARREL_DAV_MAX_REDIRECTS` | 5 | Цепочка редиректов DAV |
| `CARREL_CACHE_MAX_COLLECTIONS` | 256 | Коллекций в кэше сессии |
| `CARREL_CACHE_MAX_ETAG_ENTRIES` | 4096 | ETag-записей в кэше сессии |
| `CARREL_CACHE_MAX_THUMB_ENTRIES` | 512 | Миниатюр в кэше сессии |
| `CARREL_IMPORT_MAX_RECORDS` | 5000 | Записей в одном импорте |
| `CARREL_FILES_MAX_ENTRIES` | 2000 | Членов папки в листинге |
| `CARREL_MAIL_MAX_RECIPIENTS` | 20 | Получателей в письме |
| `CARREL_MAIL_MAX_PER_DAY` | 50 | Писем на человека в сутки |
| `CARREL_DUPLICATES_THRESHOLD` | 60 | Очков для кандидата-дубликата |

### 4.3. Сессия, хранилище, сеть (зашито)

| Что | Значение | Место |
|---|---|---|
| Черновики между запросами | 3 слота | `internal/session/drafts.go` |
| История поисковых запросов | 10 записей | `internal/session/session.go` |
| Записей в audit log | 10 000 | `internal/store/audit.go` |
| Макс. длина identity в audit | 320 символов | `internal/store/audit.go` |
| Max idle DAV connections | 10 | `internal/dav/guard.go` |
| Суффикс entity ID | 10 Crockford-символов | `internal/user/id.go` |
| Версия формата state.enc | 1 | `internal/store/format.go` |

### 4.4. Дубликаты — веса сигналов (📋 в коде scoring ещё нет)

| Сигнал | Очки |
|---|---|
| Email (нормализованный) | 50 |
| Телефон (последние 10 цифр) | 50 |
| UID | 80 |
| Имя (fold-equal) | 20 |
| День рождения (бонус) | +10 |
| **Порог** | **60** (`CARREL_DUPLICATES_THRESHOLD`) |

---

## 5. Криптография и KDF

### 5.1. Размеры ключей и энтропии

| Что | Значение | Место |
|---|---|---|
| AEAD-ключ | 32 байта | `crypt.KeyLen` |
| Nonce XChaCha20 | 24 байта | `crypt.NonceLen` |
| Соль KDF | 16 байт | `crypt.SaltLen` |
| `server.key` | 32 байта | `crypt/serverkey.go` |
| Escrow private scalar | 32 байта | `crypt/escrow.go` |
| Секрет invite/forgot/wipe | 32 байта | `internal/user/tokens.go` |
| CSRF (анонимный) | 32 байта энтропии | `internal/web/csrf_anon.go` |
| CSRF (сессия) | 32 байта энтропии | `internal/session/session.go` |
| Session ID | 24 байта энтропии | `internal/session/session.go` |

### 5.2. Argon2id (профили по умолчанию)

| Профиль | t | m | p | KeyLen |
|---|---|---|---|---|
| Login | 3 | 64 MiB | 1 | 32 |
| Wrap | 3 | 64 MiB | 1 | 32 |
| Master (escrow) | 4 | 256 MiB | 1 | 32 |

Константы: `LoginKDF`, `WrapKDF`, `MasterKDF` в `internal/crypt/crypt.go`.

### 5.3. Допустимые границы KDF (`ValidateKDFParams`)

| Параметр | Мин | Макс |
|---|---|---|
| Memory | 8 MiB | 1 GiB |
| Time, threads | 1 | — |
| KeyLen | 32 (строго) | 32 |

### 5.4. Параллелизм KDF

| Что | Значение | Место |
|---|---|---|
| Одновременных Argon2id в процессе | 4 | `internal/crypt/kdf.go` |

Сверх порога — ожидание с контекстом запроса, не отказ.

---

## 6. Порты и развёртывание

| Что | Умолчание | Переменная / место |
|---|---|---|
| Порт веб-сервера | 8080 | `CARREL_PORT` (диапазон 1–65535) |
| Каталог данных | `/var/lib/carrel` | `CARREL_DATA_DIR` |
| davloom | выключен (0) | `CARREL_DAVLOOM_PORT` (0–65535) |
| Docker: bind | `0.0.0.0` | `Dockerfile` ENV |
| Docker: data dir | `/data` | `Dockerfile` ENV |

---

## 7. Typecase (инструменты разработки)

Не продакшен-лимиты сервера, но зашиты в репозитории:

| Что | Значение | Место |
|---|---|---|
| Ожидание port-файла Chrome | 30 с | `typecase/browser/chrome.go` |
| Тайм-аут browser-теста | 90 с | `typecase/browser/harness.go` |
| Паузы layout settle | 50–80 мс | `typecase/browser/check.go` |
| Допустимых TODO в invariant gate | 0 | `typecase/gate/markup_invariants.go` |

---

## 8. Явно без лимита

| Что | Пояснение |
|---|---|
| Размер файла на чужой WebDAV | Поток идёт на/с удалённого сервера; ограничивает он или обратный прокси (C-12) |
| Число устройств davloom | architecture.md |
| Число учётных записей | — |

---

## 9. Маршруты конфигурации

```
defaults()  →  {CARREL_DATA_DIR}/settings.env  →  env CARREL_*  →  validate()
```

Файл настроек: `{CARREL_DATA_DIR}/settings.env` (`0600` рекомендуется).
Полный перечень переменных C-12 с колонкой «Показ» — [carrel-common.md](carrel-common.md) § C-12.
