# -*- coding: utf-8 -*-
"""Собирает наборы .vcf и .ics, которыми экраны сверяются с макетами.

Данные здесь — ровно те записи, что нарисованы в carrel-ui-mockups.html.
Скрипт нужен по одной причине: и vCard, и iCalendar считают длину строки
в октетах, а не в символах, и кириллическая строка сворачивается не там,
где кажется по буквам. Свёрткой и CRLF занимается fold(), поэтому файлы
можно не править руками — правится словарь ниже, потом:

    python docs/visual/fixtures/make-fixtures.py
"""

import base64
import io
import os

HERE = os.path.dirname(os.path.abspath(__file__))


def esc(v):
    """Экранирование текстового значения по RFC 5545 §3.3.11 / RFC 6350 §3.4."""
    return (v.replace(u'\\', u'\\\\')
             .replace(u'\n', u'\\n')
             .replace(u',', u'\\,')
             .replace(u';', u'\\;'))


def fold(line):
    """Свёртка по 75 октетам; продолжение начинается с одного пробела.

    Режем по границам символов, а считаем байты: иначе разорванный
    UTF-8 попадает в файл и на той стороне получается мусор.
    """
    out, cur, size = [], u'', 0
    limit = 75
    for ch in line:
        n = len(ch.encode('utf-8'))
        if size + n > limit:
            out.append(cur)
            cur, size, limit = u' ' + ch, n + 1, 74
        else:
            cur += ch
            size += n
    out.append(cur)
    return u'\r\n'.join(out)


def write(relpath, lines):
    path = os.path.join(HERE, relpath)
    d = os.path.dirname(path)
    if not os.path.isdir(d):
        os.makedirs(d)
    body = u''.join(fold(l) + u'\r\n' for l in lines)
    with io.open(path, 'w', encoding='utf-8', newline='') as fh:
        fh.write(body)
    print(u'%-42s %5d B' % (relpath, len(body.encode('utf-8'))))


# ─────────────────────────────────────────────────────────────────────────
# Контакты
# ─────────────────────────────────────────────────────────────────────────

def vcard(uid, family, given, rev, tels=(), emails=(), org=None, title=None,
          bday=None, cats=(), note=None, extra=(), fn=None):
    L = [u'BEGIN:VCARD', u'VERSION:3.0']
    L.append(u'UID:%s' % uid)
    L.append(u'N:%s;%s;;;' % (esc(family), esc(given)))
    L.append(u'FN:%s' % esc(fn or u'%s %s' % (given, family)))
    if org:
        L.append(u'ORG:%s' % esc(org))
    if title:
        L.append(u'TITLE:%s' % esc(title))
    for typ, num in tels:
        L.append(u'TEL;TYPE=%s:%s' % (typ, num))
    for typ, addr in emails:
        L.append(u'EMAIL;TYPE=%s:%s' % (typ, addr))
    if bday:
        L.append(u'BDAY:%s' % bday)
    if cats:
        L.append(u'CATEGORIES:%s' % u','.join(cats))
    if note:
        L.append(u'NOTE:%s' % esc(note))
    L.extend(extra)
    L.append(u'REV:%s' % rev)
    L.append(u'END:VCARD')
    return L


# nextcloud · home / «Контакты» — три карточки группы «А» и «В»
kovaleva = vcard(
    uid=u'4f2b-9c31-a0e7', family=u'Ковалёва', given=u'Анна',
    rev=u'20260814T090412Z',
    org=u'Издательство «Слово»', title=u'Выпускающий редактор',
    tels=[(u'CELL', u'+7 912 445-10-88'), (u'WORK', u'+7 495 120-45-00')],
    emails=[(u'WORK', u'a.kovaleva@example.org'),
            (u'second', u'anna.kovaleva@slovo.example')],
    bday=u'1987-03-14', cats=[u'работа', u'издательство'],
    note=u'Пишет по вторникам, отвечает в тот же день.',
    # Адрес — одной строкой, как его набирают (s-contacts-6, mentions-1).
    # Три свойства, которых Carrel не знает: карточка показывает
    # «Kept as the server sent them · Show 3».
    extra=[u'ADR;TYPE=WORK:;;Москва\\, Тверская 12\\, стр. 3;;;;',
           u'X-EVOLUTION-FILE-AS:Ковалёва\\, Анна',
           u'X-ABLABEL:издательство',
           u'X-CARREL-UNKNOWN-FLAG:1'])

# Анна Ковалёва лежит ещё в двух книгах — та же карточка с тем же UID, как
# её пишет «Copy» (s-contacts-17, -33: адреса копий; s-dup-6), — и все три
# связаны (записка Typecase 117 п. 2: «linked ×3», «in 3 books»). Расходятся
# копии только телефоном и почтой — их и спрашивает карточка группы
# (s-dup-1), о них и «differ» (s-contacts-33): в «Команде» рабочий телефон
# другой — его и выбрали, — в «Архиве» нет ни рабочего телефона, ни второй
# почты. Организация и теги у всех трёх одни: о них кадры не спрашивают.
kovaleva_team = vcard(
    uid=u'4f2b-9c31-a0e7', family=u'Ковалёва', given=u'Анна',
    rev=u'20260809T113000Z', org=u'Издательство «Слово»',
    tels=[(u'CELL', u'+7 912 445-10-88'), (u'WORK', u'+7 495 120-45-77')],
    emails=[(u'WORK', u'a.kovaleva@example.org')],
    cats=[u'работа', u'издательство'])

kovaleva_archive = vcard(
    uid=u'4f2b-9c31-a0e7', family=u'Ковалёва', given=u'Анна',
    rev=u'20231105T164200Z', org=u'Издательство «Слово»',
    tels=[(u'CELL', u'+7 912 445-10-88')],
    emails=[(u'WORK', u'a.kovaleva@example.org')],
    cats=[u'работа', u'издательство'])

# Рабочий телефон у Смирнова тот же, что у Анны в «Контактах»: общий номер
# редакции. Это пара, признанная «not duplicates» (s-dup-1, s-dup-6 — там и
# UID; записка Typecase 117 п. 2).
smirnov = vcard(
    uid=u'9a1c-03de-77f2', family=u'Смирнов', given=u'Алексей',
    rev=u'20260731T174501Z',
    tels=[(u'CELL', u'+7 916 200-71-15'), (u'WORK', u'+7 495 120-45-00')])

goncharov = vcard(
    uid=u'a90d-5512-4e83', family=u'Гончаров', given=u'Виктор',
    rev=u'20260805T120000Z',
    org=u'Городская библиотека №4',
    emails=[(u'WORK', u'v.goncharov@example.org')])

# Артём Белов лежит в двух книгах разными записями — это и есть
# «linked ×2» в списке и группа в «Дубликатах», пока их не связали.
belov_home = vcard(
    uid=u'c3f7-8a21-1b09', family=u'Белов', given=u'Артём',
    rev=u'20260612T101500Z', org=u'Тензор',
    tels=[(u'CELL', u'+7 903 118-22-04')],
    emails=[(u'WORK', u'a.belov@example.org')])

belov_team = vcard(
    uid=u'd41a-6f30-92be', family=u'Белов', given=u'Артём',
    rev=u'20260810T081200Z', org=u'Тензор', title=u'Ведущий инженер',
    # Один телефон, как в «Контактах»: группа Артёма спрашивает только фото
    # (s-dup-1, записка Typecase 117).
    tels=[(u'CELL', u'+7 903 118-22-04')],
    emails=[(u'WORK', u'a.belov@example.org')],
    extra=[u'PHOTO;VALUE=uri:https://example.org/avatars/belov.jpg'])

lebedeva = vcard(
    uid=u'e7c2-3b95-6a14', family=u'Лебедева', given=u'Вера',
    rev=u'20240218T203311Z',
    tels=[(u'CELL', u'+7 905 331-09-42')],
    # Два адреса: подсказка участников рисует её двумя строками
    # (s-agenda-12; записка Typecase 46).
    emails=[(u'HOME', u'vera@example.net'), (u'WORK', u'vera.l@work.example')])

# Та же Вера Лебедева, заведённая второй раз — фамилией вперёд и с рабочим
# адресом: в списке она своей строкой в рубрике «Л», в «Дубликатах» —
# кандидат к карточке из «Архива» по тому же телефону (s-contacts-1,
# s-dup-1; записка Typecase 44, решение владельца 10.10.2026: список и
# дубликаты рисуют одних людей).
lebedeva_work = vcard(
    uid=u'b52e-7d08-c6f1', family=u'Лебедева', given=u'Вера',
    fn=u'Лебедева Вера',
    rev=u'20260722T141000Z', org=u'Библиотека №4',
    tels=[(u'CELL', u'+7 905 331-09-42')],
    emails=[(u'WORK', u'v.lebedeva@example.org')],
    extra=[u'PHOTO;VALUE=uri:https://example.org/avatars/lebedeva.jpg'])

write(u'nextcloud-home/kontakty.vcf',
      kovaleva + smirnov + goncharov + belov_home + lebedeva_work)
write(u'nextcloud-home/arhiv.vcf', lebedeva + kovaleva_archive)
write(u'radicale-vps/komanda.vcf', belov_team + kovaleva_team)


# ─────────────────────────────────────────────────────────────────────────
# Календарь, задачи, заметки
# ─────────────────────────────────────────────────────────────────────────

VTZ_MSK = [
    u'BEGIN:VTIMEZONE', u'TZID:Europe/Moscow',
    u'BEGIN:STANDARD', u'DTSTART:19700101T000000',
    u'TZOFFSETFROM:+0300', u'TZOFFSETTO:+0300', u'TZNAME:MSK',
    u'END:STANDARD', u'END:VTIMEZONE']

VTZ_YEKB = [
    u'BEGIN:VTIMEZONE', u'TZID:Asia/Yekaterinburg',
    u'BEGIN:STANDARD', u'DTSTART:19700101T000000',
    u'TZOFFSETFROM:+0500', u'TZOFFSETTO:+0500', u'TZNAME:+05',
    u'END:STANDARD', u'END:VTIMEZONE']


def cal(name, body, zones=(VTZ_MSK,)):
    L = [u'BEGIN:VCALENDAR', u'VERSION:2.0',
         u'PRODID:-//Carrel//mockup fixtures//RU', u'CALSCALE:GREGORIAN',
         u'NAME:%s' % esc(name), u'X-WR-CALNAME:%s' % esc(name)]
    for z in zones:
        L.extend(z)
    L.extend(body)
    L.append(u'END:VCALENDAR')
    return L


# «Личный» — nextcloud · home

# Чек-лист релиза лежит внутри события (s-agenda-1, -2: «in the event»,
# «text/markdown · 3 KB»): 3072 байта Markdown, вложение ATTACH с двоичным
# значением (S-20).
def checklist():
    text = u'# Release checklist\n\n'
    i = 0
    while len(text.encode('utf-8')) < 3072:
        i += 1
        text += u'- [ ] Step %02d: check the build, sign off, note the result.\n' % i
    return text.encode('utf-8')[:3071] + b'\n'


sinkap = [
    u'BEGIN:VEVENT',
    u'UID:8f1c-4a02-b7d3',
    u'DTSTAMP:20260807T121500Z',
    u'CREATED:20260807T121500Z',
    u'LAST-MODIFIED:20260807T121500Z',
    # Заведено в Екатеринбурге: 12:00 там — это 10:00 в Москве.
    # Ради этой метки у времени в макете и стоит исходный пояс (§23.8).
    u'DTSTART;TZID=Asia/Yekaterinburg:20260810T120000',
    u'DTEND;TZID=Asia/Yekaterinburg:20260810T130000',
    u'RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z',
    u'SUMMARY:Синк-ап по релизу',
    u'LOCATION:Переговорная 2',
    u'STATUS:CONFIRMED',
    u'CATEGORIES:release,team',
    u'DESCRIPTION:%s' % esc(u'Обсудить оставшиеся блокеры, дату заморозки '
                            u'и кто пишет письмо пользователям.'),
    u'ORGANIZER;CN=Михаил Енученко:mailto:mix@example.org',
    u'ATTENDEE;CN=Анна Ковалёва;PARTSTAT=ACCEPTED:mailto:a.kovaleva@example.org',
    u'ATTENDEE;CN=Артём Белов;PARTSTAT=TENTATIVE:mailto:a.belov@example.org',
    # Адрес не из книг — незнакомый участник (задача 77: участник — адрес,
    # книги только подсказывают; s-agenda-1, -2 рисуют его адресом).
    u'ATTENDEE;CN=Виктор Гончаров;PARTSTAT=NEEDS-ACTION:'
    u'mailto:v@example.org',
    u'ATTACH;FMTTYPE=text/markdown;ENCODING=BASE64;VALUE=BINARY;'
    u'FILENAME=release-checklist.md:' + base64.b64encode(checklist()).decode('ascii'),
    u'RELATED-TO;RELTYPE=SIBLING:5c81-77aa-0d64',
    # Чужие свойства, которые правка показывает как есть: «Kept as the
    # server sent them» (s-agenda-3).
    u'SEQUENCE:2',
    u'X-MOZ-GENERATION:3',
    u'X-APPLE-TRAVEL-ADVISORY-BEHAVIOR:AUTOMATIC',
    # Напоминания за 10 минут и за день (s-agenda-1, -2: «Reminders · 10
    # minutes before · 1 day before»; s-agenda-3 — оба в правке).
    u'BEGIN:VALARM',
    u'ACTION:DISPLAY',
    u'DESCRIPTION:Синк-ап по релизу',
    u'TRIGGER:-PT10M',
    u'END:VALARM',
    u'BEGIN:VALARM',
    u'ACTION:DISPLAY',
    u'DESCRIPTION:Синк-ап по релизу',
    u'TRIGGER:-P1D',
    u'END:VALARM',
    u'END:VEVENT']

mfc = [
    u'BEGIN:VEVENT',
    u'UID:1d55-90fe-3c27',
    u'DTSTAMP:20260809T173000Z',
    u'DTSTART;TZID=Europe/Moscow:20260811T180000',
    u'DTEND;TZID=Europe/Moscow:20260811T190000',
    u'SUMMARY:Забрать документы',
    u'LOCATION:МФЦ\\, Мясницкая 13',
    u'END:VEVENT']

write(u'nextcloud-home/lichnyj.ics',
      cal(u'Личный', sinkap + mfc, zones=(VTZ_MSK, VTZ_YEKB)))

# «Работа» — nextcloud · home
sozvon = [
    u'BEGIN:VEVENT',
    u'UID:7ae3-1c48-55b0',
    u'DTSTAMP:20260808T090000Z',
    u'DTSTART;TZID=Europe/Moscow:20260810T150000',
    u'DTEND;TZID=Europe/Moscow:20260810T154500',
    u'SUMMARY:Созвон с подрядчиком',
    u'LOCATION:Jitsi',
    u'STATUS:TENTATIVE',
    u'END:VEVENT']

# Первая из двух копий дежурства: вторая лежит в radicale · Дежурства,
# UID у них разные — иначе это один объект, а не «duplicate? ×2».
duty_work = [
    u'BEGIN:VEVENT',
    u'UID:2f04-8b71-e39a',
    u'DTSTAMP:20260601T060000Z',
    u'DTSTART;VALUE=DATE:20260810',
    u'DTEND;VALUE=DATE:20260811',
    u'RRULE:FREQ=WEEKLY;BYDAY=MO',
    u'SUMMARY:Дежурство: первая линия',
    u'END:VEVENT']

write(u'nextcloud-home/rabota.ics', cal(u'Работа', sozvon + duty_work))

# «Дни рождения» — снят флажок в макете, но коллекция существует
write(u'nextcloud-home/dni-rozhdeniya.ics', cal(u'Дни рождения', [
    u'BEGIN:VEVENT',
    u'UID:b604-2ae9-1f77',
    u'DTSTAMP:20260101T000000Z',
    u'DTSTART;VALUE=DATE:20260314',
    u'DTEND;VALUE=DATE:20260315',
    u'RRULE:FREQ=YEARLY',
    u'SUMMARY:День рождения — Анна Ковалёва',
    u'END:VEVENT']))

# radicale · vps
write(u'radicale-vps/proekty.ics', cal(u'Проекты', [
    u'BEGIN:VEVENT',
    u'UID:9c27-0e5d-4412',
    u'DTSTAMP:20260809T140000Z',
    u'DTSTART;TZID=Europe/Moscow:20260811T093000',
    u'DTEND;TZID=Europe/Moscow:20260811T110000',
    u'SUMMARY:Ревью архитектуры',
    u'LOCATION:Переговорная 1',
    u'ORGANIZER;CN=Михаил Енученко:mailto:mix@example.org',
    u'ATTENDEE;CN=Артём Белов;PARTSTAT=ACCEPTED:mailto:a.belov@example.org',
    u'END:VEVENT']))

duty_radicale = [
    u'BEGIN:VEVENT',
    u'UID:5e90-33c1-a8d2',
    u'DTSTAMP:20260601T061500Z',
    u'DTSTART;VALUE=DATE:20260810',
    u'DTEND;VALUE=DATE:20260811',
    u'RRULE:FREQ=WEEKLY;BYDAY=MO',
    u'SUMMARY:Дежурство: первая линия',
    u'END:VEVENT']
write(u'radicale-vps/dezhurstva.ics', cal(u'Дежурства', duty_radicale))


# Задачи (VTODO). «Сегодня» в макетах — неделя 10–16 августа 2026,
# поэтому 7 и 8 августа попадают в «Overdue».
def todo(uid, summary, due=None, cats=(), percent=None, stamp=u'20260810T060000Z',
         desc=None, priority=None, alarm=None, related=None, attach=(), extra=()):
    L = [u'BEGIN:VTODO', u'UID:%s' % uid, u'DTSTAMP:%s' % stamp,
         u'SUMMARY:%s' % esc(summary)]
    if due:
        L.append(u'DUE%s' % due)
    if desc:
        L.append(u'DESCRIPTION:%s' % esc(desc))
    if priority is not None:
        L.append(u'PRIORITY:%d' % priority)
    if cats:
        L.append(u'CATEGORIES:%s' % u','.join(cats))
    if percent is not None:
        L.append(u'PERCENT-COMPLETE:%d' % percent)
    if related:
        L.append(u'RELATED-TO;RELTYPE=SIBLING:%s' % related)
    L.extend(extra)
    L.extend(attach)
    if alarm:
        L.extend([u'BEGIN:VALARM', u'ACTION:DISPLAY',
                  u'DESCRIPTION:%s' % esc(summary), u'TRIGGER:%s' % alarm,
                  u'END:VALARM'])
    L.append(u'END:VTODO')
    return L


# Посадочный внутри задачи (s-tasks-1, -2: «посадочный-казань.pdf ·
# application/pdf · 128 KB · in the task»): 131 072 байта, вложение ATTACH с
# двоичным значением (S-20). Содержимое — заглушка: сверяются имя, тип и
# размер.
def boarding_pass():
    head = b'%PDF-1.4\n% boarding pass, a stub of the mockup set\n'
    body = (b'0123456789abcdef' * 8192)[:131072 - len(head)]
    return head + body


# «Поездка в Казань» — событие, с которым связана «Собрать чемодан»
# (s-tasks-1, -2, -10: «Поездка в Казань · event · 12 Aug»). Кадры агенды
# его не рисуют, и в «Личный» его кладут сами проверки задач — файл лежит
# отдельно и в коллекцию набора не входит (README).
write(u'nextcloud-home/lichnyj-poezdka.ics', cal(u'Личный', [
    u'BEGIN:VEVENT',
    u'UID:3e6b-71d4-a209',
    u'DTSTAMP:20260801T090000Z',
    u'DTSTART;VALUE=DATE:20260812',
    u'DTEND;VALUE=DATE:20260815',
    u'SUMMARY:Поездка в Казань',
    u'END:VEVENT']))

write(u'nextcloud-home/dela.ics', cal(u'Дела',
      # Правка задачи (s-tasks-3): описание, приоритет «High · 1»,
      # напоминание за день, связь с событием «Забрать документы» и три
      # свойства, которых Carrel не знает, — рубрика «Kept as the server sent
      # them» (записка Typecase 69, задача 169).
      todo(u'aa10-77bc-0e31', u'Отправить документы в МФЦ',
           due=u';VALUE=DATE:20260807', cats=[u'дом'],
           desc=u'Опись, копия паспорта и заявление. Приём до 18:00, окно 4.',
           priority=1, alarm=u'-P1D', related=u'1d55-90fe-3c27',
           extra=[u'X-APPLE-SORT-ORDER:712', u'X-OC-HIDESUBTASKS:0', u'SEQUENCE:1']) +
      # Карточка и панель задачи (s-tasks-1, -2): описание, приоритет 5,
      # напоминание за день, посадочный внутри, связь с поездкой.
      todo(u'aa11-2d09-5f84', u'Собрать чемодан',
           due=u';TZID=Europe/Moscow:20260812T180000', cats=[u'дом'],
           desc=u'Зарядки, переходник, распечатать посадочный.',
           priority=5, alarm=u'-P1D', related=u'3e6b-71d4-a209',
           attach=[u'ATTACH;FMTTYPE=application/pdf;ENCODING=BASE64;'
                   u'VALUE=BINARY;FILENAME=посадочный-казань.pdf:' +
                   base64.b64encode(boarding_pass()).decode('ascii')]) +
      todo(u'aa12-9e44-c703', u'Продлить страховку', percent=100) +
      # Выполненная: тег «read» рейла (s-tasks-1) и «Done 1» сегмента.
      [u'BEGIN:VTODO',
       u'UID:aa13-5b2e-71c0',
       u'DTSTAMP:20260810T060000Z',
       u'SUMMARY:Прочитать договор аренды',
       u'STATUS:COMPLETED',
       u'COMPLETED:20260805T120000Z',
       u'PERCENT-COMPLETE:100',
       u'CATEGORIES:read',
       u'END:VTODO']))

write(u'nextcloud-home/rabota-zadachi.ics', cal(u'Работа · задачи',
      todo(u'bb20-4c17-8a52', u'Ответить подрядчику по смете',
           due=u';VALUE=DATE:20260808', cats=[u'релиз'], percent=40) +
      todo(u'bb21-0fa8-3d66', u'Дописать раздел про кэш',
           due=u';VALUE=DATE:20260814', cats=[u'релиз'], percent=70)))


# Заметки (VJOURNAL)
NOTE_TEXT = (u'Договорились заморозить ветку в среду.\n'
             u'\n'
             u'Письмо пользователям пишет Анна, черновик к вечеру вторника. '
             u'Виктор проверяет миграцию на копии базы — если не успевает, '
             u'переносим на следующий спринт.\n'
             u'\n'
             u'## Что делаем\n'
             u'\n'
             u'- [ ] проверить changelog\n'
             u'- [x] попросить у Артёма скриншоты\n'
             u'\n'
             u'Перенос старых вложений в новую папку — Виктор посмотрит, '
             u'сколько их вообще, и скажет в четверг.')

# Доска со встречи внутри заметки (s-notes-1, -2, -20: «2026-08-10-sync-
# board.png · 412 KB · in the note»): 421 888 байт, ATTACH с двоичным
# значением. Содержимое — заглушка: сверяются имя, тип и размер. Запись
# встречи (24 MB) лежит в папке вложений, и ссылку на неё кладут проверки:
# адрес хранилища известен только им (README).
def sync_board():
    head = b'\x89PNG\r\n\x1a\n' + b'sync board, a stub of the mockup set\n'
    body = (b'fedcba9876543210' * 26368)[:421888 - len(head)]
    return head + body


write(u'nextcloud-home/zapisi.ics', cal(u'Записи',
      [u'BEGIN:VJOURNAL',
       u'UID:5c81-77aa-0d64',
       u'DTSTAMP:20260810T080500Z',
       u'DTSTART;TZID=Europe/Moscow:20260810T110500',
       u'SUMMARY:Заметка со встречи 10.08',
       u'CATEGORIES:встречи,релиз',
       u'DESCRIPTION:%s' % esc(NOTE_TEXT),
       u'RELATED-TO;RELTYPE=SIBLING:8f1c-4a02-b7d3',
       # Вторая связь — задача «Дописать раздел про кэш» (s-notes-2).
       u'RELATED-TO;RELTYPE=SIBLING:bb21-0fa8-3d66',
       u'ATTACH;FMTTYPE=image/png;ENCODING=BASE64;VALUE=BINARY;'
       u'FILENAME=2026-08-10-sync-board.png:' +
       base64.b64encode(sync_board()).decode('ascii'),
       # Три свойства, которых Carrel не знает: «X-JTX-COLOR and two more»
       # (s-notes-20).
       u'X-JTX-COLOR:#4A6B52',
       u'X-JTX-PINNED:FALSE',
       u'X-MOZ-LASTACK:20260810T081000Z',
       u'END:VJOURNAL'] +
      [u'BEGIN:VJOURNAL',
       u'UID:6d92-13ef-b408',
       u'DTSTAMP:20260808T190000Z',
       u'DTSTART;TZID=Europe/Moscow:20260808T211000',
       u'SUMMARY:Что почитать про CRDT',
       u'CATEGORIES:книги',
       u'DESCRIPTION:%s' % esc(u'Кляйн, глава 5; статья Шапиро 2011; '
                               u'демка automerge — посмотреть, как они '
                               u'решают конфликт двух правок одного поля.'),
       u'END:VJOURNAL']))

write(u'nextcloud-home/dnevnik.ics', cal(u'Дневник',
      [u'BEGIN:VJOURNAL',
       u'UID:7f03-5b26-cc19',
       u'DTSTAMP:20260803T160000Z',
       u'DTSTART;TZID=Europe/Moscow:20260803T190000',
       u'SUMMARY:Про дорогу до дачи',
       u'DESCRIPTION:%s' % esc(u'Съезд после заправки, потом грунтовка два '
                               u'километра — в дождь лучше в объезд через '
                               u'посёлок.'),
       u'RELATED-TO;RELTYPE=SIBLING:1d55-90fe-3c27',
       u'END:VJOURNAL']))

print(u'\nГотово.')
