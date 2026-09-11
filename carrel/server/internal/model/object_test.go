package model

import (
	"errors"
	"testing"
)

func TestPatchRejectedDoesNotChangeObject(t *testing.T) {
	obj := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"FN": {values: []propValue{{text: "Ada"}}},
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
	obj := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"FN": {values: []propValue{{text: "Ada"}}},
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
	obj1 := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"NOTE": {values: []propValue{{text: "x"}}},
	})
	obj2 := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"NOTE": {values: []propValue{{text: "x"}}},
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
	obj := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"FN": {values: []propValue{{text: "Ada"}}},
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
