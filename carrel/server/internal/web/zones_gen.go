//go:build ignore

// zones_gen writes zones.txt: the zones an event may be moved to
// (s-agenda-8), read from the zone database of the Go that runs it
// ($GOROOT/lib/time/zoneinfo.zip, the same data time/tzdata embeds). Kept
// are the Area/Location names — Africa/…, America/…, Europe/… and the rest —
// and UTC; the old names of a country (US/…, Canada/…) and of a rule
// (EST5EDT, Etc/GMT+3) are left out. Run by go generate in this directory.
package main

import (
	"archive/zip"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var areas = map[string]bool{
	"Africa": true, "America": true, "Antarctica": true, "Arctic": true, "Asia": true,
	"Atlantic": true, "Australia": true, "Europe": true, "Indian": true, "Pacific": true,
}

func main() {
	root, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		log.Fatal(err)
	}
	zr, err := zip.OpenReader(filepath.Join(strings.TrimSpace(string(root)), "lib", "time", "zoneinfo.zip"))
	if err != nil {
		log.Fatal(err)
	}
	defer zr.Close()
	names := []string{"UTC"}
	for _, f := range zr.File {
		area, _, ok := strings.Cut(f.Name, "/")
		if ok && areas[area] && !strings.HasSuffix(f.Name, "/") {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	if err := os.WriteFile("zones.txt", []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		log.Fatal(err)
	}
}
