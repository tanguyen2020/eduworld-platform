package selection

import (
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// DifficultyForLevel is the light adaptive difficulty of LEARNING_EXPERIENCE §5 (F032-AC-05): beginning 1–2,
// developing 2–3, proficient 3–4, mastered 4–5; an objective not started yet is taken as beginning (its value is the
// prior M0 < 0.40) [Proposed].
func DifficultyForLevel(level string) Range {
	switch level {
	case "developing":
		return Range{Min: 2, Max: 3}
	case "proficient":
		return Range{Min: 3, Max: 4}
	case "mastered":
		return Range{Min: 4, Max: 5}
	}
	return Range{Min: 1, Max: 2}
}

// WrongItem is one question of the review list (retry_wrong): the original question and what a sibling must share.
type WrongItem struct {
	QuestionID   uuid.UUID
	ObjectiveIDs []uuid.UUID
	Type         string
	Difficulty   int
}

// Pick is the question chosen for one wrong item; Original is true when no sibling exists and the current version of
// the original question is reused (QUESTION_BANK §6 step 6).
type Pick struct {
	Wrong     int // index in the wrong items
	Candidate Candidate
	Original  bool
}

func rng(seed uint64, salt string) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(salt))
	return rand.New(rand.NewPCG(seed, h.Sum64())) //nolint:gosec // reproducible selection, not a secret
}

// Siblings picks one sibling per wrong item (F032-AC-22, QUESTION_BANK §6 step 6): a published question sharing at
// least one objective, of the same type and difficulty, another question_id, not seen in the avoid_recent window;
// then difficulty ±1; then a recently seen sibling; last the current version of the original question. A wrong item
// whose original is no longer in the pool and has no sibling is skipped; no pick at all is ErrPoolInsufficient.
// The same pool, items and seed give the same picks.
func Siblings(pool []Candidate, wrong []WrongItem, recent []uuid.UUID, seed uint64) ([]Pick, error) {
	sorted := slices.Clone(pool)
	slices.SortFunc(sorted, func(a, b Candidate) int { return strings.Compare(a.VersionID.String(), b.VersionID.String()) })
	r := rng(seed, "qb-siblings")
	perm := r.Perm(len(sorted))
	shuffled := make([]Candidate, len(sorted))
	for i, p := range perm {
		shuffled[i] = sorted[p]
	}
	originals := map[uuid.UUID]bool{}
	for _, w := range wrong {
		originals[w.QuestionID] = true
	}
	taken := map[uuid.UUID]bool{}
	var out []Pick
	for i, w := range wrong {
		found := false
		for level := 0; level < 4 && !found; level++ {
			for _, c := range shuffled {
				if taken[c.QuestionID] || originals[c.QuestionID] || c.ExamOnly || c.Type != w.Type || !overlapsID(w.ObjectiveIDs, c.ObjectiveIDs) {
					continue
				}
				if (level == 0 || level == 2) && c.Difficulty != w.Difficulty {
					continue
				}
				if (level == 1 || level == 3) && abs(c.Difficulty-w.Difficulty) > 1 {
					continue
				}
				if level < 2 && slices.Contains(recent, c.QuestionID) {
					continue
				}
				taken[c.QuestionID] = true
				out = append(out, Pick{Wrong: i, Candidate: c})
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, c := range sorted {
			if c.QuestionID == w.QuestionID && !taken[c.QuestionID] {
				taken[c.QuestionID] = true
				out = append(out, Pick{Wrong: i, Candidate: c, Original: true})
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrPoolInsufficient
	}
	return out, nil
}

// Mix is interleave_mix (CONFIG_DEFAULTS, owner LEARNING_EXPERIENCE §5.2).
type Mix struct {
	RecentShare    float64 // share of the most recent topic (0.5)
	MinTopics      int     // 2
	MaxTopics      int     // 4
	MaxConsecutive int     // 2
}

// DefaultMix is the value of LEARNING_EXPERIENCE §5.2 [Confirmed — Technical Architect 2026-11-15].
func DefaultMix() Mix { return Mix{RecentShare: 0.5, MinTopics: 2, MaxTopics: 4, MaxConsecutive: 2} }

// Split divides count questions between topics (the most recent first, F032-AC-24): RecentShare of them to the most
// recent topic, the rest evenly to the others (the remainder to the earlier ones in the list).
func (m Mix) Split(count int, topics []uuid.UUID) []int {
	out := make([]int, len(topics))
	if len(topics) == 0 || count <= 0 {
		return out
	}
	if len(topics) == 1 {
		out[0] = count
		return out
	}
	first := int(math.Ceil(float64(count) * m.RecentShare))
	first = min(max(first, 1), count-(len(topics)-1))
	out[0] = first
	rest := count - first
	others := len(topics) - 1
	for i := 1; i < len(topics); i++ {
		out[i] = rest / others
		if i-1 < rest%others {
			out[i]++
		}
	}
	return out
}

// Interleave orders questions so that no more than maxConsecutive follow each other from the same topic (F032-AC-24,
// QUESTION_BANK §6 step 7): at each step the topic with the most questions left that may follow is taken, ties broken
// by seed. When one topic outnumbers the others too much to respect the limit, the rest is appended.
func Interleave(cs []Candidate, maxConsecutive int, seed uint64) []Candidate {
	if maxConsecutive < 1 {
		maxConsecutive = 1
	}
	byTopic := map[uuid.UUID][]Candidate{}
	var topics []uuid.UUID
	for _, c := range cs {
		if _, ok := byTopic[c.TopicID]; !ok {
			topics = append(topics, c.TopicID)
		}
		byTopic[c.TopicID] = append(byTopic[c.TopicID], c)
	}
	slices.SortFunc(topics, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	r := rng(seed, "qb-interleave")
	r.Shuffle(len(topics), func(i, j int) { topics[i], topics[j] = topics[j], topics[i] })
	out := make([]Candidate, 0, len(cs))
	var last uuid.UUID
	run := 0
	for len(out) < len(cs) {
		best := -1
		for i, t := range topics {
			if len(byTopic[t]) == 0 || (t == last && run >= maxConsecutive) {
				continue
			}
			if best < 0 || len(byTopic[t]) > len(byTopic[topics[best]]) {
				best = i
			}
		}
		if best < 0 { // only the topic of the last run is left
			for _, t := range topics {
				out = append(out, byTopic[t]...)
				byTopic[t] = nil
			}
			break
		}
		t := topics[best]
		out = append(out, byTopic[t][0])
		byTopic[t] = byTopic[t][1:]
		if t == last {
			run++
		} else {
			last, run = t, 1
		}
	}
	return out
}

// Diagnostic picks the questions of a warm-up diagnostic (LEARNING_ANALYTICS §3.5 step 1, F032-AC-25): for the
// objectives in order (prerequisites first), round by round (two each before a third), up to perMax each, never
// more than maxTotal; questions seen recently come last. ErrPoolInsufficient when no objective has a question.
func Diagnostic(pool []Candidate, objectives []uuid.UUID, perMax, maxTotal int, recent []uuid.UUID, seed uint64) ([]Candidate, error) {
	sorted := slices.Clone(pool)
	slices.SortFunc(sorted, func(a, b Candidate) int { return strings.Compare(a.VersionID.String(), b.VersionID.String()) })
	r := rng(seed, "qb-diagnostic")
	r.Shuffle(len(sorted), func(i, j int) { sorted[i], sorted[j] = sorted[j], sorted[i] })
	slices.SortStableFunc(sorted, func(a, b Candidate) int {
		ra, rb := slices.Contains(recent, a.QuestionID), slices.Contains(recent, b.QuestionID)
		switch {
		case ra == rb:
			return 0
		case rb:
			return -1
		}
		return 1
	})
	byLO := map[uuid.UUID][]Candidate{}
	taken := map[uuid.UUID]bool{}
	for _, lo := range objectives {
		for _, c := range sorted {
			if !c.ExamOnly && !taken[c.QuestionID] && slices.Contains(c.ObjectiveIDs, lo) && len(byLO[lo]) < perMax {
				byLO[lo] = append(byLO[lo], c)
				taken[c.QuestionID] = true
			}
		}
	}
	// Round by round: the first rounds give every objective its minimum before any objective gets more.
	var out []Candidate
	for round := 0; round < perMax && len(out) < maxTotal; round++ {
		for _, lo := range objectives {
			if round < len(byLO[lo]) && len(out) < maxTotal {
				out = append(out, byLO[lo][round])
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrPoolInsufficient
	}
	return out, nil
}

// Shift moves a difficulty range by d steps, kept within 1–5 (practice_difficulty of a live recommendation_rule
// experiment arm, F035-AC-24).
func (r Range) Shift(d int) Range {
	clamp := func(v int) int { return min(5, max(1, v)) }
	return Range{Min: clamp(r.Min + d), Max: clamp(r.Max + d)}
}
