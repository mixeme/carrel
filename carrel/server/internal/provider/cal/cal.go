package cal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

var ErrEmptyETag = errors.New("cal: update requires ETag")

// Provider reads and writes iCalendar records.
type Provider struct {
	t dav.Transport
	c dav.Creator
}

// New wires a calendar provider.
func New(t dav.Transport) (*Provider, error) {
	c, ok := t.(dav.Creator)
	if !ok {
		return nil, fmt.Errorf("cal: transport lacks PutCreate")
	}
	return &Provider{t: t, c: c}, nil
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
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	return p.c.PutCreate(ctx, path, &buf)
}

// Update writes with If-Match (S-8).
func (p *Provider) Update(ctx context.Context, obj *model.Object) (string, error) {
	if obj == nil || obj.ETag == "" {
		return "", ErrEmptyETag
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		return "", err
	}
	tag := strings.Trim(obj.ETag, `"`)
	return p.t.Put(ctx, obj.Path, &buf, `"`+tag+`"`)
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

// Description returns the Markdown body for notes and other components.
func Description(obj *model.Object) string {
	if txt := obj.PropText("DESCRIPTION"); len(txt) > 0 {
		return txt[0]
	}
	return ""
}
