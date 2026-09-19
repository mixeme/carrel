# Хвост: формы правки и действия записи по кадрам

Найдено ревью перед S-30 ([review-server.md](review-server.md), Р-10) при
сверке живого экземпляра с кадрами. Карточки показывают больше, чем формы дают
править. В кадрах это обычные поля, а на сервере их нет в
`web.editFields`. Человек видит день рождения, адрес, категории, повторение,
но изменить их из Carrel не может. Исправить можно только в другом клиенте, то
есть там, откуда Carrel обещал увести.

Правило записи не меняется: только патч по изменённым полям (S-7), значения
параметров и группа строки сохраняются (SV-32). Каждая форма — свой коммит,
тест вперёд. Живой тест — Baikal и Radicale: правка поля, остальное байт в байт.

---

## Что есть и чего нет

| Род | Правится сейчас | Нет, а кадр даёт |
|---|---|---|
| Контакт (`s-contacts-3`) | Display name, Given, Family, Email[], Phone[], Organisation, Title, Note | Nickname, Birthday, Address[], Categories, метка строки телефона и почты (`TYPE`), «Show N empty fields» |
| Событие (`s-agenda-3`) | Title, Starts, Ends, Location, Description, Reminders | Status, Categories, Time zone («as sent»), Repeats: Frequency, Every, On, Until, Count, Advanced RRULE |
| Задача (`s-tasks-3`) | Title, Due, Done, Percent, Description, Reminders | Start date, Status, Priority, Location, Categories, Related to |
| Заметка (`s-notes-4`) | Title, Body | Date, Time, Tags, Related to, Notebook (переход — S-18, уже есть) |

Действия записи, которых нет:

| Где | Кадр | Чего нет |
|---|---|---|
| Карточка задачи | `s-tasks-2` | **Mark done** рядом с Edit — три свойства (`STATUS`, `COMPLETED`, `PERCENT-COMPLETE`), что и флажок в строке |
| Строка списка задач | `s-tasks-1` | флажок выполнения строки (`TaskRow.Done` показан, действия нет) |
| Карточка события | `s-agenda-2` | New task — задача с `RELATED-TO` этого события |
| Карточка контакта | `s-contacts-6` | Send — письмо с карточкой (S-26.2: лист вложения уже умеет запись) |
| Карточка заметки | `s-notes-2` | Export Markdown — одна заметка тем же форматом, что экспорт тетради (S-19) |

## Закрытые развилки

- **Категории — одно поле списком**, запятая — разделитель, как их и видит
  человек в тегах. Запись — `SetValues` по `CATEGORIES`. Список у vCard пишется
  без экранирования запятой (`card.listProps`, `v0.0.124`), у iCalendar —
  значением TEXT-списка.
- **Метка строки** (`TYPE` у телефона и почты) — выбор из `home`, `work`,
  `cell`, `voice`, `fax`, `other` плюс то, что уже стоит у строки. Чужое
  значение, которого нет в списке, остаётся выбором, как чужое напоминание в
  `reminderPresets`.
- **Адрес** — пять полей ADR (улица, город, регион, индекс, страна). Части PO
  box и extended не показываются и сохраняются как были, так же как `nameOps`
  хранит отчество.
- **Повторение правится целиком** (S-11). Простые поля собирают `RRULE`;
  поле «Advanced RRULE» — сырое значение, и если оно задано, оно главнее.
  `EXDATE`, `RDATE` и экземпляры с `RECURRENCE-ID` не трогаются. Лист говорит
  прямо, что отдельные экземпляры не правятся, и это уже есть в
  `RecurrenceBlock.Note`.
- **Пояс события** — `TZID` у `DTSTART` и `DTEND`: выбор из известных поясов,
  по умолчанию «as sent». Пояс экземпляра формы не меняет (C-10).
- **Related to** — поиск по уже загруженному в сессии (`Jump`, S-15.3), без
  запроса к серверам; «Enter a UID by hand» — запасной путь. Пишется
  `RELATED-TO` (RFC 5545 §3.8.4.5), обратная связь по-прежнему читается из
  загруженного (S-11).
- **Mark done** — POST на правку записи с одним полем `done=on` и версией
  записи. Отдельного адреса нет, C-5 не расширяется. Строка списка даёт то же
  действие со своей версией. 412 — экран расхождения, как у формы.

## Шаги

- [x] Контакт: Nickname, Birthday (дата; `BDAY` без года — `--MMDD` читается и
      пишется как был), Categories. Тест: правка каждого поля не меняет
      остальные строки (`v0.0.143`, `TestContactNicknameBirthdayCategories`)
- [x] Контакт: метка строки телефона и почты; Address[] по пяти частям
      (`v0.0.144`, `TestContactRowLabelsAndAddresses`,
      `TestRowLabelSurvivesTheConflictScreen`). Попутно: черновик конфликта
      заполнял N против серверной версии и предлагал имя, которого никто
      не набирал
- [x] Событие: Status, Categories, пояс (`v0.0.145`,
      `TestEventStatusAndCategories`, `TestEventTimeZone`,
      `cal.TestTimezoneFromTheZoneDatabase`). Новый `TZID` приносит свой
      `VTIMEZONE` (RFC 5545 §3.2.19), собранный из базы поясов сервера;
      список поясов — UTC, пояс экземпляра и ~60 распространённых
- [x] Событие: Repeats — простые поля и Advanced RRULE; `RecurrenceBlock.Fields`
      заполняется для формы (`v0.0.146`, `TestEventRepeats`,
      `TestNewEventRepeats`: серия правится, `EXDATE` и перенесённый
      экземпляр целы)
- [ ] Живой тест Repeats на Baikal и Radicale: серия правится, исключения
      целы
- [x] Задача: Start date, Status, Priority, Location, Categories, Related to
      (`v0.0.147`, `TestTaskFields`)
- [x] Заметка: Date и Time (`DTSTART` журнала), Tags, Related to
      (`v0.0.152`, `TestNoteDateTagsRelated`). Правка контракта (решение
      владельца 19.09.2026: ошибка контракта, форма получает всё, что ей
      нужно): `NoteDoc.Fields []FormRow` в C-6 и U
- [x] Mark done в карточке задачи и в строке списка (`v0.0.148`,
      `TestMarkDoneFromTheCardAndTheRow`). Флажку строки своего поля в
      `TaskRow` нет — действие лежит в `TaskRow.Menu` с ID `task.done`;
      клиент рисует его флажком
- [x] New task из события (быстрый лист с `RELATED-TO`), Send у контакта,
      Export Markdown у заметки (`v0.0.149`, `TestNewTaskFromAnEvent`,
      `TestSendAContactFromItsCard`, `TestExportOneNoteAsMarkdown`). Новых
      адресов нет: быстрый лист задачи с `?related=`, POST листа вложения,
      `export?uid=`
- [x] «Show N empty fields»: форма отдаёт пустые необязательные поля отдельным
      `FieldSet` с `Legend` — клиент сворачивает его сам (правка C-6 не нужна,
      если хватает `FieldSet`; иначе — согласовать). `v0.0.150`,
      `TestContactEmptyFieldsInTheirOwnSet`: хватило
