package web

import "carrel.dev/carrel/server/internal/model"

// TamperEdit adds ops to every change a form makes until the function it
// returns is called: a patch that is not the edit, for the test of S-7.3.
func TamperEdit(ops ...model.PatchOp) (restore func()) {
	editTamper = func(ch *change) { ch.ops = append(ch.ops, ops...) }
	return func() { editTamper = nil }
}
