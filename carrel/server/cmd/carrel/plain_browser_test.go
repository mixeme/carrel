package main

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"carrel.dev/carrel/server/internal/conf"
)

// A browser without scripts (U-8): it goes by links and sends forms, as a
// browser with scripts off does — the hidden fields of a form, the boxes
// ticked, the options chosen, the button pressed and its own name and
// address, a button outside its form naming it by form=, files as
// multipart. It never reads JSON and never sends a header a script would.
type plainBrowser struct {
	t      *testing.T
	origin string
	client *http.Client
	page   *plainPage
}

// plainPage is a page as the browser holds it: where it came from, its
// status and its tree.
type plainPage struct {
	url    *url.URL
	status int
	body   string
	doc    *html.Node
}

// startPlain starts an instance served with its client's pages, whose
// outbound requests may reach the stands of the test (allow, as host:port
// or host), and a browser at its door.
func startPlain(t *testing.T, allow ...string) *plainBrowser {
	t.Helper()
	return startPlainOn(t, 0, allow...)
}

// startPlainOn is startPlain on a port of the caller's (the acceptance
// stand's, TestStand); 0 takes one the machine has free.
func startPlainOn(t *testing.T, port int, allow ...string) *plainBrowser {
	t.Helper()
	if port == 0 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
	}
	t.Setenv("CARREL_DATA_DIR", t.TempDir())
	t.Setenv("CARREL_BIND", "127.0.0.1")
	t.Setenv("CARREL_PORT", strconv.Itoa(port))
	t.Setenv("CARREL_DAV_SSRF_ALLOWLIST", strings.Join(allow, ","))
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("serve did not return")
		}
	})
	origin := "http://127.0.0.1:" + strconv.Itoa(port)
	for i := 0; ; i++ {
		if resp, err := http.Get(origin + "/healthz"); err == nil {
			resp.Body.Close()
			break
		}
		if i == 100 {
			t.Fatal("the server did not come up")
		}
		time.Sleep(20 * time.Millisecond)
	}
	jar, _ := cookiejar.New(nil)
	return &plainBrowser{t: t, origin: origin, client: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

// open goes to an address of the instance, as typed.
func (b *plainBrowser) open(path string) *plainPage {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodGet, b.origin+path, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	return b.do(req)
}

func (b *plainBrowser) do(req *http.Request) *plainPage {
	b.t.Helper()
	req.Header.Set("Accept", "text/html")
	resp, err := b.send(req)
	if err != nil {
		b.t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	p := &plainPage{url: resp.Request.URL, status: resp.StatusCode, body: string(raw)}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		if p.doc, err = html.Parse(bytes.NewReader(raw)); err != nil {
			b.t.Fatalf("%s: %v", p.url, err)
		}
	}
	b.page = p
	return p
}

// send sends a request as a person who waits: a session asked faster than
// a person asks (SessionRateLimit, 120 a minute) is told 429, and the
// browser waits and asks again. A body is sent once it is read again.
func (b *plainBrowser) send(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	for tries := 0; ; tries++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := b.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusTooManyRequests || tries == 60 {
			return resp, err
		}
		resp.Body.Close()
		time.Sleep(time.Second)
		// Told 429 at the end of a redirect: the form was taken, and only
		// the page it leads to is asked again — a tick sent twice is a tick
		// taken back (the acceptance stand, 18.11.2026).
		if resp.Request != nil && resp.Request.URL.String() != req.URL.String() {
			again, _ := http.NewRequest(http.MethodGet, resp.Request.URL.String(), nil)
			again.Header = req.Header.Clone()
			req, body = again, nil
		}
	}
}

// follow goes by the link whose words hold text, on the page the browser
// holds; nothing — the test stops, naming what the page has.
func (b *plainBrowser) follow(text string) *plainPage {
	b.t.Helper()
	a := b.page.find(func(n *html.Node) bool {
		return n.Data == "a" && attr(n, "href") != "" &&
			(strings.Contains(words(n), text) || strings.Contains(attr(n, "title"), text) || strings.Contains(attr(n, "aria-label"), text))
	})
	if a == nil {
		b.t.Fatalf("%s: no link %q; links: %s", b.page.url.Path, text, b.page.links())
	}
	return b.go_(attr(a, "href"))
}

// go_ goes to an address the page names.
func (b *plainBrowser) go_(href string) *plainPage {
	b.t.Helper()
	u, err := b.page.url.Parse(href)
	if err != nil {
		b.t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	return b.do(req)
}

// press presses the button whose words (or title, or value) hold text and
// sends its form, with what a person typed or chose in it (set: a field's
// name and value; a box's name and "on" ticks it, "" unticks it) and the
// files picked (name → file name and contents).
func (b *plainBrowser) press(text string, set url.Values, files map[string][2]string) *plainPage {
	b.t.Helper()
	btn := b.page.find(func(n *html.Node) bool {
		if n.Data != "button" && !(n.Data == "input" && (attr(n, "type") == "submit")) {
			return false
		}
		if t := attr(n, "type"); n.Data == "button" && t != "" && t != "submit" {
			return false
		}
		return strings.Contains(words(n), text) || strings.Contains(attr(n, "title"), text) || attr(n, "value") == text
	})
	if btn == nil {
		b.t.Fatalf("%s: no button %q; buttons: %s", b.page.url.Path, text, b.page.buttons())
	}
	return b.pressButton(btn, text, set, files)
}

// pressIn presses the button that says text in the form whose words hold
// scope: a decision of one card among cards with the same buttons (a group
// of the duplicates screen by its name).
func (b *plainBrowser) pressIn(scope, text string, set url.Values) *plainPage {
	b.t.Helper()
	btn := b.page.find(func(n *html.Node) bool {
		if n.Data != "button" || (attr(n, "type") != "" && attr(n, "type") != "submit") {
			return false
		}
		if !strings.Contains(words(n), text) {
			return false
		}
		form := b.page.formOf(n)
		return form != nil && strings.Contains(words(form), scope)
	})
	if btn == nil {
		b.t.Fatalf("%s: no button %q in a form that says %q; buttons: %s", b.page.url.Path, text, scope, b.page.buttons())
	}
	return b.pressButton(btn, text, set, nil)
}

// pressButton sends the form of btn as press does.
func (b *plainBrowser) pressButton(btn *html.Node, text string, set url.Values, files map[string][2]string) *plainPage {
	b.t.Helper()
	form := b.page.formOf(btn)
	if form == nil {
		b.t.Fatalf("%s: the button %q sends no form", b.page.url.Path, text)
	}
	action, method := attr(form, "action"), strings.ToUpper(attr(form, "method"))
	if a := attr(btn, "formaction"); a != "" {
		action = a
	}
	if m := attr(btn, "formmethod"); m != "" {
		method = strings.ToUpper(m)
	}
	if method == "" {
		method = http.MethodGet
	}
	if action == "" {
		action = b.page.url.String()
	}
	fields := b.page.fieldsOf(form)
	if name := attr(btn, "name"); name != "" {
		fields.Add(name, attr(btn, "value"))
	}
	for k, vs := range set {
		fields.Del(k)
		for _, v := range vs {
			if v != "" {
				fields.Add(k, v)
			}
		}
	}
	u, err := b.page.url.Parse(action)
	if err != nil {
		b.t.Fatal(err)
	}
	var req *http.Request
	switch {
	case method == http.MethodGet:
		u.RawQuery = fields.Encode()
		req, _ = http.NewRequest(http.MethodGet, u.String(), nil)
	case strings.Contains(attr(form, "enctype"), "multipart") || len(files) > 0:
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		for k, vs := range fields {
			for _, v := range vs {
				_ = mw.WriteField(k, v)
			}
		}
		for name, f := range files {
			fw, _ := mw.CreateFormFile(name, f[0])
			_, _ = fw.Write([]byte(f[1]))
		}
		_ = mw.Close()
		req, _ = http.NewRequest(method, u.String(), &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
	default:
		req, _ = http.NewRequest(method, u.String(), strings.NewReader(fields.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return b.do(req)
}

// enter types value into the field named name and presses Enter: the form
// the field stands in goes as it is, with no button of its own.
func (b *plainBrowser) enter(name, value string) *plainPage {
	b.t.Helper()
	field := b.page.find(func(n *html.Node) bool { return n.Data == "input" && attr(n, "name") == name })
	if field == nil {
		b.t.Fatalf("%s: no field %q", b.page.url.Path, name)
	}
	form := b.page.formOfField(field)
	if form == nil {
		b.t.Fatalf("%s: the field %q stands in no form", b.page.url.Path, name)
	}
	fields := b.page.fieldsOf(form)
	fields.Set(name, value)
	u, err := b.page.url.Parse(attr(form, "action"))
	if err != nil {
		b.t.Fatal(err)
	}
	if m := strings.ToUpper(attr(form, "method")); m != "" && m != http.MethodGet {
		req, _ := http.NewRequest(m, u.String(), strings.NewReader(fields.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return b.do(req)
	}
	u.RawQuery = fields.Encode()
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	return b.do(req)
}

// says fails the test unless the page holds text.
func (b *plainBrowser) says(text string) {
	b.t.Helper()
	if !strings.Contains(b.page.text(), text) {
		b.t.Fatalf("%s (%d): the page does not say %q:\n%.2000s", b.page.url.Path, b.page.status, text, b.page.text())
	}
}

func (p *plainPage) find(match func(*html.Node) bool) *html.Node {
	var hit *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if hit != nil {
			return
		}
		if n.Type == html.ElementNode && match(n) {
			hit = n
			return
		}
		if n.Type == html.ElementNode && n.Data == "template" {
			return // what a template holds is not on the page
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	if p.doc != nil {
		walk(p.doc)
	}
	return hit
}

func (p *plainPage) all(match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
		}
		if n.Type == html.ElementNode && n.Data == "template" {
			return // what a template holds is not on the page
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	if p.doc != nil {
		walk(p.doc)
	}
	return out
}

// formOf is the form a button sends: the one it names by form=, else the
// one it stands in.
func (p *plainPage) formOf(btn *html.Node) *html.Node {
	if id := attr(btn, "form"); id != "" {
		return p.find(func(n *html.Node) bool { return n.Data == "form" && attr(n, "id") == id })
	}
	for n := btn.Parent; n != nil; n = n.Parent {
		if n.Type == html.ElementNode && n.Data == "form" {
			return n
		}
	}
	return nil
}

// fieldsOf are what a form sends as it stands: its own fields and those
// outside it that name it by form=.
func (p *plainPage) fieldsOf(form *html.Node) url.Values {
	id := attr(form, "id")
	out := url.Values{}
	for _, n := range p.all(func(n *html.Node) bool {
		return n.Data == "input" || n.Data == "select" || n.Data == "textarea"
	}) {
		if f := p.formOfField(n); f != form && !(id != "" && attr(n, "form") == id) {
			continue
		}
		name := attr(n, "name")
		if name == "" || has(n, "disabled") {
			continue
		}
		switch n.Data {
		case "textarea":
			out.Add(name, words(n))
		case "select":
			var chosen []string
			for _, o := range nodes(n, "option") {
				if has(o, "selected") {
					chosen = append(chosen, optionValue(o))
				}
			}
			if len(chosen) == 0 && !has(n, "multiple") {
				if os := nodes(n, "option"); len(os) > 0 {
					chosen = []string{optionValue(os[0])}
				}
			}
			for _, v := range chosen {
				out.Add(name, v)
			}
		default:
			switch attr(n, "type") {
			case "submit", "button", "file", "reset", "image":
			case "checkbox", "radio":
				if has(n, "checked") {
					v := attr(n, "value")
					if v == "" {
						v = "on"
					}
					out.Add(name, v)
				}
			default:
				out.Add(name, attr(n, "value"))
			}
		}
	}
	return out
}

func (p *plainPage) formOfField(n *html.Node) *html.Node {
	if id := attr(n, "form"); id != "" {
		return p.find(func(f *html.Node) bool { return f.Data == "form" && attr(f, "id") == id })
	}
	for f := n.Parent; f != nil; f = f.Parent {
		if f.Type == html.ElementNode && f.Data == "form" {
			return f
		}
	}
	return nil
}

func (p *plainPage) text() string {
	if p.doc == nil {
		return p.body
	}
	return words(p.doc)
}

func (p *plainPage) links() string {
	var out []string
	for _, a := range p.all(func(n *html.Node) bool { return n.Data == "a" }) {
		out = append(out, strings.TrimSpace(words(a)+" "+attr(a, "title"))+" → "+attr(a, "href"))
	}
	return strings.Join(out, " | ")
}

func (p *plainPage) buttons() string {
	var out []string
	for _, b := range p.all(func(n *html.Node) bool { return n.Data == "button" || n.Data == "input" && attr(n, "type") == "submit" }) {
		out = append(out, strings.TrimSpace(words(b)+" "+attr(b, "title")+" "+attr(b, "value")))
	}
	return strings.Join(out, " | ")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func has(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func nodes(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, c)
		}
		if c.Type == html.ElementNode && c.Data == "optgroup" {
			out = append(out, nodes(c, tag)...)
		}
	}
	return out
}

func optionValue(o *html.Node) string {
	if has(o, "value") {
		return attr(o, "value")
	}
	return strings.TrimSpace(words(o))
}

// words are the text of a node and what is under it, scripts and styles
// and templates aside.
func words(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "template") {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
