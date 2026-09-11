package widget

import (
	"strconv"
	"sync/atomic"
)

var idSeq uint64

// uniqueID returns a page-unique id with the given prefix.
func uniqueID(prefix string) string {
	n := atomic.AddUint64(&idSeq, 1)
	return prefix + "-" + strconv.FormatUint(n, 10)
}
