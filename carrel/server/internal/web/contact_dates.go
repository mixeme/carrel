package web

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

// A person's dates (Typecase's note 71, task 171; s-contacts-34, the owner's
// decision of 16.10.2026): a rubric of rows, each what the date marks and
// the day, as Apple writes them — item1.X-ABDATE and item1.X-ABLabel. vCard
// has one ANNIVERSARY and no words for it: a card that holds one shows it as
// a row marked «Anniversary», and it stays that line while it is so marked.
const (
	abDate          = "X-ABDATE"
	abLabel         = "X-ABLABEL"
	anniversaryWord = "Anniversary"
)

// dateRow is a row of Dates as the card holds it: what it marks, the value,
// and whether it is the card's ANNIVERSARY.
type dateRow struct {
	mark  string
	value model.RawValue
	anniv bool
}

// contactDates are the rows of Dates of a card: its ANNIVERSARY first, then
// each X-ABDATE with the X-ABLabel of its group.
func contactDates(props map[string][]model.RawValue) []dateRow {
	var out []dateRow
	for _, v := range props["ANNIVERSARY"] {
		out = append(out, dateRow{mark: anniversaryWord, value: v, anniv: true})
	}
	labels := dateLabels(props)
	for _, v := range props[abDate] {
		out = append(out, dateRow{mark: appleWords(labels[strings.ToUpper(v.Group)].Text), value: v})
	}
	return out
}

// dateLabels are the X-ABLabels of the card by the group they name.
func dateLabels(props map[string][]model.RawValue) map[string]model.RawValue {
	out := map[string]model.RawValue{}
	for _, l := range props[abLabel] {
		if l.Group != "" {
			out[strings.ToUpper(l.Group)] = l
		}
	}
	return out
}

// dateGroups are the groups the card's X-ABDATEs stand in: their labels are
// the rows' words, not properties of their own.
func dateGroups(props map[string][]model.RawValue) map[string]bool {
	out := map[string]bool{}
	for _, v := range props[abDate] {
		if v.Group != "" {
			out[strings.ToUpper(v.Group)] = true
		}
	}
	return out
}

// appleWords is a label as a person reads it: Apple writes its own labels
// as _$!<Anniversary>!$_.
func appleWords(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "_$!<") && strings.HasSuffix(v, ">!$_") {
		return v[len("_$!<") : len(v)-len(">!$_")]
	}
	return v
}

// appleLabel is the words of a row as Apple writes them: its own labels in
// its marks, which its Contacts shows in the person's language; any other
// as typed.
func appleLabel(words string) string {
	switch {
	case strings.EqualFold(words, anniversaryWord):
		return "_$!<Anniversary>!$_"
	case strings.EqualFold(words, "Other"):
		return "_$!<Other>!$_"
	}
	return words
}

// datesValues are the rows of Dates as the form shows them: each day typed
// as a birthday is, and what it marks.
func datesValues(props map[string][]model.RawValue) (days, marks []string) {
	for _, d := range contactDates(props) {
		days = append(days, birthdayTyped(d.value.Text, d.value.Params))
		marks = append(marks, d.mark)
	}
	return append(days, ""), append(marks, "")
}

// sameDates reports whether two forms of Dates show the same rows.
func sameDates(days, marks, otherDays, otherMarks []string) bool {
	a, am := filledRows(days, marks)
	b, bm := filledRows(otherDays, otherMarks)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !sameBirthday(a[i], b[i]) || !strings.EqualFold(strings.TrimSpace(am[i]), strings.TrimSpace(bm[i])) {
			return false
		}
	}
	return true
}

// datesOps rewrite Dates row by row (S-7). The row that came from
// ANNIVERSARY and still marks «Anniversary» stays that line; every other row
// is an X-ABDATE with its X-ABLabel in one group — a row kept keeps its
// group and, untouched, its very line; a new one takes the next itemN of
// the card. The labels of other groups (a phone's) are left as they are.
func datesOps(props map[string][]model.RawValue, days, marks []string, version string) ([]model.PatchOp, error) {
	cur := contactDates(props)
	next := nextItem(props)
	type row struct {
		text, group, mark string
		params            map[string]string
	}
	var ops []model.PatchOp
	var rows []row
	anniv := false
	for i, typed := range days {
		if typed = strings.TrimSpace(typed); typed == "" {
			continue
		}
		mark := ""
		if i < len(marks) {
			mark = strings.TrimSpace(marks[i])
		}
		var from *dateRow
		if i < len(cur) {
			from = &cur[i]
		}
		if from != nil && from.anniv && !anniv && strings.EqualFold(mark, anniversaryWord) {
			anniv = true
			if sameBirthday(typed, birthdayTyped(from.value.Text, from.value.Params)) {
				continue
			}
			written, err := birthdayWritten("ANNIVERSARY", typed, props["ANNIVERSARY"], version)
			if err != nil {
				return nil, err
			}
			ops = append(ops, model.SetTextAt{Prop: "ANNIVERSARY", Index: 0, Value: written})
			continue
		}
		r := row{mark: mark}
		var held []model.RawValue
		if from != nil && !from.anniv {
			held = []model.RawValue{from.value}
			r.group = from.value.Group
		}
		if r.group == "" {
			r.group = "item" + strconv.Itoa(next)
			next++
		}
		if len(held) > 0 && sameBirthday(typed, birthdayTyped(held[0].Text, held[0].Params)) {
			r.text, r.params = held[0].Text, held[0].Params
		} else {
			written, err := birthdayWritten(abDate, typed, held, version)
			if err != nil {
				return nil, err
			}
			r.text = written
			if _, yearless, _ := birthdayDigits(typed); yearless && apple3(abDate, held, version) {
				r.params = map[string]string{appleOmitYear: appleYear}
			}
		}
		rows = append(rows, r)
	}
	if !anniv && len(props["ANNIVERSARY"]) > 0 {
		ops = append(ops, model.RemoveProp{Prop: "ANNIVERSARY"})
	}

	// The days, each in its group.
	if len(rows) == 0 {
		if len(props[abDate]) > 0 {
			ops = append(ops, model.RemoveProp{Prop: abDate})
		}
	} else {
		texts := make([]string, len(rows))
		for i, r := range rows {
			texts[i] = r.text
		}
		ops = append(ops, model.SetValues{Prop: abDate, Values: texts})
		for i, r := range rows {
			ops = append(ops, model.SetGroup{Prop: abDate, Index: i, Group: r.group})
			for _, k := range sortedKeys(r.params) {
				ops = append(ops, model.SetParam{Prop: abDate, Index: i, Param: k, Value: r.params[k]})
			}
		}
	}

	// The words: the labels of other groups as they are, then each row's.
	ours := dateGroups(props)
	held := dateLabels(props)
	var labels []model.RawValue
	for _, l := range props[abLabel] {
		if !ours[strings.ToUpper(l.Group)] {
			labels = append(labels, l)
		}
	}
	for _, r := range rows {
		if r.mark == "" {
			continue
		}
		l := model.RawValue{Text: appleLabel(r.mark), Group: r.group}
		if old, ok := held[strings.ToUpper(r.group)]; ok && strings.EqualFold(appleWords(old.Text), r.mark) {
			l.Text = old.Text
		}
		labels = append(labels, l)
	}
	switch {
	case len(labels) == 0 && len(props[abLabel]) > 0:
		ops = append(ops, model.RemoveProp{Prop: abLabel})
	case len(labels) > 0:
		texts := make([]string, len(labels))
		for i, l := range labels {
			texts[i] = l.Text
		}
		ops = append(ops, model.SetValues{Prop: abLabel, Values: texts})
		for i, l := range labels {
			ops = append(ops, model.SetGroup{Prop: abLabel, Index: i, Group: l.Group})
		}
	}
	return ops, nil
}

// nextItem is the number of the next group a card has none of: item3 after
// item1 and item2, whatever property they stand in.
func nextItem(props map[string][]model.RawValue) int {
	n := 0
	for _, values := range props {
		for _, v := range values {
			g := strings.ToLower(v.Group)
			if !strings.HasPrefix(g, "item") {
				continue
			}
			if k, err := strconv.Atoi(g[len("item"):]); err == nil && k > n {
				n = k
			}
		}
	}
	return n + 1
}

// markRow is what a row of Dates marks, free words (s-contacts-34).
func markRow(name, mark string) view.FormRow {
	return view.FormRow{Name: name + ".mark", Label: "What it marks", Kind: view.InputText, Value: text(mark)}
}

// dayRow is the day of a row of Dates, typed as a birthday is.
func dayRow(name, day string) view.FormRow {
	return view.FormRow{Name: name, Label: "Date", Kind: view.InputText, Value: text(day), Placeholder: "DD.MM.YYYY", Mono: true}
}

// datesRows are the rows of Dates on the form: each what it marks, its day
// and Remove; then «Add date» with the rows it adds (s-contacts-34, -27).
func datesRows(f editField, days, marks, removed []string, errText view.Text) []view.FormRow {
	var out []view.FormRow
	for i, v := range shownRows(days, true) {
		name := fmt.Sprintf("%s.%d", f.name, i)
		if slices.Contains(removed, name) {
			out = append(out, removedRows(f, name)...)
			continue
		}
		day := dayRow(name, v)
		if i == 0 {
			day.Error = errText
		}
		out = append(out, markRow(name, first(marks[min(i, len(marks)):])), day, removeButton(name))
	}
	tpl := f.name + ".{n}"
	return append(out, addButton(f, markRow(tpl, ""), dayRow(tpl, ""), removeButton(tpl)))
}
