package proxy

import (
	"bytes"
	"io"
	"sort"
)

// rewriter replaces prefixes in a stream (S-27.3): the body of a big
// collection is never held whole, only a window as long as the longest
// pattern. Patterns are tried longest first, so a full URL is replaced
// before the path inside it.
type rewriter struct {
	src   io.Reader
	pairs [][2][]byte
	max   int
	in    []byte // read, not yet written out
	out   bytes.Buffer
	eof   bool
	err   error
}

func newRewriter(src io.Reader, pairs map[string]string) io.Reader {
	rw := &rewriter{src: src}
	for from, to := range pairs {
		if from == "" || from == to {
			continue
		}
		rw.pairs = append(rw.pairs, [2][]byte{[]byte(from), []byte(to)})
		if len(from) > rw.max {
			rw.max = len(from)
		}
	}
	if len(rw.pairs) == 0 {
		return src
	}
	sort.Slice(rw.pairs, func(i, j int) bool { return len(rw.pairs[i][0]) > len(rw.pairs[j][0]) })
	return rw
}

func (rw *rewriter) Read(p []byte) (int, error) {
	for rw.out.Len() == 0 {
		if rw.eof {
			if len(rw.in) > 0 {
				rw.flush(len(rw.in))
				continue
			}
			if rw.err != nil {
				return 0, rw.err
			}
			return 0, io.EOF
		}
		buf := make([]byte, 32<<10)
		n, err := rw.src.Read(buf)
		rw.in = append(rw.in, buf[:n]...)
		if err != nil {
			rw.eof = true
			if err != io.EOF {
				rw.err = err
			}
		}
		// Everything but the last max-1 bytes is safe: a pattern starting
		// there is whole in the window.
		if safe := len(rw.in) - (rw.max - 1); safe > 0 && !rw.eof {
			rw.flush(safe)
		}
	}
	return rw.out.Read(p)
}

// flush writes out the first limit bytes of the window, replacing every
// pattern that starts in them.
func (rw *rewriter) flush(limit int) {
	i := 0
	for i < limit {
		matched := false
		for _, pr := range rw.pairs {
			if bytes.HasPrefix(rw.in[i:], pr[0]) {
				rw.out.Write(pr[1])
				i += len(pr[0])
				matched = true
				break
			}
		}
		if !matched {
			rw.out.WriteByte(rw.in[i])
			i++
		}
	}
	rw.in = append(rw.in[:0], rw.in[i:]...)
}
