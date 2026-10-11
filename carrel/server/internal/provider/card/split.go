package card

import (
	"fmt"
	"strings"

	"github.com/emersion/go-vcard"

	"carrel.dev/carrel/server/internal/model"
)

// Cutting a file into contacts (S-19). A .vcf holds one card or a thousand,
// one after another; each becomes a record of its own.

// Broken is a card of a file that could not be read: the name it gives
// itself, when a line says one, and why. It is named in the report of the
// import, never dropped in silence (S-19).
type Broken struct {
	Name   string
	Reason string
}

// Split reads a stream of vCards into one object each. A card without a UID
// is given one: a collection addresses its records by UID, and a card without
// one could be written but never found again. Each card is read by itself:
// one that cannot be read — cut short, without its END:VCARD, or refused by
// the reader — is answered as broken, and the cards after it are read all
// the same. A file in which no card can
// be read is an error.
func Split(doc []byte) ([]*model.Object, []Broken, error) {
	var out []*model.Object
	var broken []Broken
	var first error
	for _, card := range cutCards(doc) {
		lines := card.lines
		if card.open {
			broken = append(broken, Broken{Name: nameIn(lines), Reason: "the card has no END:VCARD"})
			if first == nil {
				first = fmt.Errorf("card: a card has no END:VCARD")
			}
			continue
		}
		c, err := vcard.NewDecoder(strings.NewReader(strings.Join(lines, "\r\n") + "\r\n")).Decode()
		if err != nil {
			if first == nil {
				first = err
			}
			broken = append(broken, Broken{Name: nameIn(lines), Reason: err.Error()})
			continue
		}
		props := cardToProps(c, lines)
		if c.Value(vcard.FieldUID) == "" {
			uid, err := model.NewUID()
			if err != nil {
				return nil, nil, err
			}
			props[vcard.FieldUID] = []model.RawValue{{Text: uid}}
		}
		out = append(out, model.NewObject("", "", props))
	}
	switch {
	case len(out) == 0 && first != nil:
		return nil, broken, first
	case len(out) == 0:
		return nil, nil, fmt.Errorf("card: the file holds no vCard")
	}
	return out, broken, nil
}

// cutCard is one card of a file as its lines stand, unfolded; open is a card
// that was never closed.
type cutCard struct {
	lines []string
	open  bool
}

// cutCards cuts a file into its cards for reading them one by one. A card
// ends at its END:VCARD — or where the next one begins: a card cut short
// does not swallow the card after it, which is whole and is read.
func cutCards(doc []byte) []cutCard {
	var out []cutCard
	var cur []string
	inside := false
	for _, l := range model.LogicalLines(doc) {
		switch up := strings.ToUpper(strings.TrimSpace(l)); {
		case up == "BEGIN:VCARD":
			if inside {
				out = append(out, cutCard{lines: cur, open: true})
			}
			cur, inside = []string{l}, true
		case up == "END:VCARD" && inside:
			out = append(out, cutCard{lines: append(cur, l)})
			cur, inside = nil, false
		case inside:
			cur = append(cur, l)
		}
	}
	if inside {
		out = append(out, cutCard{lines: cur, open: true})
	}
	return out
}

// nameIn is the name a card that could not be read gives itself: its FN
// line, as far as it can be told.
func nameIn(lines []string) string {
	for _, s := range lines {
		if l, ok := model.ParseLine(s); ok && l.Name == vcard.FieldFormattedName {
			return strings.TrimSpace(l.Value)
		}
	}
	return ""
}

// SetUID gives a contact a new identity, for an import that met the old one
// in the collection it writes to (S-19).
func SetUID(obj *model.Object, uid string) (*model.Object, error) {
	if obj == nil {
		return nil, fmt.Errorf("card: no contact")
	}
	props := obj.Props()
	props[vcard.FieldUID] = []model.RawValue{{Text: uid}}
	return model.NewObject(obj.Path, obj.ETag, props), nil
}
