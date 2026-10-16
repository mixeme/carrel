package publish

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

// A subscription is the other half of S-25: somebody else's feed — one
// .ics or .vcf at an address — read on every visit and never kept (C-2).
// The address is typed by a person, so the request goes through the guard
// like any other (S-5.1); the caller hands in the guarded client.

// ErrFeedTooLarge is a feed over the response ceiling (C-12).
var ErrFeedTooLarge = errors.New("publish: the feed is larger than CARREL_DAV_MAX_RESPONSE_BYTES")

// ErrNotAFeed is an answer that is neither iCalendar nor vCard.
var ErrNotAFeed = errors.New("publish: the address answered with neither a calendar nor an address book")

// Feed is one read of a feed: what came, how big, how long it took.
type Feed struct {
	Status int
	Type   string
	Size   int64
	Took   time.Duration
	Doc    []byte
}

// carrelLink is a secret link of another Carrel: /p/{token}, whose file is
// /p/{token}/download.
var carrelLink = regexp.MustCompile(`/p/[A-Za-z0-9_-]{40,}/?$`)

// FeedURL is the address a feed is fetched from: webcal:// is https://, and a
// secret link of another Carrel is its download.
func FeedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("publish: %q is not an address", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "webcal", "webcals":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("publish: %s:// is not a feed address", u.Scheme)
	}
	if carrelLink.MatchString(u.Path) {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/download"
	}
	return u.String(), nil
}

// FolderURL is the WebDAV address of a folder someone shares: a secret link
// of another Carrel is its read-only mount, /p/{token}/dav/; any other address
// is taken as it is. A folder address ends in "/".
func FolderURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("publish: %q is not an address", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	case "webdav", "webdavs", "dav", "davs":
		u.Scheme = "https"
	default:
		return "", fmt.Errorf("publish: %s:// is not a folder address", u.Scheme)
	}
	if carrelLink.MatchString(u.Path) {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/dav/"
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u.String(), nil
}

// nextcloudShare is the path of a Nextcloud or ownCloud share link:
// /s/{token}, or /index.php/s/{token}, under wherever the server lives.
var nextcloudShare = regexp.MustCompile(`^(.*?)/(?:index\.php/)?s/([A-Za-z0-9]{8,64})/?$`)

// ShareLink turns the link a Nextcloud or ownCloud user sends — a page for
// people, not WebDAV — into the public WebDAV of that share: the address,
// and the token as its login; a password of the link goes with it as the
// password. ok is false for any other address.
func ShareLink(raw string) (addr, login string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", "", false
	}
	m := nextcloudShare.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", false
	}
	u.Path, u.RawQuery, u.Fragment = strings.TrimSuffix(m[1], "/")+"/public.php/webdav/", "", ""
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		u.Scheme = "https"
	}
	return u.String(), m[2], true
}

// Fetch reads a feed whole, with Basic credentials when login is set, up to
// max bytes.
func Fetch(ctx context.Context, client *http.Client, raw, login, password string, max int64) (Feed, error) {
	addr, err := FeedURL(raw)
	if err != nil {
		return Feed{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return Feed{}, err
	}
	req.Header.Set("Accept", "text/calendar, text/vcard, text/x-vcard;q=0.9, */*;q=0.1")
	if login != "" {
		req.SetBasicAuth(login, password)
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Feed{}, err
	}
	defer resp.Body.Close()
	f := Feed{Status: resp.StatusCode, Type: resp.Header.Get("Content-Type")}
	if resp.StatusCode != http.StatusOK {
		f.Took = time.Since(start)
		return f, &StatusError{Code: resp.StatusCode, Path: req.URL.Path, Status: resp.Status}
	}
	doc, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	f.Took, f.Size = time.Since(start), int64(len(doc))
	if err != nil {
		return f, err
	}
	if int64(len(doc)) > max {
		return f, ErrFeedTooLarge
	}
	f.Doc = doc
	return f, nil
}

// Parsed is what a feed holds, by the kind each record lands in, and the
// name it gives itself (X-WR-CALNAME, or the address book's first FN).
type Parsed struct {
	Records map[view.Kind][]*model.Object
	Name    string
}

// Kinds are the kinds a feed carries, in the order of the sections.
func (p Parsed) Kinds() []view.Kind {
	var out []view.Kind
	for _, k := range []view.Kind{view.KindContact, view.KindEvent, view.KindTask, view.KindNote} {
		if len(p.Records[k]) > 0 {
			out = append(out, k)
		}
	}
	return out
}

var calName = regexp.MustCompile(`(?m)^X-WR-CALNAME:(.*?)\r?$`)

// Parse splits a feed into its records. A record gets an address of its own
// under the feed — "feed/{uid}" — since nothing on a server holds it.
func Parse(doc []byte, now time.Time) (Parsed, error) {
	p := Parsed{Records: map[view.Kind][]*model.Object{}}
	text := strings.TrimSpace(string(doc))
	switch {
	case strings.HasPrefix(strings.ToUpper(text), "BEGIN:VCALENDAR"):
		objs, _, err := cal.Split(doc, now)
		if err != nil {
			return p, err
		}
		for _, o := range objs {
			o.Path = "feed/" + cal.UID(o)
			k := cal.Kind(o)
			p.Records[k] = append(p.Records[k], o)
		}
		if m := calName.FindStringSubmatch(text); m != nil {
			p.Name = strings.TrimSpace(m[1])
		}
	case strings.HasPrefix(strings.ToUpper(text), "BEGIN:VCARD"):
		objs, _, err := card.Split(doc)
		if err != nil {
			return p, err
		}
		for _, o := range objs {
			o.Path = "feed/" + card.UID(o)
		}
		p.Records[view.KindContact] = objs
	default:
		return p, ErrNotAFeed
	}
	return p, nil
}

// StatusError is a feed's answer other than 200: the card of the
// subscription tells a password that stopped fitting (401) from the rest
// (Нс25).
type StatusError struct {
	Code         int
	Path, Status string
}

func (e *StatusError) Error() string { return fmt.Sprintf("GET %s → %s", e.Path, e.Status) }
