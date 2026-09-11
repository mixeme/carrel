package gate

import "testing"

func TestCheckRefValid_rejectsSmoke(t *testing.T) {
	if checkRefValid("widget:smoke") {
		t.Fatal("widget:smoke must not be a valid invariant binding")
	}
	if checkRefValid("todo:pending") {
		t.Fatal("todo: must not be accepted")
	}
	if !checkRefValid("gate:inv-avatar-not-empty") {
		t.Fatal("inv gate checks must be accepted")
	}
}
