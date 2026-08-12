# Тестовые учётные данные (локально)

Папка **`dev/`** не отслеживается git (`.gitignore`). Секреты **никогда** не коммитить.

Все файлы ниже лежат в:

```
dev/credentials/
```

Формат — на усмотрение владельца машины (`.env`, отдельные файлы). Рекомендуется **разделять** Baikal (CalDAV/CardDAV) и WebDAV (файлы / вложения §23.10).

---

## Baikal (CalDAV + CardDAV)

Для discovery, контактов, календаря, заметок (этапы 2–5).

| Переменная | Пример | Назначение |
|------------|--------|------------|
| `CARREL_TEST_DAV_URL` | `https://host/dav.php/` | Базовый URL Baikal (прямой путь, §6) |
| `CARREL_TEST_DAV_USER` | `user@example` | Логин |
| `CARREL_TEST_DAV_PASSWORD` | `…` | Пароль |

Опционально:

| Переменная | Назначение |
|------------|------------|
| `CARREL_TEST_DAV_ALLOW_INSECURE` | `1` — только dev с self-signed TLS |

Файл-пример: `dev/credentials/baikal.env`

---

## WebDAV (файлы и вложения)

Отдельный аккаунт для этапа 7: браузер файлов, `ATTACH` на событиях и заметках (§23.10), import `.md` с WebDAV (§23.9).

| Переменная | Пример | Назначение |
|------------|--------|------------|
| `CARREL_TEST_WEBDAV_URL` | `https://host/webdav/` | Корень или каталог file-коллекции |
| `CARREL_TEST_WEBDAV_USER` | `files@example` | Логин |
| `CARREL_TEST_WEBDAV_PASSWORD` | `…` | Пароль |

Опционально:

| Переменная | Назначение |
|------------|------------|
| `CARREL_TEST_WEBDAV_ATTACH_DIR` | Подкаталог для вложений заметок (default attachment folder §23.10) |
| `CARREL_TEST_WEBDAV_ALLOW_INSECURE` | `1` — self-signed TLS в dev |

Файл-пример: `dev/credentials/webdav.env`

Может быть **тот же хост**, что у Baikal, но **другой логин/путь** — file-коллекция не обязана совпадать с principal календаря.

---

## Использование

### Загрузка env (PowerShell)

```powershell
function Import-DotEnv($path) {
  Get-Content $path | ForEach-Object {
    if ($_ -match '^\s*([^#=]+)=(.*)$') {
      Set-Item -Path "env:$($matches[1].Trim())" -Value $matches[2].Trim()
    }
  }
}
Import-DotEnv dev/credentials/baikal.env    # этапы 2–5
Import-DotEnv dev/credentials/webdav.env    # этап 7
```

### Интеграционные тесты

```powershell
go test -tags=integration ./...
```

| Набор переменных | Этап | Если не задано |
|------------------|------|----------------|
| `CARREL_TEST_DAV_*` | 2–5 | `t.Skip` в DAV/integration-тестах |
| `CARREL_TEST_WEBDAV_*` | 7 | `t.Skip` в files/attach-тестах |

Unit-тесты не зависят от живых серверов.

**Ручная приёмка** — те же данные в UI; чеклист: [manual-acceptance.md](manual-acceptance.md) §2 (Baikal), §7 (WebDAV).

**Локальный сервер** — [dev/local-testing.md](../dev/local-testing.md).

---

## Безопасность

- Не логировать пароли, не коммитить `dev/credentials/`.
- Integration-тесты — только на машине разработчика или приватном runner с injected secrets.
