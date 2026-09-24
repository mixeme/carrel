# Davis как стенд живого набора

sabre/dav с администраторским интерфейсом на Symfony. Движок тот же, что у
Baikal и Nextcloud, — стенд стоит в линейке **для полноты, а не за
новостями**. Зато дёшев: один контейнер на Linux-машине.

## Состояние

Первый прогон — 24.09.2026, все восемь наборов зелёные: общий набор и
файлы. Стенд стоит в обычном ряду ворот, без флага. Образ —
`ghcr.io/tchapi/davis-standalone:latest` от 06.09.2026, sabre/dav 4.7.1
(«Линейка» в `docs/servers.md`). Своё поведение Davis показал одно —
SV-49: адресная книга без имени даёт 500.

## Учётная запись: через форму админки, скриптом

**У Davis нет способа завести пользователя DAV, кроме админки.** Это
проверено по исходникам:

- API умеет пользователей только **читать**: `GET /api/v1/users`,
  `GET /api/v1/users/:id`. Создание есть у календарей, у пользователей его
  нет.
- Консольных команд три — `api:generate`, `mail:test`,
  `SyncBirthdayCalendars`. Заведения пользователя среди них нет.
- Прямая запись в базу означала бы переписать логику приложения. Пароль
  хранится как `password_hash($p, PASSWORD_DEFAULT)` — **несмотря на имя
  столбца `digesta1`**, это не md5 sabre/dav. А создание пользователя пишет
  **четыре** сущности разом: `User`, `Principal`, календарь по умолчанию с
  экземпляром и адресную книгу.

Поэтому учётку заводит сама форма Davis. `make-account.sh` входит в админку
как `admin` / `carrel-admin`, открывает «новый пользователь» и отправляет
форму вместе с её CSRF-токеном. Все четыре сущности пишет Davis, а не мы.
Скрипт идемпотентен: если учётка уже отвечает по DAV, он её не трогает.
Логин и пароль по умолчанию — `carrel` / `carrel-pass`, те же, что по
умолчанию берёт набор. Переопределяются через `CARREL_DAVIS_USER` и
`CARREL_DAVIS_PASS`. Адрес DAV — `CARREL_DAVIS_URL`, по умолчанию
`http://127.0.0.1:8082/dav/`.

```bash
docker compose -f docker-compose.linux.yml up -d davis
docker compose -f docker-compose.linux.yml run --rm davis-setup
bash testdata/davis/make-account.sh
```

## Почему standalone, а не обычный образ

Обычный `ghcr.io/tchapi/davis` — это **только PHP-FPM**: ему нужен nginx
впереди и общий том с кодом. Образ `davis-standalone` несёт Caddy, и стенд
выходит одной службой. Внутри он слушает 9000, наружу отдан
`127.0.0.1:8082`.

## Разовый шаг `davis-setup`

- **Миграции.** Davis не накатывает их сам. Пока
  `doctrine:migrations:migrate` не прошёл, в базе нет таблиц, и всякий экран
  отвечает 500.
- **Владелец данных — `www-data`.** Миграции идут от root, а PHP-FPM
  работает от `www-data`. Базу, созданную root, PHP может читать, но не
  писать: первая же форма падала с «attempt to write a readonly database».
  Поэтому шаг отдаёт весь `/data` пользователю `www-data`.
- **Каталоги WebDAV.** При `WEBDAV_ENABLED` Davis требует, чтобы каталоги
  tmp и public существовали и были доступны на запись. Без них 500 отвечает
  **всякий** запрос DAV, календарный тоже. Шаг создаёт `/data/webdav/tmp`,
  `/data/webdav/public` и `/data/webdav/homes`. Домашние каталоги по
  умолчанию у Davis выключены; у нас включены, вне public, как советует
  `.env` Davis. Файлы учётки лежат в `/dav/home/<логин>/`.

## Что ещё открыто

- **Версия образа.** Взят `:latest`; закрепить точный тег.
- **`APP_SECRET`** задан фиксированной строкой: это стенд на петле. В
  боевой установке так нельзя.

## Откуда взято

- образ `davis-standalone`, порт 9000, пара nginx+FPM у обычного образа,
  `DATABASE_URL` с четырьмя косыми, требование прав на файл SQLite и
  обязательность миграций: `github.com/tchapi/davis`, README и файлы
  `docker/docker-compose-sqlite.yml`, `docker/docker-compose-standalone.yml`,
  `docker/.env`;
- точка DAV `/dav`, `AUTH_METHOD`, `AUTH_REALM`, флаги CalDAV/CardDAV/WebDAV,
  переменные `WEBDAV_TMP_DIR`, `WEBDAV_PUBLIC_DIR`, `WEBDAV_HOMES_DIR`:
  README и `.env` того же репозитория;
- состав API: `docs/api/README.md` там же;
- хранение пароля и четыре сущности при заведении пользователя:
  `src/Entity/User.php` и `src/Controller/Admin/UserController.php`;
- поля формы нового пользователя (`user[username]`, `user[displayName]`,
  `user[email]`, `user[password][first|second]`, `user[_token]`) и форма
  входа (`_username`, `_password`, `_csrf_token`): сами страницы стенда,
  24.09.2026.
