package cal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

var ErrEmptyETag = errors.New("cal: update requires ETag")

// Provider reads and writes iCalendar records.
type Provider struct {
	t   dav.Transport
	c   dav.Creator
	now func() time.Time
}

// New wires a calendar provider.
func New(t dav.Transport) (*Provider, error) {
	c, ok := t.(dav.Creator)
	if !ok {
		return nil, fmt.Errorf("cal: transport lacks PutCreate")
	}
	return &Provider{t: t, c: c, now: time.Now}, nil
}

// stamp marks a write as a revision of the main component. For an object
// without METHOD, DTSTAMP is when it was last revised (RFC 5545 §3.8.7.2), and
// LAST-MODIFIED says the same (§3.8.7.3); left as they were, a client shows the
// time of change from before the edit. The loss check ignores both.
func (p *Provider) stamp(obj *model.Object) error {
	v := p.now().UTC().Format("20060102T150405Z")
	patch, err := model.NewPatch(
		model.SetText{Prop: "DTSTAMP", Value: v},
		model.SetText{Prop: "LAST-MODIFIED", Value: v},
	)
	if err != nil {
		return err
	}
	return patch.Apply(obj)
}

// Get fetches and parses one calendar object.
func (p *Provider) Get(ctx context.Context, path string) (*model.Object, error) {
	rc, etag, err := p.t.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return Parse(path, etag, rc)
}

// Create writes a new object with If-None-Match.
func (p *Provider) Create(ctx context.Context, path string, obj *model.Object) (string, error) {
	if err := p.stamp(obj); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	return p.c.PutCreate(ctx, path, dav.Typed(&buf, MediaType))
}

// MediaType is the Content-Type of a calendar object on PUT.
const MediaType = "text/calendar; charset=utf-8"

// Update writes with If-Match (S-8).
func (p *Provider) Update(ctx context.Context, obj *model.Object) (string, error) {
	if obj == nil || obj.ETag == "" {
		return "", ErrEmptyETag
	}
	if err := p.stamp(obj); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	tag := strings.Trim(obj.ETag, `"`)
	return p.t.Put(ctx, obj.Path, dav.Typed(&buf, MediaType), `"`+tag+`"`)
}

// Kind reports event, task, or note from the main component.
func Kind(obj *model.Object) view.Kind {
	switch obj.MainComponent() {
	case "VEVENT":
		return view.KindEvent
	case "VTODO":
		return view.KindTask
	case "VJOURNAL":
		return view.KindNote
	default:
		return view.KindEvent
	}
}

// Description returns the Markdown body for notes and other components,
// unescaped: a line break is a line break, not "\n".
func Description(obj *model.Object) string {
	return Text(obj, "DESCRIPTION")
}
