// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package contacts

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

const collection = "/dav.php/addressbooks/mix/default/"

// fakeServer is an address book that speaks the part of CardDAV the provider
// uses, and counts what it was asked, so a test can tell a cached read from one
// that went to the server.
type fakeServer struct {
	ctag    string
	objects map[string]*fakeObject

	propfindDepth0 int
	propfindDepth1 int
	reports        int
	puts           int
	deletes        int

	// dropOnPut names properties the server refuses to store, standing in for
	// the CardDAV servers that quietly do exactly this (§8).
	dropOnPut []string
	// failPrecondition makes the next write answer 412.
	failPrecondition bool
	// refuseReport makes REPORT unavailable, as on a server that only
	// answers PROPFIND and GET.
	refuseReport bool
}

type fakeObject struct {
	etag string
	body string
}

func newFakeServer() *fakeServer {
	return &fakeServer{ctag: "ctag-1", objects: map[string]*fakeObject{}}
}

func (s *fakeServer) add(name, etag string, lines ...string) string {
	path := collection + name
	body := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:" + strings.TrimSuffix(name, ".vcf") + "\r\n" +
		strings.Join(lines, "\r\n") + "\r\nEND:VCARD\r\n"
	s.objects[path] = &fakeObject{etag: etag, body: body}
	return path
}

func (s *fakeServer) PropFind(_ context.Context, path string, depth dav.Depth, _ []xml.Name) (*dav.MultiStatus, error) {
	if depth == dav.DepthZero {
		s.propfindDepth0++
		if obj, ok := s.objects[path]; ok {
			return parseMS(fmt.Sprintf(`<multistatus xmlns="DAV:"><response><href>%s</href>`+
				`<propstat><prop><getetag>%s</getetag></prop><status>HTTP/1.1 200 OK</status></propstat>`+
				`</response></multistatus>`, path, obj.etag))
		}
		return parseMS(fmt.Sprintf(`<multistatus xmlns="DAV:" xmlns:cs="http://calendarserver.org/ns/">`+
			`<response><href>%s</href><propstat><prop><cs:getctag>%s</cs:getctag></prop>`+
			`<status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, path, s.ctag))
	}

	s.propfindDepth1++
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:">`)
	// The collection itself is part of a Depth: 1 answer and must be skipped.
	fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><resourcetype><collection/></resourcetype>`+
		`</prop><status>HTTP/1.1 200 OK</status></propstat></response>`, path)
	for _, objPath := range s.paths() {
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><resourcetype/>`+
			`<getetag>%s</getetag><getcontenttype>text/vcard</getcontenttype></prop>`+
			`<status>HTTP/1.1 200 OK</status></propstat></response>`, objPath, s.objects[objPath].etag)
	}
	b.WriteString(`</multistatus>`)
	return parseMS(b.String())
}

func (s *fakeServer) Report(_ context.Context, _ string, _ dav.Depth, body any) (*dav.MultiStatus, error) {
	s.reports++
	if s.refuseReport {
		return nil, &dav.HTTPError{Code: http.StatusNotImplemented}
	}
	multiget, ok := body.(*dav.AddressBookMultiget)
	if !ok {
		return nil, fmt.Errorf("unexpected report body %T", body)
	}
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav">`)
	for _, href := range multiget.Hrefs {
		obj, ok := s.objects[href]
		if !ok {
			fmt.Fprintf(&b, `<response><href>%s</href><status>HTTP/1.1 404 Not Found</status></response>`, href)
			continue
		}
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><getetag>%s</getetag>`+
			`<card:address-data>%s</card:address-data></prop><status>HTTP/1.1 200 OK</status>`+
			`</propstat></response>`, href, obj.etag, escapeXML(obj.body))
	}
	b.WriteString(`</multistatus>`)
	return parseMS(b.String())
}

func (s *fakeServer) Get(_ context.Context, path string, _ *dav.Range) (io.ReadCloser, string, error) {
	obj, ok := s.objects[path]
	if !ok {
		return nil, "", &dav.HTTPError{Code: http.StatusNotFound}
	}
	return io.NopCloser(strings.NewReader(obj.body)), "text/vcard", nil
}

func (s *fakeServer) PutOpts(_ context.Context, path string, body io.Reader, opts dav.PutOptions) (string, error) {
	s.puts++
	if s.failPrecondition {
		return "", &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	existing, exists := s.objects[path]
	if opts.IfNoneMatch && exists {
		return "", &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	if opts.IfMatch != "" && (!exists || existing.etag != opts.IfMatch) {
		return "", &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	stored := s.applyDrops(string(raw))
	etag := fmt.Sprintf("%q", fmt.Sprintf("v-put-%d", s.puts))
	s.objects[path] = &fakeObject{etag: etag, body: stored}
	s.ctag = fmt.Sprintf("ctag-put-%d", s.puts)
	return etag, nil
}

// applyDrops removes the named properties from what was sent, and stamps REV as
// a real server would.
func (s *fakeServer) applyDrops(body string) string {
	var kept []string
	for _, line := range strings.Split(body, "\r\n") {
		drop := false
		for _, name := range s.dropOnPut {
			if strings.HasPrefix(line, name+":") || strings.HasPrefix(line, name+";") {
				drop = true
				break
			}
		}
		if line == "END:VCARD" {
			kept = append(kept, "REV:20260812T101500Z")
		}
		if !drop {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\r\n")
}

func (s *fakeServer) Delete(_ context.Context, path, ifMatch string) error {
	s.deletes++
	if s.failPrecondition {
		return &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	obj, ok := s.objects[path]
	if !ok {
		return &dav.HTTPError{Code: http.StatusNotFound}
	}
	if ifMatch != "" && obj.etag != ifMatch {
		return &dav.HTTPError{Code: http.StatusPreconditionFailed}
	}
	delete(s.objects, path)
	s.ctag = "ctag-deleted"
	return nil
}

func (s *fakeServer) paths() []string {
	out := make([]string, 0, len(s.objects))
	for path := range s.objects {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func (s *fakeServer) requests() int {
	return s.propfindDepth0 + s.propfindDepth1 + s.reports + s.puts + s.deletes
}

func parseMS(body string) (*dav.MultiStatus, error) {
	return dav.ParseMultiStatus(bytes.NewReader([]byte(xml.Header + body)))
}

func escapeXML(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		panic(err)
	}
	return buf.String()
}

func newProvider(t *testing.T, srv *fakeServer, opts Options) *Provider {
	t.Helper()
	if opts.AccountID == "" {
		opts.AccountID = "acc-1"
	}
	p, err := New(srv, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestListReadsObjectMap(t *testing.T) {
	srv := newFakeServer()
	srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	srv.add("alan.vcf", `"v2"`, "FN:Alan Turing")

	p := newProvider(t, srv, Options{})
	listing, err := p.List(context.Background(), "/dav.php/addressbooks/mix/default")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listing.Collection != collection {
		t.Errorf("collection = %q, want %q", listing.Collection, collection)
	}
	if listing.CTag != "ctag-1" {
		t.Errorf("ctag = %q", listing.CTag)
	}
	if len(listing.ETags) != 2 {
		t.Fatalf("etags = %v", listing.ETags)
	}
	if listing.ETags[collection+"ada.vcf"] != `"v1"` {
		t.Errorf("etag of ada = %q", listing.ETags[collection+"ada.vcf"])
	}
	if got := listing.Paths(); len(got) != 2 || got[0] != collection+"ada.vcf" {
		t.Errorf("paths = %v", got)
	}
	if listing.FromCache {
		t.Error("the first listing claims to come from the cache")
	}
}

// TestSecondListStaysOffTheWire is the §21 cache criterion: reopening a
// collection that has not changed costs no requests at all.
func TestSecondListStaysOffTheWire(t *testing.T) {
	srv := newFakeServer()
	srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	cache := session.NewCache(session.CacheConfig{}, nil)

	p := newProvider(t, srv, Options{Cache: cache})
	if _, err := p.List(context.Background(), collection); err != nil {
		t.Fatalf("first List: %v", err)
	}
	before := srv.requests()

	listing, err := p.List(context.Background(), collection)
	if err != nil {
		t.Fatalf("second List: %v", err)
	}
	if srv.requests() != before {
		t.Errorf("the second listing made %d requests, want none", srv.requests()-before)
	}
	if !listing.FromCache {
		t.Error("the second listing does not report coming from the cache")
	}
	if len(listing.ETags) != 1 {
		t.Errorf("etags = %v", listing.ETags)
	}
}

// TestUnchangedCTagSkipsTheObjectMap covers the other half of §12: once the soft
// TTL has passed, an unchanged collection tag is enough to keep the cached map
// instead of reading every ETag again.
func TestUnchangedCTagSkipsTheObjectMap(t *testing.T) {
	srv := newFakeServer()
	srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")

	clock := time.Now()
	cache := session.NewCache(session.CacheConfig{CollectionTTL: time.Minute}, func() time.Time { return clock })
	p := newProvider(t, srv, Options{Cache: cache})
	if _, err := p.List(context.Background(), collection); err != nil {
		t.Fatalf("first List: %v", err)
	}
	deep := srv.propfindDepth1

	clock = clock.Add(2 * time.Minute)
	listing, err := p.List(context.Background(), collection)
	if err != nil {
		t.Fatalf("second List: %v", err)
	}
	if srv.propfindDepth1 != deep {
		t.Errorf("the object map was read again despite an unchanged collection tag")
	}
	if !listing.FromCache {
		t.Error("the listing does not report coming from the cache")
	}

	// A changed tag must send it back to the server.
	clock = clock.Add(2 * time.Minute)
	srv.ctag = "ctag-2"
	if _, err := p.List(context.Background(), collection); err != nil {
		t.Fatalf("third List: %v", err)
	}
	if srv.propfindDepth1 != deep+1 {
		t.Error("a changed collection tag did not force the object map to be read")
	}
}

func TestMultigetBatchesAndKeepsOrder(t *testing.T) {
	srv := newFakeServer()
	var paths []string
	for i := 0; i < 7; i++ {
		paths = append(paths, srv.add(fmt.Sprintf("c%d.vcf", i), fmt.Sprintf("%q", fmt.Sprint(i)), fmt.Sprintf("FN:Contact %d", i)))
	}
	sort.Strings(paths)

	p := newProvider(t, srv, Options{BatchSize: 3})
	result, err := p.Multiget(context.Background(), collection, paths, nil)
	if err != nil {
		t.Fatalf("Multiget: %v", err)
	}
	if len(result.Objects) != 7 {
		t.Fatalf("objects = %d, want 7", len(result.Objects))
	}
	if srv.reports != 3 {
		t.Errorf("reports = %d, want 3 batches of at most 3", srv.reports)
	}
	for i, obj := range result.Objects {
		if obj.Path != paths[i] {
			t.Errorf("object %d = %s, want %s", i, obj.Path, paths[i])
		}
	}
	if len(result.Failed) != 0 {
		t.Errorf("failed = %v", result.Failed)
	}
}

// TestMultigetServesCachedBodies checks the body cache is keyed by version: the
// same ETag is a hit, a newer one is not.
func TestMultigetServesCachedBodies(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	cache := session.NewCache(session.CacheConfig{}, nil)
	p := newProvider(t, srv, Options{Cache: cache})

	etags := map[string]string{path: `"v1"`}
	if _, err := p.Multiget(context.Background(), collection, []string{path}, etags); err != nil {
		t.Fatalf("first Multiget: %v", err)
	}
	if srv.reports != 1 {
		t.Fatalf("reports = %d, want 1", srv.reports)
	}

	if _, err := p.Multiget(context.Background(), collection, []string{path}, etags); err != nil {
		t.Fatalf("second Multiget: %v", err)
	}
	if srv.reports != 1 {
		t.Errorf("the cached body was not used: reports = %d", srv.reports)
	}

	result, err := p.Multiget(context.Background(), collection, []string{path}, map[string]string{path: `"v2"`})
	if err != nil {
		t.Fatalf("third Multiget: %v", err)
	}
	if srv.reports != 2 {
		t.Errorf("a newer version was served from the cache: reports = %d", srv.reports)
	}
	if len(result.Objects) != 1 {
		t.Fatalf("objects = %d", len(result.Objects))
	}
}

// TestMultigetKeepsGoingPastABadCard: one card this build cannot parse should
// cost that card, not the whole address book.
func TestMultigetKeepsGoingPastABadCard(t *testing.T) {
	srv := newFakeServer()
	good := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	bad := collection + "broken.vcf"
	srv.objects[bad] = &fakeObject{etag: `"v9"`, body: "this is not a vCard"}

	p := newProvider(t, srv, Options{})
	result, err := p.Multiget(context.Background(), collection, []string{good, bad}, nil)
	if err != nil {
		t.Fatalf("Multiget: %v", err)
	}
	if len(result.Objects) != 1 || result.Objects[0].Path != good {
		t.Fatalf("objects = %+v", result.Objects)
	}
	if len(result.Failed) != 1 || result.Failed[0].Path != bad {
		t.Fatalf("failed = %+v", result.Failed)
	}
	if result.Failed[0].Error() == "" {
		t.Error("the failure carries no explanation")
	}
}

func TestGetFallsBackWhenReportIsRefused(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	srv.refuseReport = true

	p := newProvider(t, srv, Options{})
	obj, err := p.Get(context.Background(), collection, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if obj.ETag != `"v1"` {
		t.Errorf("etag = %q, want the version from PROPFIND", obj.ETag)
	}
	contact, err := obj.Contact()
	if err != nil {
		t.Fatalf("Contact: %v", err)
	}
	if contact.DisplayName() != "Ada Lovelace" {
		t.Errorf("display name = %q", contact.DisplayName())
	}
}

func TestCreateWritesConditionally(t *testing.T) {
	srv := newFakeServer()
	p := newProvider(t, srv, Options{})

	obj, err := model.NewVCard("3.0", "new-uid")
	if err != nil {
		t.Fatalf("NewVCard: %v", err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("FN", "Grace Hopper")); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	result, err := p.Create(context.Background(), collection, obj)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.ETag == "" {
		t.Error("the write returned no version")
	}
	if !result.Verified || !result.Loss.Empty() {
		t.Errorf("loss = %+v, verified = %v", result.Loss, result.Verified)
	}
	stored, ok := srv.objects[collection+"new-uid.vcf"]
	if !ok {
		t.Fatalf("nothing was stored; server holds %v", srv.paths())
	}
	if !strings.Contains(stored.body, "FN:Grace Hopper") {
		t.Errorf("stored body = %q", stored.body)
	}

	// A second create at the same path must be refused rather than overwrite.
	again, err := model.NewVCard("3.0", "new-uid")
	if err != nil {
		t.Fatalf("NewVCard: %v", err)
	}
	if _, err := p.Create(context.Background(), collection, again); !IsConflict(err) {
		t.Errorf("second Create returned %v, want a conflict", err)
	}
}

func TestUpdateRefusesWithoutTheVersionItRead(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	p := newProvider(t, srv, Options{})

	obj, err := p.Get(context.Background(), collection, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	obj.ETag = ""
	if _, err := p.Update(context.Background(), collection, obj); err == nil {
		t.Fatal("Update without a version succeeded")
	}
	if srv.puts != 0 {
		t.Error("an unconditional overwrite reached the server")
	}
}

// TestUpdateReportsPropertyLoss is phase 3 of the stage: the server accepted the
// contact but dropped the X- property, and the write says so (§8).
func TestUpdateReportsPropertyLoss(t *testing.T) {
	srv := newFakeServer()
	srv.dropOnPut = []string{"X-CUSTOM-FLAG"}
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace", "X-CUSTOM-FLAG:kept")

	losses := model.NewLossRegistry(nil)
	p := newProvider(t, srv, Options{Losses: losses})

	obj, err := p.Get(context.Background(), collection, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("FN", "Ada King")); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	result, err := p.Update(context.Background(), collection, obj)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.Verified {
		t.Fatal("the write was not verified")
	}
	if got := result.Loss.Missing; len(got) != 1 || got[0] != "X-CUSTOM-FLAG" {
		t.Fatalf("missing = %v, want X-CUSTOM-FLAG", got)
	}
	if !result.ReportLoss {
		t.Error("the first loss on this account was not flagged for the person")
	}
	if !strings.Contains(result.Loss.String(), "X-CUSTOM-FLAG") {
		t.Errorf("message = %q", result.Loss.String())
	}
	if result.Object == nil || result.Object.ETag == "" {
		t.Error("the result carries no stored object to base the next edit on")
	}

	// The same loss again is aggregated rather than repeated inline.
	next := result.Object
	if err := next.Apply((&model.Patch{}).SetText("NICKNAME", "Ada")); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	second, err := p.Update(context.Background(), collection, next)
	if err != nil {
		t.Fatalf("second Update: %v", err)
	}
	if second.ReportLoss {
		t.Error("a repeated loss was reported inline again")
	}
	report := p.LossReport()
	if report.Empty() || report.Properties[0].Name != "X-CUSTOM-FLAG" {
		t.Fatalf("report = %+v", report)
	}
}

// TestUpdateKeepsForeignPropertiesEndToEnd is the §21 criterion read back off the
// server: a contact edited through the provider still carries what other clients
// left on it.
func TestUpdateKeepsForeignPropertiesEndToEnd(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`,
		"FN:Ada Lovelace",
		"CATEGORIES:Mathematics",
		"X-EVOLUTION-FILE-AS:Lovelace",
		"X-CUSTOM-FLAG;TYPE=ODD:kept")

	p := newProvider(t, srv, Options{})
	obj, err := p.Get(context.Background(), collection, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("FN", "Ada King")); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	result, err := p.Update(context.Background(), collection, obj)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.Loss.Empty() {
		t.Fatalf("loss = %+v, want none", result.Loss)
	}

	stored := srv.objects[path].body
	for _, want := range []string{
		"FN:Ada King",
		"CATEGORIES:Mathematics",
		"X-EVOLUTION-FILE-AS:Lovelace",
		"X-CUSTOM-FLAG;TYPE=ODD:kept",
		"VERSION:3.0",
	} {
		if !strings.Contains(stored, want) {
			t.Errorf("the stored card lost %q:\n%s", want, stored)
		}
	}
}

// TestUpdateOnAChangedObjectIsAConflict is §9: a refused precondition produces
// both versions and no overwrite.
func TestUpdateOnAChangedObjectIsAConflict(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	cache := session.NewCache(session.CacheConfig{}, nil)
	p := newProvider(t, srv, Options{Cache: cache})

	obj, err := p.Get(context.Background(), collection, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("FN", "Ada King")); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Someone else saves first.
	srv.objects[path] = &fakeObject{etag: `"v2"`, body: "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Edited Elsewhere\r\nEND:VCARD\r\n"}

	_, err = p.Update(context.Background(), collection, obj)
	if !IsConflict(err) {
		t.Fatalf("Update returned %v, want a conflict", err)
	}
	var conflict *ConflictError
	if !asConflict(err, &conflict) {
		t.Fatal("the conflict does not carry its details")
	}
	if conflict.Remote == nil {
		t.Fatal("the conflict carries no server version to compare against")
	}
	if got := conflict.Remote.Property("FN")[0].Text; got != "Edited Elsewhere" {
		t.Errorf("server version FN = %q", got)
	}
	if conflict.Local == nil || conflict.Local.Property("FN")[0].Text != "Ada King" {
		t.Error("the conflict does not carry the refused edit")
	}
	if body := srv.objects[path].body; strings.Contains(body, "Ada King") {
		t.Error("the refused edit was written anyway")
	}
}

func TestDeleteIsConditionalAndInvalidatesTheCache(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	cache := session.NewCache(session.CacheConfig{}, nil)
	p := newProvider(t, srv, Options{Cache: cache})

	if _, err := p.List(context.Background(), collection); err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := p.Delete(context.Background(), collection, path, ""); err == nil {
		t.Error("an unconditional delete was allowed")
	}
	if err := p.Delete(context.Background(), collection, path, `"stale"`); !IsConflict(err) {
		t.Errorf("delete on a stale version returned %v, want a conflict", err)
	}
	if err := p.Delete(context.Background(), collection, path, `"v1"`); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := srv.objects[path]; ok {
		t.Error("the object survived a delete")
	}

	listing, err := p.List(context.Background(), collection)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if listing.FromCache {
		t.Error("the listing after a delete came from the cache")
	}
	if len(listing.ETags) != 0 {
		t.Errorf("etags after delete = %v", listing.ETags)
	}
}

func TestProviderWorksWithoutCacheOrRegistry(t *testing.T) {
	srv := newFakeServer()
	path := srv.add("ada.vcf", `"v1"`, "FN:Ada Lovelace")
	p := newProvider(t, srv, Options{})

	if _, err := p.List(context.Background(), collection); err != nil {
		t.Fatalf("List: %v", err)
	}
	obj, err := p.Get(context.Background(), collection, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := obj.Apply((&model.Patch{}).SetText("NICKNAME", "Ada")); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := p.Update(context.Background(), collection, obj); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if p.LossReport().Empty() != true {
		t.Error("a provider without a registry produced a report")
	}
}

func TestNewRequiresClient(t *testing.T) {
	if _, err := New(nil, Options{}); err == nil {
		t.Error("New(nil) succeeded")
	}
}

func TestListRequiresCollection(t *testing.T) {
	p := newProvider(t, newFakeServer(), Options{})
	if _, err := p.List(context.Background(), "  "); err == nil {
		t.Error("List with no collection succeeded")
	}
}

func asConflict(err error, target **ConflictError) bool {
	for err != nil {
		if c, ok := err.(*ConflictError); ok {
			*target = c
			return true
		}
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}
