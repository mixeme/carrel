package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// The rail's tags of a combined view are the section's (the owner's
// decision of 08.10.2026), as a list's are: a source that has not answered
// this poll — still asked, or refused — keeps the tags an earlier poll of
// the session read of it (states-19, -20: «релиз» of «Работа» stands while
// «Работа» is held or refuses). A session that has polled nothing before
// knows what came alone.
func TestCombinedRailKeepsTheTagsOfASourceNotAnswered(t *testing.T) {
	has := func(tags []view.Tag, label string) bool {
		for _, tg := range tags {
			if string(tg.Label) == label {
				return true
			}
		}
		return false
	}
	app := newMockupApp(t)
	first := combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	if !has(first.Rail.Tags, "релиз") {
		t.Fatalf("tags = %+v: «релиз» is not in the set's tasks", first.Rail.Tags)
	}
	nc := app.servers["nextcloud"]
	for _, m := range []string{"PROPFIND", "REPORT"} {
		nc.RefuseAt(m, app.realPath(t, "rabota"), 503, "the server is busy")
		defer nc.Accept(m)
	}
	again := combined(t, app.h, app.sess, "/app/tasks?again=1", func(p view.TasksPage) *view.PollBar { return p.Poll })
	refused := false
	for _, sp := range again.Poll.Sources {
		if string(sp.Stamp) == "nextcloud · Работа" && sp.State != view.SourceDone && sp.State != view.SourceEmpty {
			refused = true
		}
	}
	if !refused {
		t.Fatalf("sources = %+v: «Работа» answered", again.Poll.Sources)
	}
	if !has(again.Rail.Tags, "релиз") {
		t.Errorf("tags = %+v: «релиз» of «Работа» went with its refusal", again.Rail.Tags)
	}
}
