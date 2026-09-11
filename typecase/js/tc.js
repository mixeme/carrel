(function () {
	"use strict";

	var OPEN = "is-open";
	var trapSelector = "[data-tc-trap]";
	var menuSelector = "[data-tc-menu]";
	var popoverSelector = "[data-tc-menu-popover]";
	var scrimSelector = "[data-tc-scrim]";
	var closeSelector = "[data-tc-close]";
	var overlaySelector = "[data-tc-overlay]";

	var keyboardMenuOpen = false;

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

	function attachTrap(container, autofocus) {
		var nodes = focusables(container);
		if (!nodes.length) return function () {};
		var first = nodes[0];
		var last = nodes[nodes.length - 1];
		function onKey(e) {
			if (e.key !== "Tab") return;
			if (e.shiftKey && document.activeElement === first) {
				e.preventDefault();
				last.focus();
			} else if (!e.shiftKey && document.activeElement === last) {
				e.preventDefault();
				first.focus();
			}
		}
		container.addEventListener("keydown", onKey);
		if (autofocus) first.focus();
		return function release() {
			container.removeEventListener("keydown", onKey);
		};
	}

	function releaseOverlayTrap(overlay) {
		if (overlay && overlay._tcReleaseTrap) {
			overlay._tcReleaseTrap();
			overlay._tcReleaseTrap = null;
		}
	}

	function syncOverlayTraps() {
		document.querySelectorAll(overlaySelector).forEach(function (ov) {
			var trapEl = ov.querySelector(trapSelector);
			if (!trapEl) return;
			if (ov.hidden) {
				releaseOverlayTrap(ov);
				return;
			}
			if (!ov._tcReleaseTrap) {
				ov._tcReleaseTrap = attachTrap(trapEl, !!ov._tcOpenedByUser);
				ov._tcOpenedByUser = false;
			}
		});
	}

	function showOverlay(overlay, byUser) {
		if (!overlay) return;
		releaseOverlayTrap(overlay);
		overlay.hidden = false;
		overlay.classList.add(OPEN);
		if (byUser) {
			overlay._tcOpenedByUser = true;
			overlay._tcTrigger = document.activeElement;
		}
		syncOverlayTraps();
	}

	function positionPopover(menuRoot) {
		var pop = menuRoot.querySelector(popoverSelector);
		var btn = menuRoot.querySelector(menuSelector);
		if (!pop || !btn) return;
		pop.classList.remove("is-flip-up");
		var prev = pop.style.display;
		pop.style.display = "block";
		pop.style.visibility = "hidden";
		var btnRect = btn.getBoundingClientRect();
		var popRect = pop.getBoundingClientRect();
		pop.style.display = prev || "";
		pop.style.visibility = "";
		if (btnRect.bottom + popRect.height + 4 > window.innerHeight) {
			pop.classList.add("is-flip-up");
		}
	}

	function openMenu(menuRoot, fromKeyboard) {
		var pop = menuRoot.querySelector(popoverSelector);
		var btn = menuRoot.querySelector(menuSelector);
		if (!pop || !btn || menuRoot.classList.contains("is-disabled")) return;
		menuRoot.classList.add(OPEN);
		btn.setAttribute("aria-expanded", "true");
		menuRoot._tcTrigger = document.activeElement;
		menuRoot._tcOpenedByKeyboard = !!fromKeyboard;
		positionPopover(menuRoot);
		var item = pop.querySelector('[role="menuitem"]');
		if (item) item.focus();
	}

	function closeMenu(menuRoot, restoreFocus) {
		if (!menuRoot) return;
		var btn = menuRoot.querySelector(menuSelector);
		menuRoot.classList.remove(OPEN);
		if (btn) btn.setAttribute("aria-expanded", "false");
		if ((restoreFocus || menuRoot._tcOpenedByKeyboard) && menuRoot._tcTrigger) {
			menuRoot._tcTrigger.focus();
			menuRoot._tcTrigger = null;
		}
		menuRoot._tcOpenedByKeyboard = false;
	}

	function closeSide(side) {
		if (!side) return;
		side.classList.remove(OPEN);
		if (side._tcTrigger) {
			side._tcTrigger.focus();
			side._tcTrigger = null;
		}
	}

	function closeOverlays(except) {
		document.querySelectorAll(".tc-menu.is-open, .tc-usermenu.is-open").forEach(function (m) {
			if (m !== except) closeMenu(m);
		});
		document.querySelectorAll(overlaySelector).forEach(function (ov) {
			if (ov === except) return;
			releaseOverlayTrap(ov);
			ov.hidden = true;
			ov.classList.remove(OPEN);
		});
		document.querySelectorAll(scrimSelector + ".is-on").forEach(function (s) {
			if (except && s.getAttribute("data-tc-for") === except.id) return;
			s.classList.remove("is-on");
			s.hidden = true;
		});
	}

	function menuItems(pop) {
		return Array.prototype.slice.call(
			pop.querySelectorAll('[role="menuitem"]:not(.is-disabled)')
		);
	}

	function focusMenuItem(pop, index) {
		var items = menuItems(pop);
		if (!items.length) return;
		var i = Math.max(0, Math.min(index, items.length - 1));
		items[i].focus();
	}

	function openMenuRoot() {
		return document.querySelector(".tc-menu.is-open, .tc-usermenu.is-open");
	}

	function onMenuKeydown(e) {
		var root = openMenuRoot();
		if (!root) return;
		var pop = root.querySelector(popoverSelector);
		if (!pop) return;
		var items = menuItems(pop);
		if (!items.length) return;
		var idx = items.indexOf(document.activeElement);
		if (e.key === "ArrowDown") {
			e.preventDefault();
			focusMenuItem(pop, idx < 0 ? 0 : idx + 1);
			return;
		}
		if (e.key === "ArrowUp") {
			e.preventDefault();
			focusMenuItem(pop, idx < 0 ? items.length - 1 : idx - 1);
			return;
		}
		if (e.key === "Home") {
			e.preventDefault();
			focusMenuItem(pop, 0);
			return;
		}
		if (e.key === "End") {
			e.preventDefault();
			focusMenuItem(pop, items.length - 1);
			return;
		}
		if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
			var ch = e.key.toLowerCase();
			var start = idx < 0 ? 0 : idx + 1;
			for (var n = 0; n < items.length; n++) {
				var j = (start + n) % items.length;
				var text = items[j].textContent.trim().toLowerCase();
				if (text.charAt(0) === ch) {
					e.preventDefault();
					items[j].focus();
					break;
				}
			}
		}
	}

	function onDocClick(e) {
		var menuBtn = closest(e.target, menuSelector);
		if (menuBtn) {
			e.preventDefault();
			var root = closest(menuBtn, ".tc-menu, .tc-usermenu");
			if (!root) return;
			var open = root.classList.contains(OPEN);
			closeOverlays(root);
			if (!open) openMenu(root, keyboardMenuOpen);
			keyboardMenuOpen = false;
			return;
		}
		var menuItem = closest(e.target, '[role="menuitem"]');
		if (menuItem) {
			var menuRoot = closest(menuItem, ".tc-menu, .tc-usermenu");
			if (menuRoot) closeMenu(menuRoot, true);
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
		if (e.key === "Enter" || e.key === " ") {
			if (closest(e.target, menuSelector)) {
				keyboardMenuOpen = true;
			}
		}
		onMenuKeydown(e);
		if (e.key !== "Escape") return;
		var inInput = closest(e.target, "input, textarea, select");
		if (inInput) return;
		var openMenuEl = openMenuRoot();
		if (openMenuEl) {
			e.preventDefault();
			closeMenu(openMenuEl, true);
			return;
		}
		var openSide = document.querySelector("[data-tc-side].is-open");
		if (openSide) {
			e.preventDefault();
			closeSide(openSide);
			return;
		}
		var overlays = document.querySelectorAll(overlaySelector + ":not([hidden])");
		if (!overlays.length) return;
		var top = overlays[overlays.length - 1];
		e.preventDefault();
		releaseOverlayTrap(top);
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
		var target = document.getElementById(id);
		if (target && target.hasAttribute("data-tc-side")) {
			closeSide(target);
			return;
		}
		closeOverlays(null);
		if (target && target._tcTrigger) target._tcTrigger.focus();
	}

	function applyCropTransform(img) {
		var z = img.getAttribute("data-tc-zoom") || "100";
		var r = img.getAttribute("data-tc-rotate") || "0";
		var x = img.getAttribute("data-tc-offset-x") || "0";
		var y = img.getAttribute("data-tc-offset-y") || "0";
		img.style.transform =
			"rotate(" + r + "deg) scale(" + z / 100 + ") translate(" + x + "px, " + y + "px)";
	}

	function initDynamic() {
		// Цвет метки приходит значением. Встроенных стилей в разметке не бывает
		// (ворота no-inline), поэтому значение ставит сценарий; без него метка
		// остаётся непокрашенной, а не берёт цвет из палитры темы.
		document.querySelectorAll("[data-tc-colour]").forEach(function (el) {
			var c = el.getAttribute("data-tc-colour");
			if (c) el.style.backgroundColor = c;
		});
		document.querySelectorAll("[data-tc-pct]").forEach(function (el) {
			var bar = el.querySelector(".tc-meter__bar");
			var pct = el.getAttribute("data-tc-pct");
			if (bar && pct) bar.style.width = pct + "%";
		});
		document.querySelectorAll("[data-tc-width]").forEach(function (el) {
			var w = el.getAttribute("data-tc-width");
			if (w) el.style.width = w;
		});
		document.querySelectorAll(".tc-crop__img").forEach(applyCropTransform);
	}

	function initRoots() {
		// Базовые стили висят на самом атрибуте data-tc-root, поэтому класс
		// вешать не нужно: без сценария страница выглядит так же.
		syncOverlayTraps();
	}

	function syncTreeChildren(toggle) {
		var item = closest(toggle, "[data-tc-tree-item]");
		if (!item) return;
		var children = item.querySelector(":scope > .tc-tree__children");
		if (!children) return;
		var expanded = toggle.getAttribute("aria-expanded") === "true";
		children.hidden = !expanded;
	}

	function initTree(tree) {
		tree.querySelectorAll("[data-tc-tree-toggle]").forEach(function (toggle) {
			syncTreeChildren(toggle);
		});
	}

	function updateShell(shell) {
		shell.classList.toggle("tc-shell--narrow", shell.clientWidth < 640);
	}

	function updateBar(bar) {
		bar.classList.toggle("is-collapsed", bar.scrollWidth > bar.clientWidth + 1);
	}

	function parseDateInput(input) {
		if (!input || !input.value) return null;
		var parts = input.value.split("-");
		if (parts.length !== 3) return null;
		var d = new Date(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2]));
		return isNaN(d.getTime()) ? null : d;
	}

	function validateRange(range) {
		var inputs = range.querySelectorAll("input");
		if (inputs.length < 2) return;
		var from = parseDateInput(inputs[0]);
		var to = parseDateInput(inputs[1]);
		range.classList.remove("is-invalid");
		if (from && to && from > to) {
			range.classList.add("is-invalid");
		}
	}

	function initBehaviors() {
		document.querySelectorAll("[data-tc-search-clear]").forEach(function (btn) {
			btn.addEventListener("click", function () {
				var field = closest(btn, ".tc-search__field");
				if (!field) return;
				var input = field.querySelector("input");
				if (input) {
					input.value = "";
					input.focus();
				}
			});
		});

		document.querySelectorAll("[data-tc-search]").forEach(function (form) {
			var input = form.querySelector(".tc-search__input");
			if (!input) return;
			input.addEventListener("focus", function () {
				form.classList.add("is-suggesting");
			});
			input.addEventListener("blur", function () {
				window.setTimeout(function () {
					if (!form.contains(document.activeElement)) {
						form.classList.remove("is-suggesting");
					}
				}, 0);
			});
		});

		document.querySelectorAll("[data-tc-tree]").forEach(initTree);

		document.addEventListener("click", function (e) {
			var toggle = closest(e.target, "[data-tc-tree-toggle]");
			if (toggle) {
				var expanded = toggle.getAttribute("aria-expanded") === "true";
				toggle.setAttribute("aria-expanded", expanded ? "false" : "true");
				syncTreeChildren(toggle);
				return;
			}
			var label = closest(e.target, "[data-tc-tree-item] .tc-tree__label");
			if (label && !label.disabled) {
				var node = closest(label, "[data-tc-tree-item]");
				if (!node || node.classList.contains("is-disabled")) return;
				var tree = closest(node, "[data-tc-tree]");
				if (!tree) return;
				tree.querySelectorAll(".tc-tree__node.is-chosen").forEach(function (n) {
					n.classList.remove("is-chosen");
				});
				var parent = closest(node, ".tc-tree__node");
				if (parent) parent.classList.add("is-chosen");
			}
		});

		document.querySelectorAll("[data-tc-seg]").forEach(function (group) {
			function selectSeg(btn) {
				// Класс и озвучка переставляются вместе: вид держится на
				// is-chosen, и без него выбранных оказывалось двое — один
				// глазами, другой для скринридера.
				group.querySelectorAll("button[data-tc-seg]").forEach(function (b) {
					var on = b === btn;
					b.setAttribute("aria-pressed", on ? "true" : "false");
					b.classList.toggle("is-chosen", on);
				});
			}
			group.addEventListener("click", function (e) {
				var btn = closest(e.target, "button[data-tc-seg]");
				if (!btn || !group.contains(btn)) return;
				selectSeg(btn);
			});
			group.addEventListener("keydown", function (e) {
				var buttons = Array.prototype.slice.call(group.querySelectorAll("button[data-tc-seg]"));
				if (!buttons.length) return;
				var idx = buttons.indexOf(document.activeElement);
				if (e.key === "ArrowRight" || e.key === "ArrowDown") {
					e.preventDefault();
					var next = buttons[Math.min(idx < 0 ? 0 : idx + 1, buttons.length - 1)];
					selectSeg(next);
					next.focus();
				} else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
					e.preventDefault();
					var prev = buttons[Math.max(idx < 0 ? 0 : idx - 1, 0)];
					selectSeg(prev);
					prev.focus();
				} else if (e.key === "Home") {
					e.preventDefault();
					selectSeg(buttons[0]);
					buttons[0].focus();
				} else if (e.key === "End") {
					e.preventDefault();
					selectSeg(buttons[buttons.length - 1]);
					buttons[buttons.length - 1].focus();
				}
			});
		});

		document.querySelectorAll("[data-tc-rail]").forEach(function (rail) {
			var toggle = rail.querySelector("[data-tc-rail-toggle]");
			function flipRail() {
				var open = rail.classList.toggle("is-open");
				rail.setAttribute("aria-expanded", open ? "true" : "false");
			}
			if (toggle) {
				toggle.addEventListener("click", flipRail);
			}
			var header = rail.querySelector(".tc-rail__chips");
			if (header) {
				header.addEventListener("dblclick", flipRail);
			}
		});

		document.querySelectorAll("[data-tc-shell]").forEach(function (shell) {
			updateShell(shell);
		});

		document.querySelectorAll("[data-tc-bar]").forEach(function (bar) {
			updateBar(bar);
		});

		document.querySelectorAll("[data-tc-diag]").forEach(function (diag) {
			var summary = diag.querySelector(".tc-diag__summary");
			if (!summary) return;
			summary.addEventListener("click", function () {
				diag.open = !diag.open;
			});
		});

		document.querySelectorAll("[data-tc-list]").forEach(function (list) {
			list.addEventListener("keydown", function (e) {
				if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
				var rows = Array.prototype.slice.call(list.querySelectorAll("[data-tc-row]"));
				if (!rows.length) return;
				var idx = rows.indexOf(closest(document.activeElement, "[data-tc-row]"));
				if (idx < 0) idx = 0;
				else if (e.key === "ArrowDown") idx = Math.min(idx + 1, rows.length - 1);
				else idx = Math.max(idx - 1, 0);
				e.preventDefault();
				var link = rows[idx].querySelector("a[href]") || rows[idx];
				if (link.focus) link.focus();
			});
		});

		document.querySelectorAll("[data-tc-range]").forEach(function (range) {
			range.querySelectorAll("input").forEach(function (input) {
				input.addEventListener("change", function () {
					validateRange(range);
				});
			});
			validateRange(range);
		});

		document.addEventListener("click", function (e) {
			var tick = closest(e.target, "[data-tc-tickrow] .tc-tickrow__tick");
			if (!tick || tick.disabled) return;
			tick.classList.toggle("is-on");
		});

		document.querySelectorAll("[data-tc-crop]").forEach(function (crop) {
			var img = crop.querySelector(".tc-crop__img");
			if (!img) return;
			crop.querySelectorAll("input").forEach(function (input) {
				input.addEventListener("input", function () {
					var val = input.value || "0";
					if (input.name === "zoom") img.setAttribute("data-tc-zoom", val);
					else if (input.name === "offsetX") img.setAttribute("data-tc-offset-x", val);
					else if (input.name === "offsetY") img.setAttribute("data-tc-offset-y", val);
					applyCropTransform(img);
				});
			});
			var rotateBtn = crop.querySelector(".tc-crop__controls .tc-btn");
			if (rotateBtn) {
				rotateBtn.addEventListener("click", function () {
					var r = parseInt(img.getAttribute("data-tc-rotate") || "0", 10);
					img.setAttribute("data-tc-rotate", String((r + 90) % 360));
					applyCropTransform(img);
				});
			}
		});

		window.addEventListener("resize", function () {
			document.querySelectorAll("[data-tc-shell]").forEach(updateShell);
			document.querySelectorAll("[data-tc-bar]").forEach(updateBar);
		});
	}

	document.addEventListener("DOMContentLoaded", function () {
		initRoots();
		initDynamic();
		initBehaviors();
		document.addEventListener("click", onDocClick);
		document.addEventListener("keydown", onKeydown);
		document.addEventListener("click", onCloseClick);
		window.__tc = { showOverlay: showOverlay };
	});
})();
