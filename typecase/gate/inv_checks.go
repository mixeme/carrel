package gate

import (
	"fmt"
	"regexp"
	"strings"
)

type invGateSpec struct {
	widget string
	fn     func(widget string, html []string, widgetDir string) error
}

var invGateChecks = map[string]invGateSpec{
	"seg-one-chosen":                {widget: "tc-seg", fn: checkSegOneChosen},
	"crumbs-last-not-link":          {widget: "tc-crumbs", fn: checkCrumbsLastNotLink},
	"range-order":                   {widget: "tc-range", fn: checkRangeOrder},
	"table-number-columns":          {widget: "tc-table", fn: checkTableNumberColumns},
	"inv-stat-string-value":         {widget: "tc-stat", fn: checkInvStatStringValue},
	"inv-stats-no-wrap":             {widget: "tc-stats", fn: checkInvStatsNoWrap},
	"inv-avatar-not-empty":          {widget: "tc-avatar", fn: checkInvAvatarNotEmpty},
	"inv-avatar-square":             {widget: "tc-avatar", fn: checkInvAvatarSquare},
	"inv-badge-has-text":            {widget: "tc-badge", fn: checkInvBadgeHasText},
	"inv-badge-not-link":            {widget: "tc-badge", fn: checkInvBadgeNotLink},
	"inv-brand-single-mark":         {widget: "tc-brand", fn: checkInvBrandSingleMark},
	"inv-brand-no-dup-word":         {widget: "tc-brand", fn: checkInvBrandNoDupWord},
	"inv-card-has-body":             {widget: "tc-card", fn: checkInvCardHasBody},
	"inv-chip-state-class":          {widget: "tc-chip", fn: checkInvChipStateClass},
	"inv-crop-has-frame":            {widget: "tc-crop", fn: checkInvCropHasFrame},
	"inv-crop-has-zoom":             {widget: "tc-crop", fn: checkInvCropHasZoom},
	"inv-crumbs-has-root":           {widget: "tc-crumbs", fn: checkInvCrumbsHasRoot},
	"inv-diag-collapsed-default":    {widget: "tc-diag", fn: checkInvDiagCollapsedDefault},
	"inv-diag-mono":                 {widget: "tc-diag", fn: checkInvDiagMono},
	"inv-diff-options-column":       {widget: "tc-diff", fn: checkInvDiffOptionsColumn},
	"inv-diff-one-chosen":           {widget: "tc-diff", fn: checkInvDiffOneChosen},
	"inv-doc-mode-switch":           {widget: "tc-doc", fn: checkInvDocModeSwitch},
	"inv-doc-no-silent-save":        {widget: "tc-doc", fn: checkInvDocNoSilentSave},
	"inv-docbar-actions":            {widget: "tc-docbar", fn: checkInvDocbarActions},
	"inv-docbar-dirty-flag":         {widget: "tc-docbar", fn: checkInvDocbarDirtyFlag},
	"inv-docmeta-sections":          {widget: "tc-docmeta", fn: checkInvDocmetaSections},
	"inv-fields-dl-structure":       {widget: "tc-fields", fn: checkInvFieldsDLStructure},
	"inv-fset-rubric":               {widget: "tc-fset", fn: checkInvFsetRubric},
	"inv-grid-tiles":                {widget: "tc-grid", fn: checkInvGridTiles},
	"inv-grid-uniform":              {widget: "tc-grid", fn: checkInvGridUniform},
	"inv-head-title":                {widget: "tc-head", fn: checkInvHeadTitle},
	"inv-kbd-not-button":            {widget: "tc-kbd", fn: checkInvKbdNotButton},
	"inv-list-empty-action":         {widget: "tc-list", fn: checkInvListEmptyAction},
	"inv-main-body":                 {widget: "tc-main", fn: checkInvMainBody},
	"inv-meter-has-value":           {widget: "tc-meter", fn: checkInvMeterHasValue},
	"inv-nav-one-current":           {widget: "tc-nav", fn: checkInvNavOneCurrent},
	"inv-num-unknown-hidden":        {widget: "tc-num", fn: checkInvNumUnknownHidden},
	"inv-overlay-kind-class":        {widget: "tc-overlay", fn: checkInvOverlayKindClass},
	"inv-queue-question":            {widget: "tc-queue", fn: checkInvQueueQuestion},
	"inv-queue-meter":               {widget: "tc-queue", fn: checkInvQueueMeter},
	"inv-rail-foot":                 {widget: "tc-rail", fn: checkInvRailFoot},
	"inv-rail-drawer-class":         {widget: "tc-rail", fn: checkInvRailDrawerClass},
	"inv-railfoot-build":            {widget: "tc-railfoot", fn: checkInvRailfootBuild},
	"inv-railfoot-actions":          {widget: "tc-railfoot", fn: checkInvRailfootActions},
	"inv-railsec-state":             {widget: "tc-railsec", fn: checkInvRailsecState},
	"inv-railsec-untitled-no-title": {widget: "tc-railsec", fn: checkInvRailsecUntitledNoTitle},
	"inv-range-field-width":         {widget: "tc-range", fn: checkInvRangeFieldWidth},
	"inv-row-mark":                  {widget: "tc-row", fn: checkInvRowMark},
	"inv-row-tail":                  {widget: "tc-row", fn: checkInvRowTail},
	"inv-row-menu-in-tail":          {widget: "tc-row", fn: checkInvRowMenuInTail},
	"inv-rubric-not-heading":        {widget: "tc-rubric", fn: checkInvRubricNotHeading},
	"inv-runbar-meter-hidden-idle":  {widget: "tc-runbar", fn: checkInvRunbarMeterHiddenIdle},
	"inv-runbar-states":             {widget: "tc-runbar", fn: checkInvRunbarStates},
	"inv-scrim-for-attr":            {widget: "tc-scrim", fn: checkInvScrimForAttr},
	"inv-search-popover-below":      {widget: "tc-search", fn: checkInvSearchPopoverBelow},
	"inv-search-no-empty-submit":    {widget: "tc-search", fn: checkInvSearchNoEmptySubmit},
	"inv-sec-like-card":             {widget: "tc-sec", fn: checkInvSecLikeCard},
	"inv-seg-narrow-menu":           {widget: "tc-seg", fn: checkInvSegNarrowMenu},
	"inv-shell-four-zones":          {widget: "tc-shell", fn: checkInvShellFourZones},
	"inv-skel-shape":                {widget: "tc-skel", fn: checkInvSkelShape},
	"inv-steps-one-current":         {widget: "tc-steps", fn: checkInvStepsOneCurrent},
	"inv-steps-count":               {widget: "tc-steps", fn: checkInvStepsCount},
	"inv-table-column-kind":         {widget: "tc-table", fn: checkInvTableColumnKind},
	"inv-table-scrollable":          {widget: "tc-table", fn: checkInvTableScrollable},
	"inv-tag-not-editable":          {widget: "tc-tag", fn: checkInvTagNotEditable},
	"inv-thumb-placeholder":         {widget: "tc-thumb", fn: checkInvThumbPlaceholder},
	"inv-thumb-icon-fallback":       {widget: "tc-thumb", fn: checkInvThumbIconFallback},
	"inv-tickrow-count-optional":    {widget: "tc-tickrow", fn: checkInvTickrowCountOptional},
	"inv-tile-has-name":             {widget: "tc-tile", fn: checkInvTileHasName},
	"inv-top-brand-once":            {widget: "tc-top", fn: checkInvTopBrandOnce},
	"inv-top-search":                {widget: "tc-top", fn: checkInvTopSearch},
	"inv-tree-one-chosen":           {widget: "tc-tree", fn: checkInvTreeOneChosen},
	"inv-usermenu-open-state":       {widget: "tc-usermenu", fn: checkInvUsermenuOpenState},
}

var (
	crumbsCurrentRe = regexp.MustCompile(`class="tc-crumbs__current"`)
)

func isRunnableInvGate(name string) bool {
	_, ok := invGateChecks[name]
	return ok
}

func invGateWidget(name string) string {
	if spec, ok := invGateChecks[name]; ok {
		return spec.widget
	}
	return ""
}

// RunnableInvGateNames returns sorted inv gate check names for bite tests.
func RunnableInvGateNames() []string {
	names := make([]string, 0, len(invGateChecks))
	for name := range invGateChecks {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// InvGateWidget returns the widget a gate check applies to.
func InvGateWidget(name string) string {
	return invGateWidget(name)
}

// CheckInvGate runs one markup invariant gate check.
func CheckInvGate(name, widget string, catalog []CatalogEntry, widgetDir string) error {
	return checkInvariantGate("gate:"+name, widget, catalog, widgetDir)
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

// widgetHTML returns the catalog entries declared for the widget. Structural
// invariants — «rail must include foot» — only make sense on the widget's own
// entry, so this stays narrow.
func widgetHTML(catalog []CatalogEntry, widget string) []string {
	var html []string
	for _, e := range catalog {
		if e.Widget == widget {
			html = append(html, e.HTML)
		}
	}
	return html
}

// compositeInvariants are per-occurrence checks: they hold wherever the widget
// is drawn, including inside another widget. tc-usermenu shipped an empty
// tc-badge for an unset tag, and the badge check never saw it because it only
// read entries named "tc-badge" (В-19).
var compositeInvariants = map[string]bool{
	"inv-badge-has-text":            true,
	"inv-badge-not-link":            true,
	"inv-railsec-untitled-no-title": true,
}

// composedHTML returns every catalog entry in which the widget is drawn,
// whoever owns the entry. The class is matched at its boundary so that
// tc-rail does not swallow tc-railsec.
func composedHTML(catalog []CatalogEntry, widget string) []string {
	var html []string
	for _, e := range catalog {
		if hasClass(e.HTML, widget) {
			html = append(html, e.HTML)
		}
	}
	return html
}

func hasClass(html, widget string) bool {
	needle := `class="` + widget
	for i := 0; ; {
		j := strings.Index(html[i:], needle)
		if j < 0 {
			return false
		}
		i += j + len(needle)
		if i >= len(html) {
			return false
		}
		switch c := html[i]; c {
		case '"', ' ', '-':
			return true
		}
	}
}

func forEachHTML(widget string, html []string, fn func(h string) error) error {
	if len(html) == 0 {
		return fmt.Errorf("%s: no catalog HTML for invariant check", widget)
	}
	for _, h := range html {
		if err := fn(h); err != nil {
			return err
		}
	}
	return nil
}

func checkSegOneChosen(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Count(h, `aria-pressed="true"`) != 1 {
			return fmt.Errorf("%s: seg must have exactly one chosen option", widget)
		}
		return nil
	})
}

func checkCrumbsLastNotLink(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !crumbsCurrentRe.MatchString(h) {
			return fmt.Errorf("%s: crumbs last item must not be a link", widget)
		}
		return nil
	})
}

func checkRangeOrder(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "is-invalid") {
			return fmt.Errorf("%s: range must not be invalid in catalog", widget)
		}
		return nil
	})
}

func checkTableNumberColumns(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-table__th--number") && !strings.Contains(h, "tc-table__td--number") {
			return fmt.Errorf("%s: number columns must align in header and body", widget)
		}
		return nil
	})
}

func checkInvStatStringValue(widget string, html []string, widgetDir string) error {
	if err := checkStatValueIsString(widgetDir); err != nil {
		return err
	}
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, `class="tc-stat__value"`) {
			return fmt.Errorf("%s: stat must render pre-formatted value in tc-stat__value", widget)
		}
		if !strings.Contains(h, ">1,234<") {
			return fmt.Errorf("%s: stat value must be passed through as a string", widget)
		}
		return nil
	})
}

func checkInvStatsNoWrap(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, `class="tc-stats"`) {
			return fmt.Errorf("%s: stats must use tc-stats root", widget)
		}
		if strings.Count(h, `class="tc-stat"`) < 2 {
			return fmt.Errorf("%s: stats must contain tc-stat items", widget)
		}
		return nil
	})
}

func checkInvAvatarNotEmpty(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-avatar__empty") {
			return fmt.Errorf("%s: avatar must not be empty", widget)
		}
		return nil
	})
}

func checkInvAvatarSquare(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, `class="tc-avatar"`) && !strings.Contains(h, `class="tc-avatar `) {
			return fmt.Errorf("%s: avatar must use tc-avatar root", widget)
		}
		return nil
	})
}

func checkInvBadgeHasText(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, `class="tc-badge`) {
			return fmt.Errorf("%s: badge must have visible text", widget)
		}
		// Каждый бейдж, а не только первый: пустой мог прийти из составного
		// виджета вторым или третьим.
		for rest := h; ; {
			start := strings.Index(rest, `class="tc-badge`)
			if start < 0 {
				return nil
			}
			rest = rest[start:]
			open := strings.Index(rest, ">")
			end := strings.Index(rest, "</span>")
			if open < 0 || end <= open+1 || strings.TrimSpace(rest[open+1:end]) == "" {
				return fmt.Errorf("%s: badge must have visible text", widget)
			}
			rest = rest[end:]
		}
	})
}

func checkInvBadgeNotLink(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, `<a`) && strings.Contains(h, "tc-badge") {
			return fmt.Errorf("%s: badge must not be a link", widget)
		}
		return nil
	})
}

func checkInvBrandSingleMark(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-brand__mark") {
			return fmt.Errorf("%s: brand must render mark", widget)
		}
		return nil
	})
}

func checkInvBrandNoDupWord(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-brand--mark-only") {
			return nil
		}
		mark := extractBetween(h, `tc-brand__mark`, "</span>")
		word := extractBetween(h, `tc-brand__word`, "</span>")
		if mark != "" && mark == word {
			return fmt.Errorf("%s: brand mark and word must not duplicate", widget)
		}
		return nil
	})
}

func checkInvCardHasBody(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-card__body") {
			return fmt.Errorf("%s: card must have tc-card__body", widget)
		}
		return nil
	})
}

func checkInvChipStateClass(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "is-on") {
			return nil
		}
		if strings.Contains(h, "tc-chip") && !strings.Contains(h, "is-on") && strings.Contains(h, "Inactive") {
			return nil
		}
		if strings.Contains(h, "tc-chip") && strings.Contains(h, "Active") && !strings.Contains(h, "is-on") {
			return fmt.Errorf("%s: on chip must have is-on class", widget)
		}
		return nil
	})
}

func checkInvCropHasFrame(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-crop__frame") {
			return fmt.Errorf("%s: crop must show selection frame", widget)
		}
		return nil
	})
}

func checkInvCropHasZoom(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-crop__controls") || !strings.Contains(h, `name="zoom"`) {
			return fmt.Errorf("%s: crop must expose zoom control", widget)
		}
		return nil
	})
}

func checkInvCrumbsHasRoot(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-crumbs__link") {
			return fmt.Errorf("%s: crumbs must keep root link when collapsed", widget)
		}
		return nil
	})
}

func checkInvDiagCollapsedDefault(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "is-expanded") && !strings.Contains(h, " open") {
			return fmt.Errorf("%s: expanded diag must use open attribute", widget)
		}
		return nil
	})
}

func checkInvDiagMono(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-diag__body") {
			return fmt.Errorf("%s: diag body must be monospace pre", widget)
		}
		return nil
	})
}

func checkInvDiffOptionsColumn(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-diff__options") {
			return fmt.Errorf("%s: diff options must stack in a column", widget)
		}
		return nil
	})
}

func checkInvDiffOneChosen(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Count(h, `checked`) > 1 {
			return fmt.Errorf("%s: diff must have at most one chosen option", widget)
		}
		return nil
	})
}

func checkInvDocModeSwitch(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-doc--reading") && !strings.Contains(h, "tc-doc--editing") {
			return fmt.Errorf("%s: doc must declare reading or editing mode", widget)
		}
		return nil
	})
}

func checkInvDocNoSilentSave(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-docbar") {
			return fmt.Errorf("%s: doc must include visible docbar", widget)
		}
		return nil
	})
}

func checkInvDocbarActions(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-btn") && !strings.Contains(h, "tc-bar") {
			return fmt.Errorf("%s: docbar actions must use tc-bar", widget)
		}
		return nil
	})
}

func checkInvDocbarDirtyFlag(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "is-dirty") && !strings.Contains(h, "tc-docbar__dirty") {
			return fmt.Errorf("%s: dirty docbar must show dirty flag", widget)
		}
		return nil
	})
}

func checkInvDocmetaSections(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-docmeta__section") {
			return fmt.Errorf("%s: docmeta must use sections", widget)
		}
		return nil
	})
}

func checkInvFieldsDLStructure(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, `<dl class="tc-fields"`) {
			return fmt.Errorf("%s: fields must use dl.tc-fields", widget)
		}
		return nil
	})
}

func checkInvFsetRubric(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-fset__legend") && !strings.Contains(h, "tc-rubric") {
			return fmt.Errorf("%s: fieldset must have legend rubric", widget)
		}
		return nil
	})
}

func checkInvGridTiles(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-tile") {
			return fmt.Errorf("%s: grid must contain tiles", widget)
		}
		return nil
	})
}

func checkInvGridUniform(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-grid") {
			return nil
		}
		parts := strings.Split(h, `class="tc-grid">`)
		if len(parts) < 2 {
			return nil
		}
		inner := parts[1]
		if end := strings.Index(inner, "</div>"); end > 0 {
			inner = inner[:end]
		}
		inner = strings.TrimSpace(inner)
		if inner != "" && !strings.Contains(inner, "tc-tile") {
			return fmt.Errorf("%s: grid children must be uniform tc-tile", widget)
		}
		return nil
	})
}

func checkInvHeadTitle(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, `class="tc-head__title"`) {
			return fmt.Errorf("%s: head must render title", widget)
		}
		return nil
	})
}

func checkInvKbdNotButton(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "<button") && strings.Contains(h, "tc-kbd") {
			return fmt.Errorf("%s: kbd must not be a button", widget)
		}
		if !strings.Contains(h, "tc-kbd") {
			return fmt.Errorf("%s: kbd must use tc-kbd root", widget)
		}
		return nil
	})
}

func checkInvListEmptyAction(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "tc-list--empty") && !strings.Contains(h, "tc-empty") {
			return fmt.Errorf("%s: empty list must show tc-empty with action", widget)
		}
	}
	return nil
}

func checkInvMainBody(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-main__body") {
			return fmt.Errorf("%s: main must have scrollable body", widget)
		}
		return nil
	})
}

func checkInvMeterHasValue(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-meter__label") && !strings.Contains(h, "aria-valuenow") {
			return fmt.Errorf("%s: meter must show numeric progress", widget)
		}
		return nil
	})
}

func checkInvNavOneCurrent(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Count(h, "is-current") != 1 {
			return fmt.Errorf("%s: nav must mark exactly one current item", widget)
		}
		return nil
	})
}

func checkInvNumUnknownHidden(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-num--unknown") && !strings.Contains(h, `aria-hidden="true"`) {
			return fmt.Errorf("%s: unknown num must be aria-hidden", widget)
		}
		return nil
	})
}

func checkInvOverlayKindClass(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-overlay--") {
			return fmt.Errorf("%s: overlay must declare kind class", widget)
		}
		return nil
	})
}

func checkInvQueueQuestion(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "tc-queue__question") {
			if !strings.Contains(h, "tc-queue__row--waiting") && !strings.Contains(h, "tc-btn") {
				return fmt.Errorf("%s: queue question must offer choices", widget)
			}
		}
	}
	return nil
}

func checkInvQueueMeter(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "tc-queue__row--running") && !strings.Contains(h, "tc-meter") {
			return fmt.Errorf("%s: running queue row must show meter", widget)
		}
	}
	return nil
}

func checkInvRailFoot(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-railfoot") {
			return fmt.Errorf("%s: rail must include foot", widget)
		}
		return nil
	})
}

func checkInvRailDrawerClass(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "is-open") && strings.Contains(h, "data-tc-rail") && !strings.Contains(h, "tc-rail--drawer") {
			return fmt.Errorf("%s: open drawer rail must use tc-rail--drawer", widget)
		}
	}
	return nil
}

func checkInvRailfootBuild(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		actions := strings.Index(h, "tc-railfoot__actions")
		build := strings.Index(h, "tc-railfoot__build")
		if build >= 0 && actions >= 0 && build < actions {
			return fmt.Errorf("%s: build label must follow actions", widget)
		}
		return nil
	})
}

func checkInvRailfootActions(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-railfoot__actions") {
			return fmt.Errorf("%s: railfoot must expose actions row", widget)
		}
		return nil
	})
}

func checkInvRailsecState(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "tc-railsec--muted") || strings.Contains(h, "tc-railsec--alert") {
			return nil
		}
	}
	// ok state or empty — at least one entry should have state class when catalog includes muted/alert
	for _, h := range html {
		if strings.Contains(h, "Muted") && !strings.Contains(h, "tc-railsec--muted") {
			return fmt.Errorf("%s: muted rail section must use state class", widget)
		}
		if strings.Contains(h, "Alert") && strings.Contains(h, "tc-railsec") && !strings.Contains(h, "tc-railsec--alert") {
			return fmt.Errorf("%s: alert rail section must use state class", widget)
		}
	}
	return nil
}

func checkInvRailsecUntitledNoTitle(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, `<h3 class="tc-railsec__title"></h3>`) {
			return fmt.Errorf("%s: empty title must not render tc-railsec__title", widget)
		}
		return nil
	})
}

func checkInvRangeFieldWidth(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-range__fields") {
			return fmt.Errorf("%s: range must expose from/to fields", widget)
		}
		return nil
	})
}

func checkInvRowMark(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-mark") {
			mark := strings.Index(h, "tc-mark")
			cells := strings.Index(h, "tc-row__cells")
			if cells >= 0 && mark > cells {
				return fmt.Errorf("%s: row mark must be left of cells", widget)
			}
		}
		return nil
	})
}

func checkInvRowTail(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-row") && !strings.Contains(h, "tc-row__tail") {
			return fmt.Errorf("%s: row must have tail column", widget)
		}
		return nil
	})
}

func checkInvRowMenuInTail(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-menu") {
			tail := strings.Index(h, "tc-row__tail")
			menu := strings.Index(h, "tc-menu")
			if tail < 0 || menu < tail {
				return fmt.Errorf("%s: row menu must live in tail", widget)
			}
		}
		return nil
	})
}

func checkInvRubricNotHeading(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "<h1") || strings.Contains(h, "<h2") || strings.Contains(h, "<h3") {
			return fmt.Errorf("%s: rubric must not use heading tags", widget)
		}
		if !strings.Contains(h, "tc-rubric") {
			return fmt.Errorf("%s: rubric must use tc-rubric", widget)
		}
		return nil
	})
}

func checkInvRunbarMeterHiddenIdle(widget string, html []string, _ string) error {
	for _, h := range html {
		if !strings.Contains(h, "is-running") && strings.Contains(h, "tc-meter") {
			return fmt.Errorf("%s: idle runbar must hide meter", widget)
		}
	}
	return nil
}

func checkInvRunbarStates(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "Syncing") && !strings.Contains(h, "is-running") {
			return fmt.Errorf("%s: running runbar must have is-running", widget)
		}
	}
	return nil
}

func checkInvScrimForAttr(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "data-tc-for=") {
			return fmt.Errorf("%s: scrim must name its overlay owner", widget)
		}
		return nil
	})
}

func checkInvSearchPopoverBelow(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "is-suggesting") && !strings.Contains(h, "tc-search__popover") {
			return fmt.Errorf("%s: search popover must sit below field", widget)
		}
		return nil
	})
}

func checkInvSearchNoEmptySubmit(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, `value=""`) && strings.Contains(h, `type="submit"`) && !strings.Contains(h, "is-suggesting") {
			return fmt.Errorf("%s: empty search must not submit", widget)
		}
		return nil
	})
}

func checkInvSecLikeCard(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-sec__body") {
			return fmt.Errorf("%s: section must have body like tc-card", widget)
		}
		return nil
	})
}

func checkInvSegNarrowMenu(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "data-tc-seg") {
			return fmt.Errorf("%s: seg options must carry data-tc-seg", widget)
		}
		return nil
	})
}

func checkInvShellFourZones(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		for _, zone := range []string{"tc-top", "tc-shell__rail", "tc-shell__main", "tc-shell__side"} {
			if !strings.Contains(h, zone) {
				return fmt.Errorf("%s: shell must have four zones (missing %s)", widget, zone)
			}
		}
		return nil
	})
}

func checkInvSkelShape(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-skel--") {
			return fmt.Errorf("%s: skel must declare shape class", widget)
		}
		return nil
	})
}

func checkInvStepsOneCurrent(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Count(h, "is-current") != 1 {
			return fmt.Errorf("%s: steps must mark exactly one current step", widget)
		}
		return nil
	})
}

func checkInvStepsCount(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-steps__count") {
			return fmt.Errorf("%s: steps must show step count", widget)
		}
		return nil
	})
}

func checkInvTableColumnKind(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-table__grid") && !strings.Contains(h, "tc-table__th--") {
			return fmt.Errorf("%s: table columns must declare kind", widget)
		}
		return nil
	})
}

func checkInvTableScrollable(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "tc-table__grid") && !strings.Contains(h, `class="tc-table"`) {
			return fmt.Errorf("%s: table must scroll inside tc-table wrapper", widget)
		}
		return nil
	})
}

func checkInvTagNotEditable(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Contains(h, "contenteditable") {
			return fmt.Errorf("%s: tag must not be editable in place", widget)
		}
		return nil
	})
}

func checkInvThumbPlaceholder(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-thumb") {
			return fmt.Errorf("%s: thumb must always occupy space", widget)
		}
		return nil
	})
}

func checkInvThumbIconFallback(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "tc-thumb--icon") && !strings.Contains(h, "tc-thumb__icon") {
			return fmt.Errorf("%s: icon thumb must render fallback icon", widget)
		}
	}
	return nil
}

func checkInvTickrowCountOptional(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "tc-tickrow") && strings.Contains(h, "tc-num--unknown") && !strings.Contains(h, `aria-hidden="true"`) {
			return fmt.Errorf("%s: absent tickrow count must hide unknown num", widget)
		}
	}
	return nil
}

func checkInvTileHasName(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-tile__name") {
			return fmt.Errorf("%s: tile must show name", widget)
		}
		return nil
	})
}

func checkInvTopBrandOnce(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Count(h, "tc-brand__mark") != 1 {
			return fmt.Errorf("%s: top must render brand exactly once", widget)
		}
		return nil
	})
}

func checkInvTopSearch(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if !strings.Contains(h, "tc-search") {
			return fmt.Errorf("%s: top must always show search", widget)
		}
		return nil
	})
}

func checkInvTreeOneChosen(widget string, html []string, _ string) error {
	return forEachHTML(widget, html, func(h string) error {
		if strings.Count(h, "is-chosen") > 1 {
			return fmt.Errorf("%s: tree must have at most one chosen node", widget)
		}
		return nil
	})
}

func checkInvUsermenuOpenState(widget string, html []string, _ string) error {
	for _, h := range html {
		if strings.Contains(h, "is-open") && !strings.Contains(h, `aria-expanded="true"`) {
			return fmt.Errorf("%s: open usermenu must expose expanded state", widget)
		}
	}
	return nil
}

func extractBetween(h, startMarker, endMarker string) string {
	start := strings.Index(h, startMarker)
	if start < 0 {
		return ""
	}
	rest := h[start:]
	open := strings.Index(rest, ">")
	if open < 0 {
		return ""
	}
	close := strings.Index(rest[open:], endMarker)
	if close < 0 {
		return ""
	}
	return strings.TrimSpace(rest[open+1 : open+close])
}
