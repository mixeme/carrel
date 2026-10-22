# -*- coding: utf-8 -*-
"""Сверка спецификации.

Запуск: python spec/check.py   (красное — ненулевой код возврата)

Проверяет то, что можно проверить машиной:
адреса кадров против карты маршрутов и разбора экранов, вью-модели против схем,
отсутствие настольного приложения в четырёх браузерных файлах, чистый лист,
ссылки на существующие разделы, разбор фикстур.
"""
import io, os, re, json, glob, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPEC = os.path.join(ROOT, "spec")
MOCK = os.path.join(ROOT, "visual/carrel-ui-mockups.html")

def read(p):
    return io.open(p, encoding="utf-8").read()

common = read(os.path.join(SPEC, "carrel-common.md"))
client = read(os.path.join(SPEC, "carrel-client.md"))
server = read(os.path.join(SPEC, "carrel-server.md"))
desk   = read(os.path.join(SPEC, "carrel-desktop.md"))
mock   = read(MOCK)

fails = []
def check(name, bad, note=""):
    if bad:
        fails.append((name, bad, note))
        print("КРАСНОЕ  %s: %d" % (name, len(bad)))
        for b in list(bad)[:12]:
            print("         -", b)
    else:
        print("зелёное  %s" % name)

# 1. Полнота маршрутов: каждый data-screen из кадров есть в C-5
screens = set()
for m in re.finditer(r'data-screen="([^"]+)"', mock):
    for s in m.group(1).split():
        screens.add(s)
def in_common(x):
    if x in common: return True
    g = re.sub(r'/app/(contacts|calendar|tasks|notes)/', '/app/{kind}/', x)
    return g in common
missing_routes = sorted(s for s in screens if not in_common(s))
check("1. адреса кадров есть в C-5", missing_routes)

# 1b. и упомянуты в U-6 (клиенте) — либо сами, либо через шаблон рода
def in_client(s):
    if s in client:
        return True
    g = re.sub(r'/app/(contacts|calendar|tasks|notes)/', '/app/{kind}/', s)
    return g in client
missing_screens = sorted(s for s in screens if not in_client(s))
check("1b. адреса кадров разобраны в U-6", missing_screens)

# 2. Полнота вью-моделей: имя из таблиц C-5 определено в C-6
vms = set()
for line in common.splitlines():
    if not line.startswith("|"):
        continue
    for m in re.finditer(r'`([A-Z][A-Za-z]*(?:Page|Sheet|Bar|Doc|Trace|Result|Group|Rail|Queue))`', line):
        vms.add(m.group(1))
defined = set(re.findall(r'type\s+([A-Z][A-Za-z]*)\s+struct', common))
missing_vm = sorted(v for v in vms if v not in defined)
check("2. вью-модели из C-5 определены в C-6", missing_vm)

# 5. Браузерные файлы не знают о настольном приложении
# «оболочка» здесь значит каркас страницы, «настольная библиотека» — GUI-тулкит:
# ищутся только те слова, что могут значить одно.
deskwords = [r"carrel-desktop\.md", r"sidecar", r"Wails", r"webview",
             r"настольн\w*\s+приложени", r"настольного приложения"]
leak = []
for name, text in [("common", common), ("server", server), ("client", client)]:
    for w in deskwords:
        for m in re.finditer(w, text, re.I):
            line = text[:m.start()].count(chr(10)) + 1
            row = text.split(chr(10))[line-1].strip()
            # Единственное законное упоминание: C-14.1 обязан назвать, что вне рубежа.
            # Рубеж, не говорящий, что в него не входит, — рубеж, о котором спорят.
            if name == "common" and "в рубеж **не входит**" in row:
                continue
            leak.append("%s:%d: %s" % (name, line, row[:70]))
check("5. в четырёх файлах нет настольного приложения", sorted(set(leak)))

# 5b. Чистый лист: ни статусов, ни следов другого репозитория
dirty = []
for name, text in [("common", common), ("server", server), ("client", client),
                   ("desktop", desk)]:
    for w in [r"\[готово\]", r"\[не сделано\]", r"\[частично\]", r"CHANGELOG",
              r"internal/", r"roadmap", r"§2[0-9]"]:
        for m in re.finditer(w, text, re.I):
            line = text[:m.start()].count(chr(10)) + 1
            dirty.append("%s:%d: %s" % (name, line, text.split(chr(10))[line-1].strip()[:70]))
check("5b. чистый лист: без статусов и следов другого репозитория", sorted(set(dirty)))

# 6. Ссылки на разделы указывают на существующие
nums = {}
for pref, text in [("C", common), ("S", server), ("U", client), ("D", desk)]:
    nums[pref] = set(re.findall(r'^##\s+' + pref + r'-(\d+)\.', text, re.M))
badrefs = []
for pref, text in [("C", common), ("S", server), ("U", client), ("D", desk)]:
    for m in re.finditer(r'\b([CSUD])-(\d+)(?:\.\d+)?\b', text):
        p, n = m.group(1), m.group(2)
        if p in nums and n not in nums[p]:
            line = text[:m.start()].count("\n") + 1
            badrefs.append("%s → %s-%s (стр. %d)" % (pref, p, n, line))
check("6. ссылки на существующие разделы", sorted(set(badrefs)))

# 6b. Названное правило, на которое ссылаются из разных файлов, обязано где-то
# вводиться заголовком. Ловит правило, которое все цитируют и никто не сформулировал.
FILES = [("common", common), ("server", server), ("client", client),
         ("desktop", desk)]
seen = {}
for name, text in FILES:
    for m in re.finditer(r'[Пп]равил\w*\s+([а-яё]+(?:\s+[а-яё]+)?)', text):
        phrase = m.group(1).strip()
        if len(phrase) < 6:
            continue
        seen.setdefault(phrase, set()).add(name)
headings = (chr(10)).join(l for _, t in FILES for l in t.split(chr(10))
                          if l.startswith("#"))
undefined = sorted(p for p, files in seen.items()
                   if len(files) > 1 and p.lower() not in headings.lower())
check("6b. названные правила введены заголовком", undefined)

# 7. Фикстуры разбираются и лежат по правилам имён
states = {"normal","empty","readonly","source-error","partial","long","conflict","narrow","loss","loss-days","unavailable","outcome","refused",
          "sent","options","no-mail","wipe-sent","asked","password","link","no-photo","new",
          "tiles","copy","elsewhere","rename","delete","attach","single","in-part",
          "email-confirmed","email-sent","email-unsent","email-unconfirmed",
          "escrow-deposited","escrow-off","no-copy","forced","edit","edit-links","add","label-taken","feed",
          "issued","mismatch","folder","preview","report","silent",
          "addressbook","note","image","text","download",
          "unknown","deleted","write-refused","moving","broken","exercised","android","thunderbird","other","jobs","job","recovered","off","davloom","match","locked","found","shown","unset","file","too-large","server","history","file-match",
          "calendar","event","link-never","gone","note-files-kept"}
badfix = []
for p in glob.glob(os.path.join(SPEC, "fixtures", "*", "*.json")):
    st = os.path.splitext(os.path.basename(p))[0]
    if st not in states:
        badfix.append("состояние вне списка: " + p)
    try:
        json.load(io.open(p, encoding="utf-8"))
    except Exception as e:
        badfix.append("не разбирается: %s (%s)" % (p, e))
check("7. фикстуры разбираются", badfix)

# 8. Фикстура рисуется в свой кадр.
# Проверка 1 говорит, что кадр назван в карте маршрутов, проверка 2 — что у
# маршрута есть вью-модель. Ни одна не смотрит, что данные фикстуры и данные
# кадра — про один и тот же экземпляр. Без этого фикстура и кадр расходятся
# молча, а клиент, который по U-15.2 сверяется с кадром машинным diff, не
# может отрисовать в него свою фикстуру ни при каком старании.
#
# Сравнивается оболочка — рейл и навигация: их рисует каждый кадр внутри
# входа, поэтому проверка общая, а не про списки контактов.
#   состав коллекций — во всех состояниях: состояние меняет числа и флажки,
#     но не то, какие книги подключены;
#   числа, флажки и счётчики разделов — только в normal: partial нарочно
#     показывает прочерки во время опроса, unavailable — упавший аккаунт.
FIXTURE_SCREEN = {           # каталог фикстуры -> маршрут из C-5
    "app-contacts": "/app/contacts",
    "app-contacts-one": "/app/contacts/{account}/{col}",
    # Подсказки поля участников — фрагмент, своего кадра маршрута нет: кадр
    # состояния — s-agenda-12 (STATE_FRAME), рейла у него нет.
    "app-contacts-suggest": "/app/contacts/suggest",
    # Общий кадр называет только общий адрес (Typecase 80d1319, задачи 97,
    # 108): s-agenda-1 — /app/calendar, s-tasks-1 — /app/tasks, s-notes-1 —
    # /app/notes; адрес коллекции — у кадров одной коллекции s-agenda-17,
    # s-tasks-15, s-notes-31. У контактов оба адреса по-прежнему у
    # s-contacts-1.
    "app-agenda-all": "/app/calendar",
    "app-tasks-all": "/app/tasks",
    "app-notes-all": "/app/notes",
    "app-contact": "/app/contacts/{account}/{col}/{uid}",
    "app-agenda": "/app/calendar/{account}/{col}",
    "app-event": "/app/calendar/{account}/{col}/{uid}",
    "app-tasks": "/app/tasks/{account}/{col}",
    "app-notes": "/app/notes/{account}/{col}",
    "app-task": "/app/tasks/{account}/{col}/{uid}",
    "app-note": "/app/notes/{account}/{col}/{uid}",
    "app-contact-edit": "/app/contacts/{account}/{col}/{uid}/edit",
    "app-event-edit": "/app/calendar/{account}/{col}/{uid}/edit",
    "app-task-edit": "/app/tasks/{account}/{col}/{uid}/edit",
    "app-note-edit": "/app/notes/{account}/{col}/{uid}/edit",
    "app-settings-account": "/app/settings/account",
    "app-settings-connections": "/app/settings/connections",
    "app-settings-connections-disconnect": "/app/settings/connections/disconnect",
    "app-settings-davloom-scope": "/app/settings/davloom/scope",
    "app-settings-davloom-tree": "/app/settings/davloom/tree",
    "app-settings-appearance": "/app/settings/appearance",
    "app-settings-attachments": "/app/settings/attachments",
    "app-settings-devices": "/app/settings/devices",
    "app-settings-backup": "/app/settings/backup",
    "app-settings-backup-trigger": "/app/settings/backup/trigger",
    "app-settings-backup-restore": "/app/settings/backup/restore",
    "app-settings-backup-report": "/app/settings/backup/restore/report",
    "app-settings-davloom": "/app/settings/davloom",
    "app-settings-escrow": "/app/settings/escrow",
    # Листы депонирования и отзыва — поверх Account; кадров со своим адресом
    # у них нет: account-2 (data-screen учётной записи, часть deposit) и
    # фрагмент account-11 (STATE_FRAME).
    "app-settings-escrow-deposit": "/app/settings/escrow/deposit",
    "app-settings-escrow-withdraw": "/app/settings/escrow/withdraw",
    "admin-users": "/admin/",
    "admin-invites": "/admin/invites",
    "admin-settings": "/admin/settings",
    "admin-audit": "/admin/audit",
    "admin-dav": "/admin/dav",
    "admin-escrow": "/admin/escrow",
    "password-history": "/password-history/{token}",
    "admin-install": "/admin/install",
    "login": "/login",
    "forgot": "/forgot",
    "about": "/about",
    "app-duplicates": "/app/duplicates",
    "app-duplicates-merge": "/app/duplicates/merge",
    "app-duplicates-merge-event": "/app/duplicates/merge",
    "app-mail-compose": "/app/mail/compose",
    "app-mail-link": "/app/mail/link",
    "app-search": "/app/search",
    "app-files": "/app/files",
    # Папка: data-screen кадра s-files-1 — маршрут C-5 с {path…} (Typecase
    # 7512157, задача 121).
    "app-files-folder": "/app/files/{account}/{col}/{path…}",
    "app-files-op": "/app/files/{account}/{col}/op/{path…}",
    "app-files-published": "/app/files/published",
    "app-files-picker": "/app/files/picker",
    "invite": "/invite/{token}",
    "setup": "/setup",
    "register": "/register",
    "forgot-token": "/forgot/{token}",
    # Мёртвый секрет своего адреса не имеет: один ответ на все адреса с
    # токеном. Кадр rest-1 — часть (data-part), и сверяется он с кадром
    # приглашения — оба без рейла, так что проверке 8 сравнивать нечего.
    "dead-link": "/invite/{token}",
    "p-token": "/p/{token}",
    # Подтверждение адреса — свой адрес C-5, но кадра экрана у него нет: оба
    # его кадра — части (rest-1 invalid, rest-2 confirmed). Мёртвый токен —
    # фикстура dead-link; normal — подтверждённый адрес, и он сверяется с
    # rest-2 по STATE_FRAME.
    "confirm-email": "/confirm-email/{token}",
    "app-collections-rename": "/app/collections/rename",
    "app-collections-move": "/app/collections/move",
    "app-collections-delete": "/app/collections/delete",
    "app-publish": "/app/publish",
    "app-password": "/app/password",
    "app-contact-timeline": "/app/contacts/{account}/{col}/{uid}/timeline",
    "app-photo-preview": "/app/contacts/{account}/{col}/{uid}/photo-preview",
    "app-contact-new": "/app/contacts/{account}/{col}/new",
    "app-event-new": "/app/calendar/{account}/{col}/new",
    "app-contact-import": "/app/contacts/{account}/{col}/import",
    # Мастер импорта прочих родов (Им19): шаг 1 заметок с деревом папок,
    # предпросмотры и отчёт — каждое состояние со своим кадром (STATE_FRAME).
    "app-event-import": "/app/calendar/{account}/{col}/import",
    "app-task-import": "/app/tasks/{account}/{col}/import",
    "app-note-import": "/app/notes/{account}/{col}/import",
    "app-contact-move": "/app/contacts/{account}/{col}/{uid}/move",
    # Листы копий связанной записи: кадров со своим адресом у них нет —
    # фрагменты s-contacts-32 и s-tasks-16 (STATE_FRAME; задача Typecase 183).
    "app-record-copies": "/app/{kind}/{account}/{col}/{uid}/copies",
    "app-task-done": "/app/tasks/{account}/{col}/{uid}/done",
    # Вопрос перед сохранением серии — ответ POST правки (AskSheet): кадров со
    # своим адресом нет, только фрагменты s-agenda-22…-24 (задача Typecase 184).
    "app-event-ask": "/app/calendar/{account}/{col}/{uid}/edit",
    # Лист удаления записи — ответ POST …/delete, когда есть что терять:
    # кадров со своим адресом нет, фрагменты s-agenda-26 и s-tasks-18
    # (задача Typecase 166, записка 69).
    "app-record-delete": "/app/{kind}/{account}/{col}/{uid}/delete",
    # Лист сброса пароля из шапки Users (Ад8): кадров со своим адресом нет,
    # фрагменты s-admin-13, -14, -16, -20.
    "admin-reset": "/admin/reset",
    # Лист удаления учётной записи поверх Users (s-admin-34, записка 83).
    "admin-delete": "/admin/delete",
    "app-contact-rewrite": "/app/contacts/{account}/{col}/{uid}/rewrite",
    "app-quick": "/app/notes/quick",
    "app-collections-new": "/app/collections/new",
    "app-settings-unlock": "/app/settings/unlock",
    "app-conflict": "/app/{kind}/{account}/{col}/{uid}/conflict",
    "app-conflict-event": "/app/calendar/{account}/{col}/{uid}/conflict",
    "app-conflict-task": "/app/tasks/{account}/{col}/{uid}/conflict",
    "app-conflict-note": "/app/notes/{account}/{col}/{uid}/conflict",
}

# FRAME_TODO — расхождения, где неверен кадр, а не фикстура. Каждое —
# записанная задача владельца в Typecase (plan/server.md). Они не красные,
# потому что кадры правятся не здесь, но печатаются каждый раз: молчаливое
# исключение — то же расхождение, только невидимое.
# Листы (C-5, ответ «лист») своего рейла не несут: они рисуются
# поверх экрана, и рейл в кадре — того экрана, что под ними. Сравнивать
# тут нечего, и молчание здесь — не исключение, а род экрана.
SHEETS = {"app-contact-move", "app-quick", "app-collections-new", "app-settings-unlock",
          "app-collections-rename", "app-collections-move", "app-collections-delete",
          "app-publish", "app-files-picker", "app-files-op", "app-mail-compose", "app-mail-link",
          "app-contact-rewrite", "app-record-copies", "app-task-done", "app-event-ask",
          "app-settings-escrow-deposit", "app-settings-escrow-withdraw", "app-record-delete",
          "admin-reset", "admin-delete", "app-settings-backup-trigger", "app-settings-connections-disconnect",
          "app-settings-davloom-scope", "app-settings-davloom-tree"}

FRAME_TODO = {
    # Полная форма нового события — свой кадр s-agenda-15 (Typecase 9e69804,
    # задача 77): строки больше нет.
    # Рейл предпросмотра событий (s-io-6) без «Дни рождения» — снятой
    # отметкой коллекции; кадр карточки того же календаря (s-agenda-2) её
    # рисует, и фикстуры карточки с ним сходятся (Им19).
    "app-event-import: коллекции рейла":
        "задача Typecase 199 п. 1 (to-typecase.md): s-io-6 не рисует «Дни рождения», s-agenda-2 рисует",
}

# Расхождения, где кадр прав, а сервер ещё нет (макет первичен): каждое —
# записка в from-typecase.md или долг сервера в plan/client.md. Закрылось на
# сервере — строка уходит. Ключ — как у FRAME_TODO: «каталог: что
# расходится», без состояния; держит проверки 8 и 9.
SERVER_TODO = {
    # «Tick every book» (states-7) основная с сервера v0.2.118 (долг C19).
    # Рейл дубликатов — все источники четырёх родов плоским списком, у
    # каждого — сколько его записей сравнено (Typecase 7512157, задача 136,
    # ДбЖ4; s-dup-1, -2); сервер шлёт рейл контактов.
    "app-duplicates: коллекции рейла":
        "записка 38 (from-typecase.md), ДбЖ4 — круг сервера по дубликатам (plan/server.md)",
    "app-duplicates-merge: коллекции рейла":
        "записка 38 (from-typecase.md), ДбЖ4 — круг сервера по дубликатам (plan/server.md)",
    # Набор держит три коллекции и вторую ленту, которых кадры не рисуют
    # (лента — источник с сервера v0.2.285, Дб3).
    "app-duplicates-merge-event: коллекции рейла":
        "данные набора против s-dup-4 (scans/duplicates.md, Дб3)",
}

# Состояние, которое рисует свой кадр маршрута — часть (data-part), а не
# кадр экрана целиком: отказ входа — s-auth-13, не s-auth-1. Шаги страниц
# вне оболочки названы по data-part своего кадра (C-14.3).
STATE_FRAME = {
    # davloom (Нс52): диалог добавления и выданный пароль — фрагменты-диалоги
    # над списком.
    "app-settings-davloom/add": "s-davloom-2",
    "app-settings-davloom/issued": "s-davloom-3",
    # Пустой, выключенный, листы области и дерева (записка Typecase 96).
    "app-settings-davloom/empty": "s-davloom-6",
    "app-settings-davloom/off": "s-davloom-8",
    "app-settings-davloom-scope/normal": "s-davloom-7",
    "app-settings-davloom-tree/normal": "s-davloom-9",
    # Лист пароля — один на всякую выдачу (записка Typecase 86): поверх
    # формы добавления устройства с набранным и удаление учётной записи.
    "app-settings-unlock/davloom": "s-davloom-5",
    "app-settings-unlock/delete": "account-15",
    # Состояния экрана расхождения (записка Typecase 77): чья правка не
    # известна, удалено на сервере, комбинация не пишется, сервер отказал в
    # записи, источник недоступен, заметка ещё не перенесена — части
    # маршрута по data-part.
    "app-conflict/unknown": "s-conflict-6",
    "app-conflict/deleted": "s-conflict-7",
    "app-conflict-event/refused": "s-conflict-8",
    "app-conflict/write-refused": "s-conflict-9",
    "app-conflict/unavailable": "s-conflict-10",
    "app-conflict-note/moving": "s-conflict-11",
    # Дубликаты (Дб10, Дб21): ничего не прочитано — s-dup-6; слияние
    # события — s-dup-4; слияние, которому сервер отказал в удалении, —
    # s-dup-5 (части маршрута по data-part; событие — своим каталогом, как
    # расхождение события).
    "app-duplicates/empty": "s-dup-6",
    "app-duplicates-merge-event/normal": "s-dup-4",
    "app-duplicates-merge/in-part": "s-dup-5",
    "login/refused": "s-auth-13",
    "register/sent": "s-auth-3",
    "forgot/options": "s-auth-6",
    "forgot/no-mail": "s-auth-7",
    "forgot/wipe-sent": "s-auth-8",
    "forgot/asked": "s-auth-9",
    "forgot-token/password": "s-auth-11",
    "confirm-email/normal": "rest-2",
    # Мёртвая ссылка отвечает на четырёх маршрутах секрета; FIXTURE_SCREEN
    # ставит её к приглашению, а рисуется она своим кадром.
    "dead-link/normal": "rest-1",
    # Итог листа «переписать адрес» на карточке рисует свой кадр, s-contacts-16
    # (без data-screen: у маршрута карточки кадр — s-contacts-6).
    "app-contact/outcome": "s-contacts-16",
    # Книга только для чтения (states-4, без data-screen) и правка карточки
    # без фотографии (s-contacts-4, часть no-photo): их рейлы — набор книг с
    # Typecase d5481b2 (задача 55).
    "app-contacts-one/readonly": "states-4",
    "app-contact-edit/no-photo": "s-contacts-4",
    # Состояния списка контактов и правки с потерей — свои кадры главы
    # состояний (Typecase 515beec, задача 66), части маршрутов по data-part.
    "app-contacts/empty": "states-7",
    "app-contacts/partial": "states-8",
    "app-contacts/unavailable": "states-9",
    "app-contacts-one/empty": "states-10",
    "app-contacts-one/source-error": "states-11",
    "app-contact-edit/loss": "states-12",
    # Состояния агенды — свои кадры (Typecase 9e69804, задача 77): опрос
    # идёт (states-13), ничего не отмечено (states-14), пустая неделя одного
    # календаря — одна строка, а не семь пустых дней (states-15).
    "app-agenda-all/partial": "states-13",
    "app-agenda-all/empty": "states-14",
    "app-agenda/empty": "states-15",
    # Приглашение ссылкой и потеря свойств при записи события — свои кадры
    # (Typecase 05ea994, 002fce3; задачи 79, 93).
    "invite/link": "s-auth-14",
    # Подсказки поля участников (Typecase de3f083, задача 77): «ве» — Вера.
    "app-contacts-suggest/normal": "s-agenda-12",
    "app-event-edit/loss": "states-17",
    # Дни серии, которых сервер не сохранил, — словами (Typecase 14fa0c2,
    # записка 62, Р-60).
    "app-event-edit/loss-days": "states-36",
    # Папка (скан файлов, ФБ5): выделено три с очередью загрузки — s-files-2,
    # плитки — s-files-5. Листы операции и выбора — кадры-фрагменты своих
    # состояний (рейла у листа нет, SHEETS): копия s-files-11, другое
    # хранилище s-files-12, отказ New folder s-files-13 и s-files-19, From
    # Files s-files-16. Листы переименования, удаления и новой папки — свои
    # кадры s-files-21 … -23 (Typecase 7512157, задача 121); фикстуры листа
    # операции их состояний пока не называют. Состояния папки — свои кадры
    # (там же): пуста (states-32), сервер хранилища не ответил (states-33),
    # только для чтения (states-34).
    # Состояния задач и заметок — свои кадры (Typecase 80d1319, задача 97):
    # ничего не отмечено (states-18, -25), сервер одного списка не ответил
    # (states-22, -29), потеря свойств при записи задачи (states-24); полные
    # формы новой задачи и новой заметки — состояние new каталога правки, их
    # кадры — s-tasks-14 и s-notes-30 (маршрут …/new). Кадры states-19 … -21,
    # -23, -26 … -28, -30, -31 (опрос идёт, источник отказал, пустой список,
    # только для чтения, потеря у заметки) фикстур пока не имеют.
    "app-tasks-all/empty": "states-18",
    "app-tasks/source-error": "states-22",
    "app-task-edit/loss": "states-24",
    "app-task-edit/new": "s-tasks-14",
    "app-notes-all/empty": "states-25",
    "app-notes/source-error": "states-29",
    "app-note-edit/new": "s-notes-30",
    "app-files-folder/empty": "states-32",
    "app-files-folder/source-error": "states-33",
    "app-files-folder/readonly": "states-34",
    "app-files-folder/long": "s-files-2",
    # Загрузка, которой отказали, — очередь под тремя выделенными, как
    # s-files-2 (строка отказа — фрагмент s-files-24).
    "app-files-folder/refused": "s-files-2",
    "app-files-folder/tiles": "s-files-5",
    # Кадры маршрута (записка 73, 183 п. 1): над книгой и над объединённым списком.
    "app-record-copies/normal": "s-contacts-35",
    "app-task-done/normal": "s-tasks-19",
    # Кадры маршрута (записка 74, 184 п. 1): вопрос поверх правки — s-agenda-27,
    # отказ поверх карточки серии — s-agenda-28; одна встреча — фрагмент s-agenda-23.
    "app-event-ask/normal": "s-agenda-27",
    "app-event-ask/single": "s-agenda-23",
    "app-event-ask/in-part": "s-agenda-28",
    "app-files-op/copy": "s-files-11",
    "app-files-op/elsewhere": "s-files-12",
    "app-files-op/refused": "s-files-13",
    "app-files-op/rename": "s-files-21",
    "app-files-op/delete": "s-files-22",
    "app-files-op/new": "s-files-23",
    "app-files/empty": "states-35",
    # Список без почты — страница целиком (записка 83 п. 15); s-publish-17 — фрагмент строк.
    "app-files-published/no-mail": "s-publish-20",
    "app-files-published/empty": "s-publish-18",  # Пб42: пустой список
    "app-files-picker/attach": "s-files-16",
    "app-files-picker/refused": "s-files-19",
    # Учётная запись (Нс9, Нс11, Нс13, Нс16): состояния блоков Email и Key
    # escrow — фрагменты account-7 … -10, -13 и s-mail-9; страница
    # депонирования без копии — account-12, со схемой выключенной —
    # account-14 (normal — копия есть, кадр маршрута account-4); листы
    # депонирования и отзыва — account-2 и account-11. Вынужденная смена
    # пароля (Нс17) — в кадре маршрута account-3 вместе с обычной.
    "app-settings-account/email-confirmed": "account-8",
    "app-settings-account/email-sent": "account-9",
    "app-settings-account/email-unsent": "account-10",
    "app-settings-account/email-unconfirmed": "s-mail-9",
    "app-settings-account/escrow-deposited": "account-7",
    "app-settings-account/escrow-off": "account-13",
    "app-settings-escrow/no-copy": "account-12",
    "app-settings-escrow/escrow-off": "account-14",
    "app-settings-escrow-deposit/normal": "account-2",
    # Вынужденная смена пароля — свой кадр (записка Typecase 101).
    "app-password/forced": "account-17",
    "app-settings-escrow-withdraw/normal": "account-11",
    # Диалог правки подключения (Нс76): вопрос о ссылках ждёт скрытым —
    # s-profile-16; Save без ответа — s-profile-23 (Typecase ad18c91).
    "app-contact-import/normal": "s-io-3",  # шаг 1 — свой кадр, не s-io-5 (Им20)
    "app-contact-import/preview": "s-io-5",
    "app-contact-import/report": "s-io-9",
    "app-event-import/preview": "s-io-6",
    "app-event-import/report": "s-io-14",
    "app-task-import/preview": "s-io-7",
    "app-note-import/normal": "s-io-4",
    "app-note-import/preview": "s-io-8",
    "app-record-delete/normal": "s-agenda-26",
    "admin-reset/normal": "s-admin-13",
    "app-collections-new/folder": "s-profile-20",
    "app-collections-new/silent": "s-profile-21",
    "app-collections-rename/folder": "s-profile-18",
    "app-collections-delete/folder": "s-profile-19",
    "app-collections-new/refused": "s-profile-6",
    "admin-reset/issued": "s-admin-14",
    "admin-reset/refused": "s-admin-16",
    "admin-reset/mismatch": "s-admin-20",
    # Прогоны, которые кадры и рисуют (Ад31, Ад33): проверка установки и
    # след обнаружения. До прогона своего кадра нет (В7, В8).
    "admin-install/report": "s-admin-4",
    # Проверка до первого прогона и отказ адреса (записка Typecase 79).
    "admin-install/normal": "s-admin-25",
    "admin-install/refused": "s-admin-26",
    "admin-dav/report": "s-admin-5",
    # Проверка DAV: пустая, обнаружение встало, второй этап (записка Typecase 80).
    "admin-dav/normal": "s-admin-27",
    "admin-dav/broken": "s-admin-28",
    "admin-dav/exercised": "s-admin-29",
    # Новое устройство по вкладкам, выдача открыта (Нс43).
    "app-settings-devices/link": "s-plan-1",
    "app-settings-devices/android": "s-plan-2",
    "app-settings-devices/thunderbird": "s-plan-4",
    "app-settings-devices/other": "s-plan-5",
    # Копии: без заданий — s-backup-6, список с заданиями — s-backup-1 (Нс54).
    "app-settings-backup/normal": "s-backup-6",
    "app-settings-backup/jobs": "s-backup-1",
    "app-settings-backup/job": "s-backup-2",
    # Лист внешнего запуска поверх списка заданий (записка Typecase 91).
    "app-settings-backup-trigger/normal": "s-backup-12",
    "app-settings-backup-trigger/issued": "s-backup-13",
    "app-settings-backup-trigger/off": "s-backup-14",
    # Восстановление на своей странице: шаг источника и сопоставление.
    "app-settings-backup-restore/normal": "s-backup-7",
    "app-settings-backup-restore/match": "s-backup-8",
    # Шаги восстановления и отчёт (записки Typecase 97, 99).
    "app-settings-backup-restore/file": "s-backup-15",
    "app-settings-backup-restore/too-large": "s-backup-16",
    "app-settings-backup-restore/server": "s-backup-17",
    # Шаг 2 архива с этого компьютера, открытого кусками (записка Typecase 109).
    "app-settings-backup-restore/file-match": "s-backup-20",
    "app-settings-backup-report/normal": "s-backup-18",
    # Депонирование у администратора и история пароля (записка Typecase 81).
    "admin-escrow/recovered": "s-admin-30",
    "admin-escrow/off": "s-admin-31",
    "admin-delete/normal": "s-admin-34",
    # Проба почты: ушла и отказ по полю (записка Typecase 82).
    "admin-settings/sent": "s-admin-32",
    "admin-settings/refused": "s-admin-33",
    # Публичные страницы (Пс1): все кадры маршрута — части (data-part),
    # у каждого вида страницы свой.
    # Готовая ссылка (Пб13): тот же лист после Create link.
    "app-publish/link": "s-publish-13",
    # «Send this link» (записка 105): лист над Published links, кадр без
    # data-screen.
    "app-mail-link/normal": "s-mail-4",
    # Листы публикации по родам (записка 105): контакт, событие; готовая
    # бессрочная без почты; мёртвая строка списка.
    "app-publish/normal": "s-publish-3",
    "app-publish/event": "s-publish-19",
    "app-publish/link-never": "s-publish-22",
    "app-files-published/gone": "s-publish-21",
    "p-token/normal": "s-publish-6",
    "p-token/addressbook": "s-publish-7",
    "p-token/note": "s-publish-8",
    "p-token/note-files-kept": "s-publish-23",
    "p-token/image": "s-publish-9",
    "p-token/text": "s-publish-10",
    "p-token/download": "s-publish-11",
    "p-token/folder": "s-publish-12",
    "app-record-delete/single": "s-tasks-18",
    "app-settings-connections/edit": "s-profile-16",
    # Нс30: диалог Connect account (s-profile-2), занятая подпись
    # (s-profile-11), панель подписки (s-profile-10, часть subscribe).
    "app-settings-connections/add": "s-profile-2",
    "app-settings-connections/label-taken": "s-profile-11",
    "app-settings-connections/feed": "s-profile-10",
    "app-settings-connections/edit-links": "s-profile-23",
    # Отказ логина, ни одного подключения, лист Disconnect (записка Typecase 92).
    "app-settings-connections/refused": "s-profile-24",
    # Новое устройство при закрытой выдаче (записка Typecase 93).
    "app-settings-devices/locked": "s-plan-6",
    "app-settings-connections/empty": "s-profile-25",
    # После Update collection list (записка Typecase 103).
    "app-settings-connections/found": "s-profile-27",
    # Вложения: папка не выбрана и недоступна (записка Typecase 93).
    "app-settings-attachments/unset": "attachments-5",
    # Новое задание копии (записка Typecase 99).
    "app-settings-backup/new": "s-backup-19",
    # Депонирование после восстановления (записка Typecase 99).
    "app-settings-escrow/history": "account-16",
    "app-settings-attachments/unavailable": "attachments-6",
    "app-settings-connections/shown": "s-profile-4",
    "app-settings-connections-disconnect/normal": "s-profile-26",
}
# Без строки: app-contact-new/normal и app-event-new/normal — полные формы
# создания (`/new`) — свои кадры маршрута, s-contacts-27 (Typecase 4af7ad7,
# задача 59) и s-agenda-15 (Typecase 9e69804, задача 77).

def frame_by_id(fid):
    # Кадр шире обычного несёт второй класс (`frame cap900`, публичные
    # страницы s-publish-6, -12).
    m = re.search(r'<div class="frame(?: [^"]*)?" data-frame="%s"[^>]*>' % re.escape(fid), mock)
    if not m:
        return None
    nxt = re.compile(r'<div class="frame[ "]').search(mock, m.end())
    return mock[m.end():nxt.start() if nxt else len(mock)]

def as_kind(x):
    """Кадр рода пишет свой маршрут, карта — шаблон: сводим к шаблону."""
    return re.sub(r'/app/(contacts|calendar|tasks|notes)/', '/app/{kind}/', x)

def canon_frame(route):
    """Кадр маршрута: тот, что рисует экран целиком, а не его кусок.

    Сначала точное совпадение, и только потом по шаблону рода:
    `/app/calendar/{account}/{col}` и `/app/contacts/{account}/{col}` сводятся
    к одному шаблону, и без этого агенда сверялась с кадром контактов.
    """
    for exact in (True, False):
        for m in re.finditer(r'<div class="frame"([^>]*)>', mock):
            attrs = m.group(1)
            if 'data-part=' in attrs:
                continue
            ds = re.search(r'data-screen="([^"]+)"', attrs)
            if not ds:
                continue
            names = ds.group(1).split()
            hit = route in names if exact else as_kind(route) in [as_kind(s) for s in names]
            if hit:
                nxt = mock.find('<div class="frame"', m.end())
                return mock[m.end():nxt if nxt > 0 else len(mock)]
    return None

def frame_rail(frame):
    cols = []
    # Только рейл: он в разметке раньше главной области (tc-main), а строки-отметки
    # бывают и в формах главной («What to copy» задания копии, s-backup-2).
    at = frame.find('<div class="tc-main')
    rail = frame[:at] if at >= 0 else frame
    # Модификаторы стоят в классе с любой стороны: "tc-tickrow is-all is-on".
    # Шаблон — сразу строка рейла: общий <div class="..."> съел бы внешнюю
    # секцию до первого </div> вместе с первой строкой.
    for m in re.finditer(r'<div class="((?:[^" ]+ )*tc-tickrow(?: [^"]*)?)">(.*?)</div>', rail, re.S):
        classes = m.group(1).split()
        lab = re.search(r'<span class="tc-tickrow__label">([^<]*)</span>', m.group(2))
        meta = re.search(r'<span class="tc-tickrow__meta">([^<]*)</span>', m.group(2))
        if lab:
            # Число — только цифры. «ro» у ленты и причина у не ответившей
            # коллекции («password · 401») стоят на месте числа, но числом не
            # являются: счёта у такой строки нет, как и прочерка у фикстуры.
            # Строка без места под число — тоже строка: снятая коллекция
            # только для чтения («Public» в s-files-6) рисуется без него, и
            # пропуск её делал состав рейла короче, чем он есть.
            num = meta.group(1).strip() if meta else "—"
            cols.append((lab.group(1), num if num.isdigit() else "—",
                         "off" if "is-off" in classes else "on"))
    # Строки навигации собираются по самим строкам, а не по обёртке: рейл
    # настроек делит их на два `tc-nav`, и поиск по первому видел одну строку
    # из девяти — и считал полный рейл расхождением.
    nav = []
    for a in re.finditer(r'<a[^>]*class="[^"]*tc-nav__item[^"]*"[^>]*>(.*?)</a>', frame, re.S):
        num = re.search(r'<span class="tc-num">([^<]*)</span>', a.group(1))
        # У возврата две подписи: полная — рейла, короткая — нижней панели
        # телефона (NavItem.Short); сервер шлёт полную, сверяется она.
        body = re.sub(r'<span class="tc-nav__short">.*?</span>', '', a.group(1))
        lab = re.sub(r'<[^>]+>', '', re.sub(r'<span class="tc-num">.*?</span>', '', body)).strip()
        # Строка без подписи — не строка: в кадрах так стоит пустой
        # якорь того же класса, и считать его значило бы требовать от сервера
        # безымянного действия.
        if lab == "":
            continue
        nav.append((lab, num.group(1) if num else None))
    return cols, nav

def fixture_rail(data):
    rail = data.get("rail") or {}
    cols = [(r.get("label"),
             "—" if r.get("count") is None else str(r.get("count")),
             "off" if r.get("tick") == "off" else "on")
            for s in rail.get("sections") or [] for r in s.get("rows") or []]
    nav = [(n.get("label"), None if n.get("count") is None else str(n.get("count")))
           for n in rail.get("nav") or []]
    return cols, nav

drift = []
for p in sorted(glob.glob(os.path.join(SPEC, "fixtures", "*", "*.json"))):
    d = os.path.basename(os.path.dirname(p))
    state = os.path.splitext(os.path.basename(p))[0]
    if d not in FIXTURE_SCREEN:
        drift.append("%s: каталог не назван в FIXTURE_SCREEN — неизвестно, какой кадр сверять" % d)
        continue
    key = "%s/%s" % (d, state)
    frame = frame_by_id(STATE_FRAME[key]) if key in STATE_FRAME else canon_frame(FIXTURE_SCREEN[d])
    if frame is None:
        drift.append("%s: нет кадра для %s" % (key, STATE_FRAME.get(key, FIXTURE_SCREEN[d])))
        continue
    try:
        data = json.load(io.open(p, encoding="utf-8"))
    except Exception:
        continue                      # об этом уже сказала проверка 7
    fc, fn = frame_rail(frame)
    xc, xn = fixture_rail(data)
    where = "%s/%s" % (d, state)
    # Кадр и фикстура обязаны сойтись в том, что экран показывает, и только
    # в этом.
    #
    # Не в порядке коллекций: его задаёт DAV-сервер, Carrel его сохраняет
    # (кадры так и рисуют — «Личный, Работа, Дни рождения»), а в наборе
    # порядок даёт davtest, который сортирует по пути. Требовать совпадения
    # значило бы требовать от тестового сервера порядка боевого.
    #
    # Не в числах: README набора говорит, что 248, 51 и 13 в макете —
    # иллюстрация, а фикстура несёт числа набора (решение 20.09.2026).
    # Сверяется прочерк: «—» в кадре — это «не отвечала», и в фикстуре ему
    # обязан отвечать null, а не ноль.
    if d in SHEETS:
        continue
    if sorted(c[0] for c in fc) != sorted(c[0] for c in xc):
        drift.append("%s: коллекции рейла: кадр %s, фикстура %s"
                     % (where, sorted(c[0] for c in fc), sorted(c[0] for c in xc)))
    elif state == "normal":
        marks = dict((c[0], (c[1] == "—", c[2])) for c in fc)
        for lab, num, tick in xc:
            dash, on = marks[lab]
            if (num == "—") != dash or tick != on:
                drift.append("%s: коллекция «%s»: кадр %s, фикстура %s"
                             % (where, lab, ("—" if dash else "число", on), (num, tick)))
        if [n[0] for n in fn] != [n[0] for n in xn]:
            drift.append("%s: навигация: кадр %s, фикстура %s"
                         % (where, [n[0] for n in fn], [n[0] for n in xn]))
known, red = [], []
for d in drift:
    # Ключ — «каталог: что расходится», без состояния: кадр неверен для
    # экрана, а не для одного из его состояний.
    flat = re.sub(r'^([^/]+)/[^:]+:', lambda m: m.group(1) + ':', d)
    for who, todo in (("Typecase", FRAME_TODO), ("сервер", SERVER_TODO)):
        hit = [k for k in todo if flat.startswith(k)]
        if hit:
            known.append("ждёт %s: %s — %s" % (who, hit[0], todo[hit[0]]))
            break
    else:
        red.append(d)
check("8. фикстура рисуется в свой кадр", red)
for k in sorted(set(known)):
    print("         " + k)

# 9. Основная кнопка и моноширинное поле — как в кадре.
# Кадр рисует основное действие классом is-primary у tc-btn, поле, куда
# набирают логин, адрес, код или UID, — is-mono у tc-in. Модель говорит то
# же: Action.Primary и FormRow.Mono (C-6). Сверяется по подписи: действие
# фикстуры, чью подпись кадр рисует кнопкой, основное ровно тогда, когда
# кадр рисует её is-primary; строка формы, чью подпись кадр рисует полем, —
# моноширинная ровно тогда, когда поле is-mono. Подпись, которую кадр рисует
# и так и так, не сверяется: по ней не понять, о какой кнопке речь.
def tag_text(s):
    s = re.sub(r'<svg.*?</svg>', '', s, flags=re.S)
    s = re.sub(r'<[^>]+>', ' ', s)
    return " ".join(html_unescape(s).split())

def html_unescape(s):
    import html
    return html.unescape(s)

def frame_buttons(frame):
    """Подпись кнопки -> множество «основная ли» по всем её местам в кадре."""
    out = {}
    # Apply отметки полосы (tc-checkform__apply, Typecase 5b9d0a9, задача 67)
    # отправляет форму без сценариев — кнопка библиотеки, не действие модели:
    # у формы «Compact rows» действий нет. Сверять с ней Apply меню столбцов
    # (columns.apply, основная в своём кадре s-contacts-25) значило бы сверять
    # чужую кнопку.
    frame = re.sub(r'<span class="tc-checkform__apply">.*?</span>', '', frame, flags=re.S)
    for m in re.finditer(r'<(button|a) class="(tc-btn(?: [^"]*)?)"[^>]*>(.*?)</\1>', frame, re.S):
        lab = tag_text(m.group(3))
        if lab:
            out.setdefault(lab, set()).add("is-primary" in m.group(2).split())
    return out

def frame_inputs(frame):
    """Подпись поля -> множество «моноширинное ли»."""
    out = {}
    for m in re.finditer(r'<(input|textarea)\s[^>]*>', frame):
        tag = m.group(0)
        c = re.search(r'class="([^"]*)"', tag)
        if not c or "tc-in" not in c.group(1).split():
            continue
        lab = None
        before = frame[:m.start()]
        i = before.rfind('<div class="tc-field')
        if i >= 0:
            seg = before[i:]
            if seg.count("<div") > seg.count("</div>"):
                l = re.search(r'<label>(.*?)</label>', seg, re.S)
                if l:
                    lab = tag_text(re.sub(r'<span class="tc-dim">.*?</span>', '', l.group(1), flags=re.S))
        if not lab:
            a = re.search(r'aria-label="([^"]*)"', tag)
            lab = html_unescape(a.group(1)) if a else None
        if lab:
            out.setdefault(lab, set()).add("is-mono" in c.group(1).split())
    return out

def walk(x, acts, rows):
    if isinstance(x, dict):
        # A row of the rail's nav (nav.*) is a link of the rail, not a
        # button: the frame draws it tc-nav__item, and a screen whose button
        # has the same words («Back to list» of the import's report) is not
        # its double.
        if "label" in x and "method" in x and "id" in x and "enabled" in x and not str(x.get("id", "")).startswith("nav."):
            acts.append(x)
        if "name" in x and "kind" in x and "label" in x and x.get("kind") != "hidden":
            rows.append(x)
        for k, v in x.items():
            if k == "under":
                continue              # экран под листом держат его фикстуры (C-6 Under)
            walk(v, acts, rows)
    elif isinstance(x, list):
        for v in x:
            walk(v, acts, rows)

marks, known9 = [], []
for p in sorted(glob.glob(os.path.join(SPEC, "fixtures", "*", "*.json"))):
    d = os.path.basename(os.path.dirname(p))
    state = os.path.splitext(os.path.basename(p))[0]
    key = "%s/%s" % (d, state)
    if d not in FIXTURE_SCREEN:
        continue                      # об этом уже сказала проверка 8
    frame = frame_by_id(STATE_FRAME[key]) if key in STATE_FRAME else canon_frame(FIXTURE_SCREEN[d])
    if frame is None:
        continue
    frame = frame.split("<figcaption")[0]
    try:
        data = json.load(io.open(p, encoding="utf-8"))
    except Exception:
        continue
    acts, rows = [], []
    walk(data, acts, rows)
    fb, fi = frame_buttons(frame), frame_inputs(frame)
    for a in acts:
        lab = " ".join(str(a.get("label") or "").split())
        drawn = fb.get(lab)
        if drawn is None or len(drawn) != 1:
            continue
        want = next(iter(drawn))
        if bool(a.get("primary")) != want:
            marks.append("%s: «%s» (%s) — кадр рисует %s" % (key, lab, a.get("id"),
                         "основной" if want else "не основной"))
            flat = "%s: «%s» (%s)" % (d, lab, a.get("id"))
            if flat in SERVER_TODO:
                known9.append("ждёт сервер: %s — %s" % (flat, SERVER_TODO[flat]))
                marks.pop()
    for r in rows:
        lab = " ".join(str(r.get("label") or "").split())
        drawn = fi.get(lab)
        if drawn is None or len(drawn) != 1:
            continue
        want = next(iter(drawn))
        if bool(r.get("mono")) != want:
            marks.append("%s: поле «%s» (%s) — кадр рисует %s" % (key, lab, r.get("name"),
                         "моноширинным" if want else "обычным"))
check("9. основная кнопка и моноширинное поле — как в кадре", sorted(set(marks)))
for k in sorted(set(known9)):
    print("         " + k)

print()
print("ИТОГ: %d красных проверок" % len(fails))
sys.exit(1 if fails else 0)
