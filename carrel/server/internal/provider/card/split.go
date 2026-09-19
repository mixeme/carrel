package card

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/emersion/go-vcard"

	"carrel.dev/carrel/server/internal/model"
)

// Cutting a file into contacts (S-19). A .vcf holds one card or a thousand,
// one after another; each becomes a record of its own.

// Split reads a stream of vCards into one object each. A card without a UID
// is given one: a collection addresses its records by UID, and a card without
// one could be written but never found again.
func Split(doc []byte) ([]*model.Object, error) {
	dec := vcard.NewDecoder(bytes.NewReader(doc))
	blocks := cardBlocks(doc)
	var out []*model.Object
	for {
		c, err := dec.Decode()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(out) == 0 {
				return nil, err
			}
			// What came before the broken card is a file's worth of contacts;
			// the caller says how many arrived.
			return out, nil
		}
		var lines []string
		if len(out) < len(blocks) {
			lines = blocks[len(out)]
		}
		props := cardToProps(c, lines)
		if c.Value(vcard.FieldUID) == "" {
			uid, err := model.NewUID()
			if err != nil {
				return nil, err
			}
			props[vcard.FieldUID] = []model.RawValue{{Text: uid}}
		}
		out = append(out, model.NewObject("", "", props))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("card: the file holds no vCard")
	}
	return out, nil
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
