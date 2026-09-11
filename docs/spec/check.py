# -*- coding: utf-8 -*-
"""Сверка спецификации.

Запуск: python spec/check.py   (красное — ненулевой код возврата)

Проверяет то, что можно проверить машиной:
адреса кадров против карты маршрутов и разбора экранов, вью-модели против схем,
виджеты против состава библиотеки, независимость библиотеки, отсутствие
настольного приложения в четырёх браузерных файлах, чистый лист, ссылки на
существующие разделы, разбор фикстур.
"""
import io, os, re, json, glob, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPEC = os.path.join(ROOT, "spec")
MOCK = os.path.join(ROOT, "visual/carrel-ui-mockups.html")

def read(p):
    return io.open(p, encoding="utf-8").read()

common = read(os.path.join(SPEC, "carrel-common.md"))
client = read(os.path.join(SPEC, "carrel-client.md"))
uilib  = read(os.path.join(SPEC, "typecase.md"))
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

# 3. Полнота библиотеки: каждый m-* из клиента объявлен в L-4
used = set(re.findall(r'`(tc-[a-z]+(?:--[a-z]+)?)`', client))
headings = re.sub(r'(?m)^(?!####).*$', '', uilib)      # только строки заголовков
declared = set(re.findall(r'`(tc-[a-z]+(?:--[a-z]+)?)`', headings))
declared |= set(re.findall(r'\|\s*`(tc-[a-z]+--[a-z]+)`\s*\|', uilib))  # таблица вариантов
missing_widgets = sorted(w for w in used if w not in declared)
check("3. виджеты из U-6 объявлены в L-4", missing_widgets)

# 3b. Карта кадров: каждый .m-* из кадров разобран в U-5.4
frame_cls = set()
for m in re.finditer(r'class="([^"]*)"', mock):
    for c in m.group(1).split():
        if re.match(r'^m-[a-z0-9]+(--[a-z0-9]+)?$', c):
            frame_cls.add(c)
unmapped = []
for c in sorted(frame_cls):
    if c in client:                      # назван в карте исключений
        continue
    if ("`tc-" + c[2:] + "`") in uilib:  # правило по умолчанию: m-x -> tc-x
        continue
    unmapped.append(c)
check("3b. классы кадров разобраны картой U-5.4", unmapped)

# 4. Независимость библиотеки: ни ссылок на другие файлы, ни доменных слов
cross = re.findall(r'\((?:carrel-(?:common|client|server|desktop))\.md\)', uilib)
domain = []
for w in ["DAV", "vCard", "iCal", "контакт", "календар", "заметк", "коллекци", "аккаунт", "Carrel"]:
    for m in re.finditer(w, uilib, re.I):
        line = uilib[:m.start()].count("\n") + 1
        if line > 90:  # §L-2.2 приводит их как отрицательные примеры
            domain.append("%s:%d" % (w, line))
check("4. typecase.md не ссылается на другие файлы", cross)
check("4b. typecase.md без доменных слов", sorted(set(domain)))

# 5. Браузерные файлы не знают о настольном приложении
# «оболочка» здесь значит каркас страницы, «настольная библиотека» — GUI-тулкит:
# ищутся только те слова, что могут значить одно.
deskwords = [r"carrel-desktop\.md", r"sidecar", r"Wails", r"webview",
             r"настольн\w*\s+приложени", r"настольного приложения"]
leak = []
for name, text in [("common", common), ("server", server), ("client", client), ("ui", uilib)]:
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
                   ("ui", uilib), ("desktop", desk)]:
    for w in [r"\[готово\]", r"\[не сделано\]", r"\[частично\]", r"CHANGELOG",
              r"internal/", r"roadmap", r"§2[0-9]"]:
        for m in re.finditer(w, text, re.I):
            line = text[:m.start()].count(chr(10)) + 1
            dirty.append("%s:%d: %s" % (name, line, text.split(chr(10))[line-1].strip()[:70]))
check("5b. чистый лист: без статусов и следов другого репозитория", sorted(set(dirty)))

# 6. Ссылки на разделы указывают на существующие
nums = {}
for pref, text in [("C", common), ("S", server), ("U", client), ("L", uilib), ("D", desk)]:
    nums[pref] = set(re.findall(r'^##\s+' + pref + r'-(\d+)\.', text, re.M))
badrefs = []
for pref, text in [("C", common), ("S", server), ("U", client), ("L", uilib), ("D", desk)]:
    for m in re.finditer(r'\b([CSULD])-(\d+)(?:\.\d+)?\b', text):
        p, n = m.group(1), m.group(2)
        if p in nums and n not in nums[p]:
            line = text[:m.start()].count("\n") + 1
            badrefs.append("%s → %s-%s (стр. %d)" % (pref, p, n, line))
check("6. ссылки на существующие разделы", sorted(set(badrefs)))

# 6b. Названное правило, на которое ссылаются из разных файлов, обязано где-то
# вводиться заголовком. Ловит правило, которое все цитируют и никто не сформулировал.
FILES = [("common", common), ("server", server), ("client", client),
         ("ui", uilib), ("desktop", desk)]
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
    "app-conflict": "/app/{kind}/{account}/{col}/{uid}/conflict",
}

def as_kind(x):
    """Кадр рода пишет свой маршрут, карта — шаблон: сводим к шаблону."""
    return re.sub(r'/app/(contacts|calendar|tasks|notes)/', '/app/{kind}/', x)

def canon_frame(route):
    """Кадр маршрута: тот, что рисует экран целиком, а не его кусок."""
    for m in re.finditer(r'<div class="frame"([^>]*)>', mock):
        attrs = m.group(1)
        if 'data-part=' in attrs:
            continue
        ds = re.search(r'data-screen="([^"]+)"', attrs)
        if ds and as_kind(route) in [as_kind(s) for s in ds.group(1).split()]:
            nxt = mock.find('<div class="frame"', m.end())
            return mock[m.end():nxt if nxt > 0 else len(mock)]
    return None

def frame_rail(frame):
    cols = []
    for m in re.finditer(r'<div class="m-src([^"]*)">(.*?)</div>', frame, re.S):
        lab = re.search(r'<span class="t">([^<]*)</span>', m.group(2))
        meta = re.search(r'<span class="m-meta">([^<]*)</span>', m.group(2))
        if lab and meta:
            cols.append((lab.group(1), meta.group(1),
                         "off" if "is-off" in m.group(1) else "on"))
    nav = []
    nm = re.search(r'<div class="m-nav">(.*?)</div>', frame, re.S)
    if nm:
        for a in re.finditer(r'<a[^>]*>(.*?)</a>', nm.group(1), re.S):
            num = re.search(r'<span class="m-num">([^<]*)</span>', a.group(1))
            lab = re.sub(r'<[^>]+>', '', re.sub(r'<span class="m-num">.*?</span>', '', a.group(1))).strip()
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
    if [c[0] for c in fc] != [c[0] for c in xc]:
        drift.append("%s: коллекции рейла: кадр %s, фикстура %s"
                     % (where, [c[0] for c in fc], [c[0] for c in xc]))
    elif state == "normal":
        if fc != xc:
            drift.append("%s: числа или флажки коллекций: кадр %s, фикстура %s" % (where, fc, xc))
        if fn != xn:
            drift.append("%s: навигация: кадр %s, фикстура %s" % (where, fn, xn))
check("8. фикстура рисуется в свой кадр", drift)

print()
print("ИТОГ: %d красных проверок" % len(fails))
sys.exit(1 if fails else 0)
