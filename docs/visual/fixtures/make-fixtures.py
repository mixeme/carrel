# -*- coding: utf-8 -*-
"""Собирает наборы .vcf и .ics, которыми экраны сверяются с макетами.

Данные здесь — ровно те записи, что нарисованы в carrel-ui-mockups.html.
Скрипт нужен по одной причине: и vCard, и iCalendar считают длину строки
в октетах, а не в символах, и кириллическая строка сворачивается не там,
где кажется по буквам. Свёрткой и CRLF занимается fold(), поэтому файлы
можно не править руками — правится словарь ниже, потом:

    python docs/visual/fixtures/make-fixtures.py
"""

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
          bday=None, cats=(), note=None, extra=()):
    L = [u'BEGIN:VCARD', u'VERSION:3.0']
    L.append(u'UID:%s' % uid)
    L.append(u'N:%s;%s;;;' % (esc(family), esc(given)))
    L.append(u'FN:%s %s' % (esc(given), esc(family)))
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
    emails=[(u'WORK', u'a.kovaleva@example.org')],
    bday=u'1987-03-14', cats=[u'работа', u'издательство'],
    note=u'Пишет по вторникам, отвечает в тот же день.',
    # Три свойства, которых Carrel не знает: карточка показывает
    # «Kept as the server sent them · Show 3».
    extra=[u'X-EVOLUTION-FILE-AS:Ковалёва\\, Анна',
           u'X-ABLABEL:издательство',
           u'X-CARREL-UNKNOWN-FLAG:1'])

smirnov = vcard(
    uid=u'6b18-2d40-77c5', family=u'Смирнов', given=u'Алексей',
    rev=u'20260731T174501Z',
    tels=[(u'CELL', u'+7 916 200-71-15')])

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
    tels=[(u'CELL', u'+7 903 118-22-04'), (u'WORK', u'+7 495 780-11-90')],
    emails=[(u'WORK', u'a.belov@example.org')],
    extra=[u'PHOTO;VALUE=uri:https://example.org/avatars/belov.jpg'])

lebedeva = vcard(
    uid=u'e7c2-3b95-6a14', family=u'Лебедева', given=u'Вера',
    rev=u'20240218T203311Z',
    tels=[(u'CELL', u'+7 905 331-09-42')],
    emails=[(u'HOME', u'vera@example.net')])

write(u'nextcloud-home/kontakty.vcf',
      kovaleva + smirnov + goncharov + belov_home)
write(u'nextcloud-home/arhiv.vcf', lebedeva)
write(u'radicale-vps/komanda.vcf', belov_team)


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
    u'ATTENDEE;CN=Виктор Гончаров;PARTSTAT=NEEDS-ACTION:'
    u'mailto:v.goncharov@example.org',
    u'ATTACH;FILENAME=release-checklist.md;FMTTYPE=text/markdown;SIZE=3072:'
    u'https://files.example.org/carrel/release-checklist.md',
    u'RELATED-TO;RELTYPE=SIBLING:5c81-77aa-0d64',
    u'X-JTX-COLOR:#4A6B52',
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
def todo(uid, summary, due=None, cats=(), percent=None, stamp=u'20260810T060000Z'):
    L = [u'BEGIN:VTODO', u'UID:%s' % uid, u'DTSTAMP:%s' % stamp,
         u'SUMMARY:%s' % esc(summary)]
    if due:
        L.append(u'DUE%s' % due)
    if cats:
        L.append(u'CATEGORIES:%s' % u','.join(cats))
    if percent is not None:
        L.append(u'PERCENT-COMPLETE:%d' % percent)
    L.append(u'END:VTODO')
    return L


write(u'nextcloud-home/dela.ics', cal(u'Дела',
      todo(u'aa10-77bc-0e31', u'Отправить документы в МФЦ',
           due=u';VALUE=DATE:20260807', cats=[u'дом']) +
      todo(u'aa11-2d09-5f84', u'Собрать чемодан',
           due=u';TZID=Europe/Moscow:20260812T180000', cats=[u'дом']) +
      todo(u'aa12-9e44-c703', u'Продлить страховку', percent=100)))

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
             u'## Отложено до следующего спринта\n'
             u'\n'
             u'Перенос старых вложений в новую папку — Виктор посмотрит, '
             u'сколько их вообще.')

write(u'nextcloud-home/zapisi.ics', cal(u'Записи',
      [u'BEGIN:VJOURNAL',
       u'UID:5c81-77aa-0d64',
       u'DTSTAMP:20260810T080500Z',
       u'DTSTART;TZID=Europe/Moscow:20260810T110500',
       u'SUMMARY:Заметка со встречи 10.08',
       u'CATEGORIES:встречи,релиз',
       u'DESCRIPTION:%s' % esc(NOTE_TEXT),
       u'RELATED-TO;RELTYPE=SIBLING:8f1c-4a02-b7d3',
       u'X-JTX-COLOR:#4A6B52',
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
