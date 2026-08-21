package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Наборы из docs/visual/fixtures загружают руками, чтобы сверить живые
// экраны с макетами. Сверять есть смысл только с тем, что вообще читается,
// поэтому парсер репозитория проходит по каждому файлу, а заодно
// проверяется то, что глазами не видно: CRLF вместо голого LF и свёртка не
// длиннее 75 октетов. Кириллица считается в октетах, и строка, короткая на
// вид, легко оказывается вдвое длиннее допустимой.
func TestMockupFixturesParse(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "visual", "fixtures")
	var files []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".vcf") || strings.HasSuffix(p, ".ics") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	total := 0
	for _, p := range files {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		raw := string(body)
		if strings.Contains(strings.ReplaceAll(raw, "\r\n", ""), "\n") {
			t.Errorf("%s: bare LF, must be CRLF", p)
		}
		for i, line := range strings.Split(raw, "\r\n") {
			if len(line) > 75 {
				t.Errorf("%s:%d: %d octets, over 75: %q", p, i+1, len(line), line)
			}
		}
		if strings.HasSuffix(p, ".vcf") {
			cards := ParseVCards(body)
			if len(cards) == 0 {
				t.Errorf("%s: no cards parsed", p)
			}
			for _, c := range cards {
				if c.Error != "" {
					t.Errorf("%s: %s", p, c.Error)
					continue
				}
				ct, err := c.Object.Contact()
				if err != nil {
					t.Errorf("%s: contact: %v", p, err)
					continue
				}
				if ct.FormattedName == "" {
					t.Errorf("%s: card without FN", p)
				}
				total++
			}
		} else {
			cals := ParseICals(body)
			if len(cals) == 0 {
				t.Errorf("%s: no calendars parsed", p)
			}
			for _, cal := range cals {
				if cal.Error != "" {
					t.Errorf("%s: %s", p, cal.Error)
					continue
				}
				total++
			}
		}
	}
	t.Logf("fixtures: %d files, %d objects", len(files), total)
}
