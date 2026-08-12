// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// The two address books of the duplicate tests: the same person written twice is
// what §15 is about, and one book cannot show it.
const (
	dupBookOne = "/addressbooks/user/one/"
	dupBookTwo = "/addressbooks/user/two/"
	dupAccount = "dup-acc"
)

// dupBooks is a CardDAV server holding several collections, so a group can span
// them the way it does in practice.
type dupBooks struct {
	*httptest.Server

	mu    sync.Mutex
	cards map[string]string
	etags map[string]string
	// puts and deleted are what the merge of §15 is judged by: what was
	// written, in what order, and what was removed afterwards.
	puts        []calPut
	deleted     []string
	failNextPut bool
	failDelete  string
	ctag        int
}

func startDupBooks(t *testing.T) *dupBooks {
	t.Helper()
	b := &dupBooks{cards: map[string]string{}, etags: map[string]string{}, ctag: 1}
	b.seed(dupBookOne+"ada.vcf", `"a1"`, card("ada", "Ada Lovelace",
		"EMAIL:ada@example.org", "TEL:+7 495 123-45-67", "X-CLIENT-FLAG:keep-me"))
	b.seed(dupBookTwo+"ada-king.vcf", `"a2"`, card("ada-king", "Ada King",
		"EMAIL:ada@example.org", "EMAIL:ada.king@example.org", "TEL:8 (495) 123 45 67",
		"NOTE:Analytical engine"))
	b.seed(dupBookTwo+"grace.vcf", `"g1"`, card("grace", "Grace Hopper", "EMAIL:grace@example.org"))
	b.Server = httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(b.Close)
	return b
}

func card(uid, name string, extra ...string) string {
	lines := append([]string{"BEGIN:VCARD", "VERSION:3.0", "UID:" + uid, "FN:" + name}, extra...)
	return strings.Join(append(lines, "END:VCARD", ""), "\r\n")
}

func (b *dupBooks) seed(path, etag, body string) {
	b.cards[path] = body
	b.etags[path] = etag
}

func (b *dupBooks) body(t *testing.T, path string) string {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	body, ok := b.cards[path]
	if !ok {
		t.Fatalf("no card at %q", path)
	}
	return body
}

func (b *dupBooks) writes() []calPut {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]calPut(nil), b.puts...)
}

func (b *dupBooks) removed() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.deleted...)
}

func (b *dupBooks) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case r.Method == "PROPFIND", r.Method == "REPORT":
		b.multistatus(w, path, r.Method == "REPORT")
	case r.Method == http.MethodGet:
		body, ok := b.cards[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", b.etags[path])
		w.Header().Set("Content-Type", "text/vcard")
		_, _ = io.WriteString(w, body)
	case r.Method == http.MethodPut:
		raw, _ := io.ReadAll(r.Body)
		b.puts = append(b.puts, calPut{
			Path: path, Body: string(raw),
			IfMatch: r.Header.Get("If-Match"), IfNoneMatch: r.Header.Get("If-None-Match"),
		})
		if b.failNextPut {
			b.failNextPut = false
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		if match := r.Header.Get("If-Match"); match != "" && b.etags[path] != match {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		b.ctag++
		etag := fmt.Sprintf("%q", fmt.Sprintf("v%d", b.ctag))
		b.cards[path], b.etags[path] = string(raw), etag
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodDelete:
		if b.failDelete == path {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		if _, ok := b.cards[path]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(b.cards, path)
		delete(b.etags, path)
		b.deleted = append(b.deleted, path)
		b.ctag++
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// multistatus answers for the collection the request was addressed to, which is
// what makes several books on one server possible.
func (b *dupBooks) multistatus(w http.ResponseWriter, path string, withData bool) {
	collection := path
	if !strings.HasSuffix(collection, "/") {
		collection += "/"
	}
	var body strings.Builder
	fmt.Fprintf(&body, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
		`<cs:getctag>ctag-%d</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status>`+
		`</d:propstat></d:response>`, collection, b.ctag)
	for cardPath, cardBody := range b.cards {
		if !strings.HasPrefix(cardPath, collection) {
			continue
		}
		data := ""
		if withData {
			data = `<card:address-data>` + xmlEscapeText(cardBody) + `</card:address-data>`
		}
		fmt.Fprintf(&body, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
			`<d:getetag>%s</d:getetag>%s</d:prop><d:status>HTTP/1.1 200 OK</d:status>`+
			`</d:propstat></d:response>`, cardPath, b.etags[cardPath], data)
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0"?>`+
		`<d:multistatus xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/" `+
		`xmlns:card="urn:ietf:params:xml:ns:carddav">`+body.String()+`</d:multistatus>`)
}

// duplicatesApp is an instance with the two books connected and the fan-out
// wired, which is what the duplicates screen polls over.
func duplicatesApp(t *testing.T, b *dupBooks) *app {
	t.Helper()
	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.Fanout = fanout.NewRegistry(nil)
	t.Cleanup(a.Fanout.Close)
	a.setupAdmin("root", "", testPassword)

	sess := a.session()
	acc := account.Account{
		ID: dupAccount, Label: "Work", BaseURL: b.URL + "/",
		Username: "mix", Password: "secret", Enabled: true,
		Collections: []discovery.Collection{
			{Path: dupBookOne, DisplayName: "Personal", Kind: discovery.KindAddressBook},
			{Path: dupBookTwo, DisplayName: "Shared", Kind: discovery.KindAddressBook},
		},
	}
	if err := a.Store.PutDAVAccount(store.Actor{ID: sess.UserID, Login: sess.Login}, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatal(err)
	}
	return a
}

// duplicateGroups renders the screen and follows the poll to the end, which is
// where the groups are.
func (a *app) duplicateGroups(t *testing.T) string {
	t.Helper()
	page := a.get("/app/duplicates")
	if page.Code != http.StatusOK {
		t.Fatalf("duplicates = %d, body = %s", page.Code, page.Body.String())
	}
	return a.findResults(t, page.Body.String(), "mode=duplicates")
}

func dupToken(collection, uid string) string {
	return encodeDupRef(dupRef{
		AccountID: dupAccount, Collection: collection, UID: uid,
		Path: collection + uid + ".vcf",
	})
}

func TestDuplicatesScreenOffersCandidatesWithReasons(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	body := a.duplicateGroups(t)
	if !strings.Contains(body, "candidate") {
		t.Fatalf("no candidate group on the screen:\n%s", body)
	}
	// §23.8: the reason is a thing a person can check, not a number alone.
	if !strings.Contains(body, "same address") {
		t.Errorf("the matching signal is not named:\n%s", body)
	}
	// Both records are named, with the collection each is in.
	for _, want := range []string{"Ada Lovelace", "Ada King", "Personal", "Shared"} {
		if !strings.Contains(body, want) {
			t.Errorf("the group is missing %q:\n%s", want, body)
		}
	}
	// The union of the repeatable fields is shown, and the same number written
	// two ways appears once.
	if !strings.Contains(body, "ada.king@example.org") {
		t.Errorf("the merged addresses are missing:\n%s", body)
	}
	if strings.Count(body, "8 (495) 123 45 67") != 0 {
		t.Errorf("the same number is offered twice:\n%s", body)
	}
	// A person who resembles nobody is not offered.
	if strings.Contains(body, "Grace Hopper") {
		t.Errorf("an unrelated card was offered as a duplicate:\n%s", body)
	}
	// Detection reads; it never writes.
	if writes := books.writes(); len(writes) != 0 {
		t.Errorf("%d writes left the process for a read-only screen", len(writes))
	}
}

// §15: linking is a decision about how records are shown, and changes nothing on
// any server. §21: it outlives the session it was made in.
func TestDuplicateLinkStoresAndChangesNothingOnTheServer(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	rec := a.post("/app/duplicates/decide", url.Values{
		"action":   {"link"},
		"kind":     {account.KindContact},
		"member":   {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
		"field:FN": {"Ada King"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("link = %d, body = %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "nothing+was+changed") {
		t.Errorf("Location = %q, want a notice saying no server was touched", loc)
	}
	if writes := books.writes(); len(writes) != 0 {
		t.Fatalf("linking wrote %d objects to the server", len(writes))
	}

	sess := a.session()
	decisions, err := a.Store.Duplicates(sess.UserID, sess.DEK())
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions.Groups) != 1 || decisions.Groups[0].Verdict != account.VerdictLinked {
		t.Fatalf("stored groups = %+v", decisions.Groups)
	}
	// The chosen value for the field the records disagree about is remembered.
	if decisions.Groups[0].Fields["FN"] != "Ada King" {
		t.Errorf("fields = %+v", decisions.Groups[0].Fields)
	}

	body := a.duplicateGroups(t)
	if !strings.Contains(body, "linked ×2") {
		t.Errorf("the link is not shown on the screen:\n%s", body)
	}
	if strings.Contains(body, "candidate") {
		t.Errorf("a linked group is still offered as a candidate:\n%s", body)
	}
	if !strings.Contains(body, "Unlink") {
		t.Errorf("a link cannot be undone from the screen:\n%s", body)
	}
}

// §15: the merged list shows a linked group as one row with a badge.
func TestLinkedGroupCollapsesInTheMergedList(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	if rec := a.post("/app/duplicates/decide", url.Values{
		"action": {"link"},
		"kind":   {account.KindContact},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("link = %d, body = %s", rec.Code, rec.Body.String())
	}

	page := a.get("/app/unified?mode=people")
	if page.Code != http.StatusOK {
		t.Fatalf("people = %d", page.Code)
	}
	body := a.findResults(t, page.Body.String(), "mode=people")
	if !strings.Contains(body, "linked ×2") {
		t.Fatalf("no badge on the collapsed row:\n%s", body)
	}
	// One row, not two: the second card's own name is not a row of its own.
	if strings.Count(body, "<strong>Ada") != 1 {
		t.Errorf("the group is not one row:\n%s", body)
	}
	// The merged row carries the union of the addresses.
	if !strings.Contains(body, "ada.king@example.org") || !strings.Contains(body, "ada@example.org") {
		t.Errorf("the merged row lost an address:\n%s", body)
	}
	if !strings.Contains(body, "Grace Hopper") {
		t.Errorf("an unrelated row disappeared:\n%s", body)
	}

	// The badge is on the single book's list too, where the record lives.
	list := a.get("/app/contacts/" + dupAccount + "/" + EncodeCollectionPath(dupBookOne))
	if list.Code != http.StatusOK {
		t.Fatalf("contacts list = %d", list.Code)
	}
	if !strings.Contains(list.Body.String(), "linked ×2") {
		t.Errorf("no badge in the address book list:\n%s", list.Body.String())
	}
}

// §21: a group marked "not duplicates" is not offered again, after a reload and
// after a restart.
func TestDuplicateIgnoreIsNotOfferedAgain(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	rec := a.post("/app/duplicates/decide", url.Values{
		"action": {"ignore"},
		"kind":   {account.KindContact},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ignore = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := a.duplicateGroups(t)
	if strings.Contains(body, "candidate") {
		t.Fatalf("a rejected pair is offered again:\n%s", body)
	}
	if !strings.Contains(body, "set aside") || !strings.Contains(body, "Offer again") {
		t.Errorf("the rejected group cannot be reconsidered:\n%s", body)
	}

	// The merged list does not collapse a pair that was rejected.
	page := a.get("/app/unified?mode=people")
	people := a.findResults(t, page.Body.String(), "mode=people")
	if strings.Contains(people, "duplicate?") || strings.Contains(people, "linked ×") {
		t.Errorf("a rejected pair was collapsed anyway:\n%s", people)
	}

	sess := a.session()
	decisions, err := a.Store.Duplicates(sess.UserID, sess.DEK())
	if err != nil {
		t.Fatal(err)
	}
	if !decisions.Ignored(
		account.Member{AccountID: dupAccount, Collection: dupBookOne, UID: "ada"},
		account.Member{AccountID: dupAccount, Collection: dupBookTwo, UID: "ada-king"},
	) {
		t.Fatalf("the verdict was not stored: %+v", decisions.Groups)
	}

	// Forgetting the group offers it again, which is the third door of §15.
	forget := a.post("/app/duplicates/decide", url.Values{
		"action": {"forget"},
		"group":  {decisions.Groups[0].ID},
	})
	if forget.Code != http.StatusSeeOther {
		t.Fatalf("forget = %d", forget.Code)
	}
	again := a.duplicateGroups(t)
	if !strings.Contains(again, "candidate") {
		t.Errorf("the group was not offered again:\n%s", again)
	}
}

func TestDuplicateDecideRefusesForeignCollections(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	rec := a.post("/app/duplicates/decide", url.Values{
		"action": {"link"},
		"kind":   {account.KindContact},
		"member": {dupToken(dupBookOne, "ada"), encodeDupRef(dupRef{
			AccountID: dupAccount, Collection: "/addressbooks/somebody/else/", UID: "x",
		})},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "not+one+of+yours") {
		t.Errorf("Location = %q, want the refusal", loc)
	}
	sess := a.session()
	decisions, err := a.Store.Duplicates(sess.UserID, sess.DEK())
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions.Groups) != 0 {
		t.Fatalf("a refused decision was stored: %+v", decisions.Groups)
	}
}

// §15: the merge on the server is confirmed first, and the confirmation says
// what will be written and what will be deleted.
func TestDuplicateMergeConfirmsBeforeWriting(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	rec := a.post("/app/duplicates/merge", url.Values{
		"action": {"plan"},
		"kind":   {account.KindContact},
		"target": {dupToken(dupBookOne, "ada")},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("plan = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"cannot be undone", "Ada Lovelace", "Ada King", "EMAIL", "NOTE"} {
		if !strings.Contains(body, want) {
			t.Errorf("the confirmation is missing %q:\n%s", want, body)
		}
	}
	if len(books.writes()) != 0 || len(books.removed()) != 0 {
		t.Fatal("the confirmation touched the server")
	}
}

func TestDuplicateMergeWritesThenDeletes(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	rec := a.post("/app/duplicates/merge", url.Values{
		"action": {"apply"},
		"kind":   {account.KindContact},
		"target": {dupToken(dupBookOne, "ada")},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("merge = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "merged into one") {
		t.Errorf("the report does not say what happened:\n%s", rec.Body.String())
	}

	writes := books.writes()
	if len(writes) != 1 {
		t.Fatalf("%d writes, want the merged card only", len(writes))
	}
	if writes[0].Path != dupBookOne+"ada.vcf" {
		t.Errorf("written to %q, want the target", writes[0].Path)
	}
	// §17: the write is conditional on the version it was read at.
	if writes[0].IfMatch == "" {
		t.Error("the merge was written without a precondition")
	}
	merged := books.body(t, dupBookOne+"ada.vcf")
	for _, want := range []string{
		"FN:Ada Lovelace", // a conflict leaves the target's own value alone
		"EMAIL:ada@example.org", "EMAIL:ada.king@example.org",
		"NOTE:Analytical engine", "X-CLIENT-FLAG:keep-me", "UID:ada",
	} {
		if !strings.Contains(merged, want) {
			t.Errorf("the merged card is missing %q:\n%s", want, merged)
		}
	}
	if strings.Count(merged, "TEL:") != 1 {
		t.Errorf("the same number was written twice:\n%s", merged)
	}
	if removed := books.removed(); len(removed) != 1 || removed[0] != dupBookTwo+"ada-king.vcf" {
		t.Errorf("deleted = %v, want the source only", removed)
	}
}

// §21: if the write fails, nothing is deleted.
func TestDuplicateMergeKeepsSourcesWhenTheWriteFails(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)
	books.mu.Lock()
	books.failNextPut = true
	books.mu.Unlock()

	rec := a.post("/app/duplicates/merge", url.Values{
		"action": {"apply"},
		"kind":   {account.KindContact},
		"target": {dupToken(dupBookOne, "ada")},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("merge = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "nothing was deleted") {
		t.Errorf("the failure is not explained:\n%s", body)
	}
	if removed := books.removed(); len(removed) != 0 {
		t.Fatalf("a failed write still deleted %v", removed)
	}
	// Both cards are still there, unchanged.
	if !strings.Contains(books.body(t, dupBookTwo+"ada-king.vcf"), "FN:Ada King") {
		t.Error("the source card was damaged by a failed merge")
	}
}

// §15: a delete that fails stops the rest, says what has gone and what has not,
// and nothing is rolled back.
func TestDuplicateMergeReportsAPartialCleanup(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)
	books.mu.Lock()
	books.failDelete = dupBookTwo + "ada-king.vcf"
	books.mu.Unlock()

	rec := a.post("/app/duplicates/merge", url.Values{
		"action": {"apply"},
		"kind":   {account.KindContact},
		"target": {dupToken(dupBookOne, "ada")},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("merge = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Still there") || !strings.Contains(body, "Nothing was undone") {
		t.Errorf("the partial cleanup is not reported:\n%s", body)
	}
	if len(books.writes()) != 1 {
		t.Errorf("writes = %d, want the merged card", len(books.writes()))
	}
	// The merged card stays: rolling it back would lose what was merged into it.
	if !strings.Contains(books.body(t, dupBookOne+"ada.vcf"), "NOTE:Analytical engine") {
		t.Error("the merged card was rolled back")
	}
}

// §15: the key of a member is not stable. A record another client deleted is
// dropped from its group silently, and a group left with one member dissolves.
func TestDuplicateGroupSurvivesADeletedMember(t *testing.T) {
	books := startDupBooks(t)
	a := duplicatesApp(t, books)

	if rec := a.post("/app/duplicates/decide", url.Values{
		"action": {"link"},
		"kind":   {account.KindContact},
		"member": {dupToken(dupBookOne, "ada"), dupToken(dupBookTwo, "ada-king")},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("link = %d", rec.Code)
	}

	// Another client removes one of the two cards.
	books.mu.Lock()
	delete(books.cards, dupBookTwo+"ada-king.vcf")
	delete(books.etags, dupBookTwo+"ada-king.vcf")
	books.ctag++
	books.mu.Unlock()
	a.post("/app/", url.Values{fieldAction: {"refresh_cache"}})

	body := a.duplicateGroups(t)
	if strings.Contains(body, "linked ×") {
		t.Errorf("the group outlived its second member:\n%s", body)
	}
	sess := a.session()
	decisions, err := a.Store.Duplicates(sess.UserID, sess.DEK())
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions.Groups) != 0 {
		t.Fatalf("the stored group was not pruned: %+v", decisions.Groups)
	}
}

// §15: the threshold is a setting, and lowering it offers more.
func TestDuplicateThresholdIsConfigurable(t *testing.T) {
	books := startDupBooks(t)
	books.mu.Lock()
	// Two cards that share nothing but a name: below the default, above a
	// threshold an administrator lowered.
	books.seed(dupBookOne+"grace-two.vcf", `"g2"`, card("grace-two", "Grace Hopper"))
	books.mu.Unlock()

	a := duplicatesApp(t, books)
	if body := a.duplicateGroups(t); strings.Contains(body, "Grace Hopper") {
		t.Errorf("a shared name alone was offered at the default threshold:\n%s", body)
	}

	a.Detection.Threshold = 30
	if body := a.duplicateGroups(t); !strings.Contains(body, "Grace Hopper") {
		t.Errorf("the lowered threshold offered nothing:\n%s", body)
	}
}
