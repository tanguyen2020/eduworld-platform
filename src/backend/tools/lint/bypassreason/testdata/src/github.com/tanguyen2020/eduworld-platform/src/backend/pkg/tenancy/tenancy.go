package tenancy

import "context"

type BypassReason string

const (
	ReasonTrash          BypassReason = "trash"
	ReasonModerationCase BypassReason = "moderation_case"
)

type BypassOption func()

func Bypass(ctx context.Context, reason BypassReason, opts ...BypassOption) (context.Context, error) {
	return ctx, nil
}

type CrossTenantScope struct{}
