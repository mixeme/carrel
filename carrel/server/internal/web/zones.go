package web

import (
	_ "embed"
	"strings"
)

//go:generate go run zones_gen.go

// zoneList is every zone the time zone of an event offers (s-agenda-8): the
// Area/Location names of Go's own zone database, the one time/tzdata embeds
// in the binary, and UTC — one a line, sorted. zones_gen.go writes it from
// $GOROOT/lib/time/zoneinfo.zip; run go generate after a Go update that
// brings new zones. A test loads every name.
//
//go:embed zones.txt
var zoneList string

// zoneNames is zoneList as a sorted slice.
var zoneNames = strings.Fields(zoneList)
