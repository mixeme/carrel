package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/carddav"
)

func unreadableBookFixture(t *testing.T) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	book := davtest.AddressBookHome + "people/"
	srv.AddAddressBook(carddav.AddressBook{Path: book, Name: "People"})
	for _, uid := range []string{"ada", "broken", "bob"} {
		srv.Seed(book+uid+".vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:"+uid+"\nFN:"+uid+"\nEND:VCARD\n")
	}
	target, _ := url.Parse(srv.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	cards := regexp.MustCompile(`(?s)BEGIN:VCARD.*?END:VCARD`)
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Request.Method != "REPORT" {
			return nil
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		data = cards.ReplaceAllFunc(data, func(b []byte) []byte {
			if strings.Contains(string(b), "UID:broken") {
				return []byte(strings.TrimSuffix(string(b), "END:VCARD"))
			}
			return b
		})
		resp.Body = io.NopCloser(strings.NewReader(string(data)))
		resp.ContentLength = int64(len(data))
		resp.Header.Del("Content-Length")
		return nil
	}
	far := httptest.NewServer(proxy)
	t.Cleanup(far.Close)
	srv.URL = far.URL
	return connectFixture(t, srv, book, nil)
}

func TestUnreadableContactSearchKeepsResults(t *testing.T) {
	for _, mode := range []string{"query", "multiget", "hollow"} {
		t.Run(mode, func(t *testing.T) {
			f := unreadableBookFixture(t)
			f.srv.RefuseCardQuery = mode == "multiget"
			f.srv.HollowCardQuery = mode == "hollow"
			tickAll(t, f.h, f.sess)
			page := search(t, f.h, f.sess, "q=b")
			bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
			src := sourceNamed(t, bar, "People")
			if src.Count == nil || *src.Count != 1 || !strings.Contains(string(src.Message), "broken.vcf") {
				t.Fatalf("partial search: %+v", src)
			}
			page = search(t, f.h, f.sess, "q=b")
			if len(page.Results) != 1 || page.Results[0].Title != "bob" {
				t.Fatalf("results: %+v", page.Results)
			}
		})
	}
}

func TestUnreadableContactBackupIsNotWhole(t *testing.T) {
	f := unreadableBookFixture(t)
	f.srv.SeedFolder(filesDocs)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: f.id, Path: filesDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})
	})
	form := jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw")
	form["src"] = []string{sel(f, f.col)}
	run := runJob(t, f, saveJob(t, f, form))
	if run.State != "partial" || run.Whole != 0 || !strings.Contains(run.Error, "broken.vcf") {
		t.Fatalf("incomplete book called whole: %+v", run)
	}
}

func TestUnreadableContactListAndExport(t *testing.T) {
	f := unreadableBookFixture(t)
	// Repeating the read must not hide the diagnostic in a successful cache entry.
	for i := 0; i < 2; i++ {
		page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
		if !hasNotice(page.Notices, "1 record not shown: invalid vCard.") || page.Bar.Loaded != 2 {
			t.Fatalf("read %d: loaded %d, notices %+v", i, page.Bar.Loaded, page.Notices)
		}
		if page.Bar.Total == nil || *page.Bar.Total != 3 {
			t.Fatalf("total = %v", page.Bar.Total)
		}
		if page.Notices[0].Details == nil || !strings.Contains(plainLines(page.Notices[0].Details.Lines), "broken.vcf") {
			t.Fatalf("missing resource path: %+v", page.Notices)
		}
	}
	rec := f.h.do(get(f.path("contacts")+"/export", f.sess))
	page := decode[view.ContactsListPage](t, rec)
	if rec.Header().Get("Content-Disposition") != "" || !hasNotice(page.Notices, "The file would lack 1 record") {
		t.Fatalf("silent incomplete export: %v, %+v", rec.Header(), page.Notices)
	}
	exported := download(t, f, f.path("contacts")+"/export?without=1", "people.vcf", "text/vcard")
	if strings.Count(exported, "BEGIN:VCARD") != 2 || strings.Contains(exported, "UID:broken") {
		t.Fatalf("explicit export: %s", exported)
	}
}
