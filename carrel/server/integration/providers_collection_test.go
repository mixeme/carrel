//go:build integration

package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
)

var davDisplayName = xml.Name{Space: "DAV:", Local: "displayname"}

func setDisplayName(ctx context.Context, c *dav.Client, path, name string) error {
	return c.PropPatch(ctx, path, []dav.Prop{{Name: davDisplayName, Value: []byte(name)}}, nil)
}

func getDisplayName(ctx context.Context, c *dav.Client, path string) (string, error) {
	ms, err := c.PropFind(ctx, path, dav.DepthZero, []xml.Name{davDisplayName})
	if err != nil {
		return "", err
	}
	for _, resp := range ms.Responses {
		if v, ok := resp.Props[davDisplayName]; ok {
			return strings.TrimSpace(string(v)), nil
		}
	}
	return "", fmt.Errorf("displayname missing on %s", path)
}

// S-30.2 / S-12: create, rename, move, delete an address book on Radicale.
func TestRadicaleAddressBookCRUD(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	g := testGuard(t)
	base := strings.TrimRight(endpoint, "/")

	slug := fmt.Sprintf("carrel-book-%d", os.Getpid())
	path := "/contacts/" + slug + "/"
	moved := "/contacts/" + slug + "-moved/"

	bookBody := `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set>
    <D:prop>
      <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
      <D:displayname>Initial</D:displayname>
    </D:prop>
  </D:set>
</D:mkcol>`
	resp, err := davRequest(ctx, g, "MKCOL", base+path, bookBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("MKCOL %s: %s", path, resp.Status)
	}

	if err := setDisplayName(ctx, client, path, "Renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := getDisplayName(ctx, client, path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Renamed" {
		t.Fatalf("displayname = %q, want Renamed", got)
	}

	if err := client.Move(ctx, path, moved, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := client.Delete(ctx, moved, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// S-30.2 / S-12: create, rename, move, delete a calendar on Radicale.
func TestRadicaleCalendarCRUD(t *testing.T) {
	endpoint := radicaleEndpoint(t)
	resetRadicale(t, endpoint)
	client := testClient(t, endpoint)
	ctx := context.Background()
	g := testGuard(t)
	base := strings.TrimRight(endpoint, "/")

	slug := fmt.Sprintf("carrel-cal-%d", os.Getpid())
	path := "/cal/" + slug + "/"
	moved := "/cal/" + slug + "-moved/"

	calBody := `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set>
    <D:prop>
      <D:displayname>Initial</D:displayname>
      <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
    </D:prop>
  </D:set>
</C:mkcalendar>`
	resp, err := davRequest(ctx, g, "MKCALENDAR", base+path, calBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("MKCALENDAR %s: %s", path, resp.Status)
	}

	if err := setDisplayName(ctx, client, path, "Renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := getDisplayName(ctx, client, path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Renamed" {
		t.Fatalf("displayname = %q, want Renamed", got)
	}

	if err := client.Move(ctx, path, moved, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := client.Delete(ctx, moved, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// S-30.2 / S-17: folder create, rename, move, delete on SFTPGo.
func TestSFTPGoFolderCRUD(t *testing.T) {
	client, _ := sftpgoClient(t)
	ctx := context.Background()

	slug := fmt.Sprintf("carrel-folder-%d", os.Getpid())
	path := "/" + slug + "/"
	moved := "/" + slug + "-moved/"

	if err := client.MkCol(ctx, path); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := setDisplayName(ctx, client, path, "Renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := getDisplayName(ctx, client, path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Renamed" {
		t.Fatalf("displayname = %q, want Renamed", got)
	}
	if err := client.Move(ctx, path, moved, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	if err := client.Delete(ctx, moved, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
