package browser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/typecase/browser"
)

func TestL9_2_browserChecks(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if os.Getenv("CI") == "" && testing.Short() {
		t.Skip("browser checks need Chrome; skip with -short outside CI")
	}
	h := browser.NewHarness(t)
	fixtures := browser.GoodFixtures()
	for _, name := range browser.AllChecks {
		t.Run(string(name), func(t *testing.T) {
			fix, ok := fixtures[name]
			if !ok {
				t.Fatalf("missing good fixture for %s", name)
			}
			if err := browser.Check(name, h, fix); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func readBroken(t *testing.T, name string) string {
	root := browser.ModuleRoot(t)
	path := filepath.Join(root, "browser", "testdata", "broken", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func brokenFixture(t *testing.T, name browser.Name) browser.Fixture {
	good := browser.GoodFixtures()[name]
	body := readBroken(t, string(name)+".html")
	if strings.TrimSpace(body) == "" {
		t.Fatalf("empty broken fixture for %s", name)
	}
	fix := good
	fix.Body = body
	if extra := readBrokenOptional(t, string(name)+".css"); extra != "" {
		if fix.Extra != "" {
			fix.Extra += "\n"
		}
		fix.Extra += extra
	}
	return fix
}

func readBrokenOptional(t *testing.T, name string) string {
	root := browser.ModuleRoot(t)
	path := filepath.Join(root, "browser", "testdata", "broken", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func TestL9_3_biteTests(t *testing.T) {
	if !browser.ChromeAvailable() {
		t.Skip("no Chromium-based browser found (set CHROME_PATH)")
	}
	if os.Getenv("CI") == "" && testing.Short() {
		t.Skip("browser bite tests need Chrome; skip with -short outside CI")
	}
	h := browser.NewHarness(t)
	for _, name := range browser.AllChecks {
		t.Run(string(name), func(t *testing.T) {
			fix := brokenFixture(t, name)
			if err := browser.Check(name, h, fix); err == nil {
				t.Fatalf("expected %s check to fail on broken fixture", name)
			}
		})
	}
}
