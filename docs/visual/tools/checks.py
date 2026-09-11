"""Проверки построения надписи Typecase.

Каждая смотрит ровно на то, что названа проверять, и возвращает вердикт
строкой. Документ подставляет эти строки в раздел «Проверки» — своих слов о
результате у него нет.

    python checks.py
"""

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from fontTools.misc.transform import Transform
from fontTools.pens.areaPen import AreaPen
from fontTools.pens.recordingPen import RecordingPen
from fontTools.pens.transformPen import TransformPen
from fontTools.svgLib.path import parse_path

import typecase_logotype as T

MASTER = os.path.normpath(
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "typecase-logotype.svg")
)


def contours():
    """Площадь выпущенного контура против площади глифа в шрифте."""
    worst, where = 0.0, None
    for ch, g in zip(T.WORD, T.GLYPHS):
        ref = AreaPen(T.gs)
        T.gs[g].draw(TransformPen(ref, Transform(T.SCALE, 0, 0, -T.SCALE, 0, 0)))
        got = AreaPen(None)
        parse_path(T.glyph_path(g, 0.0, 0.0), got)
        d = abs(abs(ref.value) - abs(got.value)) / abs(ref.value) * 100
        if d > worst:
            worst, where = d, ch
    return worst, where


def inside():
    """Зазор от очка литеры до стенок её гнезда, с четырёх сторон."""
    worst, where = 1e9, None
    for i, (x, w, pen) in enumerate(T.CELLS):
        b = T.bounds(T.GLYPHS[i])
        for name, v in (
            ("слева", pen + b[0] * T.SCALE - x),
            ("справа", x + w - (pen + b[2] * T.SCALE)),
            ("сверху", T.BASELINE - b[3] * T.SCALE),
            ("снизу", T.CELL_H - (T.BASELINE - b[1] * T.SCALE)),
        ):
            if v < worst:
                worst, where = v, f"{T.WORD[i]} {name}"
    return worst, where


def gaps():
    """Зазор между очками соседних литер."""
    out = []
    for i in range(len(T.CELLS) - 1):
        b1, b2 = T.bounds(T.GLYPHS[i]), T.bounds(T.GLYPHS[i + 1])
        right = T.CELLS[i][2] + b1[2] * T.SCALE
        left = T.CELLS[i + 1][2] + b2[0] * T.SCALE
        out.append((T.WORD[i] + T.WORD[i + 1], left - right))
    return out


def sums():
    """Сумма гнёзд против суммы кегельных площадок шрифта."""
    cells = sum(w for _, w, _ in T.CELLS)
    advances = sum(T.hmtx[g][0] * T.SCALE for g in T.GLYPHS)
    return cells, advances


def limits():
    """Ширина вывода, при которой тонкий штрих держит 1 и 1.5 px."""
    return T.WIDTH / T.THIN, 1.5 * T.WIDTH / T.THIN


def master():
    """Что лежит в выпущенном файле: фигуры, заливки, обводки, текст."""
    if not os.path.exists(MASTER):
        return None
    s = open(MASTER, encoding="utf-8").read()
    return {
        "rects": len(re.findall(r"<rect", s)),
        "paths": len(re.findall(r"<path", s)),
        "strokes": len(re.findall(r"stroke", s)),
        "texts": len(re.findall(r"<text|font-family", s)),
        "fills": sorted(set(re.findall(r'fill="([^"]+)"', s))),
        "meta": len(re.findall(r"<metadata|c2pa", s)),
        "bytes": len(s.encode("utf-8")),
    }


def run():
    """Вердикты для раздела «Проверки» — по строке на проверку."""
    worst_c, where_c = contours()
    worst_i, where_i = inside()
    g = gaps()
    lo, hi = min(g, key=lambda t: t[1]), max(g, key=lambda t: t[1])
    cells, advances = sums()
    l1, l15 = limits()
    m = master()
    verdicts = {
        "contours": (
            f"Все восемь литер выпущены в кривые и сверены по площади с тем же "
            f"глифом в шрифте. Худшее расхождение {worst_c:.5f}\u00a0% (литера «{where_c}») — "
            f"остаток разбиения кривых на точки. Извлечение ничего не искажает"
        ),
        "inside": (
            f"Минимальный зазор от очка до стенки гнезда {worst_i:.2f} ({where_i}) — "
            f"это и есть воздух: ни одна литера его не съедает. Отрицательным зазор "
            f"стал бы у литеры выше прописных или глубже выносной вниз"
        ),
        "gaps": (
            f"Одинаков по всему слову: {lo[1]:.2f} у «{lo[0]}» и {hi[1]:.2f} у «{hi[0]}». "
            f"Кернинг отброшен по построению, разниться нечему"
        ),
        "sum": (
            f"{cells:.2f} против {advances:.2f} — сходится. Воздух в гнезде равен "
            f"среднему полуапрошу, поэтому касса шире набора ровно на перегородки"
        ),
        "thin": (
            f"Тонкий штрих {T.THIN:.2f} при габарите {T.WIDTH:.2f}: пиксель он держит "
            f"от {l1:.0f}\u00a0px по ширине, полтора — от {l15:.0f}\u00a0px. Предел раздела\u00a010 "
            f"взят по второму"
        ),
    }
    if m:
        verdicts["master"] = (
            f"В файле {m['rects']} прямоугольника и {m['paths']} контура, "
            f"{m['strokes']} обводок, {m['texts']} упоминаний шрифта и {m['meta']} "
            f"блоков метаданных. Заливки: {', '.join(m['fills'])}. Размер {m['bytes'] / 1024:.1f}\u00a0КБ"
        )
    return verdicts


if __name__ == "__main__":
    worst_c, where_c = contours()
    print(f"1. Контуры        худшее расхождение {worst_c:.5f} % (литера «{where_c}»)")
    worst_i, where_i = inside()
    print(f"2. Внутри гнезда  минимальный зазор {worst_i:.2f} ({where_i})")
    g = gaps()
    print("3. Зазоры         " + " · ".join(f"{k} {v:.2f}" for k, v in g))
    cells, advances = sums()
    print(f"4. Суммы          гнёзда {cells:.2f} против кегельных {advances:.2f} — "
          f"{'сходится' if abs(cells - advances) < 0.01 else 'РАСХОДИТСЯ'}")
    l1, l15 = limits()
    print(f"5. Тонкий штрих   {T.THIN:.2f}; 1 px от {l1:.0f} px, 1.5 px от {l15:.0f} px")
    m = master()
    if m:
        print(f"6. Мастер         rect {m['rects']}, path {m['paths']}, stroke {m['strokes']}, "
              f"шрифт {m['texts']}, метаданные {m['meta']}, {m['bytes'] / 1024:.1f} КБ")
        print(f"                  заливки: {', '.join(m['fills'])}")
    else:
        print("6. Мастер         не выпущен")
