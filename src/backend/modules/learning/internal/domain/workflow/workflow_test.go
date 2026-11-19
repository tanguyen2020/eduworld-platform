package workflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

var (
	author   = uuid.New()
	reviewer = uuid.New()
)

// F025-AC-01, AC-04, AC-05, AC-07, AC-08: the review transitions, reviewer ≠ author, the task status of each action.
func TestApply(t *testing.T) {
	cases := []struct {
		name   string
		from   contentmodel.Status
		req    Request
		exempt string
		to     contentmodel.Status
		task   TaskStatus
		err    error
	}{
		{"submit", contentmodel.StatusDraft, Request{Action: contentmodel.ActionSubmitReview, Actor: author}, "", contentmodel.StatusInReview, TaskOpen, nil},
		{"approve", contentmodel.StatusInReview, Request{Action: contentmodel.ActionApprove, Actor: reviewer}, "", contentmodel.StatusApproved, TaskApproved, nil},
		{"self approve", contentmodel.StatusInReview, Request{Action: contentmodel.ActionApprove, Actor: author}, "", "", "", ErrSelfReview},
		{"reject", contentmodel.StatusInReview, Request{Action: contentmodel.ActionReject, Actor: reviewer, Note: "thiếu hình"}, "", contentmodel.StatusRejected, TaskRejected, nil},
		{"reject without note", contentmodel.StatusInReview, Request{Action: contentmodel.ActionReject, Actor: reviewer, Note: "  "}, "", "", "", ErrNoteRequired},
		{"reject long note", contentmodel.StatusInReview, Request{Action: contentmodel.ActionReject, Actor: reviewer, Note: strings.Repeat("a", 2001)}, "", "", "", ErrNoteTooLong},
		{"revise rejected", contentmodel.StatusRejected, Request{Action: contentmodel.ActionRevise, Actor: author}, "", contentmodel.StatusDraft, "", nil},
		{"revise approved", contentmodel.StatusApproved, Request{Action: contentmodel.ActionRevise, Actor: author}, "", contentmodel.StatusDraft, "", nil},
		{"approve draft", contentmodel.StatusDraft, Request{Action: contentmodel.ActionApprove, Actor: reviewer}, "", "", "", ErrInvalidTransition},
		{"submit in review", contentmodel.StatusInReview, Request{Action: contentmodel.ActionSubmitReview, Actor: author}, "", "", "", ErrInvalidTransition},
		{"revise published", contentmodel.StatusPublished, Request{Action: contentmodel.ActionRevise, Actor: author}, "", "", "", ErrInvalidTransition},
		{"reject published", contentmodel.StatusPublished, Request{Action: contentmodel.ActionReject, Actor: reviewer, Note: "x"}, "", "", "", ErrInvalidTransition},
		{"publish is F026", contentmodel.StatusApproved, Request{Action: contentmodel.ActionPublish, Actor: reviewer}, "", "", "", ErrInvalidTransition},
		{"exemption not acknowledged", contentmodel.StatusInReview, Request{Action: contentmodel.ActionApprove, Actor: reviewer}, "đọc to lộ đáp án", "", "", ErrExemptionNotAcknowledged},
		{"exemption acknowledged", contentmodel.StatusInReview, Request{Action: contentmodel.ActionApprove, Actor: reviewer, AudioExemptionAcknowledged: true}, "đọc to lộ đáp án", contentmodel.StatusApproved, TaskApproved, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Apply(Version{Status: c.from, Author: author, AudioExemptReason: c.exempt}, c.req)
			if !errors.Is(err, c.err) {
				t.Fatalf("err %v, want %v", err, c.err)
			}
			if err != nil {
				return
			}
			if res.To != c.to || res.Task != c.task || res.From != c.from {
				t.Fatalf("got %+v", res)
			}
			if c.req.Action == contentmodel.ActionApprove && (!res.Reviewer || res.ExemptionApproved != (c.exempt != "")) {
				t.Fatalf("approve flags %+v", res)
			}
		})
	}
}

func TestLaneOf(t *testing.T) {
	tenant := uuid.New()
	if LaneOf(contentmodel.Owner{Scope: contentmodel.OwnerPlatform}) != LanePlatform {
		t.Fatal("platform lane")
	}
	if LaneOf(contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &tenant}) != LaneTenant {
		t.Fatal("tenant lane")
	}
}
