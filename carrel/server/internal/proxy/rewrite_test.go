package proxy

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// The rewriter replaces across read boundaries, longest pattern first, and
// leaves everything else byte for byte.
func TestRewriterStreams(t *testing.T) {
	pairs := map[string]string{
		"https://dav.example/cal/work/": "/c/a1/%2Fcal%2Fwork%2F/",
		"/cal/work/":                    "/c/a1/%2Fcal%2Fwork%2F/",
	}
	in := `<d:href>/cal/work/</d:href><d:href>https://dav.example/cal/work/x.ics</d:href><d:href>/cal/other/</d:href>` + strings.Repeat("x", 70000) + `<d:href>/cal/work/y.ics</d:href>`
	want := `<d:href>/c/a1/%2Fcal%2Fwork%2F/</d:href><d:href>/c/a1/%2Fcal%2Fwork%2F/x.ics</d:href><d:href>/cal/other/</d:href>` + strings.Repeat("x", 70000) + `<d:href>/c/a1/%2Fcal%2Fwork%2F/y.ics</d:href>`
	for name, r := range map[string]io.Reader{
		"whole":        strings.NewReader(in),
		"byte by byte": iotest.OneByteReader(strings.NewReader(in)),
		"halves":       iotest.HalfReader(strings.NewReader(in)),
	} {
		got, err := io.ReadAll(newRewriter(r, pairs))
		if err != nil || string(got) != want {
			t.Errorf("%s: err %v, differs: %v", name, err, string(got) != want)
		}
	}
}
