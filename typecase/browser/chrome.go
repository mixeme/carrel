package browser

import (
	"os"
	"os/exec"
	"runtime"
)

var chromeExec string

func init() {
	chromeExec, _ = locateChrome()
}

// ChromePath returns the browser executable used by chromedp tests.
func ChromePath() string {
	return chromeExec
}

// ChromeAvailable reports whether a Chromium-based browser was found.
func ChromeAvailable() bool {
	return chromeExec != ""
}

func locateChrome() (string, error) {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("google-chrome"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("chromium"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("chromium-browser"); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", exec.ErrNotFound
}
