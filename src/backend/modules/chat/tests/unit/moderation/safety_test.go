package moderation

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/safety"
)

// T009 (F121): the rules of the Safety Team domain — priority of reports and signals (MODERATION §6.2), report_sla,
// merge window, the case state machine, sanction defaults and the team-lead rule (§7), ack_due_at (§6.4) and the refusal
// of a message by an active sanction (F121-AC-14).
func TestSafetySeverity(t *testing.T) {
	for reason, want := range map[string]string{
		safety.ReasonAskedPersonalInfo: safety.P1, safety.ReasonBullying: safety.P2, safety.ReasonInappropriate: safety.P2,
		safety.ReasonRudeLanguage: safety.P3, safety.ReasonSpam: safety.P3, safety.ReasonCheating: safety.P3, safety.ReasonOther: safety.P3,
	} {
		if got := safety.SeverityOfReport(reason); got != want {
			t.Errorf("%s: %s, want %s", reason, got, want)
		}
	}
	for _, c := range []struct {
		s                     safety.Signal
		sev, source, category string
	}{
		{safety.Signal{Decision: "block_and_alert", Reasons: []string{"self_harm"}}, safety.P1, safety.SourceSelfHarm, "self_harm"},
		{safety.Signal{Decision: "block_and_alert", Reasons: []string{"grooming_scam"}}, safety.P1, safety.SourceFilterAlert, "grooming_scam"},
		{safety.Signal{Decision: "hold_for_review", Reasons: []string{"pii"}}, safety.P1, safety.SourceFilterAlert, "pii"},
		{safety.Signal{Decision: "hold_for_review", Reasons: []string{"sexual"}}, safety.P2, safety.SourceFilterAlert, "sexual"},
		{safety.Signal{Decision: "block_and_alert", Reasons: []string{"violence"}}, safety.P1, safety.SourceFilterAlert, "violence"},
		{safety.Signal{Async: true, Reasons: []string{"bullying"}}, safety.P2, safety.SourceAsyncAnalysis, "bullying"},
	} {
		sev, src, cat := safety.Classify(c.s)
		if sev != c.sev || src != c.source || cat != c.category {
			t.Errorf("%+v: %s %s %s", c.s, sev, src, cat)
		}
	}
	if safety.Higher(safety.P2, safety.P1) != safety.P1 || safety.Higher(safety.P3, safety.P2) != safety.P2 || safety.Higher("", safety.P3) != safety.P3 {
		t.Error("Higher")
	}
}

func TestSafetySLAAndMerge(t *testing.T) {
	at := time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC)
	for sev, want := range map[string][2]time.Duration{safety.P1: {time.Hour, 24 * time.Hour}, safety.P2: {24 * time.Hour, 72 * time.Hour},
		safety.P3: {72 * time.Hour, 168 * time.Hour}} {
		fr, res := safety.Deadlines(sev, at)
		if fr.Sub(at) != want[0] || res.Sub(at) != want[1] {
			t.Errorf("%s: %v %v", sev, fr.Sub(at), res.Sub(at))
		}
	}
	if !safety.Mergeable(safety.StatusOpen, at, at.Add(23*time.Hour)) || !safety.Mergeable(safety.StatusInReview, at, at.Add(time.Hour)) {
		t.Error("merge within 24 h")
	}
	if safety.Mergeable(safety.StatusOpen, at, at.Add(24*time.Hour)) || safety.Mergeable(safety.StatusEscalate, at, at) ||
		safety.Mergeable(safety.StatusResolved, at, at) {
		t.Error("no merge after 24 h or in another status")
	}
}

func TestSafetyStateMachine(t *testing.T) {
	if s, err := safety.Assign(safety.StatusOpen, true, false); err != nil || s != safety.StatusInReview {
		t.Fatalf("self-claim: %s %v", s, err)
	}
	if _, err := safety.Assign(safety.StatusOpen, false, false); !errors.Is(err, safety.ErrNeedsLead) {
		t.Fatalf("assign other without lead: %v", err)
	}
	if _, err := safety.Assign(safety.StatusEscalate, true, false); !errors.Is(err, safety.ErrNeedsLead) {
		t.Fatalf("escalated needs lead: %v", err)
	}
	if s, err := safety.Assign(safety.StatusEscalate, false, true); err != nil || s != safety.StatusInReview {
		t.Fatalf("lead takes escalated: %s %v", s, err)
	}
	if _, err := safety.Assign(safety.StatusResolved, true, true); !errors.Is(err, safety.ErrClosed) {
		t.Fatalf("resolved: %v", err)
	}
	for _, c := range []struct {
		from, action, to string
		err              error
	}{
		{safety.StatusInReview, safety.ActionEscalate, safety.StatusEscalate, nil},
		{safety.StatusInReview, safety.ActionClose, safety.StatusResolved, nil},
		{safety.StatusEscalate, safety.ActionDismiss, safety.StatusResolved, nil},
		{safety.StatusInReview, safety.ActionRestrict, safety.StatusInReview, nil},
		{safety.StatusResolved, safety.ActionReopen, safety.StatusReopened, nil},
		{safety.StatusResolved, safety.ActionWarn, "", safety.ErrClosed},
		{safety.StatusOpen, safety.ActionClose, "", safety.ErrTransition},
		{safety.StatusEscalate, safety.ActionEscalate, "", safety.ErrTransition},
		{safety.StatusInReview, safety.ActionReopen, "", safety.ErrTransition},
	} {
		got, err := safety.Act(c.from, c.action)
		if got != c.to || !errors.Is(err, c.err) {
			t.Errorf("%s --%s--> %s %v", c.from, c.action, got, err)
		}
	}
	me, other := uuid.New(), uuid.New()
	if !safety.CanRead(&me, me, false) || safety.CanRead(&other, me, false) || safety.CanRead(nil, me, false) || !safety.CanRead(nil, me, true) {
		t.Error("CanRead")
	}
}

func TestSafetySanctionRules(t *testing.T) {
	for typ, want := range map[string]time.Duration{safety.SanctionMute: 24 * time.Hour, safety.SanctionRestrictText: 7 * 24 * time.Hour,
		safety.SanctionRestrictDM: 30 * 24 * time.Hour, safety.SanctionRestrictMultiplayer: 7 * 24 * time.Hour} {
		if d, ok := safety.DefaultDuration(typ); !ok || d != want {
			t.Errorf("%s default %v", typ, d)
		}
	}
	if _, ok := safety.DefaultDuration(safety.SanctionAccountLock); ok {
		t.Error("account_lock has no default end")
	}
	if !safety.RequiresLead(safety.SanctionAccountLock, 0, false) || !safety.RequiresLead(safety.SanctionRestrictMultiplayer, 8*24*time.Hour, true) ||
		safety.RequiresLead(safety.SanctionRestrictMultiplayer, 7*24*time.Hour, true) || safety.RequiresLead(safety.SanctionMute, 1000*time.Hour, true) {
		t.Error("RequiresLead")
	}
	at := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	if safety.AckDue(safety.P1, "", at).Sub(at) != 24*time.Hour || safety.AckDue(safety.P2, "self_harm", at).Sub(at) != 24*time.Hour ||
		safety.AckDue(safety.P2, "bullying", at).Sub(at) != 72*time.Hour {
		t.Error("AckDue")
	}
	if safety.SupportGuide("self_harm") != safety.SupportGuideChildProtection || safety.SupportGuide("bullying") != "" {
		t.Error("SupportGuide")
	}
	for _, c := range []struct {
		typ, kind, ch string
		match         bool
		want          safety.Refusal
	}{
		{safety.SanctionMute, "quick", "match", true, safety.RefuseMuted},
		{safety.SanctionMute, "text", "party", false, safety.RefuseNone},
		{safety.SanctionRestrictText, "text", "match", true, safety.RefuseKind},
		{safety.SanctionRestrictText, "emoji", "party", true, safety.RefuseKind},
		{safety.SanctionRestrictText, "quick", "match", true, safety.RefuseNone},
		{safety.SanctionRestrictDM, "text", "direct", true, safety.RefuseDirect},
		{safety.SanctionRestrictDM, "text", "party", true, safety.RefuseNone},
		{safety.SanctionRestrictMultiplayer, "text", "party", true, safety.RefuseNone},
		{safety.SanctionWarning, "text", "party", true, safety.RefuseNone},
	} {
		if got := safety.Refuse(c.typ, c.match, c.kind, c.ch); got != c.want {
			t.Errorf("%s %s %s: %q", c.typ, c.kind, c.ch, got)
		}
	}
	if !safety.Holds(safety.CodeCaseHold) || !safety.Holds("block_and_alert_hold") || safety.Holds("auto_spam") {
		t.Error("Holds")
	}
}
