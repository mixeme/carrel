# Дубликаты после записки 117: что серверу (скан 23.10.2026)

Записка 117 ([from-typecase.md](../from-typecase.md), «Записка 117 от
23.10.2026», Typecase `905c272`; макет — `54efd28`, где и записка 118).
Скан агента по ветке `wip/notes-117-118` (Carrel `d58b661a`), пути — от
`carrel/server/`. Это **сырьё**: перепроверить по кадрам и коду при взятии.
Уже есть: POST порога и весов, решения группы, Merge выключена у группы с
копией только для чтения, текст уведомления `s-dup-8`.

1. **Вопросы карточки группы.** `view.DuplicateGroup.Choices []MergeRow`
   (`view/cross.go`), только переключатель `choice.<PROP>` с номером
   варианта; спрашиваемое — `choiceProps` (`internal/web/duplicates.go`):
   контакт FN/ORG/PHOTO, событие и задача SUMMARY/LOCATION, заметка
   SUMMARY; читается первое значение (`choices()`), подпись — само значение.
   Нужно: отметки (`checkbox`, все отмечены) для TEL/EMAIL/CATEGORIES;
   TEXT, DUE, PRIORITY, WHEN; без SUMMARY; подпись «значение · коллекция»,
   телефон по типу («Phone · work»).
2. **Связывание и данные.** `do=link` кладёт `sealed.LinkedGroup` в
   `blob.Duplicates.Linked` (`dupes.Link`). В наборе (`docs/visual/fixtures`)
   Анна только в `nextcloud-home/kontakty.vcf`; для всех фикстур связан
   только `g-artem` ×2 (`internal/web/fixture_test.go`, около 830–863);
   `g-vera`, `g-sinkap`, `g-chemodan`, `g-crdt`, `g-vstrecha` — по тестам.
   Нужно: Анна связана в трёх книгах (Контакты, Команда, Архив; рабочий
   телефон из Команды, вторая почта из Контактов) — «linked ×3», «3
   sources», «in 3 books» (`s-contacts-1` — из-за этого защита клиента на
   ветке красная); `s-dup-1` — все связанные группы и «not duplicates»;
   лист копий `s-contacts-32`, `-35` — Анны, не Веры. Переставленные слова
   имени уже одно имя (`FoldName`).
3. **When и Due.** Есть только на странице слияния (`dupes.When`,
   `duplicates_merge.go` `mergePlan`). Нужно: `choice.WHEN` и `choice.DUE`
   на карточке группы; страницы слияния (`s-dup-2`, `-4`) вопросов не
   задают (фикстура `app-duplicates-merge` сейчас с `choice.FN`).
4. **«differ».** `differs()`, `contactDiffers`/`calendarDiffers`/
   `taskDiffers` (`duplicates_card.go`, `copies.go`), всегда «X differ»,
   относительно открытой копии, у события «time». Нужно: одно поле —
   «differs», несколько — «differ»; относительно показанного на карточке
   (выбранного); у каждой копии; у события «When».
5. **Карточка связанной записи.** Есть только `GET
   /app/{kind}/linked/{uid}/copies` и `…/{account}/{col}/{uid}/copies`; своя
   страница копии — `withLinked` (`record_view.go`), Edit на лист копий
   (`editViaCopies`), «⋯» с Move/Copy/Export/Publish (`recordOps`). Нужно:
   `/app/{kind}/linked/{uid}` — карточка связанной записи (`s-contacts-33`,
   `s-agenda-25`, `s-tasks-17`, `s-notes-32`), «⋯» Print, «Open in
   Duplicates», под чертой Unlink (`s-contacts-36`); Move, Copy, Export,
   Publish, Delete — только у копии. Панель единого списка
   (`openInPanel`, `recordDetails`) показывает первую копию — нужна
   связанная запись: «in N books», выбранные значения, Edit на лист копий,
   «Where this person is kept», без Delete и Source.
6. **Порог** — есть. Токен в макете зовётся `csrf`, у сервера `_csrf`.
7. **Веса** — есть; в макете `weight.title`, у сервера `weight.title-date`.
8. **Формы карточек групп** — форма только при вопросах; `do` и `group` в
   адресе, а не кнопками `name="do"` (`decideAction`). ID групп —
   хэш, в макете g1…
9. **Только для чтения** — Merge выключена; ветку `mergePlan`, что
   выключает вариант «Where the merged record stays», — убрать.
10. **Пустой экран** — нужны карточки «Decided earlier» (связано / not
    duplicates, коллекции, UID, «Not loaded in this session», Unlink /
    Offer again).
11. **Галка задачи** в строках групп — знак; у `DuplicateMember` нет
    признака сделанного.
12. **Полное слияние** (`s-dup-8`) — есть; нет фикстуры.

**Клиент.** Экрана дубликатов нет; `CopiesSheet` зарегистрирован для
`s-contacts-35`, `s-tasks-19`; не зарегистрированы `s-dup-*`,
`s-contacts-32`, `-33`, `-36`, `s-agenda-25`, `s-tasks-17`, `s-notes-32`.
Библиотека (записка 117, «Библиотека»): `Dim.Mark` (метка и имя коллекции
одной строкой; `Mark` больше не `ValuePart`), `FaceRow.SourceHint`, слова
строки с бейджем — в `tc-row__words` (библиотека печатает сама).
