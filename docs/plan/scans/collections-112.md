# Скан листов коллекций после записки 112

22.10.2026. Typecase `0f51d45` (задача 192 закрыта), сервер `v0.2.358`.
Записка — [from-typecase.md](../from-typecase.md), п. 112. Кадры
`s-profile-5`…`-9`, `-12`, `-17`…`-21`, `-28`…`-35`.

check.py на новой копии макета зелёный: фикстуры листов сходятся с кадрами
по адресам и состояниям. Расхождения — в смысле: слова, поля, исходы, и
кадры без фикстур (`-28`…`-35`).

## Что уже совпадает

- Имена полей (п. 2): `account`, `name`, `colour`, `components` со
  значениями `VEVENT`, `VTODO`, `VJOURNAL`, `address`; `name` у удаления;
  `to` у переноса — id подключения (`TargetOption.Value`).
- Адрес в «Rename and colour» (п. 3) — `CollectionRenameSheet.Address`,
  не поле формы.
- Отказ создания (п. 4) — форма с набранным в скрытых полях, «Back to the
  form» (`?op=back`) и «Try again» (`afterRefusal`,
  `handlers_collections.go:1003`).
- Исход переноса «записано не всё» (п. 6) — уведомление на Connections с
  «Open the copy» и диагностикой «What … did not keep» (`kept`,
  `collection_move.go:602`).

## Серверу

| № | Что | Где | Пункт |
|---|---|---|---|
| К1 | Р-75: у строк рейла нет меню (`SourceRow.Menu`) | `combined.go:979–1000` | Р-75 |
| К2 | Лист без рода из карточки аккаунта (`s-profile-29`): `?a=` без `kind`, радио `kind` (`event` — Calendar, `contact` — Address book), Address book выключает `components` (`Disables`), подсказка «Tasks and notes live in calendars…»; адрес — `addressbooks` у книги | `getCollectionNew`, `collectionNewSheet` (`handlers_collections.go:189`, `:797`); `newCollectionKind` | п. 1, `-29` |
| К3 | «New collection» у карточки аккаунта ведёт на `?a=` | `settings_connections` (карточка) | `-29` |
| К4 | Отказ при 403 — заголовок «The server would not make it» / «… rename it», тело «Nothing was created / Nothing changed, and the form is as you left it.»; объяснение 403 — `Explain` словами `-6` («This account may sign in and read, but ‹server› would not let it create here. The server does not say why — …»), у переименования объяснения нет | `davRefusal`, `createRefusal`, `renameRefusal` (`handlers_collections.go:1229–1268`) | пп. 4, 6 |
| К5 | Отказ удаления — уведомление на Connections (`s-profile-31`): «‹Архив› was not deleted.» · «‹nextcloud› refused: this account can read here but not write. ‹Архив› is as it was.» + диагностика; лист закрыт | `postCollectionDelete` (`collection_delete.go:217–230`) | п. 6 |
| К6 | Перенос «записано не всё»: «… so ‹Личный› on ‹nextcloud› was not deleted.» — коллекция с сервером | `collection_move.go:440` и соседние `kept` | п. 6 |
| К7 | Успех любого листа — `back`, без него Connections | `afterSheet`, `postCollectionDelete` (раздел вместо удалённой) | п. 6 |
| К8 | Никто не отвечает (`-21`): полей нет, «Try again» — ссылка на лист, Cancel; ни одного сервера рода (`-28`): полей нет, «Connect account» → `/app/settings/connections/new` | `collectionNewSheet`, `silentNow` | п. 9 |
| К9 | Имя и цвет ленты (`-33`): шапка «feed», поле «Feed» — адрес ленты только для чтения, подсказки Name и Colour; у ленты книги — «Rename», одно поле | `collectionRenameSheet` для `col.Feed` | п. 7 |
| К10 | Перенос папки хранилища (`-34`): цель — сервер с файлами, слова листа; меню папки на Connections — `move?…`, `&copy=1` | `collectionMoveSheet` для `KindFile`, `collectionMoveItems` | п. 8, Р-74 |
| К11 | Перенос хранилища целиком (`-35`): содержимое копируется и удаляется, адрес остаётся пустым; «What ‹server› will not let delete … is copied and stays there too; the result names it.» Сейчас — только Copy | `collectionMoveItems` (`collection_move.go:118`), `collectionMoveBlock`, `runFiles` | п. 8, Р-74 |
| К12 | Строка об отмене — последней фразой главного сообщения (`-8`, `-9`): сервер шлёт `Cancels` отдельно; клиент ставит её в конец `Warning` | клиент | п. 5 |

## Фикстуры

- `app-collections-move/normal`, `-delete/normal` — «Личный» (кадры `-8`,
  `-9`: 128 событий, встречи с участниками), открыты из Connections.
- Новые: `app-collections-new/account` (`-29`), `/none` (`-28`),
  `app-collections-rename/refused` (`-30`), `/feed` (`-33`),
  `app-collections-move/folder` (`-34`), `/storage` (`-35`);
  `app-settings-connections/delete-refused` (`-31`), `/move-kept` (`-32`).

## Пакеты

1. Сервер `v0.2.359` — К1–К9, К12 и фикстуры к ним: слова, состояния,
   исходы, лента, лист без рода.
2. Сервер `v0.2.360` — К10, К11: перенос папки и хранилища целиком.
3. Клиент — экраны четырёх листов (`v0.6.3`…), веха `v0.8.0` — с импортом
   после задачи 222.
