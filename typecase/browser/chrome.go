package browser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
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

// instance is a headless browser started by the harness.
//
// The harness starts the process itself instead of letting chromedp's exec
// allocator do it: the allocator learns the DevTools address by reading the
// "DevTools listening on" line from stderr, and a browser built as a Windows
// GUI subsystem binary (Edge, notably) writes nothing to an inherited pipe.
// The DevToolsActivePort file inside the profile directory carries the same
// address on every platform and every Chromium build.
type instance struct {
	cmd      *exec.Cmd
	dir      string
	endpoint string
}

const portFileTimeout = 30 * time.Second

func launchChrome() (*instance, error) {
	if chromeExec == "" {
		return nil, exec.ErrNotFound
	}
	dir, err := os.MkdirTemp("", "typecase-browser-")
	if err != nil {
		return nil, fmt.Errorf("profile dir: %w", err)
	}
	cmd := exec.Command(chromeExec,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-sync",
		"--remote-allow-origins=*",
		"--user-data-dir="+dir,
		"--remote-debugging-port=0",
		"about:blank",
	)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("start %s: %w", chromeExec, err)
	}
	in := &instance{cmd: cmd, dir: dir}
	port, err := waitPortFile(filepath.Join(dir, "DevToolsActivePort"))
	if err != nil {
		in.stop()
		return nil, err
	}
	in.endpoint = "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	return in, nil
}

func waitPortFile(path string) (int, error) {
	deadline := time.Now().Add(portFileTimeout)
	for {
		port, err := readPortFile(path)
		if err == nil {
			return port, nil
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("browser did not publish %s within %s: %w", filepath.Base(path), portFileTimeout, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func readPortFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	if !scan.Scan() {
		return 0, fmt.Errorf("empty port file")
	}
	port, err := strconv.Atoi(strings.TrimSpace(scan.Text()))
	if err != nil {
		return 0, fmt.Errorf("bad port file: %w", err)
	}
	if port <= 0 {
		return 0, fmt.Errorf("bad port %d", port)
	}
	return port, nil
}

func (in *instance) stop() {
	if in.cmd != nil && in.cmd.Process != nil {
		_ = in.cmd.Process.Kill()
		_, _ = in.cmd.Process.Wait()
	}
	if in.dir != "" {
		_ = os.RemoveAll(in.dir)
	}
}
