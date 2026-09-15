package card

import (
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// S-13: the older form is base64 with ENCODING=b and TYPE=JPEG in a 4.0 card,
// whose VERSION stays; PhotoOps in the same card writes the data: URI again.
func TestPhotoOpsBase64KeepsTheVersion(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff}
	obj := NewObject("ada")
	apply := func(ops []model.PatchOp) {
		t.Helper()
		patch, err := model.NewPatch(ops...)
		if err == nil {
			err = patch.Apply(obj)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	apply(PhotoOpsBase64(obj, jpeg))
	ph := obj.Props()["PHOTO"]
	if len(ph) != 1 || ph[0].Text != "/9j/" || ph[0].Params["ENCODING"] != "b" || ph[0].Params["TYPE"] != "JPEG" {
		t.Errorf("older form PHOTO = %+v", ph)
	}
	if v := obj.PropText("VERSION"); len(v) != 1 || v[0] != "4.0" {
		t.Errorf("VERSION = %v", v)
	}
	if PhotoIsDataURI(obj) {
		t.Error("the older form counts as a data: URI")
	}

	apply(PhotoOps(obj, jpeg))
	if ph := obj.Props()["PHOTO"]; len(ph) != 1 || ph[0].Text != "data:image/jpeg;base64,/9j/" || len(ph[0].Params) != 0 || !PhotoIsDataURI(obj) {
		t.Errorf("4.0 PhotoOps after the older form = %+v", ph)
	}
}
