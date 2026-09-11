"""Построение надписи Typecase из Literata.

Литеры берутся у шрифта и переводятся в кривые; сетка гнёзд выводится из
метрик того же инстанса. Единственное назначенное число — рост прописных.

Мастер пересобирается так:
    python build_typecase_logotype.py

Переменные `TC_OPSZ` и `TC_WGHT` меняют инстанс шрифта. Эталон зафиксировал
opsz 12 и wght 400; переопределение нужно только затем, чтобы построить
сравнение оптических размеров для документа.
"""

import io
import json
import os

from fontTools.ttLib import TTFont
from fontTools.varLib import instancer
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.recordingPen import RecordingPen
from fontTools.misc.transform import Transform
from fontTools.svgLib.path import parse_path
import uharfbuzz as hb

HERE = os.path.dirname(os.path.abspath(__file__))
FONT = os.path.normpath(os.path.join(HERE, "..", "..", "..", "typecase", "fonts", "Literata.ttf"))
WORD = "Typecase"
CAP = 66.0            # рост прописных в системе логотипа — единственное назначенное число
OPSZ = float(os.environ.get("TC_OPSZ", 12.0))
WGHT = float(os.environ.get("TC_WGHT", 400.0))

# --- инстанс -----------------------------------------------------------------

base = TTFont(FONT)
inst = instancer.instantiateVariableFont(
    base, {"opsz": OPSZ, "wght": WGHT}, inplace=False, updateFontNames=False
)
buf = io.BytesIO()
inst.save(buf)
inst_bytes = buf.getvalue()

upm = inst["head"].unitsPerEm
gs = inst.getGlyphSet()
cmap = inst.getBestCmap()
hmtx = inst["hmtx"]
VERSION = inst["name"].getDebugName(5)


def bounds(gname):
    bp = BoundsPen(gs)
    gs[gname].draw(bp)
    return bp.bounds  # (xMin, yMin, xMax, yMax)


cap_font = bounds(cmap[ord("T")])[3]           # верх «T»
xh_font = bounds(cmap[ord("e")])[3]            # верх «e», с наплывом
desc_font = min(bounds(cmap[ord(c)])[1] for c in "yp")
SCALE = CAP / cap_font


def path_polylines(d, steps=48, flip=False):
    """Путь SVG как набор ломаных. flip — перевернуть y, если он смотрит вниз.

    Квадратичные цепочки TrueType сперва нормализуются через parse_path,
    поэтому у каждой кривой здесь ровно одна контрольная точка.
    """
    rec = RecordingPen()
    parse_path(d, rec)
    s = -1.0 if flip else 1.0
    out, cur = [], None
    for op, raw in rec.value:
        pts = [(x, s * y) for x, y in raw]
        if op == "moveTo":
            cur = [pts[0]]
        elif op == "lineTo":
            cur.append(pts[0])
        elif op in ("curveTo", "qCurveTo"):
            prev, c, end = cur[-1], list(pts), pts[-1]
            for k in range(1, steps + 1):
                t = k / steps
                if len(c) == 3:
                    p1, p2, p3 = c
                    x = (1-t)**3*prev[0] + 3*(1-t)**2*t*p1[0] + 3*(1-t)*t*t*p2[0] + t**3*p3[0]
                    y = (1-t)**3*prev[1] + 3*(1-t)**2*t*p1[1] + 3*(1-t)*t*t*p2[1] + t**3*p3[1]
                else:
                    p1 = c[0]
                    x = (1-t)**2*prev[0] + 2*(1-t)*t*p1[0] + t*t*end[0]
                    y = (1-t)**2*prev[1] + 2*(1-t)*t*p1[1] + t*t*end[1]
                cur.append((x, y))
        elif op == "closePath":
            cur.append(cur[0]); out.append(cur); cur = None
    if cur:
        cur.append(cur[0]); out.append(cur)
    return out


def polylines(gname, steps=48, gs=None):
    """Контуры глифа как ломаные в единицах шрифта, y вверх."""
    gs = globals()["gs"] if gs is None else gs
    spen = SVGPathPen(gs)
    gs[gname].draw(spen)
    return path_polylines(spen.getCommands(), steps=steps)


def scan_width(gname, y, gs=None):
    """Абсциссы пересечений горизонтали y с контуром — для замера штамба."""
    xs = []
    for poly in polylines(gname, gs=gs):
        for (x1, y1), (x2, y2) in zip(poly, poly[1:]):
            if (y1 - y) * (y2 - y) < 0:
                xs.append(x1 + (x2 - x1) * (y - y1) / (y2 - y1))
    return sorted(xs)


def ring_apex(gname, gs=None):
    """Апекс внутреннего контура и внешний контур над ним, в единицах шрифта.

    Возвращает (x, y_внутренний, y_внешний): между двумя ординатами и лежит
    тонкий штрих кольца.
    """
    def area(poly):
        return sum(x1*y2 - x2*y1 for (x1, y1), (x2, y2) in zip(poly, poly[1:])) / 2
    polys = polylines(gname, gs=gs)
    outer = max(polys, key=lambda p: abs(area(p)))
    counters = [p for p in polys if p is not outer]
    if not counters:
        return None
    counter = max(counters, key=lambda p: max(q[1] for q in p))
    apex = max(counter, key=lambda q: q[1])
    ys = []
    for (x1, y1), (x2, y2) in zip(outer, outer[1:]):
        if (x1 - apex[0]) * (x2 - apex[0]) < 0:
            ys.append(y1 + (y2 - y1) * (apex[0] - x1) / (x2 - x1))
    return apex[0], apex[1], max(ys)


def ring_thin(gname, gs=None):
    """Толщина верха кольца — тонкий штрих гарнитуры."""
    x, y_in, y_out = ring_apex(gname, gs=gs)
    return y_out - y_in


# штамб «p» — замер на середине роста строчных; тонкий штрих — верх кольца «e»
stem_font = scan_width(cmap[ord("p")], xh_font / 2)[1] - scan_width(cmap[ord("p")], xh_font / 2)[0]
thin_font = ring_thin(cmap[ord("e")])

CAPH = CAP
XH = xh_font * SCALE               # очко «e», с наплывом
XH_OS2 = inst["OS/2"].sxHeight * SCALE
DESC = -desc_font * SCALE          # глубина выносных вниз, положительная
STEM = stem_font * SCALE           # модуль
THIN = thin_font * SCALE
EM = upm * SCALE                   # кегельная площадка по высоте

# --- шейпинг -----------------------------------------------------------------

face = hb.Face(inst_bytes)
font = hb.Font(face)
font.scale = (upm, upm)
hbuf = hb.Buffer()
hbuf.add_str(WORD)
hbuf.guess_segment_properties()
hb.shape(font, hbuf, {"kern": True, "liga": True})
shaped = [
    (inst.getGlyphName(i.codepoint), p.x_advance, p.x_offset, p.y_offset)
    for i, p in zip(hbuf.glyph_infos, hbuf.glyph_positions)
]
assert len(shaped) == len(WORD), shaped
GLYPHS = [g for g, _, _, _ in shaped]

# Полуапроши инстанса: левый — xMin, правый — кегельная площадка минус xMax.
# Ширина берётся из hmtx, а не из шейпинга: кернинг здесь неприменим, потому
# что каждая литера стоит в своём гнезде и придвигать её к соседке некуда.
BEARINGS = [
    (bounds(g)[0] * SCALE, (hmtx[g][0] - bounds(g)[2]) * SCALE) for g in GLYPHS
]
KERNS = [(a - hmtx[g][0]) * SCALE for g, a, _, _ in shaped]   # что отбросили

# --- сетка гнёзд -------------------------------------------------------------

MODULE = STEM                                  # модуль: штамб «p»
AIR = sum(sum(t) for t in BEARINGS) / (2 * len(BEARINGS))   # средний полуапрош
CELL_H = CAPH + DESC + 2 * AIR                 # гнездо: литера плюс воздух
BASELINE = AIR + CAPH                          # от верха гнезда до базовой
GAP = MODULE                                   # перегородка между гнёздами
RADIUS = MODULE                                # скругление гнезда


def layout(mode="ink"):
    """Гнёзда как (x_гнезда, ширина_гнезда, x_пера) при базовой линии BASELINE.

    ink — гнездо по литере плюс воздух с двух сторон; advance — по кегельной
    площадке шрифта; square — все гнёзда по высоте гнезда. Эталон выбрал ink,
    остальные два режима остались, чтобы документ мог показать отвергнутое.
    """
    out, x = [], 0.0
    for g in GLYPHS:
        b = bounds(g)
        ink_w = (b[2] - b[0]) * SCALE
        if mode == "advance":
            w, pen_x = hmtx[g][0] * SCALE, x
        elif mode == "square":
            w, pen_x = CELL_H, x + (CELL_H - ink_w) / 2 - b[0] * SCALE
        else:
            w, pen_x = ink_w + 2 * AIR, x + AIR - b[0] * SCALE
        out.append((x, w, pen_x))
        x += w + GAP
    return out


CELLS = layout("ink")
EMPTY_W = sum(w for _, w, _ in CELLS) / len(CELLS)     # средняя ширина гнёзд
EMPTY_X = CELLS[-1][0] + CELLS[-1][1] + GAP
WIDTH = EMPTY_X + EMPTY_W


def empty_widths():
    """Чем ещё можно было задать свободное гнездо — пробельным материалом."""
    return {
        "thin": hmtx[cmap[0x2009]][0] * SCALE,   # тонкая шпация
        "space": hmtx[cmap[ord(" ")]][0] * SCALE,  # пробел шрифта
        "mean": EMPTY_W,                         # средняя ширина гнёзд — выбрано
        "en": EM / 2,                            # полукруглая
        "widest": max(w for _, w, _ in CELLS),   # самое широкое гнездо, «T»
        "em": EM,                                # круглая, она же кегельная
    }


def glyph_path(gname, pen_x, baseline=None):
    spen = SVGPathPen(gs, ntos=lambda v: f"{v:.3f}".rstrip("0").rstrip("."))
    t = Transform(SCALE, 0, 0, -SCALE, pen_x, BASELINE if baseline is None else baseline)
    gs[gname].draw(TransformPen(spen, t))
    return spen.getCommands()


# --- цвет и акцент -----------------------------------------------------------

PAL = {
    # card=None — гнёзда не рисуются вовсе
    "light": dict(ink="#17211C", card="#E8DFCB", accent="#4A6B52"),
    "dark": dict(ink="#E8DFCB", card="#242E27", accent="#7FA085"),
    "mono": dict(ink="#17211C", card=None, accent="#17211C"),
}

# (гнёзда под акцентом, литеры под акцентом, свободное гнездо в конце)
ACCENTS = {
    "empty": (set(), set(), True),          # выбрано эталоном
    "none": (set(), set(), False),
    "cell": ({0}, set(), False),
    "letter": (set(), {0}, False),
    "split": ({4, 5, 6, 7}, set(), False),
    "last": ({7}, set(), False),
}


def wordmark_inner(accent="empty", cells="ink", theme="light", empty_mode="mean"):
    """Содержимое надписи без обёртки <svg> — чтобы строить из неё фигуры."""
    p = PAL[theme]
    cl = layout(cells)
    accent_idx, accent_glyphs, empty_nest = ACCENTS[accent]
    total_w = cl[-1][0] + cl[-1][1]
    parts = []
    if p["card"] is not None:
        for i, (x, w, _) in enumerate(cl):
            fill = p["accent"] if i in accent_idx else p["card"]
            parts.append(
                f'<rect x="{x:.2f}" y="0" width="{w:.2f}" height="{CELL_H:.2f}" '
                f'rx="{RADIUS:.2f}" fill="{fill}"/>'
            )
    if empty_nest:
        empty_x = total_w + GAP
        empty_w = empty_widths()[empty_mode]
        total_w = empty_x + empty_w
        parts.append(
            f'<rect x="{empty_x:.2f}" y="0" width="{empty_w:.2f}" height="{CELL_H:.2f}" '
            f'rx="{RADIUS:.2f}" fill="{p["accent"]}"/>'
        )
    for i, (_, _, pen_x) in enumerate(cl):
        if i in accent_idx:
            color = p["card"] or p["ink"]
        elif i in accent_glyphs:
            color = p["accent"]
        else:
            color = p["ink"]
        parts.append(f'<path d="{glyph_path(GLYPHS[i], pen_x)}" fill="{color}"/>')
    return total_w, CELL_H, "".join(parts)


def wordmark(accent="empty", cells="ink", theme="light", empty_mode="mean", pad=0.0):
    total_w, cell_h, parts = wordmark_inner(accent, cells, theme, empty_mode)
    vb = f"{-pad:.2f} {-pad:.2f} {total_w+2*pad:.2f} {cell_h+2*pad:.2f}"
    return total_w, cell_h, (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vb}" '
        f'role="img" aria-label="Typecase">' + parts + "</svg>"
    )


def metrics():
    """Всё, чем документ имеет право пользоваться, — одним словарём."""
    return {
        "word": WORD, "version": VERSION, "upm": upm, "opsz": OPSZ, "wght": WGHT,
        "cap_font": cap_font, "scale": SCALE,
        "cap": CAPH, "xh": XH, "xh_os2": XH_OS2, "desc": DESC, "em": EM,
        "stem_font": stem_font, "stem": STEM,
        "thin_font": thin_font, "thin": THIN, "contrast": STEM / THIN,
        "module": MODULE, "air": AIR, "cell_h": CELL_H, "baseline": BASELINE,
        "gap": GAP, "radius": RADIUS,
        "cells": [(round(x, 2), round(w, 2), round(pen, 3)) for x, w, pen in CELLS],
        "ring_apex": ring_apex(cmap[ord("e")]),
        "ink_widths": [round((bounds(g)[2] - bounds(g)[0]) * SCALE, 2) for g in GLYPHS],
        "advances": [round(hmtx[g][0] * SCALE, 2) for g in GLYPHS],
        "kerns": [round(k, 2) for k in KERNS],
        "cells_sum": round(sum(w for _, w, _ in CELLS), 2),
        "advances_sum": round(sum(hmtx[g][0] * SCALE for g in GLYPHS), 2),
        "empty_w": EMPTY_W, "empty_x": EMPTY_X, "width": WIDTH,
        "empty_widths": {k: round(v, 2) for k, v in empty_widths().items()},
        "width_advance": layout("advance")[-1][0] + layout("advance")[-1][1],
        "width_square": layout("square")[-1][0] + layout("square")[-1][1],
    }


if __name__ == "__main__":
    import sys
    if "--svg" in sys.argv:
        # Отдельным процессом, чтобы документ мог показать другой инстанс шрифта.
        a = (sys.argv[sys.argv.index("--svg") + 1:] + ["empty", "ink", "light"])[:3]
        w, h, svg = wordmark(accent=a[0], cells=a[1], theme=a[2])
        print(json.dumps({"w": w, "h": h, "svg": svg, "m": metrics()}))
    else:
        print(json.dumps(metrics(), indent=2, ensure_ascii=False))
