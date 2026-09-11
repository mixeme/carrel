"""Выпуск надписи Typecase: мастер и эталон.

    python build_typecase_logotype.py

Пишет два файла рядом с прочими эталонами:
    docs/visual/typecase-logotype.svg   — мастер
    docs/visual/typecase-logotype.html  — эталон построения

Числа в документе не набираются руками: и они, и картинки берутся из того же
замера, из которого собран мастер. Это лекарство от находки Л-1 в reviews.md,
где эталон Carrel разошёлся с собственным выпущенным путём.
"""

import json
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from fontTools.ttLib import TTFont
from fontTools.varLib import instancer
from fontTools.pens.boundsPen import BoundsPen

import typecase_logotype as T

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.normpath(os.path.join(HERE, ".."))

# Инстанс мастера зафиксирован эталоном. Переменная окружения нужна только
# сравнению внутри документа и не должна дотянуться до выпускаемого файла.
assert (T.OPSZ, T.WGHT) == (12.0, 400.0), (
    f"мастер собирается на opsz 12 / wght 400, а окружение задало "
    f"opsz {T.OPSZ:g} / wght {T.WGHT:g} — снимите TC_OPSZ и TC_WGHT"
)

M = T.metrics()

# --- сравнение оптических размеров ------------------------------------------


def probe(opsz):
    """Замер инстанса: то же построение, только другой оптический размер."""
    inst = instancer.instantiateVariableFont(
        TTFont(T.FONT), {"opsz": opsz, "wght": T.WGHT}, inplace=False,
        updateFontNames=False,
    )
    gs, cmap, hmtx = inst.getGlyphSet(), inst.getBestCmap(), inst["hmtx"]

    def bounds(ch):
        bp = BoundsPen(gs)
        gs[cmap[ord(ch)]].draw(bp)
        return bp.bounds

    cap_font = bounds("T")[3]
    scale = T.CAP / cap_font
    xh = bounds("e")[3]
    cuts = T.scan_width(cmap[ord("p")], xh / 2, gs=gs)
    stem = (cuts[1] - cuts[0]) * scale
    thin = T.ring_thin(cmap[ord("e")], gs=gs) * scale
    bearings = []
    for ch in T.WORD:
        g = cmap[ord(ch)]
        b = bounds(ch)
        bearings += [b[0] * scale, (hmtx[g][0] - b[2]) * scale]
    air = sum(bearings) / len(bearings)
    cells = [(bounds(ch)[2] - bounds(ch)[0]) * scale + 2 * air for ch in T.WORD]
    width = sum(cells) + len(cells) * stem + sum(cells) / len(cells)
    return dict(opsz=opsz, cap_font=cap_font, scale=scale, stem=stem, thin=thin,
                contrast=stem / thin, air=air,
                cell_h=T.CAP - bounds("p")[1] * scale + 2 * air,
                width=width, limit=width / thin, limit15=1.5 * width / thin)


OPSZ_ROWS = [probe(o) for o in (8, 12, 18, 24, 36, 72)]


def other_instance(opsz):
    """Надпись, построенная на другом инстансе, — отдельным процессом."""
    env = dict(os.environ, TC_OPSZ=str(opsz), PYTHONIOENCODING="utf-8")
    r = subprocess.run(
        [sys.executable, os.path.join(HERE, "typecase_logotype.py"), "--svg"],
        capture_output=True, text=True, encoding="utf-8", env=env, check=True,
    )
    return json.loads(r.stdout)


FAT = other_instance(72)
FAT_M = FAT["m"]

# --- фигуры ------------------------------------------------------------------

RULE, DIM = "#B04A3A", "#B04A3A"
PAPER, HAIR = "#FFFDF7", "#E2DACA"


def svg(vb, body, width=None, height=None):
    wh = (f' width="{width}"' if width else "") + (f' height="{height}"' if height else "")
    return f'<svg viewBox="{vb}"{wh}>{body}</svg>'


def wm(accent="empty", cells="ink", theme="light", empty_mode="mean", width=None):
    w, h, s = T.wordmark(accent=accent, cells=cells, theme=theme, empty_mode=empty_mode)
    if width:
        s = s.replace("<svg ", f'<svg width="{width:.0f}" height="{width*h/w:.0f}" ', 1)
    return w, h, s


def check_labels(items, room, size, advance=0.62):
    """Подписи должны влезать в поле и не наезжать друг на друга.

    Полукруглая моноширинного — 0.62 кегля; запас берётся с избытком, потому
    что смотреть на фигуру глазами при каждой пересборке никто не будет.
    """
    for y, text in items:
        need = len(text) * size * advance
        assert need <= room, f"подпись «{text}» шире поля: {need:.0f} против {room:.0f}"
    ys = sorted(y for y, _ in items)
    for a, b in zip(ys, ys[1:]):
        assert b - a >= size * 1.2, f"подписи на {a:.2f} и {b:.2f} наедут друг на друга"


def label(x, y, text, anchor="start", size=4.6, fill=DIM):
    return (f'<text x="{x:.2f}" y="{y:.2f}" text-anchor="{anchor}" '
            f'font-family="ui-monospace,monospace" font-size="{size}" '
            f'fill="{fill}">{text}</text>')


def fig_metrics():
    """Линии набора поперёк надписи.

    Линии стоят вплотную — верх гнезда в воздухе от прописных, низ в воздухе от
    выносной, — поэтому подписи разведены по разным сторонам: иначе на кегле,
    который читается, они наезжают друг на друга.
    """
    w, h, inner = T.wordmark_inner()
    left = [
        (0.0, "верх гнезда"),
        (M["baseline"] - M["xh"], f'очко строчных {n(M["xh"])}'),
        (M["cell_h"], f'низ гнезда {n(M["cell_h"])}'),
    ]
    right = [
        (M["air"], f'прописные {T.CAP:.0f}'),
        (M["baseline"], "базовая"),
        (M["baseline"] + M["desc"], f'выносная вниз {n(M["desc"])}'),
    ]
    pad_l, pad_r, size = 122.0, 122.0, 9.0
    check_labels(left, pad_l - 4, size)
    check_labels(right, pad_r - 4, size)
    body = [f'<rect x="0" y="0" width="{w:.2f}" height="{h:.2f}" fill="{PAPER}"/>', inner]
    for side, items in (("l", left), ("r", right)):
        for y, name in items:
            x1 = -pad_l + 4 if side == "l" else 0.0
            x2 = w if side == "l" else w + pad_r - 4
            body.append(f'<path d="M{x1:.2f} {y:.2f}H{x2:.2f}" stroke="{RULE}" stroke-width="0.5"/>')
            body.append(label(x1 if side == "l" else x2, y - 2.6, name,
                              "start" if side == "l" else "end", size))
    # 780 — предел ширины внутри .board: колонка 836 минус отбивка доски.
    return svg(f"{-pad_l:.2f} -12 {w + pad_l + pad_r:.2f} {h + 24:.2f}",
               "".join(body), width=780)


def fig_cell():
    """Одно гнездо крупно: воздух с четырёх сторон, перегородка, скругление."""
    i = 2                                     # «p» — выносная вниз наглядна
    x, cw, pen = T.CELLS[i]
    x2, cw2, pen2 = T.CELLS[i + 1]
    left, right = x - 6, x2 + cw2 + 6
    b = T.bounds(T.GLYPHS[i])
    ink_l, ink_r = pen + b[0] * T.SCALE, pen + b[2] * T.SCALE
    p = T.PAL["light"]
    body = [f'<rect x="{left:.2f}" y="-14" width="{right - left:.2f}" height="{M["cell_h"] + 28:.2f}" fill="{PAPER}"/>']
    for j in (i, i + 1):
        xx, ww, pp = T.CELLS[j]
        body.append(f'<rect x="{xx:.2f}" y="0" width="{ww:.2f}" height="{M["cell_h"]:.2f}" '
                    f'rx="{M["radius"]:.2f}" fill="{p["card"]}"/>')
        body.append(f'<path d="{T.glyph_path(T.GLYPHS[j], pp)}" fill="{p["ink"]}"/>')
    # воздух слева, справа, сверху, снизу
    for x1, x2_, y, text in (
        (x, ink_l, M["baseline"] - 12, f'{M["air"]:.2f}'),
        (ink_r, x + cw, M["baseline"] - 12, f'{M["air"]:.2f}'),
    ):
        body.append(f'<path d="M{x1:.2f} {y:.2f}H{x2_:.2f}" stroke="{DIM}" stroke-width="0.5"/>')
        body.append(label((x1 + x2_) / 2, y - 2, text, "middle", 4.4))
    for y1, y2, text in ((0, M["air"], f'{M["air"]:.2f}'),
                         (M["baseline"] + M["desc"], M["cell_h"], f'{M["air"]:.2f}')):
        body.append(f'<path d="M{x + cw / 2:.2f} {y1:.2f}V{y2:.2f}" stroke="{DIM}" stroke-width="0.5"/>')
        body.append(label(x + cw / 2 + 2, (y1 + y2) / 2 + 1.5, text, "start", 4.4))
    # перегородка
    body.append(f'<path d="M{x + cw:.2f} {M["cell_h"] + 7:.2f}H{x2:.2f}" stroke="{DIM}" stroke-width="0.5"/>')
    body.append(label((x + cw + x2) / 2, M["cell_h"] + 12.5, f'перегородка {M["gap"]:.2f}', "middle", 4.4))
    # скругление
    body.append(f'<path d="M{x2:.2f} {-4:.2f}h{M["radius"]:.2f}" stroke="{DIM}" stroke-width="0.5"/>')
    body.append(label(x2 + M["radius"] + 2, -2.5, f'радиус {M["radius"]:.2f}', "start", 4.4))
    return svg(f"{left:.2f} -14 {right - left:.2f} {M['cell_h'] + 28:.2f}", "".join(body), width=560)


def fig_sizes():
    """Лестница размеров вывода."""
    w, h, _ = wm()
    px = [round(w), 272, LIMIT15, LIMIT1]
    cells = []
    for p in px:
        _, _, s = wm(width=p)
        cells.append(
            f'<div class="cell"><div class="plate" style="padding:10px 12px;">{s}</div>'
            f'<div class="cap">{p:.0f} px</div></div>'
        )
    return '<div class="row">' + "".join(cells) + "</div>"


def fig_clear():
    """Охранное поле — рост строчных со всех сторон."""
    w, h, inner = T.wordmark_inner()
    q = M["xh_os2"]
    body = [f'<rect x="{-q:.2f}" y="{-q:.2f}" width="{w + 2 * q:.2f}" height="{h + 2 * q:.2f}" fill="{PAPER}"/>',
            f'<rect x="{-q:.2f}" y="{-q:.2f}" width="{w + 2 * q:.2f}" height="{h + 2 * q:.2f}" '
            f'fill="none" stroke="{RULE}" stroke-width="0.5" stroke-dasharray="3 3"/>',
            inner,
            f'<path d="M{w / 2:.2f} {-q:.2f}V0" stroke="{DIM}" stroke-width="0.5"/>',
            label(w / 2 + 4, -q / 2 + 3.2, f'{n(q)}', "start", 9.0)]
    return svg(f"{-q - 4:.2f} {-q - 4:.2f} {w + 2 * q + 8:.2f} {h + 2 * q + 8:.2f}",
               "".join(body), width=780)


LIMIT1 = M["width"] / M["thin"]              # тонкий штрих держит 1 px
LIMIT15 = 1.5 * M["width"] / M["thin"]       # тонкий штрих держит 1.5 px
CAP_AT_LIMIT = T.CAP * LIMIT15 / M["width"]
RADIUS_AT_LIMIT = M["radius"] * LIMIT15 / M["width"]

# --- мастер ------------------------------------------------------------------

w_master, h_master, master = wm()
with open(os.path.join(OUT, "typecase-logotype.svg"), "w", encoding="utf-8") as f:
    f.write(master.replace("><", ">\n  <").replace("\n  </svg>", "\n</svg>") + "\n")

# Проверки читают выпущенный файл, поэтому считаются после него.
import checks  # noqa: E402

V = checks.run()

# --- документ ----------------------------------------------------------------

CSS = """
  :root{
    --ink:#17211C;--card:#E8DFCB;--card-border:#C9BFA6;--accent:#4A6B52;
    --accent-dark:#7FA085;--bg:#F5F1E8;--paper:#FFFDF7;--rule:#D8CFBB;
    --hair:#E2DACA;--dusty:#9A9280;--muted:#6B6355;--alert:#B04A3A;
  }
  *{box-sizing:border-box;}
  body{margin:0;padding:0;background:var(--bg);color:var(--ink);
    font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
    font-size:15px;line-height:1.6;}
  .wrap{max-width:900px;margin:0 auto;padding:56px 32px 96px;}
  header{border-bottom:0.5px solid var(--rule);padding-bottom:32px;margin-bottom:44px;}
  h1{font-family:Georgia,"Source Serif 4",serif;font-size:38px;font-weight:400;margin:0 0 8px;}
  .sub{font-size:11px;letter-spacing:0.18em;color:var(--dusty);text-transform:uppercase;}
  h2{font-size:13px;letter-spacing:0.14em;text-transform:uppercase;color:var(--dusty);
     font-weight:600;margin:64px 0 20px;padding-bottom:8px;border-bottom:0.5px solid var(--rule);}
  h3{font-size:16px;font-weight:600;margin:32px 0 10px;}
  p{margin:0 0 14px;max-width:68ch;}
  .lede{font-size:17px;color:var(--muted);max-width:64ch;}
  .note{font-size:13px;color:var(--muted);padding-left:14px;
        border-left:2px solid var(--card-border);margin:18px 0;max-width:64ch;}
  table{border-collapse:collapse;width:100%;font-size:13.5px;margin:18px 0;}
  th{text-align:left;font-size:10.5px;letter-spacing:0.1em;text-transform:uppercase;
     color:var(--dusty);font-weight:600;padding:0 12px 8px 0;border-bottom:0.5px solid var(--rule);}
  td{padding:9px 12px 9px 0;border-bottom:0.5px solid var(--hair);vertical-align:top;}
  td.num,th.num{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12.5px;}
  code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12.5px;
       background:var(--paper);border:0.5px solid var(--hair);border-radius:3px;padding:1px 5px;}
  pre{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:11.5px;line-height:1.7;
      background:var(--paper);border:0.5px solid var(--hair);border-radius:5px;
      padding:16px 18px;overflow-x:auto;margin:14px 0 22px;}
  pre code{background:none;border:0;padding:0;}
  figure{margin:26px 0;}
  figcaption{font-size:12.5px;color:var(--muted);margin-top:12px;max-width:66ch;}
  .board{background:var(--paper);border:0.5px solid var(--hair);border-radius:5px;
         padding:22px;display:inline-block;max-width:100%;overflow-x:auto;}
  .board svg{display:block;}
  .row{display:flex;align-items:flex-start;gap:24px;flex-wrap:wrap;margin:20px 0;}
  .cell{display:flex;flex-direction:column;gap:9px;}
  .cap{font-size:11px;color:var(--dusty);}
  .plate{background:var(--card);border:0.5px solid var(--card-border);
         display:flex;align-items:center;justify-content:center;border-radius:4px;}
  .plate.dark{background:var(--ink);border-color:var(--ink);}
  .plate.paper{background:var(--paper);border-color:var(--hair);}
  .plate.bg{background:var(--bg);border-color:var(--card-border);}
  .plate svg{display:block;}
  .swatches{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:16px;margin:20px 0;}
  .sw{display:flex;flex-direction:column;gap:7px;}
  .chip{height:44px;border-radius:4px;border:0.5px solid var(--card-border);}
  .sw .n{font-size:13px;font-weight:500;}
  .sw .h{font-family:ui-monospace,monospace;font-size:11.5px;color:var(--dusty);}
  ul{padding-left:18px;margin:0 0 14px;}
  li{margin-bottom:7px;max-width:64ch;}
  .bad{color:var(--alert);}
  .ok{color:var(--accent);}
  .file{font-size:11px;letter-spacing:0.1em;text-transform:uppercase;color:var(--dusty);
        font-weight:600;margin:26px 0 0;}
"""


def n(v, d=2):
    return f"{v:.{d}f}"


def plate(svg_markup, cls="plate", pad="14px 18px", cap=None):
    c = f'<div class="cap">{cap}</div>' if cap else ""
    return (f'<div class="cell"><div class="{cls}" style="padding:{pad};">{svg_markup}</div>{c}</div>')


CH = list(T.WORD)

# гнёзда: таблица
cells_rows = "".join(
    f"<tr><td>{CH[i]}</td><td class=\"num\">{n(M['ink_widths'][i])}</td>"
    f"<td class=\"num\">{n(M['cells'][i][1])}</td>"
    f"<td class=\"num\">{n(M['advances'][i])}</td>"
    f"<td class=\"num\">{n(M['cells'][i][0])}</td></tr>"
    for i in range(len(CH))
)

opsz_rows = "".join(
    f"<tr><td class=\"num\">{r['opsz']:.0f}</td><td class=\"num\">{r['cap_font']:.0f}</td>"
    f"<td class=\"num\">{n(r['stem'])}</td><td class=\"num\">{n(r['thin'])}</td>"
    f"<td class=\"num\">{n(r['contrast'])} : 1</td><td class=\"num\">{n(r['width'], 1)}</td>"
    f"<td class=\"num\">{r['limit']:.0f}</td><td class=\"num\">{r['limit15']:.0f}</td></tr>"
    for r in OPSZ_ROWS
)

EMPTY_NAMES = {
    "thin": "тонкая шпация",
    "space": "пробел шрифта",
    "mean": "средняя ширина гнёзд — выбрано",
    "en": "полукруглая, ½ кегельной",
    "widest": "как самое широкое гнездо, «T»",
    "em": "круглая, она же кегельная",
}
empty_rows = "".join(
    f"<tr><td>{EMPTY_NAMES[k]}</td><td class=\"num\">{n(v)}</td>"
    f"<td class=\"num\">{v / M['cell_h'] * 100:.0f} %</td></tr>"
    for k, v in M["empty_widths"].items()
)
empty_figs = "".join(
    plate(wm(empty_mode=k, width=330)[2], cap=f"{EMPTY_NAMES[k].split(' — ')[0]} · {n(v)}")
    for k, v in M["empty_widths"].items()
)

accent_figs = "".join([
    plate(wm(accent="none", width=300)[2], cap="без акцента"),
    plate(wm(accent="cell", width=300)[2], cap="первое гнездо залито"),
    plate(wm(accent="letter", width=300)[2], cap="акцентная литера"),
    plate(wm(accent="split", width=300)[2], cap="Type | case"),
    plate(wm(accent="last", width=300)[2], cap="последнее гнездо"),
    plate(wm(accent="empty", width=300)[2], cap="свободное гнездо — выбрано"),
])

geom_figs = "".join([
    plate(wm(cells="ink", width=330)[2], cap=f"по литере — выбрано · {n(M['width'], 1)}"),
    plate(wm(cells="advance", width=330)[2], cap=f"по кегельной площадке · {n(M['width_advance'], 1)}"),
    plate(wm(cells="square", width=330)[2], cap=f"квадраты · {n(M['width_square'], 1)}"),
])

# черновик: тот же файл без метаданных C2PA
draft = open(os.path.join(OUT, "typecase-wordmark-tiles.svg"), encoding="utf-8").read()
draft = draft[:draft.index("<metadata>")] + draft[draft.index("</metadata>") + 11:]
draft = draft.replace('width="800" height="120"', 'width="400" height="60"')

fat_small = FAT["svg"].replace("<svg ", f'<svg width="{LIMIT15:.0f}" ', 1)
thin_small = wm(width=LIMIT15)[2]

HTML = f"""<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Typecase — надпись, эталон построения</title>
<style>{CSS}</style>
</head>
<body>
<div class="wrap">

<header>
  <div class="row" style="margin-bottom:24px;gap:14px;">
    {plate(wm(width=380)[2], pad="18px 22px")}
    {plate(wm(theme="dark", width=380)[2], cls="plate dark", pad="18px 22px")}
  </div>
  <h1>Надпись Typecase</h1>
  <div class="sub">Эталон построения · наборная касса · Literata opsz {M['opsz']:.0f}</div>
</header>

<p class="lede">Единственный источник правды по надписи Typecase. Литеры — настоящая
Literata, переведённая в кривые; гнёзда выведены из метрик того же инстанса.
Typecase — библиотека виджетов, отдельный модуль со своей версией, и знак у неё
свой: надпись Carrel сюда не ставится и наоборот.</p>

<h2>1. Что выбрано и почему</h2>

<p>Имя объясняет форму. <em>Typecase</em> — наборная касса: плоский ящик с
ячейками, где у каждой литеры своё гнездо, и набор собирается из готовых
элементов, а не вырезается каждый раз заново. Так это записано в
<code>docs/spec/typecase.md</code>, и знак не придумывает метафору заново, а
показывает её буквально: восемь литер слова стоят каждая в своём гнезде.</p>

<p>Отсюда всё остальное. Гнездо не декоративная плашка, а <strong>литерное
тело</strong>: его ширина равна ширине литеры плюс воздух, и сумма гнёзд
сходится с суммой кегельных площадок шрифта — {n(M['cells_sum'])} против
{n(M['advances_sum'])}. Ящик заполнен ровно тем, что в нём и должно быть.</p>

<div class="note">Знак Carrel устроен обратным образом: там лампа встроена в
литеру «l», то есть предмет входит в слово. Здесь предмет — сама касса, и слово
входит в него. Это разные приёмы, и рядом их ставить незачем: у Typecase свой
модуль, своя версия и свой каталог.</div>

<h3>Свободное гнездо</h3>

<p>Акцент — не литера и не гнездо со литерой, а <strong>пустое гнездо после
слова</strong>. В кассе такое гнездо означает ровно одно: литера оттуда взята и
уже стоит в наборе, а место под неё осталось. Для библиотеки виджетов это и есть
её обещание — набор готовых элементов, в который добавится следующий.</p>

<p>Проверялись ещё пять размещений акцента, и все отпали.</p>

<figure>
<div class="row">{accent_figs}</div>
<figcaption>Залитое гнездо в начале и акцентная литера ставят выделение внутрь
слова: одна литера начинает читаться отдельно от прочих, а касса перестаёт быть
однородной. Раскол «Type&nbsp;|&nbsp;case» разрывает слово на два, которых в имени
нет. Последнее гнездо выделяет литеру «e» без повода. Пустое гнездо ничего не
делит и добавляет смысл, которого в слове ещё не было.</figcaption>
</figure>

<h2>2. Что чинится против черновика</h2>

<figure>
<div class="row">
  {plate(draft, cls="plate paper", pad="12px 14px", cap="черновик")}
  {plate(wm(width=400)[2], pad="12px 14px", cap="эталон")}
</div>
<figcaption>Идея черновика — литеры в гнёздах — принята целиком. Не принято
ничего из его чисел.</figcaption>
</figure>

<table>
  <tr><th style="width:32%">В черновике</th><th>Что не так и что стало</th></tr>
  <tr><td>Georgia как временная подстановка</td><td>Гарнитура проекта — Literata, она же вендорится в <code>typecase/fonts</code>. Georgia меняет пропорции и рисунок засечек, то есть знак был бы построен не на том шрифте, которым набран интерфейс</td></tr>
  <tr><td>Цвета <code>#F5EFDD</code>, <code>#3B6D11</code>, <code>#2C2C2A</code>, <code>#EDE4CB</code></td><td>Ни одного из них нет в палитре. Взяты токены: <code>--tc-card</code>, <code>--tc-accent</code>, <code>--tc-ink</code></td></tr>
  <tr><td>Обводка <code>stroke-width 1.4</code> у каждого гнезда</td><td>Обводок нет: бренд-гайд разрешает заливки. Гнездо видно заливкой, а не рамкой</td></tr>
  <tr><td>Квадраты 34×34 с шагом 38</td><td>Числа взяты на глаз и одинаковы для «T» и «s». Гнездо теперь равно литере плюс воздух, шаг — перегородка в модуль</td></tr>
  <tr><td>Акцентом залита половина слова: «case» в четырёх гнёздах</td><td>Правило одного акцента. Акцент — одно гнездо, и то пустое</td></tr>
  <tr><td>Текст остался текстом (<code>&lt;text&gt;</code>)</td><td>Литеры переведены в кривые: файл не зависит от того, установлен ли шрифт</td></tr>
  <tr><td>В файле 5 КБ метаданных C2PA</td><td>Мастер — только контуры и прямоугольники</td></tr>
</table>

<h2>3. Шрифт</h2>

<table>
  <tr><th style="width:34%">Что</th><th>Значение</th></tr>
  <tr><td>Гарнитура</td><td>Literata, {M['version']}</td></tr>
  <tr><td>Лицензия</td><td>SIL Open Font License 1.1</td></tr>
  <tr><td>Файл</td><td><code>typecase/fonts/Literata.ttf</code> — тот же, что раздаёт библиотека</td></tr>
  <tr><td>Инстанс переменного шрифта</td><td class="num">opsz {M['opsz']:.0f}, wght {M['wght']:.0f}</td></tr>
  <tr><td>Единиц на кегельную</td><td class="num">{M['upm']}</td></tr>
  <tr><td>Прописные в шрифте</td><td class="num">{M['cap_font']:.0f} — верх «T»</td></tr>
  <tr><td>Масштаб в систему логотипа</td><td class="num">{M['scale']:.6f} — прописные приведены к {T.CAP:.0f}</td></tr>
</table>

<h3>Почему opsz {M['opsz']:.0f}, а не 72</h3>

<p>Literata — переменный шрифт с осью оптического размера. Крупные значения оси
рисуют шрифт для больших кеглей: контраст растёт, тонкие штрихи истончаются.
Логотип Carrel взял предельный <code>opsz 72</code> с доводом «логотип живёт
крупно». Для надписи в гнёздах этот довод не работает, и вот замер: в одной и
той же системе, при росте прописных {T.CAP:.0f}, тонкий штрих кольца «e»
меняется в {M['thin'] / OPSZ_ROWS[-1]['thin']:.1f} раза.</p>

<table>
  <tr><th class="num">opsz</th><th class="num">прописные в шрифте</th><th class="num">штамб</th>
      <th class="num">тонкий штрих</th><th class="num">контраст</th><th class="num">габарит</th>
      <th class="num">предел 1 px</th><th class="num">предел 1.5 px</th></tr>
  {opsz_rows}
</table>

<p>Последние две колонки и есть ответ. «Предел» — ширина вывода, при которой
тонкий штрих ещё держит один или полтора пикселя: ниже него штрих выцветает в
серое, и кольцо «e» разрывается. На <code>opsz 72</code> надписи потребовалось бы
{OPSZ_ROWS[-1]['limit15']:.0f}&nbsp;px, чтобы штрих держал 1.5&nbsp;px — это втрое
крупнее, чем нужно шапке. На выбранном инстансе хватает
{LIMIT15:.0f}&nbsp;px.</p>

<p>В другую сторону ось тоже кончается не сразу, и там взят не край. На
<code>opsz {OPSZ_ROWS[0]['opsz']:.0f}</code> штрих ещё толще
({n(OPSZ_ROWS[0]['thin'])} против {n(M['thin'])}), но то же слово выходит шире на
{OPSZ_ROWS[0]['width'] - M['width']:.0f} единиц: литеры раздвинуты и раскрыты для
печати мельче любого применения знака. <code>opsz {M['opsz']:.0f}</code> —
текстовый конец оси, тот инстанс, которым Literata набирает основной текст, а
библиотека — свой интерфейс на <code>--tc-font-size: 13.5px</code>. Надпись
нарисована тем же шрифтом, что строка рядом с ней.</p>

<figure>
<div class="row">
  {plate(fat_small, cap=f"opsz 72 · тонкий штрих {FAT_M['thin'] * LIMIT15 / FAT['w']:.2f} px")}
  {plate(thin_small, cap=f"opsz {M['opsz']:.0f} · тонкий штрих {M['thin'] * LIMIT15 / M['width']:.2f} px")}
</div>
<figcaption>Обе надписи выведены шириной {LIMIT15:.0f}&nbsp;px — это предел,
назначенный в разделе&nbsp;10. Верх кольца «e» и перекладина «e» слева выцветают:
браузер рисует штрих тоньше пикселя полутоном. Справа тот же штрих чёрный.
Разница только в оптическом размере инстанса.</figcaption>
</figure>

<div class="note">Заодно это причина, по которой числа раздела&nbsp;4 замеряются
по выпущенному контуру, а не переписываются из спецификации шрифта. Эталон Carrel
объявил <code>opsz 72</code> и тонкий штрих 5.98 одновременно, хотя на этом
инстансе штрих равен 1.34: два инстанса смешались в одной таблице. Находка
записана как Л-1 в <code>docs/plan/reviews.md</code>; здесь она исключена
построением — документ собирается тем же скриптом, что и мастер.</div>

<p>Начертание {M['wght']:.0f} — то же, каким набран интерфейс. Литеры переведены в
кривые: шрифт нужен, чтобы файл пересобрать, и не нужен, чтобы знак показать.</p>

<h2>4. Метрики</h2>

<p>Назначено одно число — рост прописных {T.CAP:.0f}, ради общей системы с
эталонами Carrel. Всё остальное — шрифт, умноженный на масштаб.</p>

<figure>
<div class="board">{fig_metrics()}</div>
<figcaption>Красным — линии набора. Верх и низ гнезда отстоят от прописных и от
выносной на один воздух: величина из раздела&nbsp;5.</figcaption>
</figure>

<table>
  <tr><th style="width:38%">Величина</th><th class="num" style="width:18%">Значение</th><th>Откуда</th></tr>
  <tr><td>Рост прописных</td><td class="num">{T.CAP:.0f}</td><td>назначен</td></tr>
  <tr><td>Очко строчных</td><td class="num">{n(M['xh'])}</td><td>шрифт: верх «e», с наплывом</td></tr>
  <tr><td>Рост строчных по OS/2</td><td class="num">{n(M['xh_os2'])}</td><td>шрифт: <code>sxHeight</code></td></tr>
  <tr><td>Выносная вниз</td><td class="num">{n(M['desc'])}</td><td>шрифт: низ «p» и «y»</td></tr>
  <tr><td>Кегельная площадка</td><td class="num">{n(M['em'])}</td><td>шрифт: {M['upm']} единиц</td></tr>
  <tr><td>Штамб «p» — модуль</td><td class="num">{n(M['stem'])}</td><td>шрифт: замер на середине очка, {M['stem_font']:.0f} единиц</td></tr>
  <tr><td>Тонкий штрих</td><td class="num">{n(M['thin'])}</td><td>шрифт: верх кольца «e». Контраст {n(M['contrast'])} : 1</td></tr>
  <tr><td>Базовая линия от верха гнезда</td><td class="num">{n(M['baseline'])}</td><td>воздух плюс прописные</td></tr>
</table>

<h2>5. Сетка гнёзд</h2>

<p>Гнездо строится от литеры, а не наоборот. Ширина — очко литеры плюс воздух
слева и справа; высота — от прописных до выносной вниз плюс тот же воздух сверху
и снизу. Одинаковых гнёзд в кассе нет, как нет одинаковых литер.</p>

<p><strong>Воздух — средний полуапрош инстанса, {n(M['air'])}.</strong> Это не
подобранное число: у каждой литеры шрифт уже назначил левый и правый полуапрош,
и здесь взято их среднее по восьми литерам слова. Отсюда свойство, которым сетка
проверяется: сумма гнёзд равна сумме кегельных площадок шрифта, обе
{n(M['cells_sum'])}. Касса шире набора ровно на перегородки.</p>

<figure>
<div class="board">{fig_cell()}</div>
<figcaption>Гнёзда «p» и «e». Воздух одинаков с четырёх сторон; перегородка и
радиус скругления равны модулю — штамбу «p».</figcaption>
</figure>

<table>
  <tr><th style="width:12%">Литера</th><th class="num">очко</th><th class="num">гнездо</th>
      <th class="num">кегельная площадка</th><th class="num">x гнезда</th></tr>
  {cells_rows}
  <tr><td><strong>сумма</strong></td><td class="num"></td>
      <td class="num"><strong>{n(M['cells_sum'])}</strong></td>
      <td class="num"><strong>{n(M['advances_sum'])}</strong></td><td class="num"></td></tr>
</table>

<h3>Перегородка и радиус</h3>

<p>Между гнёздами — перегородка в модуль, {n(M['gap'])}, и она же задаёт радиус
скругления. Модуль здесь — штамб «p»: самая толстая вертикаль строчной литеры.
Больше в файле назначать нечего, и это единственный способ не назначать.</p>

<div class="note">Радиус проверяется на выводе: на пределе размера
{LIMIT15:.0f}&nbsp;px он даёт {RADIUS_AT_LIMIT:.2f}&nbsp;px — тот же скруглённый
угол, что <code>--tc-radius: 3px</code> у карточек библиотеки. Гнездо и карточка
скруглены одинаково не потому, что так задумано, а потому что обе меры выведены
из одного кегля.</div>

<h3>Почему гнёзда разной ширины</h3>

<figure>
<div class="row">{geom_figs}</div>
<figcaption>Квадратные гнёзда растягивают слово почти вдвое и врут о литерах:
«s» получает столько же места, сколько «T». Гнёзда по кегельной площадке
формально честны, но воздух в них лежит несимметрично — литера прижата к одному
краю там, где у шрифта разные полуапроши.</figcaption>
</figure>

<h2>6. Свободное гнездо</h2>

<p>Ширину пустого гнезда назначать нечем: литеры в нём нет. Поэтому взята
<strong>средняя ширина гнёзд слова, {n(M['empty_w'])}</strong> — то самое место,
которое занимает литера этой кассы «в среднем». Гнездо получается заметным, но не
спорит с «T» за первенство.</p>

<figure>
<div class="row">{empty_figs}</div>
<figcaption>Пробельный материал наборной кассы, от тонкой шпации до круглой.
Шпация и пробел слишком узки: акцент читается как опечатка или как курсор. Круглая
и «как самое широкое гнездо» перевешивают слово. Полукруглая близка к выбранной,
но она пришла бы из кегля, а не из этой кассы.</figcaption>
</figure>

<table>
  <tr><th style="width:44%">Чем задавали</th><th class="num">Ширина</th><th class="num">Доля высоты гнезда</th></tr>
  {empty_rows}
</table>

<h2>7. Раскладка</h2>

<p>Разрядка не назначается и не шейпится: гнёзда стоят подряд через перегородку,
и позиция литеры — это позиция её гнезда. Кернинг при этом <strong>отброшен
осознанно</strong>: harfbuzz поджал бы «Ty» и «as» на {n(-M['kerns'][0])} каждую,
всего {n(-sum(M['kerns']))}, но придвигать литеру к соседке в кассе некуда — у
каждой своё гнездо. Это единственное место, где надпись расходится с набором тем
же шрифтом, и расхождение названо здесь.</p>

<p>Следствие: зазор между очками соседних литер одинаков по всему слову и равен
воздух + перегородка + воздух = {n(2 * M['air'] + M['gap'])}. Проверять
пары по отдельности незачем — их нечем различить.</p>

<h2>8. Проверки</h2>

<p>Считает <code>docs/visual/tools/checks.py</code>. Каждая проверка смотрит на
то, что названа проверять.</p>

<table>
  <tr><th style="width:30%">Что проверялось</th><th>Результат</th></tr>
  <tr><td>Контуры против шрифта</td><td class="ok">{V['contours']}</td></tr>
  <tr><td>Литера внутри гнезда</td><td class="ok">{V['inside']}</td></tr>
  <tr><td>Зазор между литерами</td><td class="ok">{V['gaps']}</td></tr>
  <tr><td>Сумма гнёзд против кегельных площадок</td><td class="ok">{V['sum']}</td></tr>
  <tr><td>Тонкий штрих и предел размера</td><td class="ok">{V['thin']}</td></tr>
  <tr><td>Что лежит в мастере</td><td class="ok">{V['master']}</td></tr>
  <tr><td>Порядок фигур</td><td class="ok">Обязателен: сперва все гнёзда, потом все литеры. Литера лежит на своём гнезде и не может уйти под соседнее</td></tr>
</table>

<h2>9. Цвет</h2>

<p>Заливок три, и все три — токены библиотеки. Акцент один: свободное гнездо.</p>

<div class="swatches">
  <div class="sw"><div class="chip" style="background:#E8DFCB"></div><div class="n">Гнёзда, светлая</div><div class="h">--tc-card · #E8DFCB</div></div>
  <div class="sw"><div class="chip" style="background:#17211C"></div><div class="n">Литеры, светлая</div><div class="h">--tc-ink · #17211C</div></div>
  <div class="sw"><div class="chip" style="background:#4A6B52"></div><div class="n">Свободное гнездо</div><div class="h">--tc-accent · #4A6B52</div></div>
  <div class="sw"><div class="chip" style="background:#242E27"></div><div class="n">Гнёзда, тёмная</div><div class="h">--tc-card · #242E27</div></div>
  <div class="sw"><div class="chip" style="background:#E8DFCB"></div><div class="n">Литеры, тёмная</div><div class="h">--tc-ink · #E8DFCB</div></div>
  <div class="sw"><div class="chip" style="background:#7FA085"></div><div class="n">Свободное гнездо, тёмная</div><div class="h">--tc-accent · #7FA085</div></div>
</div>

<figure>
<div class="row">
  {plate(wm(width=300)[2], cls="plate bg", cap="светлая, на --tc-bg")}
  {plate(wm(theme="dark", width=300)[2], cls="plate dark", cap="тёмная")}
  {plate(wm(theme="mono", width=300)[2], cls="plate paper", cap="монохром")}
</div>
<figcaption>В монохроме гнёзда не рисуются вовсе: одной заливкой их не отличить
от литер. Остаётся слово и одно закрашенное гнездо — акцент выживает силуэтом,
касса исчезает. Это цена одноцветной печати, и подменять её обводкой нельзя.</figcaption>
</figure>

<h2>10. Размеры и охранное поле</h2>

<p>Габарит — {n(M['width'])} × {n(M['cell_h'])}, отношение
{M['width'] / M['cell_h']:.2f} : 1. Предел задаёт тонкий штрих, а не литера:
при {LIMIT15:.0f}&nbsp;px по ширине он держит 1.5&nbsp;px, при
{LIMIT1:.0f}&nbsp;px — ровно пиксель. Ниже кольцо «e» разрывается, и слово
перестаёт быть словом раньше, чем это заметно на превью.</p>

<figure>
{fig_sizes()}
<figcaption>Хинтованных версий нет и не будет: надпись не перерисовывается под
размер, вместо этого назначен предел.</figcaption>
</figure>

<table>
  <tr><th style="width:30%">Ширина вывода</th><th>Что ставить</th></tr>
  <tr><td class="num">от {LIMIT15:.0f} px</td><td>надпись целиком</td></tr>
  <tr><td class="num">{LIMIT1:.0f}–{LIMIT15:.0f} px</td><td>допустимо там, где знак не несёт нагрузки: подвал каталога, служебная страница</td></tr>
  <tr><td class="num">меньше {LIMIT1:.0f} px</td><td>ничего. Иконки у Typecase пока нет — она делается отдельно</td></tr>
</table>

<div class="note">Предел выглядит крупным против 88&nbsp;px у надписи Carrel, но
литера на нём того же размера: прописные выходят {CAP_AT_LIMIT:.1f}&nbsp;px, у
Carrel на его пределе — 19.9&nbsp;px. Разница только в габарите: здесь в него
входят гнёзда, воздух и перегородки, а не одни чернила.</div>

<figure>
<div class="board">{fig_clear()}</div>
<figcaption>Охранное поле — {n(M['xh_os2'])} со всех сторон, рост строчных по
<code>sxHeight</code>. Половина высоты гнезда: касса не притирается к краю
панели и не читается как ещё один элемент управления.</figcaption>
</figure>

<h2>11. Исходники</h2>

<p><code>viewBox</code> — габарит гнёзд: <code>0 0 {n(w_master)} {n(h_master)}</code>.
Базовая линия лежит на <code>y = {n(M['baseline'])}</code>, начало координат — верх
первого гнезда.</p>

<div class="file">typecase-logotype.svg — мастер</div>
<pre><code>&lt;svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {n(w_master)} {n(h_master)}" role="img" aria-label="Typecase"&gt;
  &lt;rect x="0" y="0" width="{n(T.CELLS[0][1])}" height="{n(h_master)}" rx="{n(M['radius'])}" fill="#E8DFCB"/&gt;
  … ещё семь гнёзд …
  &lt;rect x="{n(M['empty_x'])}" y="0" width="{n(M['empty_w'])}" height="{n(h_master)}" rx="{n(M['radius'])}" fill="#4A6B52"/&gt;
  &lt;path d="…«T», контуры из шрифта…" fill="#17211C"/&gt;
  … ещё семь литер …
&lt;/svg&gt;</code></pre>

<div class="file">пересборка</div>
<pre><code>python docs/visual/tools/build_typecase_logotype.py</code></pre>

<p>Скрипт выпускает мастер и этот документ вместе. Числа в тексте подставлены из
того же замера, из которого построены контуры, — набранных руками величин здесь
нет. Править мастер вручную не нужно и не следует: он пересобирается из шрифта.</p>

<div class="file">тёмная тема и монохром</div>
<pre><code>тёмная:   гнёзда #242E27, литеры #E8DFCB, свободное гнездо #7FA085
монохром: гнёзда не рисуются, литеры и свободное гнездо — currentColor</code></pre>

<h2>12. Что эталон фиксирует</h2>
<ul>
  <li>Гарнитура, версия и инстанс из раздела&nbsp;3. Другой <code>opsz</code> — другой тонкий штрих, а значит другой предел размера: пересобирать надо весь файл.</li>
  <li>Литеры — глифы шрифта в кривых. Ни одна не рисуется и не правится.</li>
  <li>Гнездо равно литере плюс воздух в средний полуапрош. Сумма гнёзд сходится с суммой кегельных площадок — это проверка, а не совпадение.</li>
  <li>Перегородка и радиус равны модулю, модуль — штамб «p».</li>
  <li>Акцент — одно пустое гнездо после слова, шириной в среднее гнездо. Литеры и занятые гнёзда акцентом не красятся.</li>
  <li>Три заливки, ноль обводок, порядок «гнёзда, затем литеры».</li>
  <li>Кернинг отброшен. Зазор между литерами одинаков по всему слову.</li>
  <li>Предел {LIMIT15:.0f}&nbsp;px и охранное поле {n(M['xh_os2'])}.</li>
</ul>

<h2>13. Чего эталон не закрывает</h2>
<ul>
  <li><strong>Иконки Typecase здесь нет.</strong> Черновики <code>typecase-logo-v2.svg</code> и <code>typecase-logo-bg.svg</code> — одна и та же касса 3×3 с одним занятым гнездом, во втором добавлена подложка. Это отдельная задача и отдельный эталон; до него в квадратных форматах Typecase не появляется вовсе.</li>
  <li><strong>Только слово «Typecase» на латинице.</strong> Кириллического начертания нет, и набор Literata рядом остаётся набором.</li>
  <li><strong>Файл привязан к версии шрифта.</strong> {M['version']}. Обновление гарнитуры сдвинет полуапроши и тонкий штрих, то есть и сетку, и предел; пересборка обязательна.</li>
  <li><strong>Применение внутри Carrel не описано.</strong> Typecase — зависимость, а не часть продукта; где именно её знак уместен в интерфейсе Carrel, решает бренд-гайд, а не этот файл.</li>
</ul>

</div>
</body>
</html>
"""

with open(os.path.join(OUT, "typecase-logotype.html"), "w", encoding="utf-8") as f:
    f.write(HTML)

print("мастер :", os.path.join(OUT, "typecase-logotype.svg"), f"{w_master:.2f} x {h_master:.2f}")
print("эталон :", os.path.join(OUT, "typecase-logotype.html"), f"{len(HTML)} символов")
print("предел :", f"1 px — {LIMIT1:.0f} px, 1.5 px — {LIMIT15:.0f} px")
