package unit

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
)

// Delta F112-1 (domain): the owner edits the club, names officers and creates goals and events; an officer only handles
// join requests and invitations (SOCIAL_SYSTEM §6.2); a member neither.
func TestF112ClubRoles(t *testing.T) {
	for _, c := range []struct {
		role           string
		manager, owner bool
	}{
		{club.RoleOwner, true, true},
		{club.RoleOfficer, true, false},
		{club.RoleMember, false, false},
		{"", false, false},
	} {
		if club.Manager(c.role) != c.manager || club.Editor(c.role) != c.owner {
			t.Errorf("%q: manager %v editor %v", c.role, club.Manager(c.role), club.Editor(c.role))
		}
	}
}

// Delta F112-3 (domain): a student creates practice_session, coop_quest, boss_raid, other ("" → other);
// internal_tournament or a tournament template → ErrEventKindUnavailable; a value outside the contract → ErrEventKind.
func TestF112StudentEventKind(t *testing.T) {
	tpl := uuid.New()
	for _, c := range []struct {
		kind     string
		template *uuid.UUID
		want     string
		err      error
	}{
		{"", nil, club.EventOther, nil},
		{club.EventPractice, nil, club.EventPractice, nil},
		{club.EventCoopQuest, nil, club.EventCoopQuest, nil},
		{club.EventBossRaid, nil, club.EventBossRaid, nil},
		{club.EventOther, nil, club.EventOther, nil},
		{club.EventTournament, nil, "", club.ErrEventKindUnavailable},
		{club.EventTournament, &tpl, "", club.ErrEventKindUnavailable},
		{club.EventPractice, &tpl, "", club.ErrEventKindUnavailable},
		{"party", nil, "", club.ErrEventKind},
	} {
		got, err := club.StudentEventKind(c.kind, c.template)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%q %v: %q %v", c.kind, c.template != nil, got, err)
		}
	}
}

// Delta F112-4 (domain): AcceptedMetrics is the subset of the contract metrics counted from a verified event; the others
// of the contract are ErrMetricUnavailable, anything else ErrMetric.
func TestF112GoalMetrics(t *testing.T) {
	if !slices.Equal(club.AcceptedMetrics, []string{"activities_completed", "quests_completed", "discoveries_found"}) {
		t.Fatalf("accepted: %v", club.AcceptedMetrics)
	}
	for _, m := range club.AcceptedMetrics {
		if !slices.Contains(club.Metrics, m) || club.CheckMetric(m) != nil {
			t.Errorf("%s not accepted", m)
		}
	}
	for _, m := range []string{club.MetricMinutes, club.MetricBoss} {
		if !errors.Is(club.CheckMetric(m), club.ErrMetricUnavailable) {
			t.Errorf("%s: %v", m, club.CheckMetric(m))
		}
	}
	if !errors.Is(club.CheckMetric("score_total"), club.ErrMetric) {
		t.Error("unknown metric accepted")
	}
}

// Delta F112-7 (domain): what one level leaves of the direct channel — off when its direct rule is disabled or has no
// valid kind; the platform level also off without a direct rule; a lower level without a rule tightens nothing.
func TestF112DirectMessageEnabled(t *testing.T) {
	on, off := true, false
	doc := func(rules ...policy.Rule) policy.Document { return policy.Document{ScopeType: "x", Rules: rules} }
	direct := func(enabled *bool, kinds ...policy.Kind) policy.Rule {
		return policy.Rule{ChannelType: "direct", Enabled: enabled, AllowedKinds: kinds}
	}
	for _, c := range []struct {
		name     string
		d        policy.Document
		platform bool
		want     bool
	}{
		{"platform default", doc(direct(&on, policy.KindText, policy.KindQuick)), true, true},
		{"platform without rule", doc(policy.Rule{ChannelType: "party"}), true, false},
		{"platform disabled", doc(direct(&off, policy.KindText)), true, false},
		{"platform without kinds", doc(direct(&on)), true, false},
		{"tenant without rule", doc(policy.Rule{ChannelType: "world", Enabled: &off}), false, true},
		{"school disables", doc(direct(&off)), false, false},
		{"school narrows kinds", doc(direct(nil, policy.KindQuick)), false, true},
		{"school invalid kinds only", doc(policy.Rule{ChannelType: "direct", AllowedKinds: []policy.Kind{"voice"}}), false, false},
		{"common default", *policy.CommonDefaults(policy.CommonOverride{}), false, false},
	} {
		if got := policy.DirectMessageEnabled(c.d, c.platform); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// Delta F112-6 (domain): reopen_at of CHAT_NOT_ALLOWED is the first minute after now at which every level's hours are
// open and the curfew does not refuse it; none within a week → false.
func TestF112ReopenAt(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	monday := func(h, m int) time.Time { return time.Date(2026, 11, 23, h, m, 30, 0, ict) }
	school := []policy.TimeWindow{{Days: "mon-sun", From: "07:00", To: "20:00"}}
	platform := []policy.TimeWindow{{Days: "mon-sun", From: "06:30", To: "21:30"}}
	curfew, err := policy.ParseWindow("21:30-06:30", ict)
	if err != nil {
		t.Fatal(err)
	}
	blocked := func(t time.Time) bool { on, _ := curfew.Active(t); return on }
	got, ok := policy.NextOpen([][]policy.TimeWindow{platform, school}, monday(20, 31), ict, blocked)
	if !ok || !got.Equal(time.Date(2026, 11, 24, 7, 0, 0, 0, ict)) || got.Location() != time.UTC {
		t.Fatalf("after the school hours: %v %v", got, ok)
	}
	// Before the opening the same day; a weekend-only school window from Monday evening → Saturday.
	if got, ok := policy.NextOpen([][]policy.TimeWindow{school}, monday(5, 0), ict, nil); !ok || !got.Equal(monday(7, 0).Truncate(time.Minute)) {
		t.Fatalf("before opening: %v %v", got, ok)
	}
	weekend := []policy.TimeWindow{{Days: "sat-sun", From: "08:00", To: "10:00"}}
	if got, ok := policy.NextOpen([][]policy.TimeWindow{weekend}, monday(20, 31), ict, nil); !ok ||
		!got.Equal(time.Date(2026, 11, 28, 8, 0, 0, 0, ict)) {
		t.Fatalf("weekend: %v %v", got, ok)
	}
	// Hours only inside the curfew, or no window at all: no reopen time.
	night := []policy.TimeWindow{{Days: "mon-sun", From: "22:00", To: "23:00"}}
	if _, ok := policy.NextOpen([][]policy.TimeWindow{night}, monday(20, 31), ict, blocked); ok {
		t.Fatal("reopen inside the curfew")
	}
	if _, ok := policy.NextOpen([][]policy.TimeWindow{{}}, monday(20, 31), ict, nil); ok {
		t.Fatal("reopen with an empty window list")
	}
	e := policy.Effective{Hours: [][]policy.TimeWindow{platform, school}}
	if got, ok := e.ReopenAt(monday(20, 31), ict, blocked); !ok || !got.Equal(time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("effective: %v %v", got, ok)
	}
}

// Delta F112-8 (domain): the highlight is the display text in plain text (no markup added) with code point ranges of
// the search words — prefix, any case, with or without Vietnamese diacritics; never inside a masked part.
func TestF112Highlight(t *testing.T) {
	rg := func(start, length int) message.Range { return message.Range{Start: start, Length: length} }
	for _, c := range []struct {
		name, body, q string
		want          []message.Range
	}{
		{"contract example", "Mai mình luyện Toán nhé, nhân vật mới đẹp quá!", "nhan vat", []message.Range{rg(25, 4), rg(30, 3)}},
		{"accented query", "Mai mình luyện Toán nhé", "TOÁN", []message.Range{rg(15, 4)}},
		{"prefix", "nhân vật đẹp", "nh", []message.Range{rg(0, 2)}},
		{"đ folds to d", "Đi đâu đó", "di", []message.Range{rg(0, 2)}},
		{"no partial word inside", "bannhân", "nhan", nil},
		{"masked part", "Nhóm mình *** quá!", "nhom ngu", []message.Range{rg(0, 4)}},
		{"emoji before", "🙂 Toán vui", "toan", []message.Range{rg(2, 4)}},
		{"no word", "Xin chào", "!!", nil},
	} {
		text, got := message.Highlight(c.body, c.q, 8)
		if text != c.body {
			t.Errorf("%s: text changed: %q", c.name, text)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// Decomposed input is returned in NFC and the ranges count its code points.
	text, got := message.Highlight("Toán", "toan", 8)
	if text != "Toán" || !slices.Equal(got, []message.Range{rg(0, 4)}) {
		t.Fatalf("NFC: %q %v", text, got)
	}
	// Caps: at most HighlightMaxRunes code points and HighlightMaxRanges ranges, at most maxWords words.
	long := ""
	for range 600 {
		long += "ab "
	}
	text, got = message.Highlight(long, "ab", 8)
	if n := len([]rune(text)); n != message.HighlightMaxRunes || len(got) != message.HighlightMaxRanges {
		t.Fatalf("caps: %d runes %d ranges", n, len(got))
	}
	if _, got := message.Highlight("mot hai ba", "mot hai ba", 2); len(got) != 2 {
		t.Fatalf("max words: %v", got)
	}
}
