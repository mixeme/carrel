package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveSettingKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, settingsFile)
	if err := os.WriteFile(path, []byte("# mine\nCARREL_PORT=9000\nCARREL_PUBLIC_URL=https://old.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveSetting(dir, "CARREL_PUBLIC_URL", "https://new.example"); err != nil {
		t.Fatal(err)
	}
	if err := SaveSetting(dir, "CARREL_TIMEZONE", "UTC"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "# mine\nCARREL_PORT=9000\nCARREL_PUBLIC_URL=https://new.example\nCARREL_TIMEZONE=UTC\n"
	if string(got) != want {
		t.Errorf("got %q", got)
	}
	empty := t.TempDir()
	if err := SaveSetting(empty, "CARREL_PUBLIC_URL", "https://a.example"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(empty, settingsFile)); string(got) != "CARREL_PUBLIC_URL=https://a.example\n" {
		t.Errorf("fresh file %q", got)
	}
}
