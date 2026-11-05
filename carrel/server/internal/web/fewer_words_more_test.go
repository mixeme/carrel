package web

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Typecase's notes 169–173 and 175 (Р-77, 04.11.2026): the hints say only
// what the screen does not. These are the words of them no other check reads in the
// frames; each must stand in its frame as it is.

var (
	saidTag    = regexp.MustCompile(`(?s)<svg.*?</svg>|<template\b.*?</template>|<[^>]+>`)
	saidBefore = regexp.MustCompile(` ([.,:;])( |$)`)
)

// frameSays is what frame id says: its words, tags dropped, no space
// before a mark that a closing tag left.
func frameSays(t *testing.T, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "visual", "carrel-ui-mockups.html"))
	if err != nil {
		t.Fatal(err)
	}
	mock := string(raw)
	at := strings.Index(mock, `data-frame="`+id+`"`)
	if at < 0 {
		t.Fatalf("no frame %s in the mockup", id)
	}
	if end := strings.Index(mock[at+1:], `<div class="frame"`); end >= 0 {
		mock = mock[:at+1+end]
	}
	words := strings.Join(strings.Fields(html.UnescapeString(saidTag.ReplaceAllString(mock[at:], " "))), " ")
	return saidBefore.ReplaceAllString(words, "$1$2")
}

func runsText(r view.Runs) string {
	var out string
	for _, p := range r {
		out += string(p.Text)
	}
	return out
}

func TestTheFewerWordsStandInTheirFrames(t *testing.T) {
	plan := &rewritePlan{Unread: []rewriteUnread{{Label: "Общий · baikal"}}}
	outcome := rewriteOutcome{written: 10, refused: []string{"«Ревью архитектуры» was refused: " + clause(&dav.StatusError{Code: 403}, "")}}
	two := make([]sealed.RecordRef, 2)
	for _, tc := range []struct {
		frame string
		said  string
	}{
		{"s-contacts-21", string(outcome.notice(plan, plan.defaultAnswer()).Body)},
		{"s-dup-1", dupLinkingNote},
		{"s-dup-6", dupLinkingNote},
		{"s-dup-1", string(groupHint(dupGroup{kind: view.KindEvent, refs: two}))},
		{"s-dup-1", string(groupHint(dupGroup{kind: view.KindNote, refs: two}))},
		{"s-io-4", runsText(importHints[view.KindNote])},
		{"s-io-4", runsText(importFolderHint)},
		{"s-plan-4", thunderbirdNote},
		{"s-publish-4", attachLinksHint},
		{"s-mail-5", runsText(notesFormatHint)},
		{"s-admin-32", runsText(relaySenderHint(monoRun("carrel@slovo.example")))},
		{"s-admin-4", outsideCheckNote},
		{"s-admin-25", outsideCheckNote},
		{"s-admin-26", outsideCheckNote},
		// Note 175: the second sentence that repeats the first goes.
		{"s-conflict-10", "Контакты is unavailable. " + unavailableBody + " Try again"},
		{"states-11", "Контакты is unavailable. " + unavailableBody + " Try again"},
		{"states-22", "Дела is unavailable. " + unavailableBody + " Try again"},
		{"states-29", "Записи is unavailable. " + unavailableBody + " Try again"},
		{"states-33", "Documents is unavailable. " + unavailableBody + " Try again"},
		{"s-profile-21", noneAnswering("calendar", []string{"baikal"}) + " Try again"},
		{"s-profile-11", string(labelTakenText("nextcloud"))},
		{"s-dup-6", "Nothing is loaded yet. " + dupNotLoadedNote},
		{"s-io-19", runsText(importSilent("nextcloud · Контакты did not answer", view.KindContact)) + " Try again"},
		{"s-plan-2", scanCodesNote},
		{"s-files-17", fileGoneTitle + " " + fileGoneBody},
		{"s-find-8", tooManySearches},
		{"s-backup-1", verifiedHint},
		{"s-backup-12", verifiedHint},
		{"s-backup-13", verifiedHint},
		{"s-backup-14", verifiedHint},
		{"attachments-1", attachPathHint},
		{"attachments-1", attachFolderHint},
		{"attachments-5", attachFolderHint},
		{"attachments-6", attachFolderHint},
		{"attachments-1", attachMovesNote},
		{"attachments-6", attachMovesNote},
		{"attachments-1", "vps 1 MB" + attachLowerEnd + " Folder"},
		{"appearance-1", datesHint},
	} {
		if !strings.Contains(frameSays(t, tc.frame), tc.said) {
			t.Errorf("%s does not say %q", tc.frame, tc.said)
		}
	}
}
