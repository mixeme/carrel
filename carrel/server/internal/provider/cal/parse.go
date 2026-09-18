package cal

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

var mainKinds = []string{ical.CompEvent, ical.CompToDo, ical.CompJournal}

// Parse reads an iCalendar stream into a CalendarObject.
func Parse(path, etag string, r io.Reader) (*model.Object, error) {
	doc, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	dec := ical.NewDecoder(bytes.NewReader(doc))
	cal, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	main := findMain(cal)
	if main == nil {
		return nil, fmt.Errorf("cal: no main component")
	}
	return model.NewCalendarObject(path, etag, main.Name, propsToRaw(main.Props), doc), nil
}

// Encode writes the full calendar, merging patched main props.
func Encode(w io.Writer, obj *model.Object) error {
	if obj == nil || !obj.IsCalendar() {
		return fmt.Errorf("cal: not a calendar object")
	}
	dec := ical.NewDecoder(bytes.NewReader(obj.CalendarDoc()))
	cal, err := dec.Decode()
	if err != nil {
		return err
	}
	main := findMain(cal)
	if main == nil {
		return fmt.Errorf("cal: no main component")
	}
	main.Props = rawToProps(obj.Props())
	// Folded on the way out: go-ical writes an inline attachment as one line
	// of hundreds of kilobytes, and upstream will not fold (docs/servers.md,
	// SV-31).
	var out bytes.Buffer
	if err := ical.NewEncoder(&out).Encode(cal); err != nil {
		return err
	}
	_, err = w.Write(model.FoldLines(out.Bytes()))
	return err
}

func findMain(cal *ical.Calendar) *ical.Component {
	for _, kind := range mainKinds {
		for _, child := range cal.Children {
			if child.Name != kind {
				continue
			}
			if child.Props.Get(ical.PropRecurrenceID) != nil {
				continue
			}
			return child
		}
	}
	return nil
}

func propsToRaw(props ical.Props) map[string][]model.RawValue {
	out := make(map[string][]model.RawValue, len(props))
	for name, ps := range props {
		key := strings.ToUpper(name)
		vals := make([]model.RawValue, len(ps))
		for i, p := range ps {
			vals[i] = propToRaw(p)
		}
		out[key] = vals
	}
	return out
}

func propToRaw(p ical.Prop) model.RawValue {
	rv := model.RawValue{Text: p.Value}
	if len(p.Params) > 0 {
		rv.Params = make(map[string]string)
		for k, vs := range p.Params {
			if len(vs) > 0 {
				rv.Params[strings.ToUpper(k)] = vs[0]
			}
		}
	}
	return rv
}

func rawToProps(props map[string][]model.RawValue) ical.Props {
	out := make(ical.Props)
	for name, values := range props {
		for _, v := range values {
			p := ical.NewProp(name)
			p.Value = v.Text
			if len(v.Params) > 0 {
				for k, val := range v.Params {
					p.Params.Set(k, val)
				}
			}
			out.Add(p)
		}
	}
	return out
}
