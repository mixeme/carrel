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
	cal, err := decode(doc)
	if err != nil {
		return nil, err
	}
	main := findMain(cal)
	if main == nil {
		return nil, fmt.Errorf("cal: no main component")
	}
	return model.NewCalendarObject(path, etag, main.Name, propsToRaw(main.Props, mainLines(doc, main.Name)), doc), nil
}

// Encode writes the full calendar, merging patched main props.
func Encode(w io.Writer, obj *model.Object) error {
	if obj == nil || !obj.IsCalendar() {
		return fmt.Errorf("cal: not a calendar object")
	}
	cal, err := decode(obj.CalendarDoc())
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

// decode reads a calendar document. go-ical panics on some malformed lines —
// a parameter with no colon after it, text after a closing quote — and the
// document comes from a far server, a subscription or an uploaded file: a
// broken record is an error of that record, never the end of the process
// (S-21).
func decode(doc []byte) (cal *ical.Calendar, err error) {
	defer func() {
		if r := recover(); r != nil {
			cal, err = nil, fmt.Errorf("cal: malformed calendar: %v", r)
		}
	}()
	return ical.NewDecoder(bytes.NewReader(doc)).Decode()
}

// propsToRaw turns the main component's properties into raw values. lines
// are its own content lines as they stand in the document; when they match
// the properties go-ical read, every value keeps the line it came in, so an
// untouched value is written back as it came (S-7).
func propsToRaw(props ical.Props, lines []string) map[string][]model.RawValue {
	byName := map[string][]string{}
	for _, l := range lines {
		pl, ok := model.ParseLine(l)
		if !ok {
			byName = nil
			break
		}
		byName[pl.Name] = append(byName[pl.Name], l)
	}
	out := make(map[string][]model.RawValue, len(props))
	for name, ps := range props {
		key := strings.ToUpper(name)
		wires := byName[key]
		if len(wires) != len(ps) {
			wires = nil
		}
		vals := make([]model.RawValue, len(ps))
		for i, p := range ps {
			vals[i] = propToRaw(p)
			if wires != nil {
				vals[i].Wire = wires[i]
			}
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
				rv.Params[strings.ToUpper(k)] = strings.Join(vs, ",")
			}
		}
	}
	return rv
}

// rawToProps is the way back. A value with its line is read from that line,
// every parameter value in place; a value written fresh has its parameters
// split where a list parameter holds a list (model.ParamValues).
func rawToProps(props map[string][]model.RawValue) ical.Props {
	out := make(ical.Props)
	for name, values := range props {
		for _, v := range values {
			if l, ok := model.ParseLine(v.Wire); ok && v.Wire != "" && l.Name == strings.ToUpper(name) {
				p := ical.NewProp(l.Name)
				p.Value = l.Value
				for _, pr := range l.Params {
					p.Params[pr.Name] = append(p.Params[pr.Name], pr.Values...)
				}
				out.Add(p)
				continue
			}
			p := ical.NewProp(name)
			p.Value = v.Text
			for k, val := range v.Params {
				p.Params[k] = model.ParamValues(k, val)
			}
			out.Add(p)
		}
	}
	return out
}
