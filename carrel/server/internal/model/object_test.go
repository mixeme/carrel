package model

import (
	"errors"
	"testing"
)

func TestPatchRejectedDoesNotChangeObject(t *testing.T) {
	obj := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"FN": {{Text: "Ada"}},
	})
	before := obj.PropText("FN")

	_, err := NewPatch(
		SetText{Prop: "FN", Value: "Grace"},
		SetText{Prop: "UID", Value: "evil"},
	)
	if !errors.Is(err, ErrPatchImmutable) {
		t.Fatalf("NewPatch: got %v, want ErrPatchImmutable", err)
	}
	if got := obj.PropText("FN"); len(got) != len(before) || got[0] != before[0] {
		t.Fatal("rejected patch changed object")
	}
}

func TestPatchApplyAllOrNothing(t *testing.T) {
	obj := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"FN": {{Text: "Ada"}},
	})
	p, err := NewPatch(SetText{Prop: "NOTE", Value: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(obj); err != nil {
		t.Fatal(err)
	}
	if got := obj.PropText("NOTE"); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("NOTE = %v, want [hello]", got)
	}
	if got := obj.PropText("FN"); len(got) != 1 || got[0] != "Ada" {
		t.Fatalf("FN changed: %v", got)
	}
}

func TestRemovePropDistinctFromEmpty(t *testing.T) {
	obj1 := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"NOTE": {{Text: "x"}},
	})
	obj2 := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"NOTE": {{Text: "x"}},
	})

	p1, err := NewPatch(SetText{Prop: "NOTE", Value: ""})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := NewPatch(RemoveProp{Prop: "NOTE"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p1.Apply(obj1); err != nil {
		t.Fatal(err)
	}
	if err := p2.Apply(obj2); err != nil {
		t.Fatal(err)
	}
	if got := obj1.PropText("NOTE"); len(got) != 1 || got[0] != "" {
		t.Fatalf("SetText empty: %v", got)
	}
	if got := obj2.PropText("NOTE"); got != nil {
		t.Fatalf("RemoveProp: NOTE still %v", got)
	}
}

func TestNewPatchValidatesUpFront(t *testing.T) {
	_, err := NewPatch(SetText{Prop: "VERSION", Value: "2"})
	if !errors.Is(err, ErrPatchImmutable) {
		t.Fatalf("got %v, want ErrPatchImmutable", err)
	}
}

func TestRejectedApplyRollsBack(t *testing.T) {
	obj := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"FN": {{Text: "Ada"}},
	})
	p, err := NewPatch(
		SetText{Prop: "FN", Value: "Grace"},
		SetParam{Prop: "FN", Index: 9, Param: "TYPE", Value: "HOME"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(obj); err == nil {
		t.Fatal("expected apply error for missing index")
	}
	if got := obj.PropText("FN"); len(got) != 1 || got[0] != "Ada" {
		t.Fatalf("partial apply left FN = %v", got)
	}
}

// Providers live in another package, so the way into the model has to be
// callable from there: exported constructor, exported argument types, and the
// parsed record itself still unreachable (S-7).
func TestObjectIngressIsUsableFromOtherPackages(t *testing.T) {
	obj := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"fn":    {{Text: "Ada"}},
		"EMAIL": {{Text: "ada@example.com", Params: map[string]string{"type": "WORK"}}},
	})

	props := obj.Props()
	if got := props["FN"]; len(got) != 1 || got[0].Text != "Ada" {
		t.Fatalf("FN = %#v (property names fold to upper case)", got)
	}
	email := props["EMAIL"]
	if len(email) != 1 || email[0].Params["TYPE"] != "WORK" {
		t.Fatalf("EMAIL = %#v (parameter names fold to upper case)", email)
	}

	// Props hands out a copy: writing to it does not reach the record.
	props["FN"][0].Text = "Grace"
	if got := obj.PropText("FN"); got[0] != "Ada" {
		t.Fatalf("mutating the returned view changed the record: %q", got[0])
	}
}

// S-7.3: the edit of a value a form showed is an operation on that value. A
// property that stands twice keeps its other value, and the value changed
// keeps its parameters and its group; only its line, which no longer says
// what it holds, goes.
func TestSetTextAtChangesOneValue(t *testing.T) {
	obj := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"TEL": {
			{Text: "+1", Params: map[string]string{"TYPE": "WORK,CELL"}, Group: "item1", Wire: "item1.TEL;TYPE=WORK,CELL:+1"},
			{Text: "+2", Wire: "TEL:+2"},
		},
	})
	p, err := NewPatch(SetTextAt{Prop: "tel", Index: 0, Value: "+9"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(obj); err != nil {
		t.Fatal(err)
	}
	got := obj.Props()["TEL"]
	if len(got) != 2 || got[0].Text != "+9" || got[0].Group != "item1" || got[0].Params["TYPE"] != "WORK,CELL" || got[0].Wire != "" {
		t.Errorf("first value = %+v", got)
	}
	if got[1].Text != "+2" || got[1].Wire != "TEL:+2" {
		t.Errorf("second value = %+v", got[1])
	}

	// The same text is no change: the line stays the line it was.
	same, _ := NewPatch(SetTextAt{Prop: "TEL", Index: 1, Value: "+2"})
	if err := same.Apply(obj); err != nil {
		t.Fatal(err)
	}
	if obj.Props()["TEL"][1].Wire != "TEL:+2" {
		t.Errorf("a value set to what it held lost its line")
	}

	// A value that is not there is a rejected patch, and nothing changes.
	bad, _ := NewPatch(SetTextAt{Prop: "TEL", Index: 0, Value: "+0"}, SetTextAt{Prop: "TEL", Index: 5, Value: "x"})
	if err := bad.Apply(obj); err == nil {
		t.Fatal("no error for a value that is not there")
	}
	if got := obj.PropText("TEL"); got[0] != "+9" {
		t.Errorf("a rejected patch changed the object: %v", got)
	}
}

func TestRemoveValueTakesOneValue(t *testing.T) {
	obj := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"TITLE": {{Text: "Mathematician"}, {Text: "Countess", Wire: "TITLE:Countess"}},
	})
	first, _ := NewPatch(RemoveValue{Prop: "TITLE", Index: 0})
	if err := first.Apply(obj); err != nil {
		t.Fatal(err)
	}
	if got := obj.Props()["TITLE"]; len(got) != 1 || got[0].Text != "Countess" || got[0].Wire != "TITLE:Countess" {
		t.Errorf("TITLE = %+v", got)
	}
	// The last value takes the property with it.
	if err := first.Apply(obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.Props()["TITLE"]; ok {
		t.Errorf("an emptied property is still there")
	}
	if err := first.Apply(obj); err == nil {
		t.Errorf("no error for a value that is not there")
	}
}
