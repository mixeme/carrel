// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

import (
	"sort"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// Record is one loaded object together with the provenance a decision is keyed
// by: account, collection and UID (§15). The labels travel with it so a group can
// be printed without going back to the account list.
type Record struct {
	AccountID       string
	Collection      string
	AccountLabel    string
	CollectionLabel string
	Color           string
	// ReadOnly marks a collection that cannot be written to, which is what
	// makes it unusable as the target of a merge (§15).
	ReadOnly bool
	Object   *model.Object
}

// UID is the object identity within its collection.
func (r Record) UID() string { return r.Object.UID() }

// Key names one loaded object across every source: the tuple §15 stores a
// decision under.
func (r Record) Key() string { return r.AccountID + "|" + r.Collection + "|" + r.UID() }

// Options tune detection.
type Options struct {
	// Threshold is the score a pair must reach. Zero means DefaultThreshold.
	Threshold int
	// Skip reports a pair that must never be grouped whatever it scores. The
	// "not duplicates" verdict of §15 arrives through here, so a rejected
	// group is not offered again on the next visit.
	Skip func(a, b int) bool
}

func (o Options) threshold() int {
	if o.Threshold <= 0 {
		return DefaultThreshold
	}
	return o.Threshold
}

// Cluster is a set of fingerprint positions that scored as one thing, with the
// best score of any pair in it and the union of the reasons.
type Cluster struct {
	Indexes []int
	Score   int
	Signals []string
}

// Candidate is a group of records §15 believes describe one person or one
// meeting. Nothing has been decided about it: it is what the interface offers.
type Candidate struct {
	Kind    Kind
	Members []Record
	Score   int
	Signals []string
}

// Clusters groups fingerprints that score at or above the threshold.
//
// Only pairs that share a bucket are scored, so this is linear in the number of
// records for the shape of data that occurs in practice, rather than a full
// comparison of every record with every other one.
func Clusters(prints []Fingerprint, opts Options) []Cluster {
	threshold := opts.threshold()
	sets := newUnionFind(len(prints))
	buckets := make(map[string][]int)
	for i, print := range prints {
		if print.Empty() {
			continue
		}
		for _, key := range print.buckets() {
			buckets[key] = append(buckets[key], i)
		}
	}

	scored := make(map[[2]int]bool)
	for _, members := range buckets {
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				a, b := members[i], members[j]
				if a > b {
					a, b = b, a
				}
				pair := [2]int{a, b}
				if scored[pair] {
					continue
				}
				scored[pair] = true
				if opts.Skip != nil && opts.Skip(a, b) {
					continue
				}
				score, signals := Score(prints[a], prints[b])
				if score < threshold {
					continue
				}
				sets.union(a, b, score, signals)
			}
		}
	}
	return sets.clusters()
}

// Sets joins positions into sets: every pair given ends up in one set, and a
// position no pair names stands alone. It is how a merged list combines the
// groups a person has linked with the ones just detected — the two arrive as
// pairs from different places and have to end up as one set of groups.
func Sets(n int, pairs [][2]int) [][]int {
	sets := newUnionFind(n)
	for _, pair := range pairs {
		if pair[0] < 0 || pair[1] < 0 || pair[0] >= n || pair[1] >= n {
			continue
		}
		sets.union(pair[0], pair[1], 0, nil)
	}
	members := make(map[int][]int, n)
	for i := 0; i < n; i++ {
		root := sets.find(i)
		members[root] = append(members[root], i)
	}
	out := make([][]int, 0, len(members))
	for _, indexes := range members {
		sort.Ints(indexes)
		out = append(out, indexes)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// DetectContacts groups the address objects among records (§15).
func DetectContacts(records []Record, opts Options) []Candidate {
	kept, prints := make([]Record, 0, len(records)), make([]Fingerprint, 0, len(records))
	for _, rec := range records {
		if rec.Object == nil || rec.Object.Kind() != model.KindVCard {
			continue
		}
		contact, err := rec.Object.Contact()
		if err != nil {
			continue
		}
		kept = append(kept, rec)
		prints = append(prints, FingerprintContact(contact))
	}
	return candidates(KindContact, kept, prints, opts)
}

// DetectEvents groups the events among records (§15).
func DetectEvents(records []Record, loc *time.Location, opts Options) []Candidate {
	if loc == nil {
		loc = time.UTC
	}
	kept, prints := make([]Record, 0, len(records)), make([]Fingerprint, 0, len(records))
	for _, rec := range records {
		if rec.Object == nil || rec.Object.Component() != "VEVENT" {
			continue
		}
		event, err := rec.Object.Event(loc)
		if err != nil {
			continue
		}
		kept = append(kept, rec)
		prints = append(prints, FingerprintEvent(event))
	}
	return candidates(KindEvent, kept, prints, opts)
}

func candidates(kind Kind, records []Record, prints []Fingerprint, opts Options) []Candidate {
	// Skip is given in the caller's indexes, which are the indexes of the
	// records that survived the kind filter.
	out := make([]Candidate, 0, 4)
	for _, cluster := range Clusters(prints, opts) {
		group := Candidate{Kind: kind, Score: cluster.Score, Signals: cluster.Signals}
		for _, at := range cluster.Indexes {
			group.Members = append(group.Members, records[at])
		}
		sort.SliceStable(group.Members, func(i, j int) bool {
			return group.Members[i].Key() < group.Members[j].Key()
		})
		out = append(out, group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Members[0].Key() < out[j].Members[0].Key()
	})
	return out
}

// unionFind joins the pairs that scored into groups, carrying the best score and
// the reasons along with each set.
type unionFind struct {
	parent  []int
	score   []int
	signals []map[string]bool
}

func newUnionFind(n int) *unionFind {
	u := &unionFind{
		parent:  make([]int, n),
		score:   make([]int, n),
		signals: make([]map[string]bool, n),
	}
	for i := range u.parent {
		u.parent[i] = i
	}
	return u
}

func (u *unionFind) find(i int) int {
	for u.parent[i] != i {
		u.parent[i] = u.parent[u.parent[i]]
		i = u.parent[i]
	}
	return i
}

func (u *unionFind) union(a, b, score int, signals []string) {
	rootA, rootB := u.find(a), u.find(b)
	if rootA != rootB {
		u.parent[rootB] = rootA
		if u.score[rootB] > u.score[rootA] {
			u.score[rootA] = u.score[rootB]
		}
		for signal := range u.signals[rootB] {
			u.addSignal(rootA, signal)
		}
		u.signals[rootB] = nil
	}
	if score > u.score[rootA] {
		u.score[rootA] = score
	}
	for _, signal := range signals {
		u.addSignal(rootA, signal)
	}
}

func (u *unionFind) addSignal(root int, signal string) {
	if u.signals[root] == nil {
		u.signals[root] = make(map[string]bool, 4)
	}
	u.signals[root][signal] = true
}

func (u *unionFind) clusters() []Cluster {
	members := make(map[int][]int)
	for i := range u.parent {
		root := u.find(i)
		members[root] = append(members[root], i)
	}
	out := make([]Cluster, 0, len(members))
	for root, indexes := range members {
		// A record nothing was joined to is not a group.
		if len(indexes) < 2 {
			continue
		}
		sort.Ints(indexes)
		cluster := Cluster{Indexes: indexes, Score: u.score[root]}
		for signal := range u.signals[root] {
			cluster.Signals = append(cluster.Signals, signal)
		}
		sort.Strings(cluster.Signals)
		out = append(out, cluster)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Indexes[0] < out[j].Indexes[0] })
	return out
}
