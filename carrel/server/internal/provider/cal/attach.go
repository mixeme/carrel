package cal

import (
	"encoding/base64"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// Attachments (S-20). An ATTACH holds its file as BINARY data or links to it
// by URI. An attachment another client wrote is read as it is and never
// rewritten: adding one appends a value, removing one keeps every other value
// with its parameters.

var ErrNoAttachment = errors.New("cal: no such attachment")

// fileNameParams are the parameters clients name an attachment by: FILENAME
// of RFC 8607 first, then the ones written before it.
var fileNameParams = []string{"FILENAME", "X-FILENAME", "X-APPLE-FILENAME", "X-ORACLE-FILENAME", "X-EVOLUTION-CALDAV-ATTACHMENT-NAME"}

// Attachment is one ATTACH of the main component.
type Attachment struct {
	Index    int    // position among the ATTACH values, as Detach counts
	Inline   bool   // the file is in the record: VALUE=BINARY or ENCODING=BASE64
	URI      string // the link; empty when inline
	FileName string // as a parameter names it; empty when none does
	Type     string // FMTTYPE as written
	Size     int64  // bytes of inline data, SIZE of a link; -1 when unknown
}

// Attachments lists ATTACH of the main component in order.
func Attachments(obj *model.Object) []Attachment {
	var out []Attachment
	for i, v := range obj.Props()[ical.PropAttach] {
		a := Attachment{Index: i, Type: v.Params[ical.ParamFormatType], Size: -1}
		for _, p := range fileNameParams {
			if n := strings.TrimSpace(v.Params[p]); n != "" {
				a.FileName = n
				break
			}
		}
		if binaryValue(v.Params) {
			a.Inline = true
			a.Size = int64(len(strings.TrimRight(stripSpace(v.Text), "=")) * 3 / 4)
		} else {
			a.URI = strings.TrimSpace(v.Text)
			if n, err := strconv.ParseInt(v.Params["SIZE"], 10, 64); err == nil && n >= 0 {
				a.Size = n
			}
		}
		out = append(out, a)
	}
	return out
}

// AttachmentData is the file of an inline attachment. A link has no data here:
// ErrNoAttachment.
func AttachmentData(obj *model.Object, index int) (Attachment, []byte, error) {
	list := Attachments(obj)
	if index < 0 || index >= len(list) || !list[index].Inline {
		return Attachment{}, nil, ErrNoAttachment
	}
	raw := stripSpace(obj.Props()[ical.PropAttach][index].Text)
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(raw, "="))
	}
	if err != nil {
		return Attachment{}, nil, err
	}
	return list[index], data, nil
}

// AttachInline is the patch that appends data as a new inline ATTACH, named
// and typed.
func AttachInline(obj *model.Object, name, mediaType string, data []byte) []model.PatchOp {
	i := len(obj.Props()[ical.PropAttach])
	ops := []model.PatchOp{
		model.AddValue{Prop: ical.PropAttach, Value: base64.StdEncoding.EncodeToString(data)},
		model.SetParam{Prop: ical.PropAttach, Index: i, Param: ical.ParamEncoding, Value: "BASE64"},
		model.SetParam{Prop: ical.PropAttach, Index: i, Param: ical.ParamValue, Value: "BINARY"},
	}
	return append(ops, described(i, name, mediaType)...)
}

// AttachLink is the patch that appends a link to a file as a new ATTACH, with
// the name the file came with and its size when known.
func AttachLink(obj *model.Object, uri, name, mediaType string, size int64) []model.PatchOp {
	i := len(obj.Props()[ical.PropAttach])
	ops := append([]model.PatchOp{model.AddValue{Prop: ical.PropAttach, Value: uri}}, described(i, name, mediaType)...)
	if size >= 0 {
		ops = append(ops, model.SetParam{Prop: ical.PropAttach, Index: i, Param: "SIZE", Value: strconv.FormatInt(size, 10)})
	}
	return ops
}

func described(i int, name, mediaType string) []model.PatchOp {
	var ops []model.PatchOp
	if mediaType = paramValue(mediaType); mediaType != "" {
		ops = append(ops, model.SetParam{Prop: ical.PropAttach, Index: i, Param: ical.ParamFormatType, Value: mediaType})
	}
	if name = paramValue(name); name != "" {
		ops = append(ops, model.SetParam{Prop: ical.PropAttach, Index: i, Param: "FILENAME", Value: name})
	}
	return ops
}

// Detach is the patch that takes the ATTACH at index away, its data with it.
// Every other ATTACH is written back as it was, parameters and all.
func Detach(obj *model.Object, index int) ([]model.PatchOp, error) {
	cur := obj.Props()[ical.PropAttach]
	if index < 0 || index >= len(cur) {
		return nil, ErrNoAttachment
	}
	ops := []model.PatchOp{model.RemoveProp{Prop: ical.PropAttach}}
	n := 0
	for i, v := range cur {
		if i == index {
			continue
		}
		ops = append(ops, model.AddValue{Prop: ical.PropAttach, Value: v.Text})
		keys := make([]string, 0, len(v.Params))
		for k := range v.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ops = append(ops, model.SetParam{Prop: ical.PropAttach, Index: n, Param: k, Value: v.Params[k]})
		}
		n++
	}
	return ops, nil
}

func binaryValue(params map[string]string) bool {
	return strings.EqualFold(params[ical.ParamValue], "BINARY") || strings.EqualFold(params[ical.ParamEncoding], "BASE64")
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// paramValue is a parameter value the encoder can write: iCalendar has no
// escape for a double quote in one (RFC 5545 §3.2), and no line breaks.
func paramValue(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '"':
			return '\''
		case unicode.IsControl(r):
			return ' '
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}
