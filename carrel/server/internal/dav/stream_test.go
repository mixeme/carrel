package dav

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// S-17: a file passes Carrel whole. The client's cap (1 MiB here) still cuts
// Get, which records parse into memory; Open passes three times as much, with
// the headers a browser needs.
func TestOpenPassesTheWholeFile(t *testing.T) {
	body := strings.Repeat("x", 3<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Last-Modified", "Sun, 13 Sep 2026 10:00:00 GMT")
		if r.Header.Get("Range") == "bytes=10-19" {
			w.Header().Set("Content-Range", "bytes 10-19/"+strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, body[10:20])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	c := testClient(t, srv.URL)

	rc, _, err := c.Get(t.Context(), "/big.pdf", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(rc); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("Get read past the cap: %v", err)
	}
	rc.Close()

	d, err := c.Open(t.Context(), "/big.pdf", nil)
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(io.Discard, d.Body)
	d.Body.Close()
	if err != nil || n != int64(len(body)) {
		t.Fatalf("Open read %d bytes: %v", n, err)
	}
	if d.Status != http.StatusOK || d.Length != int64(len(body)) || d.Type != "application/pdf" || d.ETag != "v1" || d.Modified == "" {
		t.Errorf("download = %+v", d)
	}

	part, err := c.Open(t.Context(), "/big.pdf", &Range{Start: 10, End: 19})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(part.Body)
	part.Body.Close()
	if part.Status != http.StatusPartialContent || part.Range != "bytes 10-19/3145728" || len(got) != 10 {
		t.Errorf("range = %+v, %d bytes", part, len(got))
	}
}

// slowClient has a request timeout of 300 ms.
func slowClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	c := testConfig()
	c.DAVSSRFAllowlist = "127.0.0.1"
	c.DAVRequestTimeout = 300 * time.Millisecond
	client, err := NewClient(NewGuard(c), endpoint, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// dripReader gives n bytes, one every step.
type dripReader struct {
	n    int
	step time.Duration
}

func (r *dripReader) Read(p []byte) (int, error) {
	if r.n == 0 {
		return 0, io.EOF
	}
	time.Sleep(r.step)
	r.n--
	p[0] = 'x'
	return 1, nil
}

// S-17: CARREL_DAV_REQUEST_TIMEOUT bounds a record request whole, body
// included, and only silence on a file. A download or an upload that keeps
// moving for longer than the timeout passes; a stream that stops is cut, and
// the failure is a timeout, as http.Client.Timeout's was.
func TestRequestTimeoutCountsSilenceOnStreams(t *testing.T) {
	const chunks, step = 10, 80 * time.Millisecond // 800 ms in all
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			if _, err := io.Copy(io.Discard, r.Body); err != nil {
				return
			}
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/stalls":
			_, _ = io.WriteString(w, "x")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		default:
			for range chunks {
				_, _ = io.WriteString(w, "x")
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					return
				case <-time.After(step):
				}
			}
		}
	}))
	t.Cleanup(srv.Close)
	c := slowClient(t, srv.URL)

	d, err := c.Open(t.Context(), "/drips", nil)
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(io.Discard, d.Body)
	d.Body.Close()
	if err != nil || n != chunks {
		t.Errorf("Open of a moving file: %d bytes, %v", n, err)
	}

	rc, _, err := c.Get(t.Context(), "/drips", nil)
	if err == nil {
		_, err = io.Copy(io.Discard, rc)
		rc.Close()
	}
	if !errors.Is(err, ErrTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Get past the timeout: %v", err)
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Errorf("Get past the timeout is not a net timeout: %v", err)
	}

	d, err = c.Open(t.Context(), "/stalls", nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = io.Copy(io.Discard, d.Body)
	d.Body.Close()
	if !errors.Is(err, ErrTimeout) || time.Since(start) > 2*time.Second {
		t.Errorf("Open of a stalled file: %v after %v", err, time.Since(start))
	}

	if _, err := c.Put(Stream(t.Context()), "/up", Sized(&dripReader{n: chunks, step: step}, chunks), ""); err != nil {
		t.Errorf("upload that keeps moving: %v", err)
	}
	if _, err := c.Put(t.Context(), "/up", Sized(&dripReader{n: chunks, step: step}, chunks), ""); !errors.Is(err, ErrTimeout) {
		t.Errorf("record PUT past the timeout: %v", err)
	}
}

// onlyReader hides the type of its reader from http.NewRequest.
type onlyReader struct{ io.Reader }

// A body http.NewRequest cannot measure goes out with the length Sized gives,
// in either order with Typed; without a label it goes chunked.
func TestSizedBodyIsNotChunked(t *testing.T) {
	type seen struct {
		length  int64
		chunked bool
		ct      string
	}
	var got []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if len(b) != 5 {
			t.Errorf("body %q", b)
		}
		got = append(got, seen{r.ContentLength, len(r.TransferEncoding) > 0, r.Header.Get("Content-Type")})
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	c := testClient(t, srv.URL)
	body := func() io.Reader { return onlyReader{strings.NewReader("hello")} }
	if _, err := c.Put(t.Context(), "/a", Sized(body(), 5), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutCreate(t.Context(), "/b", Typed(Sized(body(), 5), "text/plain")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutCreate(t.Context(), "/c", Sized(Typed(body(), "text/plain"), 5)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Put(t.Context(), "/d", body(), ""); err != nil {
		t.Fatal(err)
	}
	want := []seen{{5, false, ""}, {5, false, "text/plain"}, {5, false, "text/plain"}, {-1, true, ""}}
	if len(got) != len(want) {
		t.Fatalf("requests = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("PUT %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// S-17.3 needs a taken name told from other refusals: MOVE and MKCOL answer
// in StatusError, a MOVE without overwrite says so, and a 207 from MOVE or
// DELETE of a folder is not a success.
func TestMoveMkColAndDeleteRefusals(t *testing.T) {
	var overwrite, destination string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "MOVE":
			overwrite, destination = r.Header.Get("Overwrite"), r.Header.Get("Destination")
			if r.URL.Path == "/partial/" {
				w.WriteHeader(http.StatusMultiStatus)
				return
			}
			w.WriteHeader(http.StatusPreconditionFailed)
		case "MKCOL":
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodDelete:
			w.WriteHeader(http.StatusMultiStatus)
		}
	}))
	t.Cleanup(srv.Close)
	c := testClient(t, srv.URL)

	if err := c.Move(t.Context(), "/a b.txt", "/dir/c d.txt", false); !IsStatus(err, http.StatusPreconditionFailed) {
		t.Errorf("MOVE onto a taken name: %v", err)
	}
	if overwrite != "F" || destination != srv.URL+"/dir/c%20d.txt" {
		t.Errorf("Overwrite %q, Destination %q", overwrite, destination)
	}
	if err := c.Move(t.Context(), "/partial/", "/elsewhere/", true); !IsStatus(err, http.StatusMultiStatus) || overwrite != "T" {
		t.Errorf("MOVE with a 207: %v, Overwrite %q", err, overwrite)
	}
	if err := c.MkCol(t.Context(), "/dir/"); !IsStatus(err, http.StatusMethodNotAllowed) {
		t.Errorf("MKCOL on a taken name: %v", err)
	}
	if err := c.Delete(t.Context(), "/dir/", ""); !IsStatus(err, http.StatusMultiStatus) {
		t.Errorf("DELETE with a 207: %v", err)
	}
}
