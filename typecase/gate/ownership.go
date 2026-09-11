package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findGitRoot(from string) (string, bool) {
	dir, err := filepath.Abs(from)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func validateOwnershipScope(p ModulePaths) error {
	if p.RepoRoot == "" {
		return fmt.Errorf("ownership: search root not configured")
	}
	modAbs, err := filepath.Abs(p.Root)
	if err != nil {
		return err
	}
	repoAbs, err := filepath.Abs(p.RepoRoot)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(repoAbs, modAbs)
	if err != nil {
		return fmt.Errorf("ownership: %v", err)
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("ownership: module %s is outside search root %s", p.Root, p.RepoRoot)
	}
	inside, err := filepath.Rel(modAbs, repoAbs)
	if err == nil && inside != "." && !strings.HasPrefix(inside, "..") {
		return fmt.Errorf("ownership: search root %s lies inside module %s", p.RepoRoot, p.Root)
	}
	return nil
}

func isUnderDir(path, dir string) bool {
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	if pathAbs == dirAbs {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(pathAbs, dirAbs+sep)
}
