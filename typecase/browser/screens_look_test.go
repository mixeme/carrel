package browser_test

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/chromedp/chromedp"

	"carrel.dev/typecase/browser"
	"carrel.dev/typecase/screens"
)

type lookProbe struct {
	NavHeight       float64  `json:"navHeight"`
	NavCurrentBg    string   `json:"navCurrentBg"`
	CardBg          string   `json:"cardBg"`
	TickRowHeight   float64  `json:"tickRowHeight"`
	ListBorder      float64  `json:"listBorder"`
	BrandHasSVG     bool     `json:"brandHasSVG"`
	SideWidth       float64  `json:"sideWidth"`
	MainResolvedBg  string   `json:"mainResolvedBg"`
	BgToken         string   `json:"bgToken"`
	TitleSize       float64  `json:"titleSize"`
	TitleLineHeight float64  `json:"titleLineHeight"`
	SymbolMissing   []string `json:"symbolMissing"`
	NavEmoji        []string `json:"navEmoji"`
	Rank2Buttons    int      `json:"rank2Buttons"`
}

type lookResult struct {
	Frames []lookProbe `json:"frames"`
}

const lookProbeJS = `(() => {
  const rgbOf = (value) => {
    const d = document.createElement('div');
    d.style.background = value;
    document.body.appendChild(d);
    const c = getComputedStyle(d).backgroundColor;
    d.remove();
    return c;
  };
  const resolvedBg = (el) => {
    let n = el;
    while (n && n !== document.documentElement) {
      const bg = getComputedStyle(n).backgroundColor;
      if (bg && bg !== 'transparent' && bg !== 'rgba(0, 0, 0, 0)') return bg;
      n = n.parentElement;
    }
    return '';
  };
  const probeRoot = (root) => {
    const nav = root.querySelector('.tc-nav__item.is-current') || root.querySelector('.tc-nav__item');
    const tick = root.querySelector('.tc-tickrow');
    const list = root.querySelector('.tc-list');
    const brandSVG = root.querySelector('.tc-brand svg');
    const side = root.querySelector('.tc-side');
    const main = root.querySelector('.tc-main');
    const title = root.querySelector('.tc-head__title');
    const missing = [];
    root.querySelectorAll('use[href^="#"]').forEach((u) => {
      const id = (u.getAttribute('href') || '').slice(1);
      if (id && !document.getElementById(id)) missing.push(id);
    });
    const navEmoji = [];
    root.querySelectorAll('.tc-nav__icon-svg').forEach((el) => {
      const t = el.textContent.trim();
      if (t) navEmoji.push(t);
    });
    let titleLH = 0, titleSize = 0;
    if (title) {
      const cs = getComputedStyle(title);
      titleLH = parseFloat(cs.lineHeight) || 0;
      titleSize = parseFloat(cs.fontSize) || 0;
    }
    const current = nav && nav.classList.contains('is-current');
    return {
      navHeight: nav ? nav.getBoundingClientRect().height : 0,
      navCurrentBg: current ? getComputedStyle(nav).backgroundColor : '',
      cardBg: rgbOf('var(--tc-card)'),
      tickRowHeight: tick ? tick.getBoundingClientRect().height : 0,
      listBorder: list ? parseFloat(getComputedStyle(list).borderTopWidth) || 0 : -1,
      brandHasSVG: !!brandSVG,
      sideWidth: side ? side.getBoundingClientRect().width : 0,
      mainResolvedBg: main ? resolvedBg(main) : '',
      bgToken: rgbOf('var(--tc-bg)'),
      titleSize,
      titleLineHeight: titleLH,
      symbolMissing: missing,
      navEmoji,
      rank2Buttons: root.querySelectorAll('[data-tc-bar-rank2] .tc-btn').length,
    };
  };
  const states = document.querySelectorAll('.screens-frame__mock');
  if (states.length) {
    return { frames: Array.from(states).map(probeRoot) };
  }
  return { frames: [probeRoot(document)] };
})()`

func TestScreensLook(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if testing.Short() {
		t.Skip("screen look probe needs Chrome")
	}
	srv := screensServer(t)
	h := browser.NewHarness(t)
	for _, sc := range screens.All() {
		sc := sc
		t.Run(sc.ID, func(t *testing.T) {
			var raw json.RawMessage
			url := screenURL(srv.URL, sc.ID, "")
			if err := h.Run(url, 1280, 900, layoutSettle(), chromedp.Evaluate(lookProbeJS, &raw)); err != nil {
				t.Fatal(err)
			}
			var res lookResult
			if err := json.Unmarshal(raw, &res); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(res.Frames) == 0 {
				t.Fatal("look probe returned no frames")
			}
			if sc.ID == "states" && len(res.Frames) < 5 {
				t.Fatalf("states frames: got %d want >= 5", len(res.Frames))
			}
			for i, p := range res.Frames {
				checkLookProbe(t, sc.ID, i, p)
			}
		})
	}
}

func checkLookProbe(t *testing.T, id string, frame int, p lookProbe) {
	t.Helper()
	prefix := id
	if frame > 0 || id == "states" {
		prefix = id + " frame " + strconv.Itoa(frame)
	}
	if p.NavHeight > 0 && !closeEnough(p.NavHeight, 29) {
		t.Fatalf("%s .tc-nav__item height %.1f want 29", prefix, p.NavHeight)
	}
	if p.NavCurrentBg != "" && p.NavCurrentBg != p.CardBg {
		t.Fatalf("%s .is-current background %s want %s (--tc-card)", prefix, p.NavCurrentBg, p.CardBg)
	}
	if p.TickRowHeight > 0 && !closeEnough(p.TickRowHeight, 26) {
		t.Fatalf("%s .tc-tickrow height %.1f want 26", prefix, p.TickRowHeight)
	}
	if p.ListBorder > 0.5 {
		t.Fatalf("%s .tc-list border %.1f want 0", prefix, p.ListBorder)
	}
	if !p.BrandHasSVG {
		t.Fatalf("%s .tc-brand svg missing", prefix)
	}
	if len(p.SymbolMissing) > 0 {
		t.Fatalf("%s missing sprite symbols: %v", prefix, p.SymbolMissing)
	}
	if len(p.NavEmoji) > 0 {
		t.Fatalf("%s emoji in nav icons: %v", prefix, p.NavEmoji)
	}
	if p.MainResolvedBg != "" && p.MainResolvedBg != p.BgToken {
		t.Fatalf("%s .tc-main background %s want %s (--tc-bg)", prefix, p.MainResolvedBg, p.BgToken)
	}
	if p.TitleSize > 0 && !closeEnough(p.TitleSize, 22) {
		t.Fatalf("%s .tc-head__title font-size %.1f want 22", prefix, p.TitleSize)
	}
	if p.TitleSize > 0 && !closeEnough(p.TitleLineHeight, p.TitleSize*1.15) {
		t.Fatalf("%s .tc-head__title line-height %.1f want %.1f (1.15)", prefix, p.TitleLineHeight, p.TitleSize*1.15)
	}
	if id == "notes" && p.SideWidth > 0 && !closeEnough(p.SideWidth, 440) {
		t.Fatalf("%s .tc-side width %.1f want 440", prefix, p.SideWidth)
	}
	if id == "notes" && p.Rank2Buttons < 2 {
		t.Fatalf("%s Rank2 buttons %d want >= 2 at 1280", prefix, p.Rank2Buttons)
	}
}

func layoutSettle() chromedp.Action {
	return chromedp.Evaluate(`new Promise(function(resolve) {
  (function tick(n) {
    if (window.__tc && window.__tc.layoutUpdates) {
      window.__tc.layoutUpdates();
      requestAnimationFrame(function() {
        window.__tc.layoutUpdates();
        requestAnimationFrame(resolve);
      });
      return;
    }
    if (n > 200) { resolve(); return; }
    setTimeout(function() { tick(n + 1); }, 25);
  })(0);
})`, nil)
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) <= 1
}
