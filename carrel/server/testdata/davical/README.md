# DAViCal как стенд живого набора

PHP поверх PostgreSQL, старый и до сих пор стоящий у хостеров и в
организациях. В линейке он ради того, что он **не sabre/dav**: движок свой,
со своим прочтением RFC, в отличие от Baikal, Nextcloud и Davis.
Календари и адресные книги у него в одном доме на принципала,
`/caldav.php/<логин>/`. WebDAV у DAViCal — доступ к тем же коллекциям, не
хранилище файлов, поэтому набора файлов нет.

## Состояние

Первый прогон — 24.09.2026, все семь наборов зелёные. Стенд стоит в
обычном ряду ворот, без флага. Своё поведение DAViCal показал дважды
(`docs/servers.md`):

- SV-50: имя коллекции он экранирует ещё раз, и все клиенты читают `&amp;`;
- SV-51: поиск по книге теряет регистр кириллицы, если среди свойств есть
  EMAIL или TEL, и отвечает пустотой. Carrel перепроверяет пустой ответ
  чтением книги.

## Образ

У проекта своего образа нет. Взят `fintechstudios/davical:1.1.12`: собран
на официальном `php:apache` (PHP 8.2.8, Apache 2.4.57), помечен выпуском
DAViCal — 1.1.12 последний, март 2023. Тег закреплён. У того же автора есть
ночные сборки из основной ветки, но в стенде нужен выпуск: его ставят люди.

База — отдельная служба `davical-db`, PostgreSQL 15. При
`RUN_MIGRATIONS_AT_STARTUP` образ сам создаёт базу и её роли от имени
суперпользователя PostgreSQL, накатывает схему и ставит пароль
администратору DAViCal из `DAVICAL_ADMIN_PASS`. Ручного
`create-database.sh` не нужно.

## Учётная запись: обычный пользователь, через форму админки

Наборы гоняются **не** под администратором DAViCal. У администратора все
права на все коллекции, и набор под ним прошёл бы там, где у обычного
человека всё упадёт. `make-account.sh` входит в админку как `admin` /
`carrel-admin`, открывает «новый принципал» и отправляет форму с её
собственными значениями по умолчанию: человек, активен, не администратор,
права — как их отмечает форма. Принципала и его коллекции по умолчанию
(календарь `calendar`, книгу `addresses`, ящики `.in`/`.out`) создаёт сам
DAViCal. Скрипт идемпотентен: если учётка уже отвечает по DAV, он её не
трогает.

Логин и пароль по умолчанию — `carrel` / `carrel-pass`, те же, что берёт
набор. Переопределяются через `CARREL_DAVICAL_USER` и `CARREL_DAVICAL_PASS`.
Адрес — `CARREL_DAVICAL_URL`, по умолчанию `http://127.0.0.1:8083/caldav.php/`.

```bash
docker compose -f docker-compose.linux.yml up -d davical-db davical
bash testdata/davical/make-account.sh
```

## Откуда взято

- образ, его переменные (`PGHOST`, `PGPASSWORD`, `ROOT_PGUSER`,
  `ROOT_PGPASSWORD`, `DAVICAL_ADMIN_PASS`, `RUN_MIGRATIONS_AT_STARTUP`) и
  пример compose: `github.com/fintechstudios/davical-docker`, README и
  `docker-compose.yml`; теги — Docker Hub `fintechstudios/davical`;
- последний выпуск DAViCal: теги `gitlab.com/davical-project/davical`;
- CardDAV: вики проекта, «the DAViCal CalDAV & CardDAV Server»;
- поля формы входа (`username`, `password`) и нового принципала
  (`xxxxusername`, `newpass1`, `newpass2`, `fullname`, `email`, `type_id`,
  `is_admin`, `user_active`, `default_privileges[…]`, `csrf_token`): сами
  страницы стенда, 24.09.2026.
