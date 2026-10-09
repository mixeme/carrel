# Скан листов записи и прочих листов (U-6.16, U-6.17)

Скан только чтением: кадры Typecase `0fe0244` против сервера `v0.2.174`
(`6a62d8a`). Разобраны листы и фрагменты, названные в U-6.16 и U-6.17
`carrel-client.md`, кроме уже разобранных или чужих: листы файлов
(`app-files-op`, `app-files-picker`), коллекций (`app-collections-*`),
публикации и письма (`app-publish`, `app-mail-compose`, с ним и
`MailAttachSheet`), импорт, дубликаты, конфликт. Фрагменты U-6.17 `PollBar`,
`Rail`, `TransferQueue`, `FileOpSheet` здесь не разбирались — они сканируются
со своими экранами (допущение исполнителя).

**Метод.** Кадры-экраны — черновиком `go tool frame2go -self` (`s-notes-5`,
`-6`, `-8`, `-9`, `-10`, `s-contacts-2`, `-15`, `-16`, `-5`, `-27`,
`s-agenda-15`, `account-2`, `s-plan-3`, `s-profile-3`) против фикстуры;
кадры-фрагменты — чтением HTML (`frame.py`) против фикстуры. Где фикстуры
состояния нет, модель читана по коду сервера (`internal/web/record_edit.go`,
`record_move.go`, `rewrite_attendees.go`, `handlers_app.go`,
`handlers_escrow.go`, `handlers_admin.go`, `photo.go`, `import.go`,
`connection_disconnect.go`) — такие пункты помечены «по коду». Сервер не
запускался.

## Что с чем сверено

| Лист | Маршрут | Вью-модель | Кадры | Фикстуры |
|---|---|---|---|---|
| Быстрое создание, вход из шапки | `/app/{kind}/quick` | `NewSheet` | `s-notes-5` (лист уже на коллекции, с полем), `s-notes-6` (писать некуда) | `app-quick/normal` — состояние «куда», без полей: **кадра у него нет**; у `s-notes-6` фикстуры нет |
| Быстрое создание на коллекции | `/app/{kind}/{account}/{col}/new/quick` | `NewSheet` | `s-notes-8` (контакт), `-9` (событие), `-10` (задача); списки `s-notes-16`, `-17`, `-19` («куда»), `-18` (Remind) | **каталога нет** — ни одного рода |
| Перенос и копия записи | `…/{uid}/move` | `MoveSheet` | `s-contacts-2`, `s-contacts-17` (копия) | `app-contact-move/normal` — ничего не выбрано: кадра нет; выбранной цели, копии, цели на другом подключении — нет |
| Переписать адрес | `…/{uid}/rewrite` | `RewriteSheet`, итог — `ContactPage.Notices` | `s-contacts-15`, `-19`, `-20`; итог `-16`, `-21` | `app-contact-rewrite/normal` — данные кадра `-19`; `-15`, `-20`, итога — нет |
| Пароль перед выдачей | `/app/settings/unlock` | `UnlockSheet` | `s-plan-3`, `s-profile-3` | `app-settings-unlock/normal`; отказа пароля — нет ни кадра, ни фикстуры |
| Депонирование ключа | `/app/settings/escrow/deposit` | `UnlockSheet` | `account-2` | своего каталога нет; тот же лист — `confirm` в `app-settings-escrow/normal` |
| Отзыв ключа | `/app/settings/escrow/withdraw` | `EscrowWithdrawSheet` | `account-11` | нет |
| Отключение подключения | `/app/settings/connections/disconnect` | `ConnectionRemoveSheet` | **нет** | нет |
| Сброс пароля | `/admin/reset` | `PasswordResetSheet` | `s-admin-13`, `-14`, `-15`, `-16`, `-20` | нет |
| Экспорт из сведённого вида | `/app/{kind}/export` | `ExportSheet` | **нет** | нет |
| Кадрирование фотографии | `…/{uid}/photo-preview` | `PhotoPreviewPage` | `s-contacts-5` | `app-photo-preview/normal` |
| Подсказки участников | `/app/contacts/suggest` | `SuggestList` | `s-agenda-12`, `-13` | `app-contacts-suggest/normal` (к `s-agenda-12`); `-13` — нет |
| Полная форма создания | `…/new` | `ContactEditPage`, `EventEditPage` | `s-contacts-27`, `s-agenda-15` | `app-contact-new/normal`, `app-event-new/normal` (задача и заметка — состояния `new` каталогов правки, здесь не разбирались) |

## Серверу

### Быстрое создание (`NewSheet`)

- **Лс1.** Фикстуры. Кадры `s-notes-5`, `-8`, `-9`, `-10` рисуют лист с
  выбранной коллекцией и полями — ответ `…/new/quick`; у этого маршрута нет
  ни каталога фикстур, ни строки в `FIXTURE_SCREEN`. Единственная фикстура
  `app-quick/normal` — лист без полей (`form` пуст, `target.chosen` пуст,
  `record.new` выключено с «Choose the collection the new note goes to.») —
  состояние, которого не рисует ни один кадр; `check.py` держит её по
  маршруту `/app/{kind}/quick`, чей кадр `s-notes-5` — другое состояние.
  «Писать некуда» (`s-notes-6`) фикстуры не имеет. Нужно: фикстуры листа с
  полями на четыре рода и «писать некуда», строки `STATE_FRAME`.
- **Лс2.** `s-notes-6`, сообщение листа: «There is nowhere to write. Connect
  an account that has a journal, or create a notebook after connecting — the
  note is not kept in this window.» Слова зависят от рода (journal, notebook,
  note). Модель в этом состоянии (по коду, `getKindQuickEntry`) — только
  `kind` и `actions` (`connection.add`, `record.cancel`); `note` пуст, другого
  текстового поля у `NewSheet` нет. Нужно: текст сообщения в модели.
- **Лс3.** Выбор «куда», пока коллекция не выбрана. Кадр: `Pick` в шапке,
  пункты — «nextcloud · Контакты» с меткой, «ro» у недоступной (`s-notes-16`,
  `-17`, `-19`). Модель: `target.options[].id` — адрес **полной формы**
  (`/app/notes/a-nc-home/dnevnik/new`), `href` у вариантов нет вовсе. U-6.16:
  «Выбрали — открывается тот же лист уже на коллекции». Нужно: у доступного
  варианта `href` на `…/new/quick` (с несомым запросом), как у
  `MoveSheet` и `ExportSheet`.
- **Лс4.** Смена коллекции в листе с полями (по коду). Кадр: тот же `Pick`
  в шапке заполненного листа (`s-notes-5`: набран текст, выбрано «nextcloud ·
  Записи»). Модель: варианты — те же адреса `/new` без `href`; форма листа
  шлётся на адрес своей коллекции (`form.action` = `…/new/quick`), поля
  «куда» в ней нет. Сменить коллекцию, не потеряв набранное, без сценариев
  нечем. Нужно: «куда» полем формы или адресом отправки (`Submits`) у
  варианта — решение за ведущим, см. вопрос В2.
- **Лс5.** Лишнее действие (по коду, `quickSheet` поверх `newSheet`). Лист с
  полями несёт `actions: [record.new «New note», GET …/new, primary,
  enabled]` рядом с `form.actions` (Save, Open the card, Cancel). Кадр рисует
  в подвале три кнопки формы и никакой «New note»; основных действий в модели
  два. Нужно: `actions` листа с полями пуст.
- **Лс6.** Событие, поле Remind (по коду, `recordFormOf`: «only the reminders
  keep their own set»). Кадр `s-notes-9`: Remind — шестое поле одной строки
  с Title, Date, Start, End, All day, рубрики нет. Модель: строка
  `reminder.0` — в своём `FieldSet` с `legend` «Reminders». Нужно: в листе
  без `legend`.
- **Лс7.** Значки кнопок. Кадры: «Open the card» со значком `expand`
  (`s-notes-5`, `-8`, `-9`, `-10`), «Connect account» со значком `server`
  (`s-notes-6`). Модель (по коду): `record.open` и `connection.add` без
  `icon`.
- **Лс8.** Заголовок листа: «New note», «New contact», «New event», «New
  task». У `NewSheet` поля `title` нет (у `MoveSheet`, `CollectionNewSheet`,
  `RewriteSheet` — есть); в состоянии «писать некуда» нет и действия
  `record.new`, из подписи которого его можно было бы взять. Нужно: `title`.
- **Лс9.** Cancel. U-6.16: лист «вызывается с любого экрана, и экран под ним
  остаётся». Модель (по коду, `cancelAction`): `record.cancel` — GET на
  список коллекции листа (`c.listPath()`), у «писать некуда» — на
  `/app/{kind}`; адреса экрана, с которого лист открыт, лист не несёт (у
  `UnlockSheet` для этого скрытое `back`). Без сценариев Cancel уводит не
  туда, откуда пришли. Нужно: `back` и у этого листа.

Сошлось (по коду): подписи и образцы полей — Display name, Phone «+7 …»,
Email «name@example.org»; Title, Date, Start, End, All day, Remind; Summary,
Due date «YYYY-MM-DD», Due time «--:--»; строки под полями трёх родов слово
в слово; список Remind (`s-notes-18` — семь пунктов с чертой перед «No
reminder», сверено по `app-event-new`); «Save contact / event / task /
note»; «Open the card» — POST на `…/new/quick?op=open`.

### Перенос и копия записи (`MoveSheet`)

- **Лс10.** Подпись справа у строки цели. Кадр `s-contacts-2`, `-17`: «Архив»
  — «nextcloud», «Команда» — «radicale»: имя сервера, моноширинно. Модель:
  `label` «Архив» и `stamp` «nextcloud · Архив» (сервер и коллекция одной
  строкой); имени подключения отдельно нет, резать штамп экрану нельзя.
  Нужно: подпись подключения у варианта отдельным полем.
- **Лс11.** Текущая против «только для чтения». Кадр: у текущей — «already
  there» моноширинно на месте сервера, у недоступной для записи — бейдж
  «read-only». Модель: обе — `readOnly: true` с `reason` («already there»,
  «read-only»); признака «это текущая» нет, различить их можно только по
  словам причины. Нужно: признак текущей коллекции.
- **Лс12.** Фикстуры. Кадр `s-contacts-2` — цель выбрана (строка `is-on`),
  кнопка включена; фикстура `normal` — ничего не выбрано, `Move` выключено с
  «Choose a collection.» (кадра у этого состояния нет). Нет фикстур: цель
  выбрана (`target.chosen`, `href` у подтверждения); копия (`s-contacts-17`:
  «Copy contact», сообщение «A copy is written into the chosen address book
  whole, with the same UID; the card here stays as it is.» — по коду слова
  те же); цель на другом подключении (подпись `s-contacts-17`: «сообщение об
  этой цели, как у файлов» — по коду есть, «This goes to another account…»,
  кадра этих слов нет).
- **Лс13.** Данные. Кадр: Контакты (текущая, первой), Архив, Команда
  («radicale»), Shared (read-only). Фикстура: Архив, Контакты, Команда
  (read-only, «radicale · vps»), Рассылки. Порядок у сервера — порядок
  каталога; первой ли стоит текущая по правилу или по данным кадра — не
  проверено.

Сошлось: `title` «Move contact», `subject` «Анна Ковалёва · from nextcloud ·
Контакты», сообщение с моноширинным `MOVE` слово в слово, метки цветом,
Cancel на карточку.

### Переписать адрес (`RewriteSheet`)

- **Лс14.** Фикстуры. `app-contact-rewrite/normal` — данные фрагмента
  `s-contacts-19` (одна запись, «Rewrite 1 record», «Общий · baikal — could
  not be read…»), а `check.py` держит её по кадру маршрута `s-contacts-15`
  (строки `STATE_FRAME` нет). Нет фикстур: `s-contacts-15` (четыре строки
  «What rewriting can set off», «2 records in Shared», «Rewrite 12
  records»); `s-contacts-20` («Rewrite 0 records» выключена, причина
  «Nothing here can be rewritten: every record that names the old address is
  read-only or could not be read.» — по коду это `reason` действия); итог
  на карточке `s-contacts-16`, `-21` («Rewritten in 11 of 12 records.»,
  «Rewritten in 12 records.» — `ContactPage` с `Notice`, по коду слова те
  же).

Расхождений модели с кадрами не найдено: три блока, строка под ними, обе
кнопки со своими адресами и `csrf`, форма с адресом — есть. Не проверено:
кавычки «…» вокруг имени записи в строках отказа итога (`s-contacts-21`).

### Пароль перед выдачей и депонирование (`UnlockSheet`)

- **Лс15.** Заголовок и чей пароль. Кадры `s-plan-3`, `s-profile-3`:
  заголовок «Show password», справа в шапке — «nextcloud» (подпись кадра:
  «В шапке листа стоит его имя, чтобы „Show password“ с двух карточек подряд
  нельзя было перепутать»). Модель: ни заголовка, ни подключения; `purpose` —
  фраза «Confirm your password to reveal stored credentials, download a
  device profile, issue a davloom device password, change key escrow, or
  delete your account.», которой кадр не рисует. У депонирования то же поле
  `purpose` — «Deposit my key», и это заголовок (`account-2`): одно поле в
  двух ролях. Нужно: `title` и `subject` листа (что открывают и у кого).
- **Лс16.** Строка под полем. Кадр `s-profile-3`: «Unlocks credentials
  everywhere for five minutes: passwords, device profiles, key escrow —
  anything handed out or handed over.» Модель: `warning` — «Everything
  revealed in this window is written to the audit log.»; слов кадра нет.
  «five minutes» — настройка (`CARREL_UNLOCK_TTL`), слово от данных. Нужно:
  строка кадра со сроком экземпляра.
- **Лс17.** Cancel. Кадры: «Cancel» и Esc. Модель: `form.actions` — одно
  `unlock.submit` «Show»; выхода нет, скрытое `back` в фикстуре пусто (у
  листа депонирования `escrow.deposit.cancel` есть). Нужно: действие Cancel с
  адресом `back`.
- **Лс18.** Фикстуры. Нет: отказ пароля (ошибка у поля — по C-5.7 «лист
  страницей с ошибкой у поля»); лист с непустым `back`; депонирование своим
  каталогом (есть только как `confirm` страницы Key escrow); отзыв ключа
  (`account-11`).

Сошлось: депонирование `account-2` — заголовок, тревожное сообщение, подпись
и подсказка поля, «Deposit my key», Cancel — слово в слово; отзыв
`account-11` (по коду) — заголовок, сообщение, опасная кнопка, Cancel.

### Сброс пароля (`PasswordResetSheet`)

- **Лс19.** Образец в поле подтверждения. Кадр `s-admin-13`: в пустом «Type
  the login to confirm» образец «anna» — логин выбранного в Whose. Модель
  (по коду, `resetSheet`): строка `confirm` без `placeholder`.
- **Лс20.** Фикстуры: у маршрута `/admin/reset` нет ни одной — ни листа
  (`s-admin-13`), ни ответа с `issued` (`s-admin-14`), ни выключенных пунктов
  Whose (`s-admin-15`), ни отказов у полей (`s-admin-16`, `-20`).

Сошлось (по коду): предупреждение, подсказка временного пароля, причины
выключенных пунктов Whose, «Password must be at least 8 characters.» у поля
— не проверено дословно (текст даёт `formError`), «Type anna to confirm whose
password is reset.», ответ: сообщение с логином жирным, логин и пароль,
Copy, строка, Done.

### Кадрирование фотографии (`PhotoPreviewPage`)

- **Лс21.** Строка под заголовком. Кадр `s-contacts-5`: «Анна Ковалёва ·
  metadata will be stripped». Модель: `title` «Photo», подзаголовка и имени
  человека нет.
- **Лс22.** Кадр без сценариев. Модель несёт числа (`zoom`, `offsetX`,
  `offsetY`, `rotate`) и действия со своими адресами (`photo.rotate.*`,
  `photo.save`, `photo.cancel` — POST), но не форму: имена полей кадра —
  «the page's own» (комментарий `photoPreviewPage`), в контракте их нет;
  повороты и Save photo не помечены `submits`. Поворот без сценариев уходит
  своей формой без масштаба и смещения. Нужно: `Form` страницы (адрес, поля
  `zoom`, `offsetX`, `offsetY`) и `submits` у поворотов и Save photo.
- **Лс23.** Подсказка «Saved as». Кадр: «…a square of 512 px at quality 85
  keeps a vCard near 200 KB…». Сторона — настройка (`PhotoMaxSide`, она же в
  `saved`: «512 × 512 · JPEG · about …»); подсказки в модели нет, числа в
  ней — от данных экземпляра. Нужно: подсказка строкой сервера (или решить,
  что числа постоянны).
- **Лс24.** Данные. Кадр: «about 90 KB», Zoom «1.6×», есть «Remove photo»
  (у карточки уже есть фотография). Фикстура: «about 1.9 KB», `zoom` 100,
  `photo.remove` нет (по коду — только при `HadPhoto`). Состояния «у карточки
  есть фотография» в фикстурах нет. Не проверено: единица `zoom` (100 — это
  «1.0×»?).

### Подсказки участников (`SuggestList`)

- **Лс25.** Данные. `s-agenda-12`: три подсказки на «ве» — Вера Лебедева
  дважды (vera@example.net, vera.l@work.example; «Архив») и Вениамин Орлов
  (orlov@example.com; «Контакты»); фикстура — одна строка. Состояния
  `s-agenda-13` (первой «Add ivan.petrov@mail.example», под чертой «Not in
  the ticked address books.») в фикстурах нет; поля под него (`add`, `note`)
  у модели есть.

### Полная форма создания (`…/new`)

- **Лс26.** Новый контакт, почта. Кадр `s-contacts-27`: в «Email addresses»
  — пустая строка: Label «work», Address, Remove — и «Add email». Фикстура
  `app-contact-new/normal`: в наборе одна «Add email»; у телефона пустая
  строка есть (`tel.0.type` «cell», `tel.0`, Remove). Нужно: пустая строка
  почты с подписью «work».

Остальное у нового контакта и нового события сошлось (шапка, подзаголовок,
Cancel и Save, наборы, Attach со стрелкой, подсказка участников); данные:
время нового события 10:00–11:00 у кадра, 18:00–19:00 у фикстуры.

### Экспорт и отключение

- **Лс27.** Фикстур нет у `ExportSheet` (`/app/{kind}/export`) и у
  `ConnectionRemoveSheet` (`/app/settings/connections/disconnect`); кадров у
  обоих тоже нет (вопросы В8, В9), сверять не с чем.

## Клиенту (не контракт)

- Значок листа по роду и назначению: `quill`, `con`, `cal`, `task` у
  создания, `move` у переноса и копии, `refresh` у переписывания, `shield` у
  пароля и депонирования (у `account-2` — `SheetIconInk`).
- Ширина: 480 у «писать некуда», депонирования, отзыва, сброса; 520 у
  задачи, переноса, переписывания; 400 у пароля; без ширины у заметки,
  контакта, события. `ScrimPad26`.
- Клавиши подвала: Ctrl+Enter у заметки, Enter у остальных родов, Esc у
  Cancel; у выключенной «Rewrite 0 records» Enter не рисуется.
- Поля быстрого листа — `SheetFields`: первое `Grown`, Phone и Email
  `FieldFill200`, Date `FieldFix130`, Start и End `FieldFix96`, All day
  `FieldFill120` с пустой подписью, Remind `FieldFill200`; строка под полями —
  `Dim11_5`, `DimWrap`; тело заметки — `AreaBodySm`.
- «куда» в шапке — `PickT12W200`, пункты — `stamp` с меткой, «ro» по
  `readOnly`.
- Перенос: рубрика «Write into», недоступные строки `Unavailable`, сообщение
  `MsgFlushTop2`; подписи кнопок — известная запись («Move» / «Copy»).
- Переписывание: рубрики «The address», «What rewriting can set off», «What
  will not change», стрелка «→» между адресами, тире перед пояснением,
  причина выключенной кнопки — тревожным `Msg` над подвалом.
- Сброс: рубрика «Temporary password · visible only now», «login … password
  …» одной строкой, Reset password и Withdraw — `ButtonDangerSolid`.
- Фотография: «Drag the photo to choose the frame. The box is what other
  clients will see.», «EXIF orientation is already applied, metadata is
  stripped.», подписи Zoom, Rotate, Saved as; `Dialog520`.
- Подсказки: найденное жирным из `name` (Runs), адрес моноширинно, книга
  справа; плашка из `value`.

## Вопросы к кадрам (кандидаты задач Typecase)

- **В1.** Быстрый лист — не форма. В `s-notes-5`, `-8`, `-9`, `-10` у
  `Sheet` нет `Action`, «Save …» и «Open the card» — не `Submit`
  (`type="button"`), поля без имён. Подпись `s-notes-5` обещает ту же форму
  страницей «когда JavaScript выключен». У модели всё есть: `form.action`,
  имена полей, `record.open` со своим адресом (formaction).
- **В2.** «куда» в шапке листа: пункты `s-notes-16`, `-17`, `-19` — кнопки
  без адреса и без поля; тот же список в `desktop-17` — радио
  `name="notebook"` со значениями. Ссылка (лист на другой коллекции) или поле
  формы — кадры говорят разное; от ответа зависит Лс4.
- **В3.** Перенос записи: строки дерева в `s-contacts-2`, `-17` — `div`, не
  ссылки и не отметки поля, «Move here» — не `Submit`. Лист-близнец файлов
  `s-files-4` — форма с `TreeRow.Send`. Сервер здесь — ссылки
  (`TargetOption.Href`) и свой POST подтверждения.
- **В4.** Переписывание (`s-contacts-15`, `-19`, `-20`), депонирование
  (`account-2`), отзыв (`account-11`), сброс (`s-admin-13`): листы не формы,
  кнопки `type="button"`, поле пароля без имени — без сценариев не шлют
  ничего.
- **В5.** `s-plan-3` против `s-profile-3` — один лист: разные слова под полем
  («…anything else handed out.» / «…key escrow — anything handed out or
  handed over.»), у `s-plan-3` нет значка, нет Enter у Show, Esc отдельно от
  Cancel.
- **В6.** Тот же лист пароля для профиля устройства, пароля davloom,
  удаления учётной записи: заголовка и подписи кнопки под эти случаи кадры
  не рисуют (везде «Show password» / «Show»).
- **В7.** Фотография `s-contacts-5`: Zoom — полоса из `span`, не поле;
  Save photo — `type="button"`; страница не форма.
- **В8.** Лист экспорта из сведённого вида — кадра нет. U-6.17 называет
  «диапазон у календаря»; у `ExportSheet` диапазона и заголовка нет, сервер
  отдаёт календарь целиком (`?whole=1`).
- **В9.** Лист отключения подключения — кадра нет; компонент
  `components-37` рисует Disconnect прямым POST на
  `/app/settings/connections/nextcloud/disconnect` с полем `csrf` — не тот
  адрес и не то имя поля, что у сервера (`…/connections/disconnect?…`,
  `_csrf`).
- **В10.** Состояния без кадра: быстрый лист «куда» без полей
  (`app-quick/normal`); перенос без выбранной цели и с целью на другом
  подключении; неверный пароль в листе пароля; сброс без выбранного в Whose;
  подсказки, когда ничего не найдено.
- **В11.** `s-contacts-27`: «Show 6 empty fields» на новой форме, где
  Organisation, Title и остальные поля уже нарисованы; у фикстуры свёрнутого
  набора нет.
- **В12.** U-6.16: последняя коллекция «помечена `Last` в списке „куда“».
  Списки шапки листа (`s-notes-16`, `-17`, `-19`) пометки не рисуют, сервер
  `TargetOption.Last` нигде не ставит (поиск по `internal/web`); «last used»
  рисуют только меню стрелки New (`s-contacts-9`, `s-notes-13`). Править
  спецификацию или кадры.

## Итог

Серверу — 27 пунктов (Лс1–Лс27). Из них 3 — только данные (Лс13, Лс24,
Лс25), 6 — только недостающие фикстуры и строки `STATE_FRAME` (Лс1, Лс12,
Лс14, Лс18, Лс20, Лс27), 18 — расхождения модели с кадром. Вопросов к кадрам
— 12 (В1–В12). Маршруты и модель меняют Лс3, Лс4, Лс9, Лс15, Лс22; ждут
ответа на В2 — Лс4, на В7 — Лс22; остальное — слова, поля и фикстуры.
