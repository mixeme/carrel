package gate

import (
	"regexp"
	"strings"
)

var (
	buttonBlockRe = regexp.MustCompile(`(?is)<button\b[^>]*>.*?</button>`)
	btnLinkRe     = regexp.MustCompile(`(?is)<a\b[^>]*class="[^"]*\btc-btn\b[^"]*"[^>]*>.*?</a>`)
	inputTagRe    = regexp.MustCompile(`<(?:input|textarea|select)\b[^>]*>`)
	ariaLabelRe   = regexp.MustCompile(`aria-label="([^"]*)"`)
	attrIDRe      = regexp.MustCompile(`\sid="([^"]+)"`)
	labelTextRe   = regexp.MustCompile(`tc-(?:btn|menu|usermenu|tree|chip)__label">([^<]+)</`)
	buttonBodyRe  = regexp.MustCompile(`(?is)<button\b[^>]*>(.*)</button>`)
	tagStripRe    = regexp.MustCompile(`<[^>]*>`)
)

func eachControl(html string, re *regexp.Regexp, fn func(tag string) bool) {
	for _, m := range re.FindAllString(html, -1) {
		if !fn(m) {
			return
		}
	}
}

func buttonHasAccessibleName(buttonHTML string) bool {
	if m := ariaLabelRe.FindStringSubmatch(buttonHTML); m != nil && strings.TrimSpace(m[1]) != "" {
		return true
	}
	if strings.Contains(buttonHTML, `class="tc-close"`) && ariaLabelRe.MatchString(buttonHTML) {
		return true
	}
	if m := labelTextRe.FindStringSubmatch(buttonHTML); m != nil && strings.TrimSpace(m[1]) != "" {
		return true
	}
	if m := buttonBodyRe.FindStringSubmatch(buttonHTML); m != nil {
		body := strings.TrimSpace(tagStripRe.ReplaceAllString(m[1], ""))
		if body != "" {
			return true
		}
	}
	return false
}

func inputInsideLabel(html, tag string) bool {
	idx := strings.Index(html, tag)
	if idx < 0 {
		return false
	}
	before := html[:idx]
	return strings.LastIndex(before, "<label") > strings.LastIndex(before, "</label>")
}

func inputHasAccessibleName(html, tag string) bool {
	if m := ariaLabelRe.FindStringSubmatch(tag); m != nil && strings.TrimSpace(m[1]) != "" {
		return true
	}
	if inputInsideLabel(html, tag) {
		return true
	}
	if m := attrIDRe.FindStringSubmatch(tag); m != nil {
		id := m[1]
		if strings.Contains(html, `for="`+id+`"`) {
			return true
		}
	}
	return false
}

func checkEntryA11y(html string) []string {
	var bad []string
	eachControl(html, buttonBlockRe, func(tag string) bool {
		if !buttonHasAccessibleName(tag) {
			bad = append(bad, "button")
		}
		return true
	})
	eachControl(html, btnLinkRe, func(tag string) bool {
		if !buttonHasAccessibleName(tag) {
			bad = append(bad, "button-link")
		}
		return true
	})
	for _, tag := range inputTagRe.FindAllString(html, -1) {
		if strings.Contains(tag, `type="hidden"`) {
			continue
		}
		if !inputHasAccessibleName(html, tag) {
			bad = append(bad, "field")
		}
	}
	return bad
}
