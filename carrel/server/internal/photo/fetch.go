package photo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrLink: a card's photo link that is not http or https.
var ErrLink = errors.New("photo: the link is not http or https")

// Fetch reads a photo a card links to (PHOTO as a URI). The card provider
// never fetches it; this does, through the client it is given, which is the
// guard's (S-5.1): no credentials go with it, and the guard's cap on a reply
// holds. The reply's Content-Type names the format, and the bytes have to be
// that format — nothing is guessed.
func Fetch(ctx context.Context, hc *http.Client, link string, maxPixels int64) (Source, error) {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Source{}, ErrLink
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Source{}, ErrLink
	}
	resp, err := hc.Do(req)
	if err != nil {
		return Source{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Source{}, fmt.Errorf("photo: %s answered %d", u.Host, resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "image/") {
		return Source{}, fmt.Errorf("%w: %s sent %q", ErrFormat, u.Host, ct)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Source{}, err
	}
	return Open(data, ct, maxPixels)
}
