# Раздел настроек (U-6.11): пробелы контракта — скан 09.10.2026

Скан только чтением. Кадры — Typecase `0fe0244`
(`docs/visual/carrel-ui-mockups.html`), сервер — `v0.2.174` (`6a62d8a`).

**Что сверено.** Десять маршрутов U-6.11: `/app/settings/account`,
`/app/settings/escrow` (с листами `…/deposit`, `…/withdraw`), `/app/password`,
`/app/settings/connections` (с `…/test`, `…/disconnect`),
`/app/settings/attachments` (с листом `/app/files/picker?for=attachments`),
`/app/settings/devices`, `/app/settings/davloom`, `/app/settings/backup`
(список, задание, восстановление), `/app/settings/appearance`,
`/app/settings/unlock`. 48 кадров: `account-1…4`, `-7…-14`, `s-mail-9`,
`s-profile-1…4`, `-10…-12`, `-14…-17`, `attachments-1…4`, `s-plan-1…5`,
`s-davloom-1…3`, `s-backup-1…11`, `appearance-1`. Десять фикстур — по одной
`normal.json` на каталог `app-settings-*` и `app-password`.

**Метод.** Полные кадры — черновиком `frame2go -self` (25 штук, лежат в
`settings-work/draft-*.go`) и главной зоной кадра против фикстуры; фрагменты —
HTML против фикстуры. Фикстура у каждого экрана одна, поэтому состояния без
фикстуры сверены с тем, что собирает сервер: `carrel/server/view/settings.go`
и обработчики `internal/web` (`account.go`, `account_sessions.go`,
`handlers_escrow.go`, `handlers_app.go`, `handlers_connections.go`,
`feeds_settings.go`, `attach_settings.go`, `devices.go`, `davloom.go`,
`backup.go`, `backup_trigger.go`, `restore.go`, `rail_status.go`,
`rails_settings.go`). Где утверждение держится на коде, а не на фикстуре,
сказано «по коду».

**Не моё и не сверялось:** `/app/files/published`, листы коллекций
(`s-profile-5…9`, `-13`, `-18…-21`), меню учётной записи и лист клавиш
(`account-5`, `-6`), страница самого davloom (`s-davloom-4` — не экран
Carrel), отложенные `s-mailbox-26`, `-29`. Кадров `states-*` у настроек нет:
`states-1` и `states-2` — экраны разделов данных.

---

## 1. Экраны, модели, кадры, фикстуры

| Экран | Маршрут | Вью-модель | Кадры | Фикстура `normal` — какое это состояние |
|---|---|---|---|---|
| Учётная запись | `/app/settings/account` | `AccountPage` | `account-1` (канон), `s-mail-9` (часть `mail`), фрагменты `account-8`, `-9`, `-10` (Email), `-7`, `-13` (Key escrow) | ждёт смена адреса, письмо ушло, копии ключа нет, один сеанс — блок Email как в `account-1` |
| Депонирование | `/app/settings/escrow` | `EscrowPage` | `account-4` (канон, копия есть), фрагменты `account-12` (копии нет), `-14` (схема выключена) | **копии нет** — это `account-12`, а не канон `account-4` |
| Лист депонирования | `/app/settings/escrow/deposit` | `UnlockSheet` | `account-2` | своей нет; тот же лист лежит в `app-settings-escrow/normal` полем `confirm` |
| Лист отзыва | `/app/settings/escrow/withdraw` | `EscrowWithdrawSheet` | `account-11` | **нет** |
| Смена пароля | `/app/password` | `PasswordPage` | `account-3` (оба случая в одном кадре) | обычная (с `back`, без `warning`) |
| Подключения | `/app/settings/connections` | `ConnectionsPage` | `s-profile-1` (канон), `-10` (часть `subscribe`), `-2`, `-14`, `-16`, `-11` (диалог), `-4` (пароль показан), фрагменты `-12`, `-15`, `-17` | список: 4 сервера, 2 подписки, выдача закрыта, ни одна коллекция сеансом не читана |
| Лист Disconnect | `/app/settings/connections/disconnect` | `ConnectionRemoveSheet` | **нет** | **нет** |
| Вложения | `/app/settings/attachments` | `AttachmentsPage` | `attachments-1`, фрагменты `-2`, `-3` (списки) | папка выбрана |
| Лист Browse | `/app/files/picker?for=attachments` | `FilePickerSheet` | `attachments-4` | **нет** (в `app-files-picker` — `normal`, `attach`, `refused`) |
| Новое устройство | `/app/settings/devices` | `DevicesPage` | `s-plan-1` (Apple, канон), `-2` (android), `-4` (thunderbird), `-5` (other) — все четыре с открытой выдачей | Apple, выдача **закрыта**, ссылки нет — такого кадра нет |
| davloom | `/app/settings/davloom` | `DavloomPage` | `s-davloom-1` (канон, два устройства), `-2` (диалог), `-3` (пароль выдан) | устройств нет, форма добавления на странице — такого кадра нет |
| Резервная копия | `/app/settings/backup` | `BackupPage` | `s-backup-1` (канон, три задания), `-6` (заданий нет), `-2`, `-3` (задание), фрагменты `-4`, `-5`, `-10`, `-11` (карточка запуска), `-9` (список), `-7`, `-8` (восстановление) | **заданий нет** — это `s-backup-6`, а не канон `s-backup-1` |
| Внешний вид | `/app/settings/appearance` | `AppearancePage` | `appearance-1` | единственное состояние |
| Подтверждение пароля | `/app/settings/unlock` | `UnlockSheet` | `s-plan-3` (канон), `s-profile-3` | единственное состояние |

**Состояния с кадром, без фикстуры:** `account-8`, `-9`, `-10`,
`s-mail-9`, `account-7`, `-13`, `account-4`, `-14`, `account-11`,
вынужденная `account-3`, `s-profile-2`, `-14`, `-16`, `-11`, `-10`, `-4`,
`-15`, `-17`, отказавшее подключение и отказавшая лента `s-profile-1`,
`attachments-4`, `s-plan-1` (ссылка выдана), `-2`, `-4`, `-5`,
`s-davloom-1` (с устройствами), `-2`, `-3`, `s-backup-1`, `-2`, `-3`,
`-4`/`-5`/`-10`/`-11`, `-7`, `-8`.

**Фикстуры без своего кадра:** `app-settings-devices/normal` (выдача
закрыта), `app-settings-davloom/normal` (устройств нет). Ещё две —
`app-settings-escrow/normal` и `app-settings-backup/normal` — держатся
`check.py` против канона (`account-4`, `s-backup-1`), хотя рисуют
`account-12` и `s-backup-6`: строк `STATE_FRAME` для них нет.

---

## 2. Пробелы — серверу (Нс)

### А. Сквозное

- **Нс1. Ни одна форма настроек в кадрах не форма.** Во всех 48 кадрах
  раздела в главной зоне нет ни `<form>`, ни `type="submit"`, ни одного
  `name=` (посчитано по HTML); черновики `frame2go` не печатают ни `Action`,
  ни `Submit`, ни `Send`. Модель же несёт формы с адресами (`Profile`,
  `Add`, `Form`, `Source`, `Match.Form`) и кнопки-`Extra`. Сверять «кадр шлёт
  то же, что модель» не с чем: это вопрос к кадрам (В1), но до ответа круг
  сервера по настройкам правит слова и поля, а не способ отправки.
- **Нс2. Даты в таблицах — числами, моноширинно.** Кадры: Started
  «2026-08-16 09:02» (`account-1`), Created «2026-06-02», Last request
  «2026-08-15 09:31» (`s-davloom-1`), Last backup и When истории
  «2026-08-16 03:10» (`s-backup-1`). Модель: `SessionRow.Started` — «10
  August 2026, 17:04» (фикстура), `ProxyDevice.Created`/`LastSeen` и
  `BackupRun.Started` — тем же `formatWhen` (по коду). Тот же вопрос, что
  ФН17 у файлов: C-10 говорит словами, кадр — числами. Относительные
  («just now», «2 hours ago», «3 h ago») совпадают по роду.
- **Нс3. Подсказка поля с выделенными кусками.** Кадры пишут в
  `tc-field__hint` моноширинные и жирные куски: «…and as ‹Reply-To› on
  letters you send.» (`account-1`), «‹.well-known› is tried first…»
  (`s-profile-2`), «‹https://›, ‹webcal://› and a secret link…»
  (`s-profile-10`), «‹2026-07›, ‹2026-08›. Off means…», «‹2026-08-10
  sync-board.png›. A name already taken…», «(‹max-resource-size›)»
  (`attachments-1`), «…are deleted **after** the new one…» (`s-backup-2`).
  Модель: `FormRow.Hint` — `Text`, одна строка без выделений. Нужно `Runs`
  у подсказки (или выделение своим полем).
- **Нс4. Рейл страницы депонирования.** Кадр `account-4`: в рейле отмечен
  Account (`is-on`). Фикстура `app-settings-escrow/normal`: ни у одной
  строки `rail.nav` нет `on` (у `app-password/normal` Account отмечен).
  Нужно `on` у `settings.account` на `/app/settings/escrow`.
- **Нс5. Подсказка у «verified» в подвале рейла.** Кадры (рейл
  `s-profile-1`, `account-3`, `-4`): «Last backup ‹2026-08-16 03:10› ·
  verified», у слова — подсказка «The archive was read back from the
  destination and the checksum matched». Модель: `RailStatus.Line` — `Runs`,
  у `Run` есть `mark`, подсказки нет (по коду — `join(last, " · verified")`).
  Нужна подсказка куска или слово экрана по признаку.
- **Нс6. Рейл — данные.** Кадры: «Connections 3», «davloom 3», «Backup 3»,
  два состояния в подвале («davloom · running · 3 devices / Last response
  ‹2026-08-16 09:41›», «Backups are running / …»). Фикстуры: Connections 4,
  davloom 0, у Backup счёта нет, в подвале одно состояние «davloom · running
  · 0 devices / No device has connected since the start.». Модель всё это
  умеет (`rail_status.go`); в наборе нет устройств и заданий. **данные**
- **Нс7. Метка «credentials unlocked · 4:12».** Кадры `s-profile-4`,
  `s-plan-1`, `-2`, `-4`, `-5`: в значке — остаток окна, минуты и секунды.
  Модель: `TopBar.Unlocked` — `When`, готовая строка момента конца окна
  (`formatWhen(*until, …)`, по коду); остаток из неё не вывести. Нужен
  остаток (или момент числом), если значок считает; фикстуры с открытой
  выдачей нет ни у одного экрана настроек.

### Б. Учётная запись (`AccountPage`)

- **Нс8. Сеансы — данные.** Кадр `account-1`: рубрика «Sessions 2 active»,
  строки «Firefox · Windows» со значком «this one» и «Carrel desktop ·
  Windows» с End, под таблицей «End every other session». Фикстура: один
  сеанс, `agent` и `address` пусты, действия «End every other session» нет
  (по коду оно приходит при двух и более сеансах — `account.session.others`).
  **данные**; заодно: «Carrel desktop» `agentLabel` не выдаёт — в его
  списке только браузеры и `curl` (по коду).
- **Нс9. Состояния Email без фикстур.** `account-8` (значок «confirmed 14
  February», зелёный), `s-mail-9` («Nobody can reply to letters you send:
  until this address is confirmed, they go out without a reply address and
  say so at the foot.» и Send confirmation), `account-9` («…until you open
  the link we sent to ‹mix@example.org› on 12 February at 11:42.» и Send
  again), `account-10` (значок «pending: new@example.org» и «No link has
  gone to ‹new@example.org›: until one is opened, ‹mix@example.org› stays in
  use.» с Send again). Слова по коду (`emailStatus`) совпадают с кадрами;
  фикстур четырёх состояний нет, строки `STATE_FRAME` для `s-mail-9` нет.
- **Нс10. Адреса в сообщении Email — моноширинно.** Кадры `account-9`,
  `-10`: адрес в сообщении — `tc-mono`. Модель: `EmailStatus.Message` —
  `Notice`, `Body` — `Text`. Нужны `Runs` (как у `Subtitle` подключений).
- **Нс11. Состояния Key escrow без фикстур.** `account-7`: значок у рубрики
  «deposited 2026-03-02» (тревогой), слова «A copy of your data key is
  deposited on this server. If you lose your password, an administrator who
  holds the escrow master password can recover the account — and, with it,
  can read the DAV credentials you save here.», кнопка «Withdraw my
  deposited key». `account-13`: одна строка «Key escrow is disabled on this
  instance.», кнопок нет. По коду (`escrowStatus`) слова те же; фикстур нет.
- **Нс12. Удаление учётной записи.** Кадр `account-1`: «Delete account»
  залитой кнопкой в опасной зоне, листа подтверждения не нарисовано.
  Модель: вне окна выдачи действие ведёт на общий лист
  `/app/settings/unlock` (`target: sheet`), а внутри окна — это POST
  `?op=delete` без вопроса (по коду, `deleteAction`). Лист пароля не
  называет, что подтверждают (Нс19). Нужен кадр (В4) и по нему — лист.

### В. Депонирование и смена пароля

- **Нс13. Фикстура страницы — не то состояние.**
  `app-settings-escrow/normal` — «копии нет» (`subtitle` «No copy of your
  data key is deposited», `rubric` «What a deposit does») и совпадает с
  фрагментом `account-12`; `check.py` ставит её к `account-4`. Нужны:
  строка `STATE_FRAME` (`account-12`), фикстура «копия есть» под `account-4`
  (предупреждение, пары Deposited «2026-03-02» и Last recovery «never»,
  «Withdraw my deposited key», строка «After withdrawal nobody can recover
  this account, including you.» — по коду слова те же) и «схема выключена»
  под `account-14` (заголовок без подзаголовка и одно обычное сообщение).
- **Нс14. Лист в модели страницы.** Фикстура страницы несёт `confirm` —
  весь лист депонирования — на обычном GET; кадр `account-12` рисует
  страницу без листа, лист (`account-2`) открывает кнопка
  (`…/escrow/deposit`, `target: sheet`). По модели не отличить «лист
  открыт» от «лист приложен». Нужно: `confirm` только в ответе с отказом
  (неверный пароль без сценариев) либо признак.
- **Нс15. Модель несёт больше кадра.** `EscrowPage.History` не заполняется
  нигде (по коду) и не нарисована ни в `account-4`, ни в `-12`, ни в `-14`;
  `EscrowStatus.By` («administrator with master password») и `Covered` кадры
  не рисуют. Убрать из контракта или нарисовать (В5).
- **Нс16. Листы без фикстур.** Отзыв (`account-11`: заголовок и залитая
  кнопка «Withdraw my deposited key», одно предупреждение «After withdrawal
  nobody can recover this account, including you. The copy of your data key
  on this server is destroyed; your records and connections stay as they
  are.») — каталога фикстур у маршрута нет. Депонирование (`account-2`) —
  своей фикстуры у `/app/settings/escrow/deposit` нет. У
  `EscrowWithdrawSheet.Actions[0]` есть `danger`, залитой её делает экран.
- **Нс17. Вынужденная смена пароля.** Кадр `account-3` рисует оба случая
  разом: красную строку «You are signed in with a temporary password. Choose
  a new one before continuing.» и «Back to settings». Фикстура — только
  обычный (`warning` пуст, `back` есть). Нужна фикстура вынужденного
  (`warning`, `back: null`); кадр такого вида целиком не рисует (В6).

### Г. Подтверждение пароля (`UnlockSheet`)

- **Нс18. Заголовок и предмет листа.** Кадры `s-plan-3`, `s-profile-3`: в
  шапке «Show password» и справа приглушённо «nextcloud» — имя подключения,
  чей пароль просят («чтобы „Show password“ с двух карточек подряд нельзя
  было перепутать»). Модель: ни заголовка, ни предмета; `reveal.href`
  карточки — голый `/app/settings/unlock` без подключения. Нужны заголовок
  и предмет листа и адрес, который их называет.
- **Нс19. Слова листа.** Кадр: одна приглушённая строка под полем —
  «Unlocks credentials everywhere for five minutes: passwords, device
  profiles, key escrow — anything handed out or handed over.»
  (`s-profile-3`). Модель: `purpose` «Confirm your password to reveal stored
  credentials, download a device profile, issue a davloom device password,
  change key escrow, or delete your account.» и `warning` «Everything
  revealed in this window is written to the audit log.» — обеих фраз в
  кадрах нет, срок («five minutes») фикстура не называет (`until` — nil).
- **Нс20. Cancel.** Кадр: в подвале Show и Cancel. Модель: `form.actions` —
  один `unlock.submit`; Cancel и адреса возврата (скрытое `back` в фикстуре
  пусто) нет.

### Д. Подключения (`ConnectionsPage`)

- **Нс21. Отметка строки коллекции.** Кадр `s-profile-1`, `-10`: первый
  столбец каждой строки — отметка (`tc-tickrow__tick`), у снятой — `is-off`
  и приглушённая строка («Снятая коллекция видна снятым флажком: её не
  опрашивают»). Модель: у `CollectionRow` нет ни признака «отмечена», ни
  действия переключить. Нужны оба.
- **Нс22. Objects.** Кадр: столбец «Objects» — «128», «74», «31»…
  Модель: `CollectionRow.Objects` есть, но `connectionCard` его не
  заполняет (по коду); в фикстуре у всех 17 строк `null`.
- **Нс23. Отказавшее подключение.** Кадр (карточка baikal): тревожная
  карточка, «Test again», строка адреса «… · m.yenuchenko · last answer
  refused», след в `tc-diag` и под ним строка «The server refused the
  password. It may have been changed on the server: the one Carrel saved no
  longer fits.». Модель: `State` «unavailable», `Trace`, подпись «Test
  again» есть (по коду); «refused» в `Answered` (это время последнего
  чтения) не выразить, строки-вывода под следом у `ConnectionCard` нет.
  Фикстуры такого состояния нет.
- **Нс24. «Look for changes».** Модель: у каждой карточки действие
  `connection.rediscover` «Look for changes». В макете этих слов нет нигде
  (0 вхождений); кадр рисует New collection, Show password, Test, Edit,
  Disconnect. Убрать, либо кадр (В7).
- **Нс25. Карточка подписки.** Кадр: строка адреса «… · no password · read
  2 min ago · 34 KB · 0.4 s» и у отказавшей «webcal://… · password saved ·
  last read yesterday»; у отказавшей — Show password, «Check again», след
  `[fetch] GET /season.ics → 401…`, строка-вывод «The feed asks for a
  password and the saved one no longer fits. A feed that stopped answering
  must not look like a feed with nothing in it.». Модель
  (`SubscriptionCard`): `Title`, `URL`, `Colour`, `Kinds`, `Stale`,
  `Checked`, `Size`, `Took`, `Actions`; нет слов о пароле, нет
  `SecretField`, нет следа и вывода, `Stale` не ставится нигде, подпись
  всегда «Check» (по коду, `subscriptionCard`).
- **Нс26. Таблица подписки «Lands in».** Кадр: строка на род — Calendar
  «118», Tasks «6», Notes «2», у каждой своя отметка (Tasks снята), столбец
  «Changed on the server» один на три строки («yesterday»). Модель: `Kinds`
  списком — ни счёта по роду, ни отметки, ни «Changed».
- **Нс27. Панель подписки (`s-profile-10`).** Кадр против `feedForm` (по
  коду):
  - кнопка «Check the address» — своей строкой под адресом, след проверки —
    `tc-diag` внутри формы; у модели след — `Notice` страницы с `Details`;
  - «Name here», подсказка «Taken from the feed; change it if you like —
    other people’s feed names are often longer than the rail.»; у модели —
    «Taken from the feed when left empty. The name is yours and is never
    sent to the feed.»;
  - «Label colour» — пять меток и «derived from the address» с подсказкой;
    у модели поля цвета нет;
  - отметка «This feed asks for a password» с подсказкой; у модели — два
    всегда видимых поля «Login of the link, if it asks for one» и «Password
    of the link»;
  - «Sections» — «Calendar · 118», «Tasks · 6», «Notes · 2» и подсказка
    «Not a choice: …»; у модели этого нет (счёт — только в теле
    уведомления);
  - тревожное сообщение «The feed is fetched by the Carrel server, not by
    your browser. An address on your local network works only if…»; у
    модели первая фраза — хвост подсказки адреса;
  - подвал Subscribe и Cancel, в шапке панели закрытие; у модели Cancel
    нет.
- **Нс28. Подсказка Server URL.** `s-profile-2`: «‹.well-known› is tried
  first, then the URL exactly as typed — which is the reliable path for
  Baikal.»; `s-profile-14`, `-16`: «…exactly as typed.»; `s-profile-11` —
  подсказки нет. Модель: всегда короткая. Кадры расходятся между собой
  (В8).
- **Нс29. Пароль показан (`s-profile-4`).** Кадр: в карточке поля «User
  name» и «Password» со значением, кнопки Copy и «Hide». Модель (по коду,
  `revealPassword`): `Password.Shown`, `Value`, `Reveal.Label` — «Hide
  password». Привести подпись к «Hide»; фикстуры нет.
- **Нс30. Состояния без фикстур.** Диалог Connect account (`s-profile-2`),
  с отметкой «The server asks no password» (`-14`), правка (`-16`), занятая
  подпись (`-11`: «There is already a connection called “nextcloud”. Choose
  another label: it is how your sources are told apart.» — по коду слова те
  же), панель подписки (`-10`), открытая выдача (`-4`), карточка без пароля
  (`-15`: «… · mix · no password · last answer 2 min ago», без Show
  password), хранилище целиком (`-17`: Rename и «Delete on the server…»
  выключены с «This is the whole storage, not a folder in it.», Move нет —
  по коду так), след «Test only». Ни одного в фикстурах.
- **Нс31. Лист Disconnect.** `ConnectionRemoveSheet` есть в контракте и в
  коде, каталога фикстур нет, кадра нет (В9).
- **Нс32. Данные набора.** Кадр: «3 servers · 12 collections», логин
  «mix», у карточек «last answer 2 min ago», «Changed on the server»
  заполнен. Фикстура: «4 servers · 17 collections», логин «alice»,
  `answered`, `changed`, `lastRead` пусты у всех (сеанс ничего не читал),
  подписки без `checked`, `size`, `took`. **данные**

### Е. Вложения (`AttachmentsPage`)

- **Нс33. Save и «Open in Files».** Кадр `attachments-1`: в шапке одна
  кнопка «Open in Files», кнопки сохранения на экране нет. Модель: обе —
  в `form.actions` («Save», «Open in Files»). Нужно: «Open in Files» —
  действием шапки страницы, не формы; про Save — В2.
- **Нс34. «Small files stay in the record».** Кадр: пары «Up to 2 MB —
  written into the event, the task or the note itself, and travels with it»,
  «Larger — put in the folder below; the record keeps a link to it»
  (со строчной, без точки) и одна подсказка: «A collection that says it
  takes less (‹max-resource-size›) lowers the boundary for itself:
  ‹nextcloud› 2 MB, ‹radicale · vps› 1 MB. Nothing here raises it.» —
  предел назван по серверам, внутри фразы. Модель: значения с прописной и с
  точкой; подсказка без пределов, а пределы — десятью строками
  `limit.‹подключение›.‹путь›`, по строке на коллекцию («nextcloud · Дела —
  2 MB»…). Нужно: слова по кадру, пределы — по подключениям кусками одной
  фразы.
- **Нс35. Вариант «только для чтения».** Кадры `attachments-2`,
  `s-backup-9`: у варианта справа значок «ro». Модель: `Option.Disabled` и
  `Reason` «Read-only» — признака «только для чтения» у `Option` нет (у
  `MenuItem` он есть). У вариантов `dest` задания нет и цвета
  (`Option.Colour` пуст), кадр `s-backup-9` рисует метки.
- **Нс36. «In this folder now» — моноширинно.** Кадр: значения Files,
  Oldest, Path — `tc-mono`. Модель: строки `now.files`, `now.oldest`,
  `now.path` без `mono`.
- **Нс37. Лист Browse без фикстуры.** `attachments-4`: «Attachment folder»,
  «Where large attachments go: a folder of a file collection you can write
  to. Carrel makes it on first use.», дерево, «Choose this folder». По коду
  (`files_picker.go`) слова те же и причина «Choose a folder in the tree.»
  есть; фикстуры `app-files-picker/attachments` нет.
- **Нс38. Данные набора.** Кадр: третий вариант Collection — «sftpgo · vps
  · Public files», цвета меток другие; Files «128 · 1.4 GB», Oldest
  «2026-02-19», Path «https://files.example.org/dav/Documents/carrel/attachments/».
  Фикстура: «sftpgo · vps · Public»; «5 · 4.4 MB», «2026-07-01»,
  «…/dav/alice/files/Documents/carrel/attachments/». **данные**

### Ж. Новое устройство (`DevicesPage`)

- **Нс39. Отметка аккаунта.** Кадры `s-plan-1`, `-2`: у строки аккаунта —
  отметка (у radicale снята), «профиль берёт аккаунт целиком». Модель:
  признак — `DeviceRow.Included` у каждой коллекции, а переключатель —
  действие страницы `device.exclude` / `device.include` с подписью «Leave
  out nextcloud»; с аккаунтом его связывает только подпись. Нужны признак и
  действие у `DeviceAccount`.
- **Нс40. Выданная ссылка.** Кадр `s-plan-1`: в правой колонке карточка
  «Link ready until 09:46» со значком «local», подсказка «It opens once, for
  you signed in to Carrel. Open it on the device, install the profile, then
  delete the file.», кнопки «Download profile» и «Revoke» (тревожная).
  Модель: это `Notice` страницы (роль accent, те же слова, два действия) —
  только в ответе на POST, после перезагрузки его нет (по коду); у Revoke
  нет `danger`. Нужно поле страницы.
- **Нс41. Метка коллекции.** Кадры `s-plan-4`, `-5`: у строки коллекции —
  цветная метка. Модель: у `DeviceRow` нет `Colour`.
- **Нс42. Строка аккаунта таблицы.** Кадры `s-plan-4`, `-5`: «**nextcloud**
  login ‹mix› password ‹z3-brick-lamp-9f2› [Copy password]» — подпись
  подключения жирным отдельно от логина. Модель: `Title` — «nextcloud ·
  alice» (подпись и логин одной строкой), `Login` отдельно; подписи без
  логина нет. На вкладках Apple и Android кадр пишет именно «nextcloud ·
  mix» — там сходится.
- **Нс43. Состояния.** Фикстура одна — Apple, выдача закрыта, ссылки нет;
  кадра такого нет (все четыре — с открытой выдачей). Нет фикстур: Apple с
  выдачей и ссылкой (`s-plan-1`), Android с кодами (`-2`), Thunderbird
  (`-4`), Anything else (`-5`).
- **Нс44. «Anything else»: модель несёт больше кадра.** `s-plan-5` под
  таблицей рисует одно сообщение — «Credentials are unlocked for five
  minutes — here and everywhere else they are handed out. …» (`UnlockNote`).
  По коду на этой вкладке приходит и `Warning` — «A WebDAV address is for
  "Map network drive" in Windows…»; в кадрах этих слов нет. Так же на
  вкладках Thunderbird и Anything else приходят «Leave out …», а отметок
  там кадр не рисует.
- **Нс45. Данные набора.** Кадр: аккаунты «nextcloud · mix» (Личный,
  Работа, Контакты), «radicale · vps · mix» (Проекты); фикстура — три
  аккаунта «… · alice» с 9, 4 и 1 коллекцией. **данные**

### З. davloom (`DavloomPage`)

- **Нс46. Заголовок.** Кадр `s-davloom-1`: «davloom» и значок «local».
  Фикстура: `title` «DAV proxy».
- **Нс47. Добавление — отдельное состояние.** Кадр: на странице списка
  формы нет, в шапке основная кнопка «Add device»; `s-davloom-2` — диалог
  «Add device» с подзаголовком «An app password for one device, with its own
  key wrapper». Модель: `Add` — не указатель, форма добавления лежит в
  каждом ответе (в фикстуре — тоже), действия «Add device» и состояния
  диалога нет.
- **Нс48. Диалог добавления.** Кадр `s-davloom-2` против `deviceForm`:
  подпись группы «What the device will see — changed later too, without
  re-issuing the password» и вторая — «Collections» (у модели один набор,
  строки подряд, подписей групп нет); у родов приглушённый хвост отдельно
  от подписи — «Tasks · VTODO — not every client understands them» (у модели
  одной строкой `label`, хотя `LabelAside` есть); тревожное сообщение «The
  password is shown once and stored nowhere. Lose it and you create a new
  one and revoke this — it cannot be recovered.» (у модели нет); Cancel в
  подвале (у модели нет).
- **Нс49. Пароль выдан (`s-davloom-3`).** Кадр: заголовок — имя устройства
  («Телефон · DAVx5»), подзаголовок «Password created · read and write · 8
  collections», карточка «Device password» с Copy и предупреждением «Shown
  once and stored nowhere. …», поля Address и User name, Done. Модель (по
  коду): та же форма `Add` с одной рубрикой «Телефон · DAVx5 · Password
  created · read and write · 8 collections» и тремя полями только для
  чтения; заголовок страницы остаётся «DAV proxy». Нужны заголовок,
  подзаголовок и выданный пароль своим полем.
- **Нс50. Строка устройства.** Кадр: полоски коллекций устройства (четыре
  цветные метки или одна составная при «all kinds») и «5 collections ·
  Change». Модель: `Scope` — роды, счёт — внутри подписи действия
  `davloom.scope` (по коду), цветов коллекций нет.
- **Нс51. «What the client sees».** Кадр: дерево — `/dav/`, `principal/`,
  `cal/`, `card/`, `files/` и под ними коллекции с адресом
  («c/k4f2/%2F…%2Fpersonal%2F/»), именем и меткой. Модель: ничего.
- **Нс52. Состояния.** Фикстура — устройств нет (кадра нет: что стоит на
  месте таблицы, не нарисовано). Нет фикстур: с устройствами
  (`s-davloom-1`), диалог (`-2`), выданный пароль (`-3`); правка области
  (`?edit=`) и «davloom is off» — ни кадра, ни фикстуры (В10).
- **Нс53. Данные набора.** Кадр: адрес «https://carrel.example/dav/»;
  фикстура — «http://127.0.0.1:8081/». В диалоге кадр рисует 6 коллекций,
  Tasks и Notes сняты; у модели отмечено всё (17 коллекций). **данные**

### И. Резервная копия (`BackupPage`)

- **Нс54. Фикстура — не то состояние.** `app-settings-backup/normal` —
  «заданий нет» (`subtitle` «No jobs yet», `empty`, `emptyNote`), совпадает
  с `s-backup-6`; `check.py` ставит её к `s-backup-1`. Нужны строка
  `STATE_FRAME` и фикстура списка с заданиями. На пустом списке модель несёт
  ещё `add` (вся форма нового задания) и `restore.source` (форма первого
  шага) — кадр `s-backup-6` не рисует ни того, ни другого, а признака
  «закрыто» у `Add` нет (у восстановления есть `Open`).
- **Нс55. Строка задания.** Кадр `s-backup-1` против `BackupJob`:
  - отметка «задание включено» первым столбцом, выключенное — приглушённой
    строкой; у модели признака нет («Job is on» — только поле формы);
  - «What it copies»: цветные метки и итог — «3 collections», «2
    collections + files», «1 book»; у модели `Sources` — подписи без цвета и
    рода;
  - «Where to» двумя строками — «sftpgo · vps · Documents» / «/backup/work»,
    у скачивания «download in browser» / «nothing is written»; у модели одна
    строка `Destination` («… /backup/work», «Download in the browser —
    nothing is written to a server», по коду);
  - «When» — «cron · 03:00» / «external trigger», «on sign-in» / «if older
    than 7 days», «on demand»; у модели такого поля нет;
  - Keep «—» у скачивания; у модели `Keep` — число;
  - «Last backup» — «2026-08-16 03:10» и строкой ниже «18.4 MB · verified»
    с подсказкой, у незапускавшегося «never»; у модели `LastRun` и
    `LastState`, размера и «verified» нет.
- **Нс56. «History, all jobs».** Кадр: одна таблица на все задания со
  счётом «12» — When, Job, Size, Collections («3 / 3», тревогой «2 / 3»,
  «2 / 2 + 1 240 files»), Verified («read back» или тревогой «baikal ·
  Общий: 401»), Trigger («cron», «on sign-in»), кнопки Restore и Download.
  Модель: `Runs` внутри каждого задания — `ID`, `Started`, `State`, `Size`,
  `Download`; нет счёта коллекций, причины, рода запуска, Restore у строки.
  Неполная копия у модели — ещё и `Notice` страницы («The last copy of «…»
  is not whole»), кадр его не рисует.
- **Нс57. Форма задания — рубрики.** Кадр `s-backup-2`: без рубрики Name и
  «Job is on», дальше рубрики «What to copy», «Where to», «How the archive
  is closed», «When». Модель: один набор без `legend`, строки подряд.
- **Нс58. «What to copy».** Кадр: у рубрики итог «2 collections and 1
  folder chosen · 2.1 GB last time»; дерево — аккаунт с трёхпозиционной
  отметкой и счётом («nextcloud — 4 of 6»), под ним коллекции с меткой и
  справа род и прошлый объём («cal.», «book», «notes», «files · 2.0 GB»,
  «files · ro»); сообщение «…and are copied with them — they cannot be
  picked separately. The size beside each collection is what it took last
  time.». Модель: `src` — плоские варианты «nextcloud · Дела» без цвета,
  группы, рода, объёма и «ro»; подсказка кончается на «…and are copied with
  them.».
- **Нс59. Слова формы.** Кадр: «Copies past the eighth are deleted
  **after** the new one has been read back and checked, not before. The file
  is dated and never overwritten.»; модель: «Copies past this number … An
  archive is dated and never overwritten.». Кадр: «…and then any archiver
  opens it.»; модель: «…opens it, without Carrel and without a password.».
  Слова о пароле архива кадр пишет тревожным сообщением под полем, модель —
  подсказкой поля (а у задания с паролем подменяет их на «A password is set.
  Leave the field empty to keep it.»). «Created on first use.» под Folder
  кадр не рисует.
- **Нс60. Рубрика «When».** Кадр: три отметки — «On demand — the button in
  the job list», «On sign-in, if the last copy is older than [7] days»
  (отметка и число), «On a clock of your own — ‹cron›, GoSentry, a job in
  Gitea». Модель: «On demand» нет; вход — одно число «On sign-in, if the
  last copy is older than (days)» с «0 is never. …»; третья отметка — не
  строка формы, а `BackupJob.Clock` в списке `Jobs`.
- **Нс61. Какое задание открыто.** На странице задания `Clock`,
  `ClockHint` и `Trigger` лежат в `Jobs[i]`, а у `BackupJob` нет
  идентификатора: найти открытое задание среди всех можно только по
  совпадению `Name` с `Title`. Нужно открытое задание своим полем страницы.
- **Нс62. Где появляется карточка строки.** Кадры `s-backup-2…5`, `-10`,
  `-11`: карточка «The line to paste — visible only now» — на странице
  задания, под отметкой. По коду ответ на включение (`op=trigger`) —
  `backupPageIssued` без открытого задания, то есть **список**; выключение
  тоже уводит на список.
- **Нс63. Слова карточки.** Кадр `s-backup-4`: Name «Carrel · personal»,
  `s-backup-10`: «name: Carrel backup · personal» — по идентификатору
  задания из адреса (`job=personal`); модель (по коду) ставит имя задания
  («Carrel · Личное на домашний диск»). Кадр `s-backup-5`: «Every use is
  logged, naming this password. **New password** replaces it on the spot —
  and every line already handed out stops working with it.»; модель,
  `Note`: «Visible only now. Every use is logged; New password replaces it
  on the spot, and every line already handed out stops working.». Кадр
  `s-backup-3`: «Destination: **WebDAV and download** — two steps, so this
  is a file, not a line» — у `TriggerBlock` только `Script`, слов назначения
  нет.
- **Нс64. Восстановление, шаг 1 (`s-backup-7`).** Кадр: заголовок
  «Restore», подзаголовок «Nothing is written until you have seen what goes
  where»; «Where the archive is» — «From a job», «From a server», «From this
  computer» с подсказкой «One archive, one of three. The other two stay
  empty until you switch.»; копии задания — таблицей (имя, «2026-08-14
  09:12», «2.1 GB», «4 collections»); под паролем — «The one set in the job,
  not your sign-in password. Without it the manifest cannot be read — Carrel
  will not even see the list of collections.»; подвал «Open the manifest» и
  Cancel. Модель: заголовок остаётся «Backup»; «Which archive» — «From a
  job's history», «From a storage (WebDAV)», «From this computer»; `run` —
  список с подписью «имя · когда» (без размера и счёта); подсказка пароля
  «The job's own password. An archive that is not encrypted needs none.»;
  кнопки «Open the manifest» и «Open a file from this computer», Cancel —
  только когда архив уже открыт.
- **Нс65. Восстановление, шаг 2 (`s-backup-8`).** Кадр: заголовок «Restore
  from 2026-08-14». Модель: день архива отдельно не несёт (он внутри
  `Restore.Subtitle`), заголовок — «Backup». Остальное — `Selected`, полоса,
  строки с `Pick`, `Into`, `Will`, `Danger`, `Note` — сходится по коду;
  фикстуры нет.
- **Нс66. Состояния без фикстур.** Список с заданиями (`s-backup-1`),
  задание (`-2`), задание «Both» со скриптом (`-3`), карточка на четырёх
  вкладках и пароль отдельно (`-4`, `-5`, `-10`, `-11`), новое задание
  (кадра тоже нет), восстановление открыто (`-7`), сопоставление (`-8`).

### К. Внешний вид (`AppearancePage`)

- **Нс67. Часовой пояс.** Кадр: одно поле только для чтения со значением
  «Europe/Moscow · this instance». Модель: `value` «Europe/Moscow», `hint`
  «This instance».
- **Нс68. Слова под рубрикой.** Кадр: «…so another browser sees the same.
  Time zone is one per instance, set by the administrator — an event shown
  in two zones to two people is a source of mistakes, not a convenience.».
  Модель, `notes[1]`: «Time zone is one per instance, set by the
  administrator.» — без хвоста.
- **Нс69. Save.** Модель: `account.actions` — «Save». Кадр `appearance-1`
  кнопки не рисует ни у «Dates and times», ни где-либо ещё (В2).

---

## 3. Клиенту

Неизменные слова и раскладка, серверу не нужны:

- **Учётная запись.** Рубрики Password, Sessions, Key escrow; столбцы
  Where, Started, Last seen; счёт «2 active» — длина `Sessions`; «this one»
  по `Current`; у своего сеанса ячейка действия пуста (у модели End выключен
  с причиной — В3); Delete account и отзыв ключа — `ButtonDangerSolid`,
  End и «End every other session» — тревожные с рамкой; Save и Change —
  `FormRow.Extra` в строке поля.
- **Депонирование, пароль.** Страницы — `tc-dialog` шириной 560 и 520;
  пары Deposited и Last recovery моноширинно; лист депонирования 480,
  подтверждения 400.
- **Подключения.** Рубрики «DAV servers» и «Subscriptions · external,
  read-only» со счётом; столбцы таблиц, «Collection» против «Folder» — по
  роду коллекций; слова рода («calendar», «address book», «journal»,
  «tasks», «files») и «ro» — по `Kinds` и `ReadOnly`; «last answer» перед
  `Answered`; значок «feed»; шапка панели подписки («Subscribe to a feed»,
  «Somebody else’s calendar or address book, read-only») — пока `Add`
  пришёл, а `DialogTitle` пуст; порядок кнопок карточки (New collection,
  Show password, Test, Edit, Disconnect) — у модели другой; Copy у
  показанного пароля.
- **Вложения.** Подзаголовок «Where a file attached to a note, an event or
  a task is put» (у модели поля нет — слова экрана); наборы только для
  чтения — `tc-fields`; сообщение `Note` под формой.
- **Новое устройство.** Подзаголовок и рубрика левой карточки — по `Tab`
  («Everything discovery found — as one file or one code», «DAVx5 ·
  collections, login and password», «Thunderbird · Calendar and Address Book
  URLs», «Exact strings for any client»; «What goes into the profile»,
  «Collections DAVx5 will see»); имена коллекций и «CalDAV, CardDAV» строки
  аккаунта — из `Rows`; «Address and login are in the code.»; сокращение
  адреса до «…/remote.php/dav/…» в таблице; значок «local» — по
  `Signed.Local`.
- **davloom.** «Address for the client» с подсказкой и Copy; счёт
  «Devices 2»; слова области («Calendars, Address books», «all kinds») — по
  `Scope`; переключатель прав — два действия с `On`.
- **Резервная копия.** Столбцы обеих таблиц; строка «verified — the archive
  was read back…» под списком; сообщения «**Download** streams the archive
  out…» и «No time picker here…»; неизменные слова карточки запуска на
  каждой вкладке (заголовки «The line to paste», «The fields to paste», «The
  workflow to paste», «The request to send», «The script to paste», подписи
  кнопок Copy, «Save as», пояснения), описания входов «What it opens»; шаги
  «1 · Archive», «2 · Match», «3 · Report»; порядок кнопок шапки задания
  (Cancel, Run now, Save); значок «local» у отметки запуска.
- **Внешний вид.** Наборы Theme, Density, Lists целиком — этого браузера
  (U-12; `browser` в фикстуре пуст, и это верно); рубрика «Dates and
  times»; подзаголовок.

---

## 4. Вопросы к кадрам (кандидаты задач Typecase, ведущему)

- **В1.** Ни один кадр настроек не рисует отправку: нет форм, кнопок
  `submit`, имён полей (Нс1). У кадров файлов после записок 38–42 они есть.
  Без этого родство и смысл на экранах настроек будут красными на каждом
  `Action`, `Submit`, `Send`.
- **В2.** Экраны без кнопки сохранения: `attachments-1` (четыре поля и ни
  одной кнопки, кроме Browse и «Open in Files») и `appearance-1` («Week
  starts on», «Time»). Без сценариев набранное не уходит; модель в обоих
  случаях несёт «Save».
- **В3.** Свой сеанс в `account-1`: ячейка действия пуста. Правило
  владельца — выключить с причиной, и модель так и делает (End с «This is
  the session you are in: sign out to end it.»). Кадр или правило?
- **В4.** Удаление учётной записи: кадра подтверждения нет (Нс12).
- **В5.** История депонирования: U-6.11 называет у `/app/settings/escrow`
  `tc-table` (история), кадры её не рисуют (Нс15).
- **В6.** `account-3` рисует обычный и вынужденный случаи в одном кадре
  (красная строка вместе с «Back to settings»), по подписи кадра вместе они
  не бывают.
- **В7.** «Look for changes» (повторное обнаружение) названо в U-6.11, в
  макете отсутствует (Нс24). Обнаруженные изменения сервер показывает
  уведомлением (`rediscoveryNotice`) — кадра тоже нет.
- **В8.** Кадры подключений расходятся между собой: подсказка Server URL в
  трёх видах (Нс28); подзаголовок `s-profile-4` — «3 servers · 12
  collections · 2 subscriptions» без «Signed in as **mix**», что стоит в
  `s-profile-1`, `-10`; значок «credentials unlocked · 4:12» и Lock now в
  `s-profile-4` — справа в шапке, в `s-plan-1…5` — внутри подзаголовка;
  карточка с показанным паролем в `s-profile-4` — только Test и Edit, без
  New collection и Disconnect, и другой вёрсткой, чем в `s-profile-1`;
  карточка `s-profile-15` — третьей вёрсткой (`tc-card__line`,
  `tc-card__acts`).
- **В9.** Лист Disconnect («что именно уйдёт») назван в U-6.11, кадра нет;
  в образце кнопки макета (строка 2985) Disconnect шлёт POST сразу, без
  листа. Отписка от ленты (Unsubscribe) у модели — тоже POST без вопроса.
- **В10.** Состояния без кадра: подключений нет вовсе; папка вложений не
  выбрана и «The folder for attachments is not available»; новое устройство
  с закрытой выдачей (точки и «Show» названы только в подписях кадров);
  davloom без устройств, правка области («Change») и «davloom is off»;
  новое задание; карточка запуска, когда секрет уже не виден (все кадры —
  «visible only now»); карточка задания «Download»; восстановление «From a
  server» и «From this computer»; шаг «3 · Report» (у сервера итог —
  уведомление на списке).
- **В11.** Лист `s-plan-3` и лист `s-profile-3` — один маршрут, разные
  слова («…passwords, device profiles, anything else handed out.» против
  «…passwords, device profiles, key escrow — anything handed out or handed
  over.») и разный подвал (у `s-profile-3` есть «Enter»). Лист
  `attachments-4` — без выбора сервера в шапке, который после ФН9 несут все
  листы с деревом.
- **В12.** Вкладки карточки запуска (crontab · GoSentry · Gitea · curl) и
  переключатель «Where the archive is» восстановления — без адреса: модель
  отдаёт всё разом, переключает сценарий. Сказано ли это о кадре?
- **В13.** Мелочи данных кадра: в `s-backup-2` «Записи» стоит под «sftpgo ·
  vps», «Дежурства» — под «radicale · vps» без отступа группы; в
  `s-mail-9` у Display name нет кнопки Save, которая есть в `account-1`
  (подпись кадра говорит, что «других различий нет»).

---

## 5. Не проверено

- Сборка экранов, сверка, родство и смысл не запускались: скан чтением,
  ничего не собиралось и не менялось.
- Слова состояний без фикстуры взяты из обработчиков, а не из ответа
  сервера: текст сверен глазами, полного JSON этих состояний я не видел.
- `ConnectionRemoveSheet`, `collectionMoveItems`, `collectionDeleteItem`,
  `restoreMatch` целиком и разбор проверки подключения (`davcheck.go`) не
  читались — о них сказано только то, что видно из вызовов.
- Рейл и шапка сверены по `s-profile-1`, `account-3`, `-4`; у остальных
  кадров раздела рейл не перечитывался.

---

## 6. Счёт

- Пробелов серверу — **69** (Нс1–Нс69).
- Из них только данные набора — **6**: Нс6, Нс8, Нс32, Нс38, Нс45, Нс53.
- Только «нет фикстуры» при совпавших по коду словах — **9**: Нс9, Нс11,
  Нс13, Нс16, Нс17, Нс30, Нс37, Нс43, Нс66.
- Настоящих расхождений модели и кадра — **54**; из них меняют модель или
  маршруты (ждут слова владельца или ведущего) — Нс1, Нс14, Нс21, Нс25–Нс27,
  Нс39, Нс40, Нс47, Нс49, Нс51, Нс55, Нс56, Нс58, Нс60–Нс62; остальное —
  слова, поля и фикстуры.
- Вопросов к кадрам — **13** (В1–В13).
