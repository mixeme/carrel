//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// seriesProbe is on when CARREL_SERIES_REPORT is set: the checks of an
// event's fields then step aside for a report of what a server does with a
// series that has a skipped occurrence and a moved one — as written straight
// to it, after each edit through Carrel, and after the same retiming written
// straight to it, past Carrel. Nothing is asserted: the lines «SERIES|…» are
// the picture.
func seriesProbe() bool { return os.Getenv("CARREL_SERIES_REPORT") != "" }

// seriesSeed is the series of liveEventFields. linked is false for the form
// Mail.ru stores it in: the moved occurrence without its RECURRENCE-ID and
// the series without its EXDATE.
func seriesSeed(uid string, linked bool) string {
	exdate, recur := "", ""
	if linked {
		exdate = "EXDATE;TZID=Europe/Moscow:20260119T100000\n"
		recur = "RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000\n"
	}
	return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n" +
		"BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\n" +
		"DTEND;TZID=Europe/Moscow:20260112T110000\nSUMMARY:Duty\nRRULE:FREQ=WEEKLY;BYDAY=MO;WKST=MO;COUNT=6\n" +
		exdate + "X-CUSTOM:keep-me\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\n" + recur +
		"DTSTART;TZID=Europe/Moscow:20260126T120000\nDTEND;TZID=Europe/Moscow:20260126T130000\nSUMMARY:Duty moved\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
}

// seriesFacts reads a stored calendar object: how many events it holds, the
// times and rule of the series, its skipped days, and the moved occurrence.
func seriesFacts(body string) string {
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n ", ""), "\r\n", "\n")
	var series, moved []string
	events := 0
	for _, block := range strings.Split(body, "BEGIN:VEVENT")[1:] {
		events++
		lines := strings.Split(strings.SplitN(block, "END:VEVENT", 2)[0], "\n")
		if strings.Contains(block, "SUMMARY:Duty moved") {
			moved = lines
		} else {
			series = lines
		}
	}
	pick := func(lines []string, props ...string) string {
		var out []string
		for _, l := range lines {
			for _, p := range props {
				if strings.HasPrefix(l, p+":") || strings.HasPrefix(l, p+";") {
					out = append(out, l)
				}
			}
		}
		if len(out) == 0 {
			return "—"
		}
		return strings.Join(out, " ")
	}
	movedSays := "GONE"
	if moved != nil {
		movedSays = "recurrence-id=" + pick(moved, "RECURRENCE-ID") + " at=" + pick(moved, "DTSTART")
	}
	return fmt.Sprintf("events=%d | series: %s %s exdate=%s | moved: %s", events,
		pick(series, "DTSTART"), pick(series, "RRULE"), pick(series, "EXDATE"), movedSays)
}

var seriesClock = regexp.MustCompile(`(?m)^(DT(?:START|END)[^:\n]*:\d{8}T)(\d{2})`)

// retimed is body with the series' own start and end two hours earlier, and
// nothing else touched: the retiming a change of zone amounts to, as another
// client would write it.
func retimed(body string) string {
	parts := strings.SplitN(body, "END:VEVENT", 2)
	if len(parts) != 2 || strings.Contains(parts[0], "SUMMARY:Duty moved") {
		// The moved occurrence came first: retime the block after it.
		i := strings.LastIndex(body, "BEGIN:VEVENT")
		if i < 0 {
			return body
		}
		return body[:i] + seriesClock.ReplaceAllStringFunc(body[i:], earlier)
	}
	return seriesClock.ReplaceAllStringFunc(parts[0], earlier) + "END:VEVENT" + parts[1]
}

func earlier(m string) string {
	sub := seriesClock.FindStringSubmatch(m)
	h, _ := strconv.Atoi(sub[2])
	return sub[1] + fmt.Sprintf("%02d", (h+22)%24)
}

func liveSeriesProbe(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, cal, suffix string) {
	host := strings.SplitN(t.Name(), "/", 2)[0]
	say := func(step, what string) { t.Logf("SERIES|%s|%s|%s", host, step, what) }
	put := func(file, body string) error {
		return putSeed(ctx, client, file, "text/calendar; charset=utf-8", body)
	}
	stored := func(file string) string {
		rc, _, err := client.Get(ctx, file, nil)
		if err != nil {
			return ""
		}
		defer rc.Close()
		var b strings.Builder
		buf := make([]byte, 32<<10)
		for {
			n, err := rc.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return b.String()
	}
	form := func(p view.EventEditPage) view.Form { return p.Form }
	edit := func(rec string, set map[string]string) string {
		v := liveForm(form(liveDecode[view.EventEditPage](t, "edit "+rec, app.get(rec+"/edit"))))
		for k, val := range set {
			v.Set(k, val)
		}
		rr := app.post(rec+"/edit", v)
		if rr.Code == 200 {
			body := rr.Body.String()
			if i := strings.Index(body, `"loss":{`); i >= 0 {
				end := i + 400
				if end > len(body) {
					end = len(body)
				}
				return "loss report: " + body[i:end]
			}
			return "answered 200 without a loss report (a form error?)"
		}
		return fmt.Sprintf("answered %d, no loss report", rr.Code)
	}

	for _, kind := range []struct {
		name   string
		linked bool
	}{{"linked", true}, {"orphan", false}} {
		// Through Carrel.
		uid := "series-" + kind.name + "-" + suffix
		file := cal + uid + ".ics"
		if err := put(file, seriesSeed(uid, kind.linked)); err != nil {
			say(kind.name+" 0 written straight", "REFUSED: "+err.Error())
			continue
		}
		t.Cleanup(func() { _ = client.Delete(context.Background(), file, "") })
		say(kind.name+" 0 written straight", seriesFacts(stored(file)))
		rec := "/app/calendar/" + connID + "/" + url.PathEscape(cal) + "/" + url.PathEscape(uid)
		for i, step := range []struct {
			name string
			set  map[string]string
		}{
			{"Carrel: status", map[string]string{"status": "CONFIRMED"}},
			{"Carrel: every 2 weeks, 8 times", map[string]string{"repeat.interval": "2", "repeat.count": "8", "repeat.until": ""}},
			{"Carrel: zone to Yekaterinburg", map[string]string{"tz": "Asia/Yekaterinburg"}},
		} {
			says := edit(rec, step.set)
			say(fmt.Sprintf("%s %d %s", kind.name, i+1, step.name), seriesFacts(stored(file))+" || Carrel "+says)
		}

		// Past Carrel: the same object, retimed by a straight write.
		uid2 := "series-" + kind.name + "-ctl-" + suffix
		file2 := cal + uid2 + ".ics"
		if err := put(file2, seriesSeed(uid2, kind.linked)); err != nil {
			say(kind.name+" control 0 written straight", "REFUSED: "+err.Error())
			continue
		}
		t.Cleanup(func() { _ = client.Delete(context.Background(), file2, "") })
		before := stored(file2)
		_, err := client.Put(ctx, file2, dav.Typed(strings.NewReader(retimed(before)), "text/calendar; charset=utf-8"), "")
		if err != nil {
			say(kind.name+" control 1 retimed straight", "REFUSED: "+err.Error())
			continue
		}
		say(kind.name+" control 1 retimed straight", seriesFacts(stored(file2)))
	}
}

// The same picture against the in-memory server, which stores exactly what
// it is sent: what Carrel itself writes, with no server in the way.
func TestMemorySeriesReport(t *testing.T) {
	if !seriesProbe() {
		t.Skip("set CARREL_SERIES_REPORT=1 for the report")
	}
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "davpass"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	ctx := context.Background()
	client := liveClient(t, srv.URL, "alice", "davpass")
	app := newLiveApp(t)
	connID := connectLive(t, app, srv.URL, "alice", "davpass")
	liveSeriesProbe(ctx, t, app, client, connID, work, "mem")
}
