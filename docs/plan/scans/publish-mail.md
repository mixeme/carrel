# Скан U-6.12 «Публикация и почта» после записки 105 (задача 194) — что кадры рисуют, а сервер не несёт

Только чтение. Рабочее дерево `D:\Local\wt\t53\carrel` (ветка `wip/settings-client`,
`3728415d`; копия макета — Typecase `cc65705`, 504 кадра). Прежний скан —
`docs/plan/scans/publish-mail.md` (Пб1–Пб43, В1–В8; Typecase `95453f6`).
Записка 105 — `D:\Local\Git\carrel\docs\plan\from-typecase.md:3422–3523`.

**Метод.** Кадры вырезаны по `data-frame="…"` до следующего `data-frame`
(свой резак в черновой папке): `s-publish-1…23`, `s-mail-1…13` (`s-mail-11`
в макете нет). Из каждого — формы, `name=`, `formaction`, `href`,
`data-part`, `data-screen`, слова листа. Против них — `view/cross.go`,
`view/files.go`, `view/settings.go`, `internal/web/publish*.go`,
`mail_compose.go`, `mail_attach.go`, `under.go`, `routes.go`, фикстуры
`docs/spec/fixtures/{app-publish,app-mail-compose,app-files-published}`,
`docs/spec/check.py`, C-5 `carrel-common.md:381–383`. Сервер не запускался;
`frame2go` не строился. Где сказано «по коду» — из чтения, не из ответа.

## Экраны, маршруты, кадры, фикстуры (после 105)

| Экран | Маршрут кадра | Сервер сейчас | Кадры | Фикстуры / check.py |
|---|---|---|---|---|
| Лист публикации | `/app/publish?…` GET, POST (форма `Sheet.Action`) | GET, POST `/app/publish` (`routes.go:47–48`); цель `k`,`a`,`c`,`uid` / `k=files`,`f`,`dir` | `-2` календарь (над Connections), `-19` событие (над карточкой события), `-3` контакт (над карточкой), `-4` заметка (над страницей заметки), `-5` папка (над «Файлами»), `-14` список срока | `app-publish/normal` — **контакт**, сверяется с каноническим `-2` (строки `STATE_FRAME` нет); календаря, события, заметки, папки нет |
| Готовая ссылка | ответ POST | `madeSheet` (`publish.go:222–245`) | `-13` (с почтой, срок), **новый `-22`** (бессрочная, без почты) | `app-publish/link` → `-13` (`check.py:534`); `-22` нет |
| Выданные ссылки | `/app/files/published` | `getFilesPublished` (`publish.go:588–663`) | `-1`, `-18` пусто, `-20` без почты, `-15`, `-17` фрагменты, **новый `-21`** мёртвая строка | `normal`, `no-mail`→`-20`, `empty`→`-18` (`check.py:450–451`); мёртвой нет |
| Гостевая заметка | `/p/{token}` | `guestAttachments` (`publish_attach.go:155–185`) | `-8` (вложения отданы), **новый `-23`** (`data-part=note-files-kept`) | `p-token/note`→`-8`; `-23` нет |
| Письмо | GET `/app/mail/compose`, **POST `/app/mail/send`** (`?op=…`) | GET, POST `/app/mail/compose` (`routes.go:49–50`) | `-1` (над книгой «Контакты»), `-2` меню Attach, `-3` подсказка To | `app-mail-compose/normal` — пустое письмо |
| «Send this link» | **`/app/mail/link?id=…`**, форма — POST `/app/mail/send` | **маршрута нет** | `-4` (над Published links) | нет |
| Листы вложения | `/app/mail/attach?what=…` (свой `data-screen`) | GET, POST `/app/mail/attach` (`routes.go:51–52`) | `-5` note, `-6` file, `-10` calendar, **`-12` contact**, **`-13` folder** — все с `data-part` | **нет** ни каталога, ни `FIXTURE_SCREEN`, ни `SHEETS` |
| Письмо у получателя | — | — | `-7`, `-8` | — |

## 1. Пункты Пб1–Пб43 после записки 105

| Пункт | Статус | Одной строкой |
|---|---|---|
| Пб1 | закрыто раньше (`v0.2.330`) | `Title`/`Target` по роду; `-5` «Материалы к релизу · 12 files» — как есть |
| Пб2 | закрыто раньше (`v0.2.180`, `.183`) | имена `name`,`phones`,`emails`,`photo`,`note`,`download` — как в `-3` |
| Пб3 | закрыто раньше (`v0.2.345`); **доделка** по `-23` | гостю без вложений кадр — «2 attached files are not published.», сервер — «2 files are attached to this note; they are not published with it.» (`publish_attach.go:162–166`) → С9 |
| Пб4, Пб5, Пб8, Пб9, Пб10, Пб14, Пб16, Пб17, Пб18 | закрыто раньше | кадры 105 их не трогают |
| Пб6 | закрыто кадром — серверу: дни при открытии | «с сегодняшнего до конца месяца» (`-2`: 2026-08-10…31); сейчас `from`/`to` без значения (`publish.go:139–140`) → С1 |
| Пб7 | закрыто кадром — серверу: дни при обоих `how` | `-2`: `how=whole` с `data-tc-disables="from to"`, дни стоят всегда; сервер рисует их только при `range` (`publish.go:137–141`) → С1 |
| Пб11 | закрыто раньше; **доделка** | `-13`, `-22`: крестик — ссылка назад (`title=Close`); у готового листа сервер его не даёт (`publish.go:233–236`) → С4 |
| Пб12 | держится | «could not publish» — `http.Error` 500 (`publish.go:509–511`); кадра отказа нет и в 105 |
| Пб13 | держится (фикстуры) | `normal` (контакт) против `-2` (календарь); нет календаря, события `-19`, заметки `-4`, папки `-5`, `-22`, отказа дней → С11 |
| Пб15 | закрыто кадром и решением владельца 21.10 — серверу: лист `/app/mail/link` | готовая ссылка в письмо не вкладывается; «Send by letter» и Send строки открывают `-4` → С12–С14 |
| Пб19 | закрыто кадром — серверу | `-3`: строка `to.add` с набранным, «Use «бел» as typed», «Searched 4 of 5 address books · baikal · Офис did not answer», метки work/home, значения шаблонов — адрес без `mailto:` → С17 |
| Пб20 | закрыто кадром — серверу | под To — подсказка поля «2 of 10 recipients. Everyone sees the whole list.» (ошибка отправки — первой фразой там же); потолков и «N letters left today» кадр не рисует; сервер — одна `Limits` на всё (`mail_compose.go:354–367`) → С18 |
| Пб21 | закрыто раньше (`v0.2.333`); **доделка** | папка — «1 folder · 6 files» (105 п. 7); сервер — `plural(len(entries),"file")` = «7 files» (`mail_attach.go:310–313`) → С20 |
| Пб22 | закрыто кадром — серверу | пункты Attach — submit `formaction /app/mail/send?op=attach&what=…` в `details`; сервер — `POST /app/mail/compose?op=attach…` (`mail_compose.go:369–372`) → С16 |
| Пб23 | **ждёт владельца** (105 п. 7) | «Preview» — `formaction …?op=preview`; что делает — не решено |
| Пб24 | **ждёт владельца** (105 п. 7) | «Discard» сервера против Cancel-ссылки кадра (`href=/app/contacts`, Esc); адрес возврата — С21 (не зависит от ответа) |
| Пб25 | закрыто кадром — серверу: `MailSheet.Form` | имена: `to` (флажки, значение — адрес), `to.add`, `subject`, `text` (у сервера `body`, `mail_compose.go:218–222`) → С15 |
| Пб26 | держится | лист письма без подтверждённого адреса — кадра нет (подпись `-4` только говорит словами); `s-mail-9` — страница учётной записи (связана, `check.py:463`) |
| Пб27 | держится (фикстуры) | одна фикстура письма, пустая; нужны заполненное (`-1`), «Send this link» (`-4`) → С23 |
| Пб28 | держится (данные) | кадр 2 MB / 10 получателей, сервер 10 MB / 25 MB / 20 (`conf.go:164–166`); 105 называет его «предел — Пб28» |
| Пб29 | закрыто кадром — серверу | у `/app/mail/attach` теперь пять кадров со своим `data-screen` → С30 |
| Пб30 | закрыто кадром — серверу | «Attach a note / a contact / a file / a folder / a calendar» — заголовок; `MailAttachSheet` его не несёт (`cross.go:803–810`) → С24 |
| Пб31 | закрыто кадром — серверу | Attach стоит всегда; не вышло — «первое действие листа становится недоступным и называет причину» (`-10`); сервер до выбора даёт один Cancel и отказ — лишним `mail.attach.error` (`mail_attach.go:69–74`, `96–99`) → С25 |
| Пб32 | закрыто кадром — серверу | выбранная строка `is-on` (`-5`, `-6`, `-12`); у `FindRow` признака нет (`cross.go:18–41`) → С26 |
| Пб33 | закрыто кадром — серверу | одно дерево «сервер → коллекции», сверху «All notebooks»/«All address books» — поиск во всех; набранное стоит в поиске; сервер — рейл с одной отмеченной и поиск по одной (`mail_attach.go:181–232`) → С26 |
| Пб34 | закрыто кадром — серверу | `-10`: форма `attach-calendar` POST, радио `span` с `Disables`, дни всегда, Show **нет**, `data-tc-recount=attach-line`; сервер — `Range` GET со Show и днями только при `dates` (`mail_attach.go:106–127`) → С27 |
| Пб35 | закрыто кадром — серверу | `-6`: дерево папок, крошки, Name/Size/Changed, у строки «as a file» / «over 2 MB · as a link», сообщение над Attach; `-13`: «In it · 1 folder · 6 files», файлы погашены → С28 |
| Пб36 | закрыто кадром — серверу ничего (по коду) | у Attach название заметки; `Preview[0].Title` — `recordTitle` (`mail_attach.go:378`, `104`); проверить фикстурой С30 |
| Пб37 | закрыто кадром (решение 15.10, 105) — серверу: `Under` у четырёх листов | под листами — страница, откуда открыт; `PublishSheet`, `MailSheet`, `MailAttachSheet` поля нет; в `Under` нет `Files`, `Published` (`cross.go:356–382`) → С4, С13, С21, С29 |
| Пб38 | закрыто раньше (`v0.2.257`) | «Change…» теперь submit `?op=change&i=N` → С16 |
| Пб39 | **снято решением владельца 21.10** (готовая ссылка не вкладывается; размер — при выборе) | отказы Б36, Б37 уходят: `givesTooMuch` (`mail_compose.go:471–485`) больше не нужен в этом виде → С14, С19 |
| Пб40 | закрыто раньше (`v0.2.334`); C-5 снова отстаёт | строки C-5 письма (`op=link` «добавляется к письму») и вложения (пусто) — переписать в пакетах ниже |
| Пб41, Пб42 | закрыто раньше (`v0.2.334`) | `Href` строки, `STATE_FRAME` пустого списка |
| Пб43 | закрыто кадром — серверу ничего | 105 п. 7: «2 min ago» — кнопка верхней полосы та же везде; у Published links она как у настроек (`publish.go:598` ≈ `rails_settings.go:118`) |

Вопросы прежнего скана: В1 (листы — формы), В2 (письмо и вложения без
сценариев), В3 (экран под листом), В5 (мёртвая строка `-21`), В6 (готовая
ссылка `-13`, `-22`), В8 (одна подсказка `local`, обновление) — **закрыты
кадрами 105**; В4 — **решение владельца 21.10** («Send this link» всегда,
начатое ждёт черновиком; «change in Account» — ссылка `/app/settings/account`);
В7 — **устарел** (отказов нет: ссылка — при отправке, дни — ссылкой на дни).

## 2. Серверу по записке 105

Имена — как в кадре. Адреса кадров сокращены (`/app/publish?c=lichnyj` без
`k`, `a`; `/app/mail/attach?what=note&c=zapisi` без `a`) — сервер держит свои
`k`/`a` (check.py адресов не сверяет, `check.py:736–775`).

**Публикация**

- **С1. Дни календаря при обоих `how`** (`-2`; Пб6, Пб7; 105 «Серверу:»).
  `sheetFor` (`publish.go:129–142`): `from`, `to` (`InputDate`, `Mono`) —
  всегда, со значениями «сегодня» и «последний день месяца» (зона сессии);
  у варианта `whole` — `Disables: "from to"`, при `how=whole` у дней
  `ReadOnly`. **В `view.Option` поля `Disables` нет** (`settings.go:154–175`;
  есть только `FormRow.Disables`, `:113`) — добавить `Option.Disables string`
  (Typecase `SegOption.Disables`), C-6. POST (`publishChoices`,
  `publish.go:319–321`) дни при `whole` и так не читает — оставить.
- **С2. Отказ без дня** (`-2`, подпись). `postPublish` (`publish.go:482–501`):
  слова «Give both days, or choose The whole calendar.» (сейчас «…or publish
  the whole calendar.», `:493`) и под **пустым** полем (сейчас всегда под
  `from`).
- **С3. «Time» — выключена с причиной** (`-2`, `-19`: `disabled checked`,
  `title="Always shown"`). `calendarSees` (`publish_visitor.go:119`):
  `Reason: "Always shown"` у строки `time` (сейчас `ReadOnly` и
  `LabelAside "always shown"`). Срок `lives` — радио в `Drop` (`-14`): сервер
  шлёт `InputSelect` с вариантами (`publish.go:158–163`) — виджет клиента,
  менять не надо.
- **С4. Экран под листом публикации и «назад»** (`-2`, `-13`, `-22` —
  Connections; `-3` — карточка контакта; `-4` — страница заметки; `-19` —
  карточка события; `-5` — папка «Файлов»). `PublishSheet.Under *Under`
  (`cross.go:738–757`); `getPublish`/`postPublish` — `s.underOf(r, back,
  fallback)` (`under.go:58`), где fallback по цели: коллекция —
  `/app/settings/connections` (публикуют только оттуда), запись — её
  страница, файл и папка — папка в «Файлах». В `view.Under` добавить
  `Files *FilesPage` и ветку в `underPage` (`under.go:118–154`). Cancel
  (`publish.go:120`, сейчас `c.listPath()`) и **Close готового листа** — туда
  же (`cancelTo`, `under.go:104`). У готового (`madeSheet`, `publish.go:233`)
  — действие `publish.close` (крестик, ссылка).
- **С5. «Send by letter» — ссылка** (`-13`: `href=/app/mail/link?id=7Qm2`).
  `madeSheet` (`publish.go:238–243`): `Method: "GET"`, `Href:
  /app/mail/link?id=<LinkID>` вместо `POST /app/mail/compose?op=link`. `-22`
  — без почты Send нет: уже так (`canSendLetters`).
- **С6. Send строки списка — та же ссылка** (`-1`, `-15`). `getFilesPublished`
  (`publish.go:641–642`): GET `/app/mail/link?id=<l.ID>`.
- **С7. Мёртвая строка** (`-21`). `getFilesPublished` (`publish.go:625–639`):
  `Gone` — слова подсказки плашки «Deleted, or moved where Carrel could not
  follow it» (плашка «no longer there» — слово экрана; сейчас одна строка
  «No longer there: deleted, or moved…»); `Expires` — «—» (сейчас дата
  срока); Copy и Send — `Enabled: false`, `Reason: "It is no longer there"`;
  Revoke — как есть. Revoke кадра — submit `PostForm` `post-revoke`
  (`/app/publish?op=revoke`, `name=id` у кнопки): `postPublish` читает `id`
  через `FormValue` (`publish.go:450`) — и из тела, и из адреса; серверу
  менять нечего, форму строит клиент.
- **С8. C-5 `/app/publish`** (`carrel-common.md:381`): `from`/`to` при обоих
  `how`, `Option.Disables`, дни при открытии, слова отказа; ответ POST —
  готовый лист с Close; «Send by letter» — `/app/mail/link?id=`.
- **С9. Гостевая заметка без вложений** (`-23`, `data-part=note-files-kept`).
  `guestAttachments` (`publish_attach.go:161–166`): `AttachNote` — «2
  attached files are not published.» / «1 attached file is not published.»
  (слова кадра; единственное — по образцу, кадра нет).
- **С10. Вложения гостевой заметки — ссылки** (`-8`): `PublishedAttachment.Href`
  есть (`publish_attach.go:179`) — серверу ничего.
- **С11. Фикстуры публикации** (Пб13). `fixture_test.go:2493`, `:2518`:
  `app-publish/normal` → строка `STATE_FRAME` `"app-publish/normal":
  "s-publish-3"` (контакт) **или** снять контакт в `contact` и сделать
  `normal` календарём под канонический `-2`; новые состояния: `event` →
  `-19`, `note` → `-4`, `folder` → `-5`, `link-never` (без почты, `lives=never`)
  → `-22`, `range-missing` (отказ С2) → `-2`; `app-files-published/gone` →
  `-21`; `p-token/note-files-kept` → `-23`. Все — строки `STATE_FRAME`
  (`check.py:330…`).

**«Send this link» (`s-mail-4`)**

- **С12. Маршрут `GET /app/mail/link?id=…`** (`routes.go` — нет). Новая
  модель `MailLinkSheet` (C-6): `Title` «Send this link», `Subject` —
  `publishedTitle` ссылки («Личный · 10–31 August»), `From`, `FromName`,
  `ReplyTo`, `ReplyNote` (как у `MailSheet`; «Sent as» и «change in Account» →
  `/app/settings/account`), `Carries` — три строки «The letter carries» (третья
  — день срока ссылки: «It expires on 8 September, as the link does»;
  бессрочной — слова исполнителя, кадра нет), `Form` — POST `/app/mail/send`
  с полями `to` (текст, одно поле — не метки), строка «A line from you»
  (`name` в кадре — `a.line.from.you`, см. вопросы), скрытое `id`; действия
  Send (submit), «Copy link instead» (submit `formaction
  /app/mail/send?op=link`), Cancel → `/app/files/published`; `Under`.
  Без почты — 404, как письмо (`mail_compose.go:164–167`).
- **С13. Под «Send this link»** — Published links, как `-1` (105): в
  `view.Under` — `Published *PublishedLinksPage`, ветка в `underPage`;
  fallback `/app/files/published`.
- **С14. `op=link` больше не трогает черновик** (105 «Серверу:»). Убрать
  ветку `case "link"` (`mail_compose.go:196–216`) и `linkPart`
  (`:507–519`); у `mailPart` уходит `LinkID` (`:68`) с ветками в
  `givesTooMuch` (`:474–477`), `reckonLinks` (`:495`), `sendLetter`
  (`:673–675`), `linkReason` (`:784–785`), `writeLetterSheet`
  (`publish.go:410`). Отказ Б36 не бывает. Отправка письма ссылки —
  отдельная от черновика: начатое письмо ждёт (`DraftMail` не трогать).
  Что делает `op=link` листа ссылки без сценария — вопрос к кадру (ниже).

**Письмо (`s-mail-1…-3`)**

- **С15. Письмо — форма** (Пб25). `MailSheet.Form` (`cross.go:759–782`):
  POST **`/app/mail/send`** (новый маршрут вместо `POST /app/mail/compose`;
  GET `/app/mail/compose` остаётся — `data-screen` `-1`); строки: `to` —
  выбранные метками (`InputCheck`/`Multiple`, значение — адрес, подпись
  «Имя · адрес», `title` — адрес), `Add` — `to.add` «Name or address»
  (`Suggest`, `Placeholder`; образец — Related to, `settings.go:71–75`),
  `subject`, **`text`** (сейчас `body`, `mail_compose.go:218–221` и C-5).
  Чтение: `to` + `to.add` (набранное — как есть, через `splitRecipients`).
  Без `op` POST — отправка (кадр: Send — простой submit; сейчас без `op` —
  только сохранение, `:262–263`).
- **С16. Кнопки письма — submit с `formaction`** (`-1`, `-2`; Пб22, Пб38):
  `mail.attach.*` — `/app/mail/send?op=attach&what=contact|book|event|calendar|task|note|file|folder`
  (`mail_compose.go:369–372`); крестик строки — `?op=remove&i=N`
  (`:326–327`); «Change…» — **`?op=change&i=N`** (новый `op`: сохранить
  набранное, перейти на `/app/publish?…&mail=i`; сейчас GET-ссылка
  `:347–348` теряет набранное без сценария); Send — без `op`. Apply листа над
  письмом (`op=terms`, `publish.go:415`) — на `/app/mail/send`.
- **С17. Подсказка получателей** (`-3`; Пб19). Адрес подсказок у `to.add` —
  свой, не участников: `getContactsSuggest` (`attendees.go:315`) отдаёт
  `Value` с `mailto:`, а метка письма — адрес. Нужно: `FormRow.Suggest` —
  `/app/contacts/suggest?for=letter` (или свой путь), в ответе `Value` —
  адрес, метка адреса из карточки (work/home), строка «Use «‹набранное›» as
  typed» (как `Add`), строка «Searched N of M address books · ‹книга›
  did not answer» (`NotAsked`, сейчас только у `?q=` листа,
  `mail_compose.go:299–310`); выбранное помнить как `Known` (`:302–307`).
- **С18. Строка под To** (Пб20). `FormRow.Hint` у `to`: «N of M recipients.
  Everyone sees the whole list.»; ошибка отправки — первой фразой там же
  (`Error`/`HintRuns`); потолки и «N letters left today» из строки убрать
  (кадр их не рисует; потолок назван в `Links` и в листе вложения).
  `mailSheet` `:354–367`; C-6 `Limits`.
- **С19. Дни календаря сверх предела — ссылкой на эти дни** (105, решение
  21.10; Б37). `partLink` (`mail_compose.go:722–752`): у `p.From`/`p.To` —
  `target.From`, `target.To` (как диапазон листа публикации); ветку отказа
  `givesTooMuch` `:478–481` убрать. Вместе с С14 `givesTooMuch` пустеет —
  убрать с проверкой `mail.send` (`:383–385`).
- **С20. Папка — «1 folder · 6 files»** (105 п. 7; `-1`, `-13`).
  `chosenPart` (`mail_attach.go:310–313`): папки и файлы порознь, нулевое не
  писать.
- **С21. Экран под письмом и Cancel** (`-1` — над книгой «Контакты»; Пб37).
  `MailSheet.Under`; `back` письма — в адресе (`/app/mail/compose?back=…`) и в
  черновике (`mailDraft.Back`, `mail_compose.go:37–46`), чтобы его же несли
  листы вложения (С29) и возврат после Send/Cancel листа ссылки. Cancel
  кадра — ссылка на `back` (`href=/app/contacts`). Входы в письмо (меню у New
  note, `record.send` — `record_view.go:606–611`) — с `back` текущей
  страницы. **Discard не трогать** — Пб24 у владельца.
- **С22. C-5 письма** (`carrel-common.md:382`): GET `/app/mail/compose`
  (`back`), POST `/app/mail/send` — `op` `attach`(`what`), `remove`(`i`),
  `change`(`i`), `terms`(`i`), без `op` — отправка; поля `to`, `to.add`,
  `subject`, `text`; строка `/app/mail/link?id=`; `op=link` — по ответу
  на вопрос ниже.
- **С23. Фикстуры письма** (Пб27). `fixture_test.go:3082–3087`:
  `app-mail-compose/filled` (два получателя из книги, четыре вложения:
  контакт, событие, PDF ссылкой, папка ссылкой) → `s-mail-1` (сейчас
  канонический — пустое `normal`); `app-mail-link/normal` → `s-mail-4`
  (новый каталог: `FIXTURE_SCREEN` `/app/mail/link`, `SHEETS`, `view/fixtures_test.go:80–88`).
  Потолки фикстуры — 2 MB / 10 получателей, если сверка слов их требует
  (Пб28).

**Листы вложения (`s-mail-5`, `-6`, `-10`, `-12`, `-13`)**

- **С24. Заголовок и род** (Пб30). `MailAttachSheet.What` (`note`,
  `contact`, `event`, `task`, `calendar`, `book`, `file`, `folder`) и `Title`
  «Attach a note / a contact / a calendar / a file / a folder» (событие,
  задача, книга — по образцу, «тот же лист со своими строками»).
- **С25. Attach — всегда, выключен с причиной** (Пб31). `attachSheet`
  (`mail_attach.go:95–99`, `145–150`): до выбора — Attach `Enabled: false`,
  `Reason` («Choose a note.»…); отказ `chosenPart` — та же кнопка с причиной
  отказа вместо `mail.attach.error` (`:69–74`).
- **С26. Дерево и строки** (Пб32, Пб33). Слева — одно дерево: сервер, под
  ним коллекции рода (`attachRail`, `mail_attach.go:181–204`), первой строкой
  «All notebooks» / «All address books» (без `c` — поиск во всех ответивших
  коллекциях, как `suggestRecipients`; сейчас без `a` — `firstTicked`,
  `:85–87`); набранное — значение поиска (`q`); заголовок списка «Found N».
  Выбранная строка: `FindRow.On bool` (`cross.go:18–41`) — `-5`, `-6`,
  `-12`. Строки контакта — как в разделе (`Avatar`, `Subtitle`,
  `Detail`, `Caption` — `findRow` их даёт, по коду). Выбор — ссылкой,
  выбранное — в адресе формы POST (`?what=…&uid=…`; у файла `&path=…`, у
  папки `&dir=…`).
- **С27. Календарь** (`-10`; Пб34). `Range` (`mail_attach.go:111–127`):
  `Method: "POST"`, `Action /app/mail/attach?what=calendar&a=…&c=…`, новые
  `Form.ID` «attach-calendar» и **`Form.Recount`** «attach-line» (Typecase
  `Form.Recount`, `data-tc-recount`: GET адреса формы с её полями — GET
  `/app/mail/attach` уже читает `span`, `from`, `to`, `attachQuery`
  `:28–43`); радио `span` (`whole` | `dates`), у `whole` — `Option.Disables
  "from to"` (С1); дни **всегда**, по умолчанию весь календарь и дни
  выключены (значения — с сегодня до конца месяца, как С1; сейчас +29 дней,
  `:356–365`); Show убрать (`:125`); Attach — кнопка формы (`form=`).
  Строка у Attach (`Preview`) — с `id` «attach-line» (Typecase `Dim.ID`):
  поле `MailAttachSheet.PreviewID` или правило клиента — решить в C-6.
- **С28. Файл и папка** (`-6`, `-13`; Пб35). Параметры — `dir` (открытая
  папка) и `path` (выбранный файл) вместо `f`/`file` (`attachQuery` `:30`,
  `attachFileRows` `:237–281`, `chosenPart` `:301–333`). Слева — дерево папок
  (`view.FileTree` с узлами-ссылками, `files.go:164–206`, «то же, что у Move»),
  сверху крошки (`Crumbs`); справа таблица Name · Size · Changed, у файла —
  «as a file» / «over ‹потолок› · as a link» (по `reckonLinks` с черновиком);
  у листа папки — «In it · 1 folder · 6 files», вложенная папка — ссылка
  (выбирает её), файлы погашены (`Off`); «..» убрать (дерево его заменяет).
  Сообщение над Attach — новое `MailAttachSheet.Note`: файл сверх предела —
  «This file is over the limit for an attachment, so the letter will carry a
  secret link to it instead. The link is listed in Published links, works for
  30 days, and revoking it there closes it for whoever got the letter.»;
  папка — «A folder always goes as a link: the letter carries a secret link
  to it, with all that is in it. The link is listed…».
- **С29. Под листом вложения — экран письма** (105; Пб37). `MailAttachSheet.Under`
  — `underOf` по `mailDraft.Back` (С21); Cancel → `/app/mail/compose`
  (кадр так и рисует).
- **С30. Фикстуры вложения** (Пб29). Каталог `app-mail-attach`:
  `FIXTURE_SCREEN` `/app/mail/attach` (`check.py:160…`), `SHEETS`
  (`:290–295`), `view/fixtures_test.go` → `MailAttachSheet`; состояния `note` →
  `s-mail-5`, `file` → `-6`, `calendar` (Dates) → `-10`, `contact` → `-12`,
  `folder` → `-13` — у всех кадров `data-part`, канонического нет: каждая
  строка `STATE_FRAME`, `normal` тоже. C-5 `:383` — параметры `what`, `a`,
  `c`, `q`, `uid`, `path`, `dir`, `span`, `from`, `to`, `format`.

## 3. Пакеты (по одной версии)

1. **Публикация без сценариев — S/M.** С1–С11. Сначала `Option.Disables`
   и `Under.Files` в `view` (нужны и пакетам 3–4). С5–С6 ведут на
   `/app/mail/link` — выпускать вместе с пакетом 2 или сразу после него
   (иначе ссылка в 404); проще — пакет 2 первым.
2. **«Send this link» и ссылка вне черновика — M.** С12–С14, `Under.Published`,
   фикстура `app-mail-link/normal`, C-5. Б36 уходит. Зависит только от
   `Under`.
3. **Письмо формой — M/L.** С15–С23: `POST /app/mail/send`, `Form` с
   метками `to`/`to.add`/`text`, submit-кнопки (`attach`, `remove`, `change`,
   `terms`), подсказка писем, строка под To, дни ссылкой (Б37), «1 folder ·
   6 files», `back`/`Under` письма; фикстура `filled`.
4. **Листы вложения — L.** С24–С30: `What`/`Title`, дерево с «All …»,
   `FindRow.On`, файл/папка по `dir`/`path` с деревом и крошками, `Note`,
   Attach с причиной, календарь формой с `Form.ID`/`Recount`; пять фикстур.
   Зависит от 3 (`/app/mail/send`, `mailDraft.Back`).

Порядок: 2 → 1 → 3 → 4. Гейты — один прогон на пакет.

## 4. Ждут владельца (105 п. 7) — не трогать

- **Preview** (`-1`, `formaction …?op=preview`): сервер `op=preview` не
  знает — POST падает в «сохранить и показать» (`mail_compose.go:262–263`),
  вреда нет. С16 кнопку не трогает.
- **Discard против Cancel** (Пб24): `mail.discard` (`mail_compose.go:387–389`)
  оставить; С21 даёт только адрес Cancel-ссылки и `Under` — от ответа не
  зависит. Если владелец снимет Discard — убрать `op=discard` и действие.
- **Что видно после Send**: сейчас редирект `/app/mail/compose?sent=N` и
  «The letter went to N recipients.» (`mail_compose.go:170–173`, `:259`).
  Затрагивают: С15 (Send без `op`), С12 (Send листа ссылки — куда после;
  «начатое возвращается»), С21 (возврат на `back`). Делать С15/С12 с
  прежним ответом после Send, без новых слов.

## Вопросы к кадрам (кандидаты в задачи Typecase, не владельцу)

1. **Имена полей по подписи.** `s-mail-5`: «Send it as» — `name="send.it.as"`,
   значения `markdown.md`/`source.ics` (записка: `format`; сервер `md`/`ics`);
   поиск `s-mail-5`, `-12` — `name="field"` внутри POST-формы листа (Enter
   отправит Attach; сервер ждёт `q` и GET); `s-mail-4` — `a.line.from.you`.
   Похоже на имена по умолчанию — просить `format`, `q` (своя GET-форма или
   `formaction`), имя строки «A line from you» (например `line`).
2. **`s-mail-4` без `id` и «Copy link instead».** Форма — `POST /app/mail/send`
   без признака ссылки (скрытых полей кадры не рисуют — сервер добавит `id`);
   что отвечает `?op=link` без сценария — готовый лист со ссылкой (как `-13`)
   или Published links?
3. **Строки `s-publish-1`, `-17`, `-20`**: Copy, Send, Revoke — `type="button"`,
   а у `-21` Revoke — submit `post-revoke`, Send по 105 — ссылка
   `/app/mail/link?id=`. Привести живые строки к виду `-21`.
4. **Enter в `to.add`** (`s-mail-1`) без сценария шлёт первую submit-кнопку
   формы — «Contact» меню Attach (`?op=attach&what=contact`). Нужна ли
   скрытая кнопка по умолчанию (Send?) первой в форме.
5. Крестик `s-mail-1`, `-4`, `-5…-13` — `type="button"`, у `s-publish-13` —
   ссылка; без сценария листы письма не закрыть крестиком.

## 5. Не проверено

- Сервер не запускался, `frame2go -self` не строился; всё о поведении — по
  коду. Сверка check.py после копии `cc65705` не прогонялась — какие
  «красные» дают новые кадры (`-19`…`-23`, `s-mail-12`, `-13`), не знаю.
- `findRow` для контакта (`Avatar`/`Subtitle`/`Caption`) и формат дня строк
  заметки («18 Aug») — по прежнему скану, не перечитаны.
- `tf.date` в названии календаря листа вложения («10 August 2026 – 31 August
  2026», `-10`) — не сверял с кодом `timeFmt`.
- Рисует ли клиент `InputSelect` срока как `Drop` с радио (`-14`) — экранов
  `publish`/`mail` в `carrel/client/screens` нет (есть только `published`).
- Сверяет ли что-нибудь адреса `action`/`href` кадров с фикстурами (кадры
  сокращают `k`/`a`) — в check.py сверяются подписи, основная ли, моноширинное
  ли; другие проверки не читал.
- `s-publish-21`, `-22`, `-23` без `data-screen` — привязка только
  `STATE_FRAME`; не проверял, принимает ли check.py фрагмент `-21` (строки
  таблицы) как кадр сверки страницы.
