package moderation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/pipeline"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
)

// fakeCatalogs and fakeSwitches are TEST DOUBLES (R17) of the compiled catalogue store and the switch resolver.
type fakeCatalogs struct {
	view service.CatalogView
	err  error
}

func (f fakeCatalogs) Get(context.Context, uuid.UUID) (service.CatalogView, error) {
	return f.view, f.err
}

type fakeSwitches struct{ e moderation.Effective }

func (f fakeSwitches) Effective(context.Context, uuid.UUID) (moderation.Effective, uuid.UUID) {
	return f.e, uuid.Nil
}

// moderateDisplayText (F116, party/team names): the chat pipeline decides; only a clean name is allowed, a masked one
// is blocked, PII and grooming are checked even with every switch off, a catalogue failure fails closed.
func TestCheckDisplayText(t *testing.T) {
	view := service.CatalogView{Parts: []*catalog.Compiled{compiled(t)}, Categories: categories(), PlatformVersion: 7}
	allOn := moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}
	v := &pipeline.Validator{Engine: engine, Catalogs: fakeCatalogs{view: view}, Switches: fakeSwitches{e: allOn}}
	ctx, tenant := context.Background(), uuid.New()
	for text, want := range map[string]bool{
		"Nhóm Sao Băng":     true,
		"Đội zqơrốp":        false, // catalogue (block)
		"Nhóm zqlow":        false, // mask is never shown partly hidden
		"Nhóm zalo 0912345": false, // PII
		"   ":               false,
	} {
		if got, err := v.CheckDisplayText(ctx, tenant, text); err != nil || got != want {
			t.Fatalf("%q: %v %v, want %v", text, got, err, want)
		}
	}
	exp := time.Now().Add(time.Hour)
	off := moderation.Effective{Platform: moderation.AllOn(), Tenant: &moderation.Switch{EnabledAll: false, Steps: map[string]bool{}, ExpiresAt: &exp},
		At: time.Now()}
	v.Switches = fakeSwitches{e: off}
	if got, _ := v.CheckDisplayText(ctx, tenant, "Đội zqơrốp"); !got {
		t.Fatal("switched-off terms step: the name is unchecked, allowed")
	}
	if got, _ := v.CheckDisplayText(ctx, tenant, "Nhóm zalo của mình"); got {
		t.Fatal("PII must be checked with every switch off")
	}
	v.Catalogs = fakeCatalogs{err: errors.New("TEST DOUBLE: not loaded")}
	if _, err := v.CheckDisplayText(ctx, tenant, "Nhóm Sao Băng"); err == nil {
		t.Fatal("catalogue failure must fail closed")
	}
}
