(function () {
	"use strict";

	var storageKey = "tc-theme";

	function readStored() {
		try {
			return localStorage.getItem(storageKey);
		} catch (e) {
			return null;
		}
	}

	function systemDark() {
		return (
			window.matchMedia &&
			window.matchMedia("(prefers-color-scheme: dark)").matches
		);
	}

	function resolveTheme() {
		var stored = readStored();
		if (stored === "light" || stored === "dark") {
			return stored;
		}
		return systemDark() ? "dark" : "light";
	}

	document.documentElement.setAttribute("data-tc-theme", resolveTheme());
})();
