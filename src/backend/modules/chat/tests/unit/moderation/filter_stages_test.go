package moderation

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/pipeline"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/service"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
)

// stageMetrics, noSanctions and openSpam are TEST DOUBLES (R17) of the pipeline metrics, the sanction store and the spam
// window.
type stageMetrics struct {
	mu     sync.Mutex
	stages []string
}

func (m *stageMetrics) FilterDuration(stage string, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stages = append(m.stages, stage)
}
func (*stageMetrics) Match(string, string) {}
func (*stageMetrics) Unchecked(uuid.UUID)  {}
func (*stageMetrics) RE2Timeouts(int)      {}
func (*stageMetrics) DecisionDropped()     {}

type noSanctions struct{}

func (noSanctions) Active(context.Context, uuid.UUID, uuid.UUID, time.Time) ([]port.Sanction, error) {
	return nil, nil
}

type openSpam struct{}

func (openSpam) Record(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (moderation.SpamCounts, error) {
	return moderation.SpamCounts{}, nil
}
func (openSpam) Block(context.Context, uuid.UUID, time.Time) (int, error) { return 0, nil }

// MONITORING §3: eduworld_chat_filter_duration_seconds is observed once per stage that ran, in pipeline order, plus
// the whole pipeline (total); the spam stage only when the spam step runs.
func TestFilterDurationStages(t *testing.T) {
	view := service.CatalogView{Parts: nil, Categories: categories(), PlatformVersion: 1}
	m := &stageMetrics{}
	v := &pipeline.Validator{Engine: engine, Catalogs: fakeCatalogs{view: view},
		Switches: fakeSwitches{e: moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}}, Sanctions: noSanctions{},
		Spam: openSpam{}, Metrics: m}
	in := port.ValidationInput{TenantID: uuid.New(), ChannelID: uuid.New(), ChannelType: "world", SenderID: uuid.New(),
		Kind: message.KindText, Text: "chào cả nhóm"}
	if _, err := v.Validate(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if want := []string{pipeline.StageSanctions, pipeline.StageCatalog, pipeline.StageSpam, pipeline.StageEvaluate, pipeline.StageTotal}; !slices.Equal(m.stages, want) {
		t.Fatalf("stages %v, want %v", m.stages, want)
	}
	m.stages = nil
	in.Derived = true // quote excerpt: no spam window
	if _, err := v.Validate(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if want := []string{pipeline.StageSanctions, pipeline.StageCatalog, pipeline.StageEvaluate, pipeline.StageTotal}; !slices.Equal(m.stages, want) {
		t.Fatalf("stages %v, want %v", m.stages, want)
	}
}
