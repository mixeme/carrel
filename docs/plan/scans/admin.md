# Скан пробелов контракта: U-6.13 «Администрирование»

Скан только чтением, 09.10.2026. Кадры — Typecase `0fe0244`
(`docs/visual/carrel-ui-mockups.html`), сервер — v0.2.174 (`6a62d8a`).
Просмотрены семь маршрутов раздела (`/admin/`, `/admin/invites`,
`/admin/settings`, `/admin/escrow`, `/admin/audit`, `/admin/dav`,
`/admin/install`) и лист `/admin/reset`; 23 кадра `s-admin-1 … -23`; семь
фикстур `docs/spec/fixtures/admin-*/normal.json` (других состояний у раздела
нет). Кадров `states-*` у раздела нет, строк `STATE_FRAME` с `admin-` в
`check.py` нет.

Метод: у полных кадров (`s-admin-1 … -7`) — черновик `go tool frame2go -self`
против фикстуры и структуры `carrel/server/view/settings.go`; у фрагментов
(`s-admin-8 … -23`) — HTML кадра против фикстуры, а где фикстуры нет — против
структуры и обработчика (`internal/web/handlers_admin.go`,
`admin_settings.go`, `handlers_escrow.go`, `install.go`, `davcheck.go`,
`rails_settings.go`, `rail_status.go`). Слова сервера, которых нет в
фикстурах, взяты из кода обработчиков — это помечено «по коду». Черновики и
выжимки — в `scan/admin-work/`.

## Экраны, маршруты, кадры, фикстуры

| Экран | Маршрут | Вью-модель | Кадры | Фикстуры |
|---|---|---|---|---|
| Users | `/admin/` | `AdminUsersPage` | `s-admin-2` (канон), `-12` меню строки, `-21` Columns, `-22` свои столбцы | `admin-users/normal` — один человек (mix, последний администратор) |
| Reset a password | `/admin/reset` | `PasswordResetSheet` | `s-admin-13` лист, `-14` готово, `-15` Whose, `-16` короткий пароль, `-20` не тот логин | **нет ни одной**; каталога и строки `FIXTURE_SCREEN` нет |
| Add users | `/admin/invites` | `AdminInvitesPage` | `s-admin-1` (канон), `-8` Role, `-11` меню приглашения, `-17` заведён, `-18` без почты, `-19` ошибки полей | `admin-invites/normal` — почта есть, регистрация закрыта, приглашений нет |
| Settings | `/admin/settings`, `/admin/settings/mail-test` | `AdminSettingsPage` | `s-admin-3` (канон), `-9` Encryption | `admin-settings/normal` — почта настроена, пробы не было |
| Key escrow | `/admin/escrow` | `AdminEscrowPage` | `s-admin-6` | `admin-escrow/normal` — включено, одна учётная запись, не депонирована |
| Audit log | `/admin/audit` | `AdminAuditPage` | `s-admin-7` (канон), `-10` Action, `-23` Columns | `admin-audit/normal` — 22 строки, фильтра нет |
| DAV validator | `/admin/dav` | `AdminDavPage` | `s-admin-5` | `admin-dav/normal` — до проверки (`trace` и `full` пусты) |
| Installation check | `/admin/install` | `AdminInstallPage` | `s-admin-4` | `admin-install/normal` — до проверки (`rows` пусты, `publicURL` пуст) |

**Состояния с кадром, но без фикстуры:** весь лист сброса (`s-admin-13`,
`-14`, `-16`, `-20`); «пользователь заведён» (`s-admin-17`); «почта не
настроена» (`s-admin-18`); «ошибки полей» (`s-admin-19`); «свои столбцы»
(`s-admin-22`); отключённый человек и не последний администратор
(`s-admin-2`, `-12`); строки приглашений, ссылка в карточке и открытая
регистрация (`s-admin-1`); почта не настроена на Settings (`s-admin-3`);
след обнаружения (`s-admin-5`); строки проверки установки (`s-admin-4`).

**Фикстуры и поля модели без кадра:** `admin-install/normal` и
`admin-dav/normal` — экраны до проверки, кадры рисуют их после неё;
`AdminSettingsPage.Test` (итог пробного письма); `AdminEscrowPage.Result`
(итог восстановления) и состояние «схема не включена» (`Enabled` false,
Turn on key escrow); `AdminDavPage.Full` и согласие на второй этап;
диалог удаления учётной записи.

## Пробелы — серверу

### А. Оболочка и рейл администрирования (все семь кадров)

- **Ад1.** Значки строк рейла. Кадры: `plus` Add users, `con` Users, `adm`
  Settings, `shield` Installation, `search` DAV validator, `dup` Key escrow,
  `clock` Audit log. Модель: `Rail.Nav` несёт `icon` только у «Back to app»
  (`i-back`); у семи строк раздела `icon` нет (`adminRail`), хотя у рейла
  приложения значки приходят (`app-contacts/normal`: `i-con`, `i-cal`, …).
  Нужен `Icon` у строк `admin.*`.
- **Ад2.** Знак davloom в подвале. Кадры: в подвале рейла две строки
  состояния и «Carrel · 0.0.1 · 786949a», знака davloom нет ни в одном из
  семи. Модель: `Rail.Davloom` — «davloom · running · 0 devices» во всех
  семи фикстурах (`adminRail` ставит `s.davloomMark`). Нужно: у рейла
  администрирования `Davloom` пуст.
- **Ад3.** Дни и время числами. Кадры пишут моноширинно «2026-02-11»,
  «2026-08-15 09:20» (`s-admin-2`, `-22`), «2026-08-18 12:00» (`s-admin-1`,
  Expires), «2026-08-15 09:20» (`s-admin-7`, When). Модель:
  `AdminUserRow.Created` — «11 February 2026», `LastLogin` — «10 August
  2026, 17:04», `AuditRow.When` — «10 August 2026, 17:04»;
  `InviteRow.Expires` строится тем же `formatWhen` (по коду, строк в
  фикстуре нет). Рядом `EscrowCoverRow` уже числами (`dayKey`). Нужно: те же
  числа у людей, приглашений и журнала.
- **Ад4 (данные).** Счёт и состояние рейла: кадры — Add users «2», Users
  «6», «SMTP not configured» / «No mail is going out.» (`s-admin-2`, `-3`,
  `-5`, `-7`) или «Mail is going out» / «Last sent 2026-08-16 09:12»
  (`s-admin-1`, `-4`, `-6`), «Checks passed» / «Last check 2026-08-16
  09:12». Фикстуры: Add users 0, Users 1, «Mail is going out» / «No letter
  since the start.», «Not checked since the start» / «Run the installation
  check.». Слова кадров в `adminStatus` есть (по коду) — расходятся только
  данные набора; строки «Last sent ‹время›» и «Checks passed» ни одна
  фикстура не несёт.

### Б. Users (`AdminUsersPage`, `s-admin-2`, `-12`, `-21`, `-22`)

- **Ад5.** Подзаголовок. Кадр: «6 accounts · 2 administrators · 4 covered by
  escrow». Модель: поля нет (у `AdminUsersPage` только `Bar`, `Rows`,
  `Add`); число администраторов и покрытых клиент может счесть по строкам
  только пока список не отфильтрован (Ад6). Нужен `Subtitle`.
- **Ад6.** Полоса. Кадр: фильтр «Filter by login», сегменты «All ·
  Administrators · Disabled», Columns. Модель: `Bar` несёт только
  `columns`, `loaded`, `total`; `FilterForm` и `Segments` пусты (в
  структуре так и сказано: «пока только Columns»), у `/admin/` нет ни
  `?filter=`, ни `?show=` (C-5). Нужны форма фильтра и три сегмента с
  адресами.
- **Ад7.** Delete из «⋯» не исполним. Кадр `s-admin-2`: «Deleting an
  account is confirmed in a dialogue of its own.»; `s-admin-12` — пункт
  Delete. Модель: пункт `admin.user.delete` — прямой POST
  `/admin/?action=delete&target=…`; а `DeleteUser` (по коду,
  `internal/user/admin.go`) требует ещё поле `login`, равное логину, иначе
  `ErrLoginConfirm` («type the login to confirm deletion»). Адрес пункта
  поля `login` не несёт — нажатие всегда кончается отказом над страницей.
  Диалога (маршрута, вью-модели, зоны `sheet`) нет. Нужен лист
  подтверждения удаления с полем логина; пункт — GET на него.
- **Ад8.** Лист сброса без фикстур. Кадры `s-admin-13`, `-14`, `-16`,
  `-20` (и меню `-15`) ни с чем не сверяются: каталога фикстур
  `PasswordResetSheet` нет, маршрута `/admin/reset` нет в
  `FIXTURE_SCREEN`. Нужны фикстуры: лист с выбранным человеком и
  выключенными вариантами, ответ с `Issued`, отказы у `password` и
  `confirm`. По коду слова совпадают с кадрами: предупреждение, подсказка
  пароля, «Password must be at least 8 characters.» не проверено (текст
  отказа идёт из `user.ErrPasswordShort`, его не читал), «Type anna to
  confirm whose password is reset.», причины вариантов `s-admin-15`.
- **Ад9.** Образец в поле подтверждения. Кадр `s-admin-13`: поле «Type the
  login to confirm» с образцом «anna» — логином выбранного в Whose.
  Модель: у строки `confirm` `Placeholder` не ставится (`resetSheet`, по
  коду). Нужен `Placeholder` — логин выбранного.
- **Ад10.** Значок Copy выданного пароля. Кадры `s-admin-14`, `-17`: Copy
  со значком (`i-dup`). Модель: `IssuedPassword.Copy` без `Icon` в обоих
  ответах (по коду: `admin.reset.copy`, `admin.user.password.copy`). Нужен
  `Icon`.
- **Ад11 (данные).** `admin-users/normal` — одна строка (mix); кадр — три:
  администратор, человек с «deposited», отключённый («viktor» с бейджем
  «disabled», строка приглушена). `Disabled`, `Escrowed` true, пункты
  Make administrator, Enable и включённые Disable, Delete ни одна фикстура
  не несёт. `s-admin-22` (Address вторым, Created, DAV, Sessions сняты) —
  без фикстуры со своим набором столбцов.

### В. Add users (`AdminInvitesPage`, `s-admin-1`, `-8`, `-11`, `-17`, `-18`, `-19`)

- **Ад12.** Отказы сразу у всех полей. Кадр `s-admin-19`: одновременно
  «Login is already taken.» у Login, «Enter one address, like
  name@example.org.» у Email, «Password must be at least 8 characters.» у
  Temporary password. Модель: один отказ на ответ — `userErr` с одним
  `userField` (`postAdminInvites`: первая ошибка `CreateUser`, прочие поля
  без `Error`). Нужно: проверить все три поля и вернуть отказ у каждого.
- **Ад13.** Карточка после Create user. Кадр `s-admin-17`: в карточке
  только рубрика «Temporary password · visible only now», сообщение «The
  user **viktor** is created. Hand this over; until they change it, you can
  sign in as them.», строка «login viktor password 9f2-brick-lamp», Copy и
  Done — ни бейджа «hand to hand», ни предупреждения, ни формы. Модель:
  `Temporary` в том же ответе несёт и `Issued`, и `Badge`, и `Warning`, и
  пустую `Form` с новым предложенным паролем (по коду: «The form is empty
  again»). У листа сброса в таком ответе `Form` и `Warning` пусты. Нужно:
  так же — в ответе с `Issued` карточка без формы, бейджа и предупреждения.
- **Ад14.** Тон неудачи письма. Кадр `s-admin-1`: «failed — 550 mailbox
  unavailable» в столбце Mail — тревогой (`CellDanger`), «—» и прочее —
  обычно. Модель: `InviteRow.Sent` — одна строка; что это отказ, клиент
  узнаёт только по началу «failed». Нужен признак неудачи своим полем.
- **Ад15 (данные).** `admin-invites/normal`: `rows` пуст — ни одной строки
  `InviteRow` (кадр: две — ссылкой и письмом с отказом реле, с Resend, Copy
  link и «⋯»), `invite.link` пуст (кадр: «https://carrel.example/invite/7f3a…»
  с Copy), регистрация закрыта (кадр: бейдж «open», отметка стоит, адрес
  «https://carrel.example/register» с Copy). Нужны фикстуры: с
  приглашениями обоих родов и открытой регистрацией (канон `s-admin-1`),
  `?invite=`, ответ с `Issued` (`s-admin-17`), без почты (`s-admin-18`:
  Send invitation и Save выключены), отказы полей (`s-admin-19`). Слова
  причин `s-admin-18` в коде те же: «SMTP is not configured: no letter can
  go.», «Mail is not set up: nobody who registers could confirm the
  address.».

### Г. Settings (`AdminSettingsPage`, `s-admin-3`, `-9`)

- **Ад16.** Заголовок. Кадр: «Settings». Модель: `title` — «Instance
  settings». Нужно «Settings».
- **Ад17.** Одна форма с одним Save. Кадр: в шапке один «Save» (основной),
  под ним одна форма из рубрик Global, Mail, Letters from users, Test;
  подпись кадра: «Две формы становятся двумя блоками одной формы», проба —
  «своя форма со своей кнопкой … „Save“ остальных блоков её не задевает».
  Модель: три формы с тремя Save — `instance` (`op=instance`,
  `admin.settings.instance`), `smtp` (`admin.settings.save`), `limits`
  (`op=limits`, `admin.settings.limits`) — и `mail` (проба). Нужно: один
  POST и одно действие Save на Global, Mail и Letters from users.
- **Ад18.** Бейджи рубрик. Кадр: «Mail» с бейджем тревоги «not
  configured», «Letters from users» с бейджем «off». Модель: у `FieldSet`
  только `Legend`; несостояние почты сервер пишет в саму легенду — «Mail —
  not configured» (по коду; в фикстуре почта настроена, «Mail»), у
  «Letters from users» состояния нет вовсе. Нужен бейдж рубрики своим
  полем (и «off»/что вместо него при включённом — кадра включённого нет).
- **Ад19.** Подсказки, которых кадр не рисует. Кадр: в Global подсказка
  одна — у Public address («Links in letters start with it. Set with
  CARREL_PUBLIC_URL.»); в Mail подсказок нет; у потолков писем нет.
  Модель: `Hint` у Invite lifetime («From 1 to 720. …»), Idle timeout,
  Session life, Instance name, Host, Encryption, From address, Attachment,
  Letter, Recipients, Letters a day. Нужно: убрать, либо владельцу решить,
  что кадр неполон (вопрос В3).
- **Ад20.** Образцы полей Mail. Кадр: Host — «smtp.example.org», Username
  — «—», Password — «leave blank to keep the current one», From address —
  «carrel@example.org», From name — «Carrel», в пробе — «you@example.org».
  Модель: `Placeholder` у этих строк нет; про пароль — подсказкой «Leave
  blank to keep the current one.» и только когда пароль уже сохранён.
  Нужны `Placeholder`.
- **Ад21.** «Sender address». Кадр: в блоке «Letters from users», после
  отметки, сегментами «noreply@carrel.example · login@carrel.example»;
  подсказка кусками — «Your relay decides which of the two it will let
  out. The envelope sender stays ‹noreply@carrel.example› (моноширинно)
  either way, so SPF is unaffected; DKIM and DMARC for this domain are set
  up on the relay, not here.». Модель: строка `sender` стоит в форме `smtp`
  (блок Mail), `Hint` — простой текст других слов: «Of people's letters.
  Your relay decides … The envelope sender stays the From address either
  way, … Replies go to the person's confirmed address in both cases.».
  Нужно: строка в блоке писем, подсказка словами кадра с адресом куском.
- **Ад22.** Проба без почты. Кадр: Send test выключена, под полем — «Mail
  is not configured: there is nothing to send through yet.». Модель (по
  коду): `Reason` действия — «Set the relay up and save it first.». Нужны
  слова кадра.
- **Ад23 (данные).** Фикстура — почта настроена (Host «127.0.0.1», порт
  25, None, Username «relay»), кадр — не настроена (порт 587, STARTTLS);
  числа 24/2/7 против 72/12/30 и 10/25/20/100 против 2/8/10/50; Instance
  name «carrel.example» против «Carrel»; Public address «the address each
  request comes to» против «https://carrel.example»; отметка писем стоит
  против снятой; выбран `login` против `noreply`. Фикстуры «не настроено»
  (канон кадра) и ответа пробы (`test`) нет.

### Д. Audit log (`AdminAuditPage`, `s-admin-7`, `-10`, `-23`)

- **Ад24.** Дата на полосе. Кадр: между выбором действия и сегментами —
  поле даты «2026-08-01». Модель: в `filter` одна строка `action`,
  `Bar.Range` пуст; у маршрута только `?show=` и `?action=` (C-5). Нужны
  поле даты и параметр адреса; сегменты и CSV должны его держать.
- **Ад25.** Тревожная строка. Кадр: «login.fail» в столбце Action —
  тревогой, подпись: «Неудачный вход — единственная тревожная строка».
  Модель: у `AuditRow` признака нет; правило «это отказ» живёт на сервере
  (`auditIsFailure`: `.fail`, `.failed`, `.refused`). Нужен признак строки.
- **Ад26.** Target у действия над собой. Кадр: «login.ok» — Actor «mix»,
  Target «—»; «settings.instance» — Target «—». Модель: у `login.ok`,
  `connection.add`, `sources.select`, `publish.create` и прочих `whom` —
  «mix» при `who` «mix» (18 строк фикстуры из 22). Нужно: пустой `Whom`,
  когда действие не над другим человеком (или вопрос к кадру, В6).
- **Ад27.** Export CSV. Кадр: кнопка в шапке, без значка. Модель:
  `bar.actions[0]` со значком `i-down`. Место — дело клиента; значок
  убрать.
- **Ад28 (данные).** Кадр: «1 240 records», строки `login.ok` («ip
  10.0.0.4»), `login.fail` («3rd attempt · rate-limited», Actor «—»),
  `escrow.recover` («notice mailed»), `settings.instance` («smtp_host,
  smtp_port»). Фикстура: 22 строки других действий, `login.ok` без
  `detail`, ни одного отказа; Actor «system» у `setup.first_admin`. Пишет
  ли журнал адрес входа и счёт попыток — не проверено. Журнал приходит
  весь одной страницей (`loaded` = `total`): дозагрузки у 1 240 строк
  нет — не проверено, нужна ли.

### Е. Installation check (`AdminInstallPage`, `s-admin-4`)

- **Ад29.** Адрес в шапке — поле формы. Кадр: в шапке поле
  «https://carrel.example» (моноширинное, правится) и «Run the check».
  Модель: `PublicURL` — текст, формы нет; имя поля (`url`, по коду
  `postAdminInstall`) и скрытый `_csrf` модель не несёт — клиенту имя
  поля взять неоткуда. Нужна `Form` с полем `url`.
- **Ад30.** Длительность проверки. Кадр: на полосе итога справа «4.2 s».
  Модель: поля нет. Счёт «11 checks · 8 fine · 2 broken · 1 warning»
  клиент сложит по `Rows[].State` (ok, bad, unknown); длительность — нет.
  Нужно поле.
- **Ад31 (данные).** `admin-install/normal` — до проверки: `rows` пуст,
  `publicURL` пуст (кадр: адрес задан, одиннадцать строк, из них две
  `bad` с советом и одна `unknown`). Канон кадра фикстурой не покрыт;
  нужна фикстура после проверки. Слова строк сверены только у первой
  («Reachable from outside», «GET …/healthz → 200 · 41 ms» — по коду
  совпадает), остальные десять — не проверено.

### Ж. DAV validator (`AdminDavPage`, `s-admin-5`)

- **Ад32.** Поля формы. Кадр: Server URL, Username, Password и «Test
  discovery»; подсказок нет. Модель: четвёртая строка `certFingerprint`
  («Certificate fingerprint», с подсказкой) и подсказка у Password
  («Asked each time: nothing of this page is kept.»). Нужно: решить по
  кадру (убрать) или дорисовать кадр (вопрос В8).
- **Ад33 (данные).** `admin-dav/normal` — до проверки: `trace` пуст. Кадр
  рисует след пятью строками («[well-known]  GET /.well-known/caldav → 301
  → /dav.php/» …). Как `DiagnosticTrace.Steps` (`Name`, `State`, `Detail`)
  ложится на эти строки — не проверено: фикстуры со следом нет. Нужны
  фикстуры со следом (удачным и сломанным на шаге).

### З. Key escrow (`AdminEscrowPage`, `s-admin-6`)

Кадр и фикстура сходятся по структуре и словам: подзаголовок,
предупреждение, три рубрики с подсказками, Save, Change, Recover (danger),
таблица с числом `opens`.

- **Ад34.** Лишнее в модели. `History []AuditRow` кадр не рисует, сервер
  не заполняет (по коду — ни одного присваивания); `Result`
  (`EscrowRecoveryResult`: логин, временный пароль, Copy, `Mailed`) —
  ответ на Recover без кадра. Нужно: `History` убрать; `Result` привести
  к `IssuedPassword` (кадры `s-admin-14`, `-17` рисуют ровно это) или
  нарисовать кадр (вопрос В9).
- **Ад35.** Приглушённое «never». Кадр: «never» и «not deposited» —
  приглушённо (`CellDusty`), день и время — обычно. Модель:
  `LastRecovery` — строка «never»; «not deposited» клиент узнаёт по
  `Covered`, «never» — только по слову. Нужен признак (или пустое
  `LastRecovery`, а слово — экрану).
- **Ад36 (данные).** Фикстура: одна строка (mix, не депонирована),
  «0 of 1 account open with the master password», `opens` 0; кадр — три
  строки, «4 of 6 accounts …», число 4 у рубрики таблицы (в самом кадре
  при этом три строки и две депонировавших — В10). Строки депонировавшей
  учётной записи ни одна фикстура не несёт; состояния «схема не включена»
  — тоже.

## Работа клиента (не пробелы)

- Рейл: «Back to app» отдельной строкой над рубрикой «Administration» —
  рубрика и деление списка `nav` на первую строку и остальные; `RailStatus`
  из `status` (тон, заголовок, строка кусками).
- Users: заголовки столбцов, порядок и набор — по `bar.columns.items`
  (`s-admin-21`, `-22`); «—» у пустого `Address` и у не депонировавшего в
  Escrow, бейджи «deposited» и «disabled», приглушённая строка
  отключённого; «⋯» с подсказкой «More»; сообщение под таблицей — слова
  экрана.
- Лист сброса: значок шапки (`i-shield`), предупреждение тревогой, Whose —
  `Pick` с выключенными вариантами и причиной в подсказке, Esc у Cancel,
  Enter у Done; в ответе — сообщение, карточка «Temporary password ·
  visible only now», строка «login … password …» из `Login` и `Password`.
- Add users: три карточки в ряд; в Invitation — Role и Create link, строка
  ссылки с Copy, под чертой «Or send to» и Send invitation (две кнопки
  одной формы — у каждой свой `href`); «Pending invitations» с числом —
  по длине `rows`; заголовки столбцов.
- Settings: подзаголовок «Global settings and mail», ширины полей, отметка
  писем отдельной строкой, подсказка пробы под полем из `Reason`.
- Audit: подзаголовок «N records» из `bar.total` (разрядка «1 240»),
  Export CSV в шапку из `bar.actions`, «—» у пустых `who`, `whom`;
  выбор действия — `Pick`, уходит при выборе.
- Install: подзаголовок, полоса итога (счёт по `State`, слова «fine»,
  «broken», «warning», шкала 100%), флажки строк по `State`, выделенная
  строка у `bad`, сообщение под таблицей.
- DAV: подзаголовок из `Legend` набора («Discovery without saving
  credentials»), след — `Diag`, сообщение о втором этапе — слова экрана.
- Escrow: значок предупреждения (`shield`), место подсказок в наборах
  (у Policy — между отметкой и Save, у прочих — после кнопки), рубрика
  таблицы с `opens`.

## Вопросы к кадрам (кандидаты задач Typecase)

- **В1.** Кнопка опроса «2 min ago» в верхней панели всех семи кадров:
  администрирование ничего не опрашивает, у фикстур `top.refresh` пуст.
  Кадрам — убрать, либо сказать, что она здесь значит.
- **В2.** `s-admin-18`: карточка Invitation нарисована без Role и Create
  link, хотя подпись говорит «ссылка приглашения работает как прежде»;
  `s-admin-19`: карточка Temporary password без Role и без предупреждения,
  которые есть в `s-admin-1`. Фрагменты обрезаны или карточка в этих
  состояниях другая?
- **В3.** `s-admin-3` не рисует ни одной подсказки границ («From 1 to
  720») и смысла полей, которые несёт сервер (Ад19): кадр неполон или
  подсказки лишние? Кадра «почта настроена» нет (что стоит у рубрики Mail
  вместо «not configured», что у «Letters from users» вместо «off»); кадра
  итога пробного письма (`Test`: удача и полный ответ реле) нет; кадра
  отказа поля настроек нет.
- **В4.** `s-admin-2`: «Deleting an account is confirmed in a dialogue of
  its own» — самого диалога среди кадров нет (Ад7).
- **В5.** Лист сброса без сценариев: смена человека в Whose меняет образец
  поля подтверждения (и должна бы сбрасывать набранное) — у модели это
  `GET /admin/reset?target=`, у кадра `Pick` ничего не шлёт. Copy
  (`s-admin-1`, `-14`, `-17`, строки приглашений) без сценариев — ссылка
  на сам адрес или кнопка без действия; у пароля адреса нет вовсе.
- **В6.** `s-admin-7`: поле даты без подписи — «с этого дня», «за этот
  день» или «до»? Target «—» у `login.ok` при том, что сервер пишет
  `whom` — самого вошедшего (Ад26): кто прав. Выбор действия без
  сценариев: у формы `filter` нет кнопки отправки.
- **В7.** `s-admin-4`: кадра «проверка ещё не запускалась» (пустая
  таблица — что на её месте) и кадра идущей проверки (шкала меньше 100%)
  нет; отказ негодного адреса сервер шлёт уведомлением над страницей —
  кадра нет. Третье слово итога — «1 warning», а в C-8 состояние —
  `unknown` («то, что снаружи не измерить»).
- **В8.** `s-admin-5`: нет кадра до проверки, кадра сломанного шага и
  всего второго этапа — отметки согласия («Create test objects, read them
  back and delete them»), «Run the full check» и отчёта `ExerciseReport`,
  которые сервер уже отдаёт; поля «Certificate fingerprint» в кадре нет
  (Ад32). Password нарисован `type="text"` с точками.
- **В9.** `s-admin-6`: нет кадра ответа на Recover (временный пароль
  показывается один раз — как `s-admin-14`?) и кадра «схема не включена»
  (мастер-пароль и Turn on key escrow).
- **В10.** `s-admin-6` сам с собой: подзаголовок «4 of 6 accounts», число
  «4» у рубрики таблицы, а строк три и депонировавших две. `s-admin-2`:
  «6 accounts · 2 administrators · 4 covered» при трёх строках — усечённый
  список без пометки.
- **В11.** Меню Columns (`s-admin-21`, `-23`): скрытое поле кадра —
  `name="csrf"`, у сервера — `_csrf`.

## Итог

Пробелов серверу — 36 (Ад1 … Ад36): оболочка и рейл 4, Users и лист
сброса 7, Add users 4, Settings 8, Audit log 5, Installation check 3, DAV
validator 2, Key escrow 3. Из них данных и недостающих фикстур — 9 (Ад4,
Ад11, Ад15, Ад23, Ад28, Ад31, Ад33, Ад36 и Ад8 — лист без единой
фикстуры); пробелов контракта — 27. Вопросов к кадрам — 11 (В1 … В11).

Не проверено: текст отказа короткого пароля (`user.ErrPasswordShort`) и
занятого логина; десять из одиннадцати строк проверки установки; как след
обнаружения ложится на строки кадра; что журнал пишет в `detail` у входов;
ответы с `Test`, `Result`, `Full` — только по структурам, без фикстур.
