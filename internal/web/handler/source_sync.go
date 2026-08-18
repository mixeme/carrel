// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// sourceBlockView is the §23.8 Source block: when a collection was read and
// when the server last changed it. Object fields are filled on detail panels.
type sourceBlockView struct {
	Known        bool
	ObjectETag   string
	ObjectPath   string
	ReadLabel    string
	ChangedLabel string
	ObjectCount  int
	MetaLabel    string
}

type collectionSyncView struct {
	Known     bool
	MetaLabel string
}

func (s *Server) collectionSource(sess *session.Session, accountID, collectionPath string) sourceBlockView {
	now := time.Now()
	if sess == nil {
		return sourceBlockView{}
	}
	cache := sess.Cache()
	if cache == nil {
		return sourceBlockView{}
	}
	meta, ok := cache.CollectionMeta(accountID, collectionPath)
	if !ok {
		return sourceBlockView{}
	}
	return sourceBlockView{
		Known:        true,
		ReadLabel:    formatReadLabel(meta.FetchedAt, now),
		ChangedLabel: formatChangedLabel(meta.CTag, now),
		ObjectCount:  meta.ObjectCount,
		MetaLabel:    collectionMetaLabel(meta, now),
	}
}

func (s *Server) objectSource(sess *session.Session, accountID, collectionPath, objectPath, objectETag string) sourceBlockView {
	view := s.collectionSource(sess, accountID, collectionPath)
	view.ObjectPath = strings.TrimSpace(objectPath)
	view.ObjectETag = strings.TrimSpace(objectETag)
	return view
}

func collectionMetaLabel(meta session.CollectionMeta, now time.Time) string {
	var parts []string
	if meta.ObjectCount > 0 {
		parts = append(parts, strconv.Itoa(meta.ObjectCount))
	}
	if label := formatReadLabel(meta.FetchedAt, now); label != "" {
		parts = append(parts, "read "+label)
	}
	if changed := formatChangedShort(meta.CTag, now); changed != "" {
		parts = append(parts, changed)
	}
	return strings.Join(parts, " · ")
}

func formatReadLabel(fetched, now time.Time) string {
	if fetched.IsZero() {
		return ""
	}
	return formatRelativePast(fetched, now)
}

func formatChangedLabel(ctag string, now time.Time) string {
	if t, ok := parseCTagTime(ctag); ok {
		return "changed on server " + formatRelativePast(t, now)
	}
	return ""
}

func formatChangedShort(ctag string, now time.Time) string {
	if t, ok := parseCTagTime(ctag); ok {
		return "changed " + formatRelativePast(t, now)
	}
	return ""
}

func formatRelativePast(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d / time.Minute)
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", mins)
	case d < 24*time.Hour:
		hrs := int(d / time.Hour)
		if hrs == 1 {
			return "1 h ago"
		}
		return fmt.Sprintf("%d h ago", hrs)
	case d < 30*24*time.Hour:
		days := int(d / (24 * time.Hour))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.In(now.Location()).Format("2006-01-02")
	}
}

func parseCTagTime(ctag string) (time.Time, bool) {
	ctag = strings.Trim(strings.TrimSpace(ctag), `"`)
	if ctag == "" {
		return time.Time{}, false
	}
	if ts, err := strconv.ParseInt(ctag, 10, 64); err == nil {
		switch len(ctag) {
		case 10:
			return time.Unix(ts, 0).UTC(), true
		case 13:
			return time.UnixMilli(ts).UTC(), true
		}
	}
	for _, layout := range []string{
		time.RFC3339,
		"20060102T150405Z",
		"20060102T150405",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, ctag); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
