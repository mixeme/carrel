// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

// The weights of §15. An address, a telephone number or a shared UID identifies
// a person on their own; a name is a hint, and a birthday only strengthens
// something that already matched.
//
// They are points rather than a probability on purpose: the threshold is a
// setting, and a number an administrator can raise or lower is easier to reason
// about than a model nobody can inspect.
const (
	// WeightEmail is a matching normalised address (strong).
	WeightEmail = 60
	// WeightPhone is a matching telephone number (strong).
	WeightPhone = 60
	// WeightUID is a matching UID: strong, and between two servers rare.
	WeightUID = 60
	// WeightName is an exact match of the normalised name (weak on its own).
	WeightName = 30
	// WeightNameClose is a near match of the normalised name.
	WeightNameClose = 20
	// WeightNameLoose is a distant match of the normalised name.
	WeightNameLoose = 10
	// WeightBirthday strengthens a match that already exists. It never makes
	// one on its own: everybody shares a birthday with a stranger.
	WeightBirthday = 20
	// WeightStartSummary is an event at the same moment under the same title.
	WeightStartSummary = 60
)

// The similarity a name has to reach for each of its weights.
const (
	similarityClose = 90
	similarityLoose = 80
)

// DefaultThreshold is the score two records must reach to be offered as a
// group. §15 asks for a conservative default: a false positive is a suggestion
// the person has to reject on screen, and one of those is worse than a duplicate
// that goes unnoticed. One strong signal is therefore enough, and the weak ones
// together are not — a shared name and a shared birthday reach 50 and stay a
// near miss until somebody lowers the threshold on purpose.
const DefaultThreshold = 60

// Signal names the reason a pair scored. They are shown next to a group, because
// "these two share a phone number" is a reason a person can check and "0.87" is
// not (§23.8).
const (
	SignalEmail    = "address"
	SignalPhone    = "phone"
	SignalUID      = "uid"
	SignalName     = "name"
	SignalBirthday = "birthday"
	SignalStart    = "start"
)

// Score reports how strongly two fingerprints look like the same thing, and
// which signals said so. Fingerprints of different kinds never score.
func Score(a, b Fingerprint) (int, []string) {
	if a.Kind != b.Kind || a.Empty() || b.Empty() {
		return 0, nil
	}
	if a.Kind == KindEvent {
		return scoreEvent(a, b)
	}
	return scoreContact(a, b)
}

func scoreContact(a, b Fingerprint) (int, []string) {
	var (
		score   int
		signals []string
	)
	if a.UID != "" && a.UID == b.UID {
		score += WeightUID
		signals = append(signals, SignalUID)
	}
	if sharesAny(a.Emails, b.Emails) {
		score += WeightEmail
		signals = append(signals, SignalEmail)
	}
	if sharesAny(a.Phones, b.Phones) {
		score += WeightPhone
		signals = append(signals, SignalPhone)
	}
	if weight := nameWeight(a.Name, b.Name); weight > 0 {
		score += weight
		signals = append(signals, SignalName)
	}
	// The booster of §15: a birthday adds to a match, it does not make one.
	if score > 0 && birthdayMatch(a.Birthday, b.Birthday) {
		score += WeightBirthday
		signals = append(signals, SignalBirthday)
	}
	return score, signals
}

func scoreEvent(a, b Fingerprint) (int, []string) {
	if a.UID != "" && a.UID == b.UID {
		return WeightUID, []string{SignalUID}
	}
	// The same meeting invited into two accounts usually keeps its UID; when a
	// client rewrote it, the start and the title are what is left (§15).
	if !a.Start.IsZero() && a.Start.Equal(b.Start) && a.Name != "" && a.Name == b.Name {
		return WeightStartSummary, []string{SignalStart, SignalName}
	}
	return 0, nil
}

func nameWeight(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	switch ratio := similarity(a, b); {
	case ratio >= 100:
		return WeightName
	case ratio >= similarityClose:
		return WeightNameClose
	case ratio >= similarityLoose:
		return WeightNameLoose
	default:
		return 0
	}
}

func sharesAny(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for _, left := range a {
		for _, right := range b {
			if left != "" && left == right {
				return true
			}
		}
	}
	return false
}
