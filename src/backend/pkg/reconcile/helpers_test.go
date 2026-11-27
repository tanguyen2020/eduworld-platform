package reconcile

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall"
)

// nopRecorder is a TEST DOUBLE of the audit recorder (R17): the bypass audit is checked in the integration test.
type nopRecorder struct{}

func (nopRecorder) Record(context.Context, audit.Event) error { return nil }

// modcallCtx returns a context as a facade sees it inside a pkg/modcall call from caller (values kept, deadline dropped).
func modcallCtx(caller string) context.Context {
	var got context.Context
	_, _ = modcall.Call(context.Background(), modcall.Op{Caller: caller, Callee: "learning", Operation: "listOutboxEventIds",
		AllowedCallers: []string{caller}}, func(ctx context.Context) (int, error) {
		got = ctx
		return 0, nil
	})
	return context.WithoutCancel(got)
}
