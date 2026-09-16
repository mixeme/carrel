//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/provider/file"
)

const (
	contactPath = "contacts/ada.vcf"
)

// vcfCard omits X-CUSTOM when custom is empty: an empty line would still be a
// value, and the merge test checks the target carries none.
func vcfCard(uid, fn, custom string) string {
	card := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:" + uid + "\r\nFN:" + fn + "\r\n"
	if custom != "" {
		card += "X-CUSTOM:" + custom + "\r\n"
	}
	return card + "END:VCARD\r\n"
}

func icsEventCustom(uid, summary, custom string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel//Integration//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T120000Z\r\nDTSTART:20260101T100000Z\r\n" +
		"SUMMARY:" + summary + "\r\nX-CUSTOM:" + custom + "\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
}

// S-30.2: unknown vCard property survives provider round-trip (S-7).
func TestRadicaleCardForeignPropertyRoundTrip(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := contactPath
	_ = client.Delete(ctx, path, "")

	etag, err := client.Put(ctx, path, strings.NewReader(vcfCard("ada@carrel.test", "Ada", "keep-me")), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cp, err := card.New(client)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := cp.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := obj.PropText("X-CUSTOM"); len(got) != 1 || got[0] != "keep-me" {
		t.Fatalf("X-CUSTOM = %v", got)
	}

	patch, err := model.NewPatch(model.SetText{Prop: "FN", Value: "Grace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	obj.ETag = etag
	if _, err := cp.Update(ctx, obj); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := cp.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PropText("FN")[0] != "Grace" {
		t.Fatalf("FN = %v", got.PropText("FN"))
	}
	if got.PropText("X-CUSTOM")[0] != "keep-me" {
		t.Fatalf("lost foreign property: %v", got.PropText("X-CUSTOM"))
	}
	if len(card.Foreign(got)) != 1 {
		t.Fatalf("foreign fields = %+v", card.Foreign(got))
	}
}

// S-30.2: unknown calendar property survives provider round-trip (S-7).
func TestRadicaleCalForeignPropertyRoundTrip(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := calEventPath
	_ = client.Delete(ctx, path, "")

	etag, err := client.Put(ctx, path, strings.NewReader(icsEventCustom("cal-foreign@carrel.test", "Meet", "secret")), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cp, err := cal.New(client)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := cp.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := obj.PropText("X-CUSTOM"); len(got) != 1 || got[0] != "secret" {
		t.Fatalf("X-CUSTOM = %v", got)
	}

	patch, err := model.NewPatch(model.SetText{Prop: "SUMMARY", Value: "Updated"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	obj.ETag = etag
	if _, err := cp.Update(ctx, obj); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := cp.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PropText("SUMMARY")[0] != "Updated" {
		t.Fatalf("SUMMARY = %v", got.PropText("SUMMARY"))
	}
	if got.PropText("X-CUSTOM")[0] != "secret" {
		t.Fatalf("lost foreign property: %v", got.PropText("X-CUSTOM"))
	}
}

func sftpgoConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_SFTPGO_URL")
	user = os.Getenv("CARREL_SFTPGO_USER")
	pass = os.Getenv("CARREL_SFTPGO_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_SFTPGO_URL, CARREL_SFTPGO_USER, CARREL_SFTPGO_PASS required")
	}
	return endpoint, user, pass
}

func sftpgoClient(t *testing.T) (*dav.Client, *file.Provider) {
	endpoint, user, pass := sftpgoConfig(t)
	client, err := dav.NewClient(testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	return client, p
}

// SFTPGo ignores If-None-Match on PUT (docs/servers.md, SV-8,
// TestSFTPGoIgnoresIfNoneMatch): conditional create is checked on Radicale.

// S-30.2: conditional create rejects occupied names (412).
func TestRadicaleCardUploadOccupiedName(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	path := fmt.Sprintf("contacts/carrel-occupied-%d.vcf", os.Getpid())
	_ = client.Delete(ctx, path, "")

	cp, err := card.New(client)
	if err != nil {
		t.Fatal(err)
	}
	obj := model.NewObject(path, "", map[string][]model.RawValue{
		"UID": {{Text: "occupied@carrel.test"}},
		"FN":  {{Text: "First"}},
	})
	if _, err := cp.Create(ctx, path, obj); err != nil {
		t.Fatalf("create: %v", err)
	}
	dup := model.NewObject(path, "", map[string][]model.RawValue{
		"UID": {{Text: "occupied2@carrel.test"}},
		"FN":  {{Text: "Second"}},
	})
	_, err = cp.Create(ctx, path, dup)
	if err == nil {
		t.Fatal("expected occupied name to fail")
	}
	if !file.ErrOccupied(err) {
		t.Fatalf("got %v, want 412", err)
	}
	_ = client.Delete(ctx, path, "")
}

// S-30.2: large file Get reads a bounded prefix without buffering the whole body.
func TestSFTPGoGetStreams(t *testing.T) {
	client, p := sftpgoClient(t)
	ctx := context.Background()
	name := fmt.Sprintf("carrel-large-%d.bin", os.Getpid())
	path := "/" + name

	removeLater(t, client, path)
	payload := strings.Repeat("x", 2*1024*1024)
	if _, err := client.Put(ctx, path, strings.NewReader(payload), ""); err != nil {
		t.Fatalf("put: %v", err)
	}

	body, _, err := p.Get(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4096 {
		t.Fatalf("read %d bytes, want 4096", len(got))
	}
}

const (
	largeFileBytes = 2 * 1024 * 1024
	readPrefix     = 4096
	// Whole-file buffering would add ~2 MiB; allow headroom for test noise.
	maxHeapGrowth = 1024 * 1024
)

// S-30.2: reading a prefix of a large file must not buffer the whole body.
func TestSFTPGoGetLargeFileLowMemory(t *testing.T) {
	client, p := sftpgoClient(t)
	ctx := context.Background()
	name := fmt.Sprintf("carrel-rss-%d.bin", os.Getpid())
	path := "/" + name

	removeLater(t, client, path)
	payload := strings.Repeat("x", largeFileBytes)
	if _, err := client.Put(ctx, path, strings.NewReader(payload), ""); err != nil {
		t.Fatalf("put: %v", err)
	}

	before := heapAllocBytes()
	body, _, err := p.Get(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(io.LimitReader(body, readPrefix))
	body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != readPrefix {
		t.Fatalf("read %d bytes, want %d", len(got), readPrefix)
	}
	after := heapAllocBytes()
	if after > before+maxHeapGrowth {
		t.Fatalf("heap grew %d bytes (before %d, after %d), want ≤ %d",
			after-before, before, after, maxHeapGrowth)
	}
}

func baikalConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_BAIKAL_URL")
	user = os.Getenv("CARREL_BAIKAL_USER")
	pass = os.Getenv("CARREL_BAIKAL_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_BAIKAL_URL, CARREL_BAIKAL_USER, CARREL_BAIKAL_PASS required")
	}
	return endpoint, user, pass
}

func baikalClient(t *testing.T) *dav.Client {
	endpoint, user, pass := baikalConfig(t)
	client, err := dav.NewClient(testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

const baikalContactsBook = "/dav.php/addressbooks/carrel/kontakty/"

// S-30.2: second CardDAV target; same foreign-property contract as Radicale.
func TestBaikalCardForeignPropertyRoundTrip(t *testing.T) {
	client := baikalClient(t)
	ctx := context.Background()
	path := baikalContactsBook + fmt.Sprintf("carrel-foreign-%d.vcf", os.Getpid())
	_ = client.Delete(ctx, path, "")

	etag, err := client.Put(ctx, path, strings.NewReader(vcfCard("baikal@carrel.test", "Ada", "keep-me")), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	cp, err := card.New(client)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := cp.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := obj.PropText("X-CUSTOM"); len(got) != 1 || got[0] != "keep-me" {
		t.Fatalf("X-CUSTOM = %v", got)
	}

	patch, err := model.NewPatch(model.SetText{Prop: "FN", Value: "Grace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	obj.ETag = etag
	if _, err := cp.Update(ctx, obj); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := cp.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PropText("FN")[0] != "Grace" {
		t.Fatalf("FN = %v", got.PropText("FN"))
	}
	if got.PropText("X-CUSTOM")[0] != "keep-me" {
		t.Fatalf("lost foreign property: %v", got.PropText("X-CUSTOM"))
	}
	_ = client.Delete(ctx, path, "")
}

// S-30.2 / S-16 step 2: failed merged write must not delete source records.
func TestRadicaleCardMergeKeepsSourcesOnFailedWrite(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()

	targetPath := fmt.Sprintf("contacts/merge-target-%d.vcf", os.Getpid())
	sourcePath := fmt.Sprintf("contacts/merge-source-%d.vcf", os.Getpid())
	_ = client.Delete(ctx, targetPath, "")
	_ = client.Delete(ctx, sourcePath, "")

	targetETag, err := client.Put(ctx, targetPath, strings.NewReader(vcfCard("target@carrel.test", "Target", "")), "")
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	sourceETag, err := client.Put(ctx, sourcePath, strings.NewReader(vcfCard("source@carrel.test", "Source", "from-source")), "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	cp, err := card.New(client)
	if err != nil {
		t.Fatal(err)
	}
	target, err := cp.Get(ctx, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	source, err := cp.Get(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	merged := target
	patch, err := model.NewPatch(
		model.AddValue{Prop: "X-CUSTOM", Value: source.PropText("X-CUSTOM")[0]},
		model.SetText{Prop: "NOTE", Value: "merged"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(merged); err != nil {
		t.Fatal(err)
	}
	merged.ETag = `"wrong"`
	_, err = cp.Update(ctx, merged)
	if err == nil {
		t.Fatal("expected merged write to fail on stale ETag")
	}
	if !dav.IsStatus(err, http.StatusPreconditionFailed) && !dav.IsStatus(err, http.StatusConflict) {
		t.Fatalf("update error = %v, want 412 or 409", err)
	}

	gotTarget, err := cp.Get(ctx, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget.PropText("FN")[0] != "Target" {
		t.Fatalf("target FN = %v", gotTarget.PropText("FN"))
	}
	if len(gotTarget.PropText("X-CUSTOM")) != 0 {
		t.Fatalf("target should not have merged property: %v", gotTarget.PropText("X-CUSTOM"))
	}
	if gotTarget.ETag != targetETag {
		t.Fatalf("target ETag changed: %q -> %q", targetETag, gotTarget.ETag)
	}

	gotSource, err := cp.Get(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if gotSource.PropText("FN")[0] != "Source" {
		t.Fatalf("source FN = %v", gotSource.PropText("FN"))
	}
	if gotSource.PropText("X-CUSTOM")[0] != "from-source" {
		t.Fatalf("source X-CUSTOM = %v", gotSource.PropText("X-CUSTOM"))
	}
	if gotSource.ETag != sourceETag {
		t.Fatalf("source ETag changed: %q -> %q", sourceETag, gotSource.ETag)
	}
}
