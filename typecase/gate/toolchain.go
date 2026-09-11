package gate

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckGofmt fails when gofmt -l reports any file under root.
func CheckGofmt(root string) error {
	out, err := exec.Command("gofmt", "-l", root).Output()
	if err != nil {
		return fmt.Errorf("gofmt: %w", err)
	}
	broken := string(filepath.Separator) + "testdata" + string(filepath.Separator) + "broken" + string(filepath.Separator)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var bad []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, broken) {
			continue
		}
		bad = append(bad, line)
	}
	if len(bad) > 0 {
		return fmt.Errorf("gofmt: %s", strings.Join(bad, ", "))
	}
	return nil
}

// CheckVet runs go vet ./... from the module root.
func CheckVet(root string) error {
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("vet: %s", msg)
	}
	return nil
}

// ListGofmtDirty returns paths gofmt -l would rewrite under root.
func ListGofmtDirty(root string) ([]string, error) {
	out, err := exec.Command("gofmt", "-l", root).Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if !filepath.IsAbs(line) {
				line = filepath.Join(root, line)
			}
			paths = append(paths, line)
		}
	}
	return paths, nil
}
