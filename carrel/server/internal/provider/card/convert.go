package card

import (
	"strings"

	"github.com/emersion/go-vcard"

	"carrel.dev/carrel/server/internal/model"
)

func cardToProps(card vcard.Card) map[string][]model.RawValue {
	out := make(map[string][]model.RawValue)
	for name, fields := range card {
		key := strings.ToUpper(name)
		vals := make([]model.RawValue, len(fields))
		for i, f := range fields {
			vals[i] = fieldToRaw(f)
		}
		out[key] = vals
	}
	return out
}

func fieldToRaw(f *vcard.Field) model.RawValue {
	rv := model.RawValue{Text: f.Value}
	if len(f.Params) > 0 {
		rv.Params = make(map[string]string)
		for k, vs := range f.Params {
			if len(vs) > 0 {
				rv.Params[strings.ToUpper(k)] = vs[0]
			}
		}
	}
	return rv
}

func propsToCard(props map[string][]model.RawValue) vcard.Card {
	card := make(vcard.Card)
	for name, values := range props {
		for _, v := range values {
			f := &vcard.Field{Value: v.Text}
			if len(v.Params) > 0 {
				f.Params = make(vcard.Params)
				for k, val := range v.Params {
					f.Params[k] = []string{val}
				}
			}
			card.Add(name, f)
		}
	}
	return card
}
