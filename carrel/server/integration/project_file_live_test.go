//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
)

// The candidate «Projects», its second carrier (docs/plan/server.md, the
// owner's idea of 27.11.2026: the project task as an .ics file on WebDAV,
// for an account whose servers hold no tasks but give files). What decides
// it on a file server: the file comes back byte for byte, and a write with a
// stale If-Match is refused — the guard against two edits (S-8) a CalDAV
// server must give (RFC 4791) and a file server may not. Each answer is
// logged with the marker «project-file»; only a transport error fails.

func TestSFTPGoProjectFile(t *testing.T) {
	client, _ := sftpgoClient(t)
	liveProjectFile(t, client, fmt.Sprintf("/carrel-project-%d.ics", os.Getpid()))
}

func TestOpenCloudProjectFile(t *testing.T) {
	client, _, root := opencloudClient(t)
	liveProjectFile(t, client, fmt.Sprintf("%s/carrel-project-%d.ics", root, os.Getpid()))
}

func TestNextcloudProjectFile(t *testing.T) {
	client, _, home := nextcloudFiles(t)
	liveProjectFile(t, client, fmt.Sprintf("%scarrel-project-%d.ics", home, os.Getpid()))
}

func quotedTag(e string) string {
	if strings.HasPrefix(e, `"`) || strings.HasPrefix(e, "W/") {
		return e
	}
	return `"` + e + `"`
}

func liveProjectFile(t *testing.T, client *dav.Client, path string) {
	ctx := context.Background()
	suffix := fmt.Sprintf("file-%d", os.Getpid())
	doc := matrixCal("BEGIN:VTODO\n" + strings.Join(projectLines("project-"+suffix, suffix), "\n") + "\nEND:VTODO")
	doc = strings.ReplaceAll(doc, "\n", "\r\n")
	removeLater(t, client, path)
	// Registered after removeLater, so it runs first: a delete OpenCloud
	// answers 425 is sent again until the file is done with.
	t.Cleanup(func() {
		for start := time.Now(); time.Since(start) < 60*time.Second; time.Sleep(time.Second) {
			if err := client.Delete(ctx, path, ""); err == nil || !strings.Contains(err.Error(), "425") {
				return
			}
		}
	})
	if _, err := client.PutCreate(ctx, path, strings.NewReader(doc)); err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	read := func() (string, string) {
		rc, tag, err := client.Get(ctx, path, nil)
		// OpenCloud answers 425 Too Early on a file it is still processing
		// after the upload (27.11.2026): how long, is measured here.
		start := time.Now()
		for err != nil && strings.Contains(err.Error(), "425") && time.Since(start) < 60*time.Second {
			time.Sleep(time.Second)
			rc, tag, err = client.Get(ctx, path, nil)
		}
		if time.Since(start) > time.Second {
			t.Logf("project-file: GET answered 425 Too Early for %s after the write", time.Since(start).Round(time.Second))
		}
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(b), tag
	}
	body, tag1 := read()
	if body == doc {
		t.Logf("project-file: the file came back byte for byte (%d bytes)", len(doc))
	} else {
		t.Logf("project-file: the file came back CHANGED:\n%s", body)
	}
	if tag1 == "" {
		t.Logf("project-file: GET gave NO ETag — a write cannot be made conditional")
		return
	}
	t.Logf("project-file: GET gave ETag %s", tag1)

	edited := strings.Replace(doc, "SUMMARY:Ремонт офиса", `SUMMARY:Ремонт офиса\, этап 2`, 1)
	_, err := client.Put(ctx, path, strings.NewReader(edited), quotedTag("not-the-tag-0000"))
	if err != nil {
		t.Logf("project-file: PUT with a wrong If-Match refused: %v", err)
	} else {
		t.Logf("project-file: PUT with a wrong If-Match WRITTEN — the server does not keep If-Match")
	}
	body, tag2 := read()
	if strings.Contains(body, "этап 2") {
		// Put the first version back, so the steps below start the same.
		if _, err := client.Put(ctx, path, strings.NewReader(doc), ""); err != nil {
			t.Fatal(err)
		}
		body, tag1 = read()
		t.Logf("project-file: written back, ETag now %s", tag1)
	} else {
		tag1 = tag2
	}

	if _, err := client.Put(ctx, path, strings.NewReader(edited), quotedTag(tag1)); err != nil {
		t.Logf("project-file: PUT with the right If-Match REFUSED: %v", err)
		return
	}
	_, tag3 := read()
	t.Logf("project-file: PUT with the right If-Match written, ETag %s → %s", tag1, tag3)
	if tag3 == tag1 {
		t.Logf("project-file: the ETag did NOT change on a write")
	}

	stale := strings.Replace(edited, "этап 2", "этап 3", 1)
	if _, err := client.Put(ctx, path, strings.NewReader(stale), quotedTag(tag1)); err != nil {
		t.Logf("project-file: PUT with a stale If-Match refused: %v", err)
	} else {
		t.Logf("project-file: PUT with a stale If-Match WRITTEN — a second edit overwrites the first")
	}
}
