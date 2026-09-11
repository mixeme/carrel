(function () {
	"use strict";

	var OPEN = "is-open";
	var trapSelector = "[data-tc-trap]";
	var menuSelector = "[data-tc-menu]";
	var popoverSelector = "[data-tc-menu-popover]";
	var scrimSelector = "[data-tc-scrim]";
	var closeSelector = "[data-tc-close]";
	var overlaySelector = "[data-tc-overlay]";

	function closest(el, sel) {
		while (el && el.nodeType === 1) {
			if (el.matches(sel)) return el;
			el = el.parentElement;
		}
		return null;
	}

	function focusables(root) {
		return root.querySelectorAll(
			'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
		);
	}

	function trapFocus(container) {
		var nodes = focusables(container);
		if (!nodes.length) return;
		var first = nodes[0];
		var last = nodes[nodes.length - 1];
		container.addEventListener("keydown", function (e) {
			if (e.key !== "Tab") return;
			if (e.shiftKey && document.activeElement === first) {
				e.preventDefault();
				last.focus();
			} else if (!e.shiftKey && document.activeElement === last) {
				e.preventDefault();
				first.focus();
			}
		});
		first.focus();
	}

	function openMenu(menuRoot) {
		var pop = menuRoot.querySelector(popoverSelector);
		var btn = menuRoot.querySelector(menuSelector);
		if (!pop || !btn || menuRoot.classList.contains("is-disabled")) return;
		menuRoot.classList.add(OPEN);
		btn.setAttribute("aria-expanded", "true");
		menuRoot._tcTrigger = document.activeElement;
		var item = pop.querySelector('[role="menuitem"]');
		if (item) item.focus();
	}

	function closeMenu(menuRoot) {
		if (!menuRoot) return;
		var btn = menuRoot.querySelector(menuSelector);
		menuRoot.classList.remove(OPEN);
		if (btn) btn.setAttribute("aria-expanded", "false");
		if (menuRoot._tcTrigger) {
			menuRoot._tcTrigger.focus();
			menuRoot._tcTrigger = null;
		}
	}

	function closeOverlays(except) {
		document.querySelectorAll(".tc-menu.is-open, .tc-usermenu.is-open").forEach(function (m) {
			if (m !== except) closeMenu(m);
		});
		document.querySelectorAll(overlaySelector).forEach(function (ov) {
			if (ov === except) return;
			ov.hidden = true;
			ov.classList.remove(OPEN);
		});
		document.querySelectorAll(scrimSelector + ".is-on").forEach(function (s) {
			if (except && s.getAttribute("data-tc-for") === except.id) return;
			s.classList.remove("is-on");
			s.hidden = true;
		});
	}

	function onDocClick(e) {
		var menuBtn = closest(e.target, menuSelector);
		if (menuBtn) {
			e.preventDefault();
			var root = closest(menuBtn, ".tc-menu, .tc-usermenu");
			if (!root) return;
			var open = root.classList.contains(OPEN);
			closeOverlays(root);
			if (!open) openMenu(root);
			return;
		}
		if (!closest(e.target, popoverSelector)) {
			closeOverlays(null);
		}
		var scrim = closest(e.target, scrimSelector);
		if (scrim && scrim.classList.contains("is-on") && !scrim.hasAttribute("data-tc-must-answer")) {
			var id = scrim.getAttribute("data-tc-for");
			var ov = id ? document.getElementById(id) : null;
			closeOverlays(null);
			if (ov && ov._tcTrigger) ov._tcTrigger.focus();
		}
	}

	function onKeydown(e) {
		if (e.key !== "Escape") return;
		var inInput = closest(e.target, "input, textarea, select");
		if (inInput) return;
		var openMenuEl = document.querySelector(".tc-menu.is-open, .tc-usermenu.is-open");
		if (openMenuEl) {
			e.preventDefault();
			closeMenu(openMenuEl);
			return;
		}
		var overlays = document.querySelectorAll(overlaySelector + ":not([hidden])");
		if (!overlays.length) return;
		var top = overlays[overlays.length - 1];
		e.preventDefault();
		top.hidden = true;
		top.classList.remove(OPEN);
		var scrimFor = top.id;
		document.querySelectorAll(scrimSelector).forEach(function (s) {
			if (s.getAttribute("data-tc-for") === scrimFor) {
				s.classList.remove("is-on");
				s.hidden = true;
			}
		});
		if (top._tcTrigger) top._tcTrigger.focus();
	}

	function onCloseClick(e) {
		var btn = closest(e.target, closeSelector);
		if (!btn) return;
		var id = btn.getAttribute("data-tc-for");
		if (!id) return;
		var ov = document.getElementById(id);
		closeOverlays(null);
		if (ov && ov._tcTrigger) ov._tcTrigger.focus();
	}

	function initRoots() {
		document.querySelectorAll("[data-tc-root]").forEach(function (root) {
			root.classList.add("tc-root");
		});
		document.querySelectorAll(trapSelector).forEach(function (el) {
			var parent = closest(el, overlaySelector) || el;
			if (parent && !parent.hidden) trapFocus(el);
		});
	}

	document.addEventListener("DOMContentLoaded", function () {
		initRoots();
		document.addEventListener("click", onDocClick);
		document.addEventListener("keydown", onKeydown);
		document.addEventListener("click", onCloseClick);
	});
})();
