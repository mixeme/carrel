package dav

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type mutateRecord struct {
	method  string
	path    string
	ifMatch string
	dest    string
}

type mutateFake struct {
	mu       sync.Mutex
	records  []mutateRecord
	deleteOK bool // when false, DELETE returns 404
}

func (f *mutateFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := mutateRecord{
			method:  r.Method,
			path:    r.URL.Path,
			ifMatch: r.Header.Get("If-Match"),
			dest:    r.Header.Get("Destination"),
		}
		if r.Header.Get("If-None-Match") != "" {
			rec.ifMatch = r.Header.Get("If-None-Match")
		}
		f.mu.Lock()
		f.records = append(f.records, rec)
		f.mu.Unlock()

		switch r.Method {
		case http.MethodPut:
			w.Header().Set("ETag", `"new-etag"`)
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			if f.deleteOK {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case "MOVE":
			w.WriteHeader(http.StatusCreated)
		case "PROPPATCH":
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><multistatus xmlns="DAV:"/>`))
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func (f *mutateFake) last() mutateRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.records) == 0 {
		return mutateRecord{}
	}
	return f.records[len(f.records)-1]
}

func TestPutSendsIfMatch(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	etag, err := client.Put(t.Context(), "/cal/personal/a.ics", strings.NewReader("data"), `"old-etag"`)
	if err != nil {
		t.Fatal(err)
	}
	if etag != "new-etag" {
		t.Fatalf("etag = %q", etag)
	}
	got := fake.last()
	if got.method != http.MethodPut {
		t.Fatalf("method = %q", got.method)
	}
	if got.ifMatch != `"old-etag"` {
		t.Fatalf("If-Match = %q", got.ifMatch)
	}
}

func TestDelete404IsSuccess(t *testing.T) {
	fake := &mutateFake{deleteOK: false}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.Delete(t.Context(), "/missing.ics", `"gone"`); err != nil {
		t.Fatalf("404 delete should succeed: %v", err)
	}
	got := fake.last()
	if got.method != http.MethodDelete {
		t.Fatalf("method = %q", got.method)
	}
	if got.ifMatch != `"gone"` {
		t.Fatalf("If-Match = %q", got.ifMatch)
	}
}

func TestDeleteSendsIfMatch(t *testing.T) {
	fake := &mutateFake{deleteOK: true}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.Delete(t.Context(), "/cal/personal/a.ics", `"etag-1"`); err != nil {
		t.Fatal(err)
	}
	if got := fake.last(); got.ifMatch != `"etag-1"` {
		t.Fatalf("If-Match = %q", got.ifMatch)
	}
}

func TestMoveOverwrite(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.Move(t.Context(), "/src.ics", "/dst.ics", true); err != nil {
		t.Fatal(err)
	}
	got := fake.last()
	if got.method != "MOVE" {
		t.Fatalf("method = %q", got.method)
	}
	if !strings.Contains(got.dest, "dst.ics") {
		t.Fatalf("Destination = %q", got.dest)
	}
}

func TestPropPatchAgainstFakeServer(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	set := []Prop{{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: []byte("Cal")}}
	remove := []xml.Name{{Space: "DAV:", Local: "getcontentlength"}}
	if err := client.PropPatch(t.Context(), "/cal/personal/", set, remove); err != nil {
		t.Fatal(err)
	}
	if got := fake.last(); got.method != "PROPPATCH" {
		t.Fatalf("method = %q", got.method)
	}
}

// Radicale answered 400 "no element found": the body was cut before the
// encoder flushed. The fake server above never read it.
func TestPropPatchBodyIsCompleteXML(t *testing.T) {
	set := []Prop{{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: []byte("Renamed")}}
	remove := []xml.Name{{Space: "DAV:", Local: "getcontentlength"}}
	body, err := propPatchBody(set, remove)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		XMLName xml.Name `xml:"DAV: propertyupdate"`
		Set     struct {
			Name string `xml:"DAV: prop>displayname"`
		} `xml:"DAV: set"`
		Remove struct {
			Len *struct{} `xml:"DAV: prop>getcontentlength"`
		} `xml:"DAV: remove"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("body is not XML: %v\n%s", err, body)
	}
	if doc.Set.Name != "Renamed" {
		t.Fatalf("displayname = %q\n%s", doc.Set.Name, body)
	}
	if doc.Remove.Len == nil {
		t.Fatalf("remove lost getcontentlength\n%s", body)
	}
}

func TestMkColBodyIsCompleteXML(t *testing.T) {
	props := []Prop{{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: []byte("Book")}}
	body, err := mkcolBody("MKCOL", props)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		XMLName xml.Name `xml:"DAV: mkcol"`
		Name    string   `xml:"DAV: set>prop>displayname"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("body is not XML: %v\n%s", err, body)
	}
	if doc.Name != "Book" {
		t.Fatalf("displayname = %q\n%s", doc.Name, body)
	}
}

func TestMkColPropsAgainstFakeServer(t *testing.T) {
	fake := &mutateFake{}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()

	client := testClient(t, srv.URL)
	props := []Prop{{Name: xml.Name{Space: "DAV:", Local: "resourcetype"}, Value: []byte(`<collection xmlns="DAV:"/>`)}}
	if err := client.MkColProps(t.Context(), "MKCOL", "/cal/new/", props); err != nil {
		t.Fatal(err)
	}
	if got := fake.last(); got.method != "MKCOL" {
		t.Fatalf("method = %q", got.method)
	}
}

func TestMkColAgainstFakeServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "MKCOL" {
			t.Fatalf("method = %q", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Fatalf("MkCol should not send a body, got %d bytes", len(body))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := testClient(t, srv.URL)
	if err := client.MkCol(t.Context(), "/cal/new/"); err != nil {
		t.Fatal(err)
	}
}

// Prop.Value went out as text (v0.0.46), so resourcetype with
// <collection/> reached the server escaped. The value is XML content now, and
// text goes through TextValue.
func TestPropValueIsXMLContent(t *testing.T) {
	caldavNS := "urn:ietf:params:xml:ns:caldav"
	props := []Prop{
		{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: TextValue("Trips & <plans>")},
		{Name: xml.Name{Space: caldavNS, Local: "supported-calendar-component-set"},
			Value: []byte(`<comp xmlns="` + caldavNS + `" name="VEVENT"/><comp xmlns="` + caldavNS + `" name="VTODO"/>`)},
	}
	body, err := mkcolBody("MKCALENDAR", props)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav mkcalendar"`
		Prop    struct {
			Name  string `xml:"DAV: displayname"`
			Comps struct {
				Comp []struct {
					Name string `xml:"name,attr"`
				} `xml:"urn:ietf:params:xml:ns:caldav comp"`
			} `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
		} `xml:"DAV: set>prop"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("body is not a mkcalendar: %v\n%s", err, body)
	}
	if doc.Prop.Name != "Trips & <plans>" {
		t.Errorf("displayname = %q\n%s", doc.Prop.Name, body)
	}
	if c := doc.Prop.Comps.Comp; len(c) != 2 || c[0].Name != "VEVENT" || c[1].Name != "VTODO" {
		t.Errorf("components = %+v\n%s", c, body)
	}

	book, err := mkcolBody("MKCOL", []Prop{{Name: xml.Name{Space: "DAV:", Local: "resourcetype"},
		Value: []byte(`<collection xmlns="DAV:"/><addressbook xmlns="urn:ietf:params:xml:ns:carddav"/>`)}})
	if err != nil {
		t.Fatal(err)
	}
	var mk struct {
		XMLName xml.Name `xml:"DAV: mkcol"`
		Type    struct {
			Collection  *struct{} `xml:"DAV: collection"`
			AddressBook *struct{} `xml:"urn:ietf:params:xml:ns:carddav addressbook"`
		} `xml:"DAV: set>prop>resourcetype"`
	}
	if err := xml.Unmarshal(book, &mk); err != nil || mk.Type.Collection == nil || mk.Type.AddressBook == nil {
		t.Errorf("resourcetype lost its elements (%v)\n%s", err, book)
	}

	if _, err := mkcolBody("MKCOL", []Prop{{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: []byte("a < b")}}); err == nil {
		t.Error("unescaped text went out as XML")
	}
}

// A 207 is not success by itself: the property that was refused says so
// inside it, and the rename did not happen.
func TestPropPatchReportsRefusedProperty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:" xmlns:I="http://apple.com/ns/ical/"><D:response><D:href>/cal/</D:href>`+
			`<D:propstat><D:prop><D:displayname/></D:prop><D:status>HTTP/1.1 424 Failed Dependency</D:status></D:propstat>`+
			`<D:propstat><D:prop><I:calendar-color/></D:prop><D:status>HTTP/1.1 403 Forbidden</D:status></D:propstat>`+
			`</D:response></D:multistatus>`)
	}))
	defer srv.Close()

	client := testClient(t, srv.URL)
	err := client.PropPatch(t.Context(), "/cal/", []Prop{{Name: xml.Name{Space: "DAV:", Local: "displayname"}, Value: TextValue("New")}}, nil)
	if !IsStatus(err, http.StatusForbidden) || !strings.Contains(err.Error(), "calendar-color") {
		t.Fatalf("err = %v, want the refused property's 403", err)
	}
}
