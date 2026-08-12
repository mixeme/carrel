// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ReadImportPayload extracts vCards from a lone .vcf body or a .zip of .vcf
// files. Takeout-specific layouts are out of scope (§23.7); this only opens
// standard cards.
func ReadImportPayload(filename string, body []byte, maxCards int) ([]ParsedCard, error) {
	name := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(name, ".zip"):
		return readZipVCards(body, maxCards)
	default:
		cards := ParseVCards(body)
		for i := range cards {
			if cards[i].Source == "" {
				cards[i].Source = filename
				if cards[i].Source == "" {
					cards[i].Source = "upload.vcf"
				}
			}
		}
		if maxCards > 0 && len(cards) > maxCards {
			return cards[:maxCards], fmt.Errorf("import exceeds %d cards; truncated", maxCards)
		}
		return cards, nil
	}
}

func readZipVCards(body []byte, maxCards int) ([]ParsedCard, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("import: open zip: %w", err)
	}
	var out []ParsedCard
	var truncated bool
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if strings.HasPrefix(base, ".") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(base), ".vcf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			out = append(out, ParsedCard{Source: f.Name, Error: err.Error()})
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(rc, 8<<20))
		rc.Close()
		if err != nil {
			out = append(out, ParsedCard{Source: f.Name, Error: err.Error()})
			continue
		}
		cards := ParseVCards(raw)
		if len(cards) == 0 {
			out = append(out, ParsedCard{Source: f.Name, Error: "no vCard found"})
			continue
		}
		for i := range cards {
			if cards[i].Source == "" {
				cards[i].Source = f.Name
			}
			out = append(out, cards[i])
			if maxCards > 0 && len(out) >= maxCards {
				truncated = true
				break
			}
		}
		if truncated {
			break
		}
	}
	if truncated {
		return out, fmt.Errorf("import exceeds %d cards; truncated", maxCards)
	}
	return out, nil
}

// ReadICSImportPayload extracts calendars from a lone .ics/.ical body or a
// .zip of those files (§23.7).
func ReadICSImportPayload(filename string, body []byte, maxEvents int) ([]ParsedCalendar, error) {
	name := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(name, ".zip"):
		return readZipICals(body, maxEvents)
	default:
		cals := ParseICals(body)
		for i := range cals {
			if cals[i].Source == "" {
				cals[i].Source = filename
				if cals[i].Source == "" {
					cals[i].Source = "upload.ics"
				}
			}
		}
		if maxEvents > 0 && len(cals) > maxEvents {
			return cals[:maxEvents], fmt.Errorf("import exceeds %d calendars; truncated", maxEvents)
		}
		return cals, nil
	}
}

func readZipICals(body []byte, maxEvents int) ([]ParsedCalendar, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("import: open zip: %w", err)
	}
	var out []ParsedCalendar
	var truncated bool
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if strings.HasPrefix(base, ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(base))
		if ext != ".ics" && ext != ".ical" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			out = append(out, ParsedCalendar{Source: f.Name, Error: err.Error()})
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(rc, 8<<20))
		rc.Close()
		if err != nil {
			out = append(out, ParsedCalendar{Source: f.Name, Error: err.Error()})
			continue
		}
		cals := ParseICals(raw)
		if len(cals) == 0 {
			out = append(out, ParsedCalendar{Source: f.Name, Error: "no VCALENDAR found"})
			continue
		}
		for i := range cals {
			if cals[i].Source == "" {
				cals[i].Source = f.Name
			}
			out = append(out, cals[i])
			if maxEvents > 0 && len(out) >= maxEvents {
				truncated = true
				break
			}
		}
		if truncated {
			break
		}
	}
	if truncated {
		return out, fmt.Errorf("import exceeds %d calendars; truncated", maxEvents)
	}
	return out, nil
}
