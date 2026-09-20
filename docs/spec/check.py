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
            # Карта кадров (U-5.4) обязана назвать классы, которые браузерному
            # продукту не принадлежат, — иначе карта неполна и 3b не проходит.
            if name == "client" and "Не принадлежит браузерному продукту" in row:
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
states = {"normal","empty","readonly","source-error","partial","long","conflict","narrow","loss","unavailable"}
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
    "app-settings-appearance": "/app/settings/appearance",
    "app-settings-attachments": "/app/settings/attachments",
    "app-settings-devices": "/app/settings/devices",
    "app-settings-backup": "/app/settings/backup",
    "app-settings-davloom": "/app/settings/davloom",
    "app-settings-escrow": "/app/settings/escrow",
    "admin-users": "/admin/",
    "admin-invites": "/admin/invites",
    "admin-settings": "/admin/settings",
    "admin-audit": "/admin/audit",
    "admin-dav": "/admin/dav",
    "admin-escrow": "/admin/escrow",
    "admin-install": "/admin/install",
    "login": "/login",
    "forgot": "/forgot",
    "about": "/about",
    "app-duplicates": "/app/duplicates",
    "app-contact-timeline": "/app/contacts/{account}/{col}/{uid}/timeline",
    "app-contact-new": "/app/contacts/{account}/{col}/new",
    "app-contact-import": "/app/contacts/{account}/{col}/import",
    "app-contact-move": "/app/contacts/{account}/{col}/{uid}/move",
    "app-quick": "/app/notes/quick",
    "app-collections-new": "/app/collections/new",
    "app-settings-unlock": "/app/settings/unlock",
    "app-conflict": "/app/{kind}/{account}/{col}/{uid}/conflict",
}

# FRAME_TODO — расхождения, где неверен кадр, а не фикстура. Каждое —
# записанная задача владельца в Typecase (plan/server.md). Они не красные,
# потому что кадры правятся не здесь, но печатаются каждый раз: молчаливое
# исключение — то же расхождение, только невидимое.
# Листы (C-5, ответ «лист») своего рейла не несут: они рисуются
# поверх экрана, и рейл в кадре — того экрана, что под ними. Сравнивать
# тут нечего, и молчание здесь — не исключение, а род экрана.
SHEETS = {"app-contact-move", "app-quick", "app-collections-new", "app-settings-unlock"}

FRAME_TODO = {
    "app-contact-edit/normal: коллекции рейла":
        "s-contacts-3 рисует рейл возврата короче — то же, что у s-contacts-6",
    "app-event-edit/normal: коллекции рейла":
        "s-agenda-3 рисует рейл возврата короче — то же, что у s-agenda-2",
    "app-note/normal: навигация":
        "три кадра называют возврат по-разному: «Back to list» у контактов, "
        "«Combined notebook» у заметок, «Back to tasks» у задач; сервер говорит одно",
    "app-task/normal: навигация":
        "то же: кадр задач говорит «Back to tasks», остальные — «Back to list»",
    "app-task-edit/normal: навигация":
        "то же; с правки возврат ведёт на карточку, а не в список",
    "app-contact-new/normal: коллекции рейла":
        "s-notes-8 объявлен кадром адреса контакта, а рисует рейл календаря",
    "app-contact-timeline/normal: коллекции рейла":
        "s-contacts-7 — хронология человека, а рейл в кадре календарный",
    "app-event/normal: коллекции рейла":
        "s-agenda-2 не рисует «Дни рождения»; рейл возврата несёт все "
        "календари своего подключения (C-6.3) — то же, что у s-contacts-6",
    "app-tasks/normal: коллекции рейла":
        "кадр рисует подписку «Праздники РФ», которой нет в наборе",
    "app-notes/normal: коллекции рейла":
        "кадр рисует тетрадь «Архив», которой нет в наборе («Архив» там — "
        "адресная книга, не тетрадь)",
    "app-agenda/normal: коллекции рейла":
        "кадр рисует две подписки — «Праздники РФ» и «Расписание лиги», — "
        "которых нет в наборе docs/visual/fixtures; добавить их в набор или снять с кадров",
    "app-contact/normal: коллекции рейла":
        "s-contacts-6 показывает одни «Контакты», s-conflict-1 — обе книги "
        "аккаунта; рейл возврата несёт коллекции своего подключения (C-6.3)",
}

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
    # Модификаторы стоят в классе с любой стороны: "tc-tickrow is-all is-on".
    # Шаблон — сразу строка рейла: общий <div class="..."> съел бы внешнюю
    # секцию до первого </div> вместе с первой строкой.
    for m in re.finditer(r'<div class="((?:[^" ]+ )*tc-tickrow(?: [^"]*)?)">(.*?)</div>', frame, re.S):
        classes = m.group(1).split()
        lab = re.search(r'<span class="tc-tickrow__label">([^<]*)</span>', m.group(2))
        meta = re.search(r'<span class="tc-tickrow__meta">([^<]*)</span>', m.group(2))
        if lab and meta:
            cols.append((lab.group(1), meta.group(1),
                         "off" if "is-off" in classes else "on"))
    # Строки навигации собираются по самим строкам, а не по обёртке: рейл
    # настроек делит их на два `tc-nav`, и поиск по первому видел одну строку
    # из девяти — и считал полный рейл расхождением.
    nav = []
    for a in re.finditer(r'<a[^>]*class="[^"]*tc-nav__item[^"]*"[^>]*>(.*?)</a>', frame, re.S):
        num = re.search(r'<span class="tc-num">([^<]*)</span>', a.group(1))
        lab = re.sub(r'<[^>]+>', '', re.sub(r'<span class="tc-num">.*?</span>', '', a.group(1))).strip()
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
    frame = canon_frame(FIXTURE_SCREEN[d])
    if frame is None:
        drift.append("%s: нет кадра для %s" % (d, FIXTURE_SCREEN[d]))
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
    head = d.split(":")[0] + ":" + d.split(":")[1] if d.count(":") > 1 else d
    for k, why in FRAME_TODO.items():
        if d.startswith(k):
            known.append("%s — %s" % (k, why))
            break
    else:
        red.append(d)
check("8. фикстура рисуется в свой кадр", red)
for k in known:
    print("         ждёт Typecase:", k)

print()
print("ИТОГ: %d красных проверок" % len(fails))
sys.exit(1 if fails else 0)
