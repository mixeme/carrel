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
			var segPick = menuItem.getAttribute("data-tc-seg-pick");
			if (segPick) {
				e.preventDefault();
				var segGroup = closest(menuItem, ".tc-seg");
				if (segGroup) {
					var segBtn = segGroup.querySelector(
						'button[data-tc-seg][data-value="' + segPick + '"]'
					);
					if (segBtn) segBtn.click();
					var segMenu = segGroup.querySelector("[data-tc-seg-menu]");
					if (segMenu) syncSegMenuLabel(segGroup, segMenu);
				}
			}
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

	var tcSeq = 0;

	function tcUnique(prefix) {
		tcSeq += 1;
		return prefix + "-" + tcSeq;
	}

	function escText(s) {
		return String(s)
			.replace(/&/g, "&amp;")
			.replace(/</g, "&lt;")
			.replace(/"/g, "&quot;");
	}

	function chosenSegButton(group) {
		return (
			group.querySelector('button[data-tc-seg][aria-pressed="true"]') ||
			group.querySelector("button[data-tc-seg]")
		);
	}

	function syncSegMenuLabel(group, menu) {
		var btn = chosenSegButton(group);
		var label = menu.querySelector(".tc-menu__label");
		if (label && btn) label.textContent = btn.textContent.trim();
	}

	function buildMenuElement(id, label, items, extraAttrs) {
		var itemHTML = "";
		items.forEach(function (it) {
			if (it.separator) {
				itemHTML += '<li class="tc-menu__sep" role="separator"></li>';
				return;
			}
			var icls = "tc-menu__item";
			if (it.disabled) icls += " is-disabled";
			var pick = it.pick ? ' data-tc-seg-pick="' + escText(it.pick) + '"' : "";
			itemHTML +=
				'<li role="none"><a href="' +
				escText(it.href || "#") +
				'" class="' +
				icls +
				'" role="menuitem"' +
				pick +
				">" +
				escText(it.label) +
				"</a></li>";
		});
		var root = document.createElement("div");
		root.className = "tc-menu is-closed";
		if (extraAttrs) {
			extraAttrs.split(/\s+/).forEach(function (pair) {
				if (!pair) return;
				var kv = pair.split("=");
				root.setAttribute(kv[0], (kv[1] || "").replace(/^"|"$/g, ""));
			});
		}
		root.id = id;
		root.setAttribute("data-tc-overlay-anchor", "");
		root.innerHTML =
			'<button type="button" class="tc-menu__control" data-tc-menu data-tc-menu-id="' +
			escText(id) +
			'" aria-haspopup="menu" aria-expanded="false" aria-controls="' +
			escText(id) +
			'-popover"><span class="tc-menu__label">' +
			escText(label) +
			'</span><span class="tc-menu__arrow" aria-hidden="true"></span></button>' +
			'<ul class="tc-menu__popover tc-menu__popover--start" role="menu" data-tc-menu-popover id="' +
			escText(id) +
			'-popover">' +
			itemHTML +
			"</ul>";
		return root;
	}

	function createSegMenu(group, buttons) {
		var id = tcUnique("tc-seg-menu");
		var chosen = chosenSegButton(group);
		var items = Array.prototype.map.call(buttons, function (btn) {
			return {
				label: btn.textContent.trim(),
				href: "#",
				pick: btn.getAttribute("data-value") || "",
			};
		});
		var menu = buildMenuElement(id, chosen ? chosen.textContent.trim() : "", items);
		menu.setAttribute("data-tc-seg-menu", "");
		return menu;
	}

	function fitsViewport(el) {
		var vw = document.documentElement.clientWidth;
		var r = el.getBoundingClientRect();
		return r.right <= vw + 1 && r.left >= -1;
	}

	function updateSeg(group) {
		var buttons = group.querySelectorAll("button[data-tc-seg]");
		if (!buttons.length) return;
		var menu = group.querySelector("[data-tc-seg-menu]");
		var wasMenu = group.classList.contains("is-menu");
		if (wasMenu) {
			group.classList.remove("is-menu");
			if (menu) menu.hidden = true;
		}
		var overflow =
			group.scrollWidth > group.clientWidth + 1 || !fitsViewport(group);
		if (overflow) {
			group.classList.add("is-menu");
			if (!menu) {
				menu = createSegMenu(group, buttons);
				group.appendChild(menu);
			}
			menu.hidden = false;
			syncSegMenuLabel(group, menu);
		} else {
			group.classList.remove("is-menu");
			if (menu) menu.remove();
		}
	}

	function topMoreLabel(top) {
		return top.getAttribute("data-tc-more") || "More";
	}

	function topActions(top) {
		return top.querySelector(".tc-top__actions");
	}

	function restoreTop(top) {
		var actions = topActions(top);
		if (!actions) return;
		var stash = actions.querySelector("[data-tc-top-stash]");
		var overflow = actions.querySelector("[data-tc-top-overflow]");
		if (stash) {
			while (stash.firstChild) {
				actions.insertBefore(stash.firstChild, stash);
			}
		}
		if (overflow) overflow.hidden = true;
	}

	function menuItemsFromWidget(widget) {
		var items = [];
		widget.querySelectorAll(
			'.tc-menu__popover a[role="menuitem"], .tc-usermenu__list a[role="menuitem"]'
		).forEach(function (a) {
			if (a.classList.contains("is-disabled")) return;
			items.push({ label: a.textContent.trim(), href: a.getAttribute("href") || "#" });
		});
		return items;
	}

	function actionsFitViewport(actions) {
		var vw = document.documentElement.clientWidth;
		var r = actions.getBoundingClientRect();
		return r.right <= vw + 1 && r.left >= -1;
	}

	function topFits(top) {
		var vw = document.documentElement.clientWidth;
		var tr = top.getBoundingClientRect();
		if (tr.right > vw + 1 || tr.left < -1) return false;
		if (top.scrollWidth > top.clientWidth + 1) return false;
		var actions = topActions(top);
		if (actions) return actionsFitViewport(actions);
		return !Array.prototype.some.call(top.children, function (child) {
			if (child.hidden) return false;
			var r = child.getBoundingClientRect();
			if (r.width === 0 && r.height === 0) return false;
			return r.right > vw + 1 || r.left < -1;
		});
	}

	function updateTop(top) {
		restoreTop(top);
		var actions = topActions(top);
		if (!actions) return;
		if (topFits(top)) return;
		var stash = actions.querySelector("[data-tc-top-stash]");
		if (!stash) {
			stash = document.createElement("div");
			stash.setAttribute("data-tc-top-stash", "");
			stash.hidden = true;
			actions.appendChild(stash);
		}
		var movable = Array.prototype.filter.call(actions.children, function (c) {
			return !c.hasAttribute("data-tc-top-stash") && !c.hasAttribute("data-tc-top-overflow");
		});
		var shell = closest(top, "[data-tc-shell]");
		var narrow = shell && shell.classList.contains("tc-shell--narrow");
		if (narrow) {
			while (movable.length > 0) {
				stash.appendChild(movable.pop());
			}
		} else {
			while (movable.length > 0 && (!actionsFitViewport(actions) || !topFits(top))) {
				stash.appendChild(movable.pop());
			}
		}
		if (!stash.children.length) return;
		var items = [];
		Array.prototype.forEach.call(stash.children, function (w) {
			var sub = menuItemsFromWidget(w);
			if (!sub.length) return;
			if (items.length) items.push({ separator: true });
			items = items.concat(sub);
		});
		var overflow = actions.querySelector("[data-tc-top-overflow]");
		if (!overflow) {
			overflow = document.createElement("div");
			overflow.setAttribute("data-tc-top-overflow", "");
			actions.appendChild(overflow);
		}
		var menu = overflow.querySelector(".tc-menu");
		if (!menu) {
			menu = buildMenuElement(tcUnique("tc-top-more"), topMoreLabel(top), items);
			overflow.appendChild(menu);
		} else {
			var pop = menu.querySelector("[data-tc-menu-popover]");
			if (pop) {
				var itemHTML = "";
				items.forEach(function (it) {
					if (it.separator) {
						itemHTML += '<li class="tc-menu__sep" role="separator"></li>';
						return;
					}
					itemHTML +=
						'<li role="none"><a href="' +
						escText(it.href || "#") +
						'" class="tc-menu__item" role="menuitem">' +
						escText(it.label) +
						"</a></li>";
				});
				pop.innerHTML = itemHTML;
			}
			var label = menu.querySelector(".tc-menu__label");
			if (label) label.textContent = topMoreLabel(top);
		}
		overflow.hidden = false;
	}

	function shellNarrow(shell) {
		return shell.hasAttribute("data-tc-shell-narrow") || shell.clientWidth < 640;
	}

	function updateShell(shell) {
		var narrow = shellNarrow(shell);
		shell.classList.toggle("tc-shell--narrow", narrow);
		var rail = shell.querySelector(".tc-shell__rail [data-tc-rail]");
		var nav = shell.querySelector(".tc-shell__rail .tc-nav");
		if (rail) {
			var icons = rail.classList.contains("tc-rail--icons");
			rail.classList.toggle("tc-rail--drawer", narrow);
			if (narrow) {
				rail.classList.remove("tc-rail--column");
			} else {
				rail.classList.remove("tc-rail--drawer", "is-open");
				rail.setAttribute("aria-expanded", "false");
				if (!icons) {
					rail.classList.add("tc-rail--column");
				}
			}
		}
		if (nav) {
			var iconNav = nav.classList.contains("tc-nav--icons");
			nav.classList.toggle("tc-nav--bottom", narrow);
			if (narrow) {
				nav.classList.remove("tc-nav--column");
			} else {
				nav.classList.remove("tc-nav--bottom");
				if (!iconNav) {
					nav.classList.add("tc-nav--column");
				}
			}
		}
	}

	function barMoreLabel(bar) {
		return bar.getAttribute("data-tc-more") || "More";
	}

	function barFits(bar) {
		return bar.scrollWidth <= bar.clientWidth + 1 && fitsViewport(bar);
	}

	function barShelfChildren(shelf) {
		return Array.prototype.filter.call(shelf.children, function (c) {
			return !c.hasAttribute("data-tc-bar-stash") && !c.hasAttribute("data-tc-bar-overflow");
		});
	}

	function menuItemsFromBarNode(node) {
		var items = [];
		if (node.matches("a.tc-btn[href]")) {
			items.push({ label: node.textContent.trim(), href: node.getAttribute("href") || "#" });
			return items;
		}
		if (node.matches("button.tc-btn")) {
			items.push({ label: node.textContent.trim(), href: "#" });
			return items;
		}
		if (node.matches(".tc-check")) {
			var lab = node.querySelector(".tc-check__label");
			if (lab) items.push({ label: lab.textContent.trim(), href: "#" });
			return items;
		}
		node.querySelectorAll('a.tc-btn[href], button.tc-btn').forEach(function (btn) {
			items.push({
				label: btn.textContent.trim(),
				href: btn.getAttribute("href") || "#",
			});
		});
		if (items.length) return items;
		var text = node.textContent.trim().replace(/\s+/g, " ").slice(0, 80);
		if (text) items.push({ label: text, href: "#" });
		return items;
	}

	function restoreBar(bar) {
		var left = bar.querySelector(".tc-bar__left");
		var right = bar.querySelector(".tc-bar__right");
		if (!left || !right) return;
		var stash = bar.querySelector("[data-tc-bar-stash]");
		var overflow = bar.querySelector("[data-tc-bar-overflow]");
		if (stash) {
			while (stash.lastChild) {
				var node = stash.lastChild;
				var shelf = node._tcBarShelf === "left" ? left : right;
				if (shelf === right && overflow) {
					shelf.insertBefore(node, overflow);
				} else {
					shelf.appendChild(node);
				}
				delete node._tcBarShelf;
			}
		}
		if (overflow) overflow.hidden = true;
		bar.classList.remove("is-collapsed");
		bar.style.maxWidth = "";
	}

	function clampBar(bar) {
		var vw = document.documentElement.clientWidth;
		var rect = bar.getBoundingClientRect();
		if (rect.right <= vw + 1) {
			bar.style.maxWidth = "";
			return;
		}
		var max = Math.max(vw - rect.left - 1, 0);
		bar.style.maxWidth = max + "px";
	}

	function updateBar(bar) {
		restoreBar(bar);
		var left = bar.querySelector(".tc-bar__left");
		var right = bar.querySelector(".tc-bar__right");
		if (!left || !right) return;
		if (barFits(bar)) {
			clampBar(bar);
			return;
		}
		var stash = bar.querySelector("[data-tc-bar-stash]");
		if (!stash) {
			stash = document.createElement("div");
			stash.setAttribute("data-tc-bar-stash", "");
			stash.hidden = true;
			bar.appendChild(stash);
		}
		var movableRight = barShelfChildren(right);
		while (movableRight.length > 0 && !barFits(bar)) {
			var node = movableRight.pop();
			node._tcBarShelf = "right";
			stash.appendChild(node);
			movableRight = barShelfChildren(right);
		}
		var movableLeft = barShelfChildren(left);
		while (movableLeft.length > 0 && !barFits(bar)) {
			var node = movableLeft.pop();
			node._tcBarShelf = "left";
			stash.appendChild(node);
			movableLeft = barShelfChildren(left);
		}
		if (!stash.children.length) {
			bar.classList.toggle("is-collapsed", !barFits(bar));
			clampBar(bar);
			return;
		}
		var items = [];
		Array.prototype.forEach.call(stash.children, function (node) {
			var sub = menuItemsFromBarNode(node);
			if (!sub.length) return;
			if (items.length) items.push({ separator: true });
			items = items.concat(sub);
		});
		var overflow = bar.querySelector("[data-tc-bar-overflow]");
		if (!overflow) {
			overflow = document.createElement("div");
			overflow.setAttribute("data-tc-bar-overflow", "");
			right.appendChild(overflow);
		}
		var menu = overflow.querySelector(".tc-menu");
		if (!menu) {
			menu = buildMenuElement(tcUnique("tc-bar-more"), barMoreLabel(bar), items);
			overflow.appendChild(menu);
		} else {
			var pop = menu.querySelector("[data-tc-menu-popover]");
			if (pop) {
				var itemHTML = "";
				items.forEach(function (it) {
					if (it.separator) {
						itemHTML += '<li class="tc-menu__sep" role="separator"></li>';
						return;
					}
					itemHTML +=
						'<li role="none"><a href="' +
						escText(it.href || "#") +
						'" class="tc-menu__item" role="menuitem">' +
						escText(it.label) +
						"</a></li>";
				});
				pop.innerHTML = itemHTML;
			}
			var label = menu.querySelector(".tc-menu__label");
			if (label) label.textContent = barMoreLabel(bar);
		}
		overflow.hidden = false;
		bar.classList.add("is-collapsed");
		clampBar(bar);
	}

	function crumbsFits(nav) {
		return nav.scrollWidth <= nav.clientWidth + 1 && fitsViewport(nav);
	}

	function restoreCrumbs(nav) {
		var ellipsis = nav.querySelector("[data-tc-crumbs-ellipsis]");
		if (ellipsis) ellipsis.remove();
		nav.querySelectorAll(".tc-crumbs__link[hidden], .tc-crumbs__sep[hidden]").forEach(function (el) {
			el.hidden = false;
		});
		nav.style.maxWidth = "";
	}

	function clampCrumbs(nav) {
		var vw = document.documentElement.clientWidth;
		var rect = nav.getBoundingClientRect();
		var max = Math.max(vw - rect.left - 1, 0);
		var head = closest(nav, ".tc-head");
		if (head) {
			var hw = head.getBoundingClientRect().width;
			if (hw > 0 && hw < max) max = hw;
		}
		nav.style.maxWidth = max + "px";
	}

	function updateCrumbs(nav) {
		restoreCrumbs(nav);
		if (crumbsFits(nav)) return;
		var links = Array.prototype.slice.call(nav.querySelectorAll(".tc-crumbs__link"));
		if (links.length >= 2) {
			var root = links[0];
			var middleItems = [];
			for (var i = 1; i < links.length; i++) {
				var link = links[i];
				middleItems.push({ label: link.textContent.trim(), href: link.getAttribute("href") || "#" });
				var sep = link.previousElementSibling;
				if (sep && sep.classList.contains("tc-crumbs__sep")) sep.hidden = true;
				link.hidden = true;
			}
			var ellipsis = document.createElement("span");
			ellipsis.className = "tc-crumbs__ellipsis";
			ellipsis.setAttribute("data-tc-crumbs-ellipsis", "");
			ellipsis.appendChild(buildMenuElement(tcUnique("tc-crumbs-more"), "…", middleItems));
			var rootSep = root.nextElementSibling;
			if (rootSep && rootSep.classList.contains("tc-crumbs__sep")) {
				rootSep.insertAdjacentElement("afterend", ellipsis);
				var sep = document.createElement("span");
				sep.className = "tc-crumbs__sep";
				sep.setAttribute("aria-hidden", "true");
				sep.textContent = "/";
				ellipsis.insertAdjacentElement("afterend", sep);
			}
		}
		if (!crumbsFits(nav)) clampCrumbs(nav);
	}

	function layoutUpdates() {
		document.querySelectorAll("[data-tc-shell]").forEach(updateShell);
		document.querySelectorAll(".tc-seg").forEach(updateSeg);
		document.querySelectorAll("[data-tc-bar]").forEach(updateBar);
		document.querySelectorAll("[data-tc-crumbs]").forEach(updateCrumbs);
		document.querySelectorAll("[data-tc-top]").forEach(updateTop);
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

		document.querySelectorAll(".tc-seg").forEach(function (group) {
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
				var menu = group.querySelector("[data-tc-seg-menu]");
				if (menu) syncSegMenuLabel(group, menu);
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

		document.querySelectorAll("[data-tc-shell]").forEach(function (shell) {
			var rail = shell.querySelector("[data-tc-rail]");
			if (!rail) return;
			var toggle = shell.querySelector("[data-tc-rail-toggle]");
			var apply = shell.querySelector("[data-tc-rail-apply]");
			function setRailOpen(open) {
				rail.classList.toggle("is-open", open);
				rail.setAttribute("aria-expanded", open ? "true" : "false");
			}
			function closeRail() {
				setRailOpen(false);
				if (toggle) toggle.focus();
			}
			function flipRail() {
				setRailOpen(!rail.classList.contains("is-open"));
			}
			if (toggle) {
				toggle.addEventListener("click", flipRail);
			}
			if (apply) {
				apply.addEventListener("click", closeRail);
			}
			var header = rail.querySelector(".tc-rail__chips");
			if (header) {
				header.addEventListener("dblclick", flipRail);
			}
			shell._tcCloseRail = closeRail;
		});

		document.addEventListener("keydown", function (e) {
			if (e.key !== "Escape") return;
			var open = document.querySelector(".tc-rail--drawer.is-open");
			if (!open) return;
			var shell = closest(open, "[data-tc-shell]");
			if (shell && shell._tcCloseRail) {
				e.preventDefault();
				shell._tcCloseRail();
			}
		});

		layoutUpdates();

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

		window.addEventListener("resize", layoutUpdates);
	}

	document.addEventListener("DOMContentLoaded", function () {
		initRoots();
		initDynamic();
		initBehaviors();
		document.addEventListener("click", onDocClick);
		document.addEventListener("keydown", onKeydown);
		document.addEventListener("click", onCloseClick);
		window.__tc = { showOverlay: showOverlay, layoutUpdates: layoutUpdates };
	});
})();
