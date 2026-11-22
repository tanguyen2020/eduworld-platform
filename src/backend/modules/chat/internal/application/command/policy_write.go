package command

import (
	"errors"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// CheckPolicyDocument is the always-on gate of every chat policy write path (F117-AC-09; the write use cases of F120/F122
// call it before storing a document, and the CHECK chat_policies_always_on_chk refuses the same rule in the database):
// 422 CHAT_POLICY_ALWAYS_ON with the offending field when a level turns off or restricts match/team/party, 422
// VALIDATION_FAILED when the document is not a ChatPolicy.
func CheckPolicyDocument(raw []byte) error {
	d, err := policy.Parse(raw)
	if err != nil {
		return problem.Invalid("rules", problem.SubInvalid, "not a ChatPolicy document")
	}
	var ae policy.AlwaysOnError
	if err := policy.ValidateAlwaysOn(d); errors.As(err, &ae) {
		return problem.New(problem.ChatPolicyAlwaysOn, "").With("channel_type", ae.ChannelType).With("field", ae.Field)
	}
	return nil
}
