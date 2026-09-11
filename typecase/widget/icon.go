package widget

// IconOnly reports whether a sprite id renders without a visible label.
func IconOnly(name string) bool {
	switch name {
	case "tc-i-close", "tc-i-plus", "tc-i-search", "tc-i-more", "tc-i-expand",
		"tc-i-refresh", "tc-i-back", "tc-i-print", "tc-i-panel", "tc-i-chevron",
		"tc-i-chevron-right", "tc-i-server", "tc-i-link", "tc-i-cols", "tc-i-pencil",
		"tc-i-up", "tc-i-down":
		return true
	default:
		return false
	}
}

func renderIcon(name string) string {
	return renderIconClass(name, "tc-icon tc-btn__icon")
}

func renderNavIcon(name string) string {
	return renderIconClass(name, "tc-icon tc-nav__icon-svg")
}

func renderIconClass(name, cls string) string {
	if name == "" {
		return ""
	}
	return `<svg class="` + cls + `" aria-hidden="true"><use href="#` + esc(name) + `"></use></svg>`
}
