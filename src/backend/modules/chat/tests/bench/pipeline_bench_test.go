// Package bench holds the latency budget test of the F116 moderation pipeline (T014, F116-AC-10, TC-CHAT-012): the
// engine on 200-character messages with a catalogue of placeholder terms (R15) and the maximum number of RE2 patterns
// per tenant must keep p95 under moderation_pipeline_latency (20 ms, ceiling 50 ms).
package bench

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/infrastructure/moderationcfg"
)

// moderation_pipeline_latency (CONFIG_DEFAULTS).
const (
	p95Budget = 20 * time.Millisecond
	ceiling   = 50 * time.Millisecond
)

var cat = uuid.MustParse("01936a00-0000-7000-8000-000000000102")

type cats struct{}

func (cats) Lookup(id uuid.UUID) (catalog.Category, bool) {
	return catalog.Category{ID: id, Code: "insult", Severity: "medium", DefaultAction: "block", Enabled: true}, true
}

// fixture is a large placeholder catalogue: 2,000 word/phrase terms with variants, 100 contains terms and 200 patterns.
func fixture() catalog.Snapshot {
	s := catalog.Snapshot{Scope: "platform", Version: 1}
	for i := range 2000 {
		s.Terms = append(s.Terms, catalog.Term{ID: uuid.New(), CategoryID: cat, Term: fmt.Sprintf("zqterm%04d", i), MatchType: catalog.MatchWord,
			Locale: "vi", Variants: []string{fmt.Sprintf("zqv%04d", i)}})
	}
	for i := range 100 {
		s.Terms = append(s.Terms, catalog.Term{ID: uuid.New(), CategoryID: cat, Term: fmt.Sprintf("zqin%03d", i), MatchType: catalog.MatchContains, Locale: "vi"})
	}
	for i := range moderation.MaxTenantPatterns {
		s.Terms = append(s.Terms, catalog.Term{ID: uuid.New(), CategoryID: cat, Term: fmt.Sprintf(`zqp%03d[0-9]{2,4}x`, i), MatchType: catalog.MatchPattern, Locale: "vi"})
	}
	for i := range 50 {
		s.Allowlist = append(s.Allowlist, catalog.AllowEntry{Phrase: fmt.Sprintf("zqallow%02d town", i), Locale: "vi"})
	}
	return s
}

func messages() []string {
	base := []string{
		"Mình làm xong câu ba rồi nè, bạn làm tới đâu rồi? Cố lên nha cả nhóm ơi, sắp hết giờ rồi đó!",
		"ko bt lm cau nay, ai chi minh voi dc ko, minh cam on nhieu nha. Bai nay kho qua di thoi",
		"HÔM NAY TRỜI ĐẸP QUÁ, CẢ NHÓM CÙNG ĐI SĂN BOSS NHÉ!!! 😀😀😀 đi theo mình nào mọi người",
		"z.q.t.e.r.m.0.0.1.2 và zq0r0p cùng zqp007123x với zqin042 abc, số 0912 345 678 nhé bạn",
	}
	out := make([]string, 0, len(base))
	for _, b := range base {
		for len([]rune(b)) < 200 {
			b += " " + b
		}
		out = append(out, string([]rune(b)[:200]))
	}
	return out
}

// T014 / F116-AC-10: p95 of the whole engine (views, catalogue, PII, spam rules, bullying, context) on 200-character
// messages stays under 20 ms; p99 stays under the 50 ms ceiling.
func TestPipelineLatencyBudget(t *testing.T) {
	engine := moderationcfg.MustEngine()
	compiled, invalid := catalog.Compile(engine.Normalizer(), fixture())
	if len(invalid) > 0 || compiled.Terms() < 2300 {
		t.Fatalf("catalogue: %d terms, invalid %v", compiled.Terms(), invalid)
	}
	sw := moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}
	msgs := messages()
	var lat []time.Duration
	for i := range 1000 {
		start := time.Now()
		out := engine.Evaluate(moderation.Input{View: engine.View(msgs[i%len(msgs)]), Strictness: "strict", Switches: sw, Categories: cats{},
			Catalog: []*catalog.Compiled{compiled}, Spam: &moderation.SpamCounts{Repeats: 1, Channels: 1}, Mentions: i % 2})
		lat = append(lat, time.Since(start))
		if out.Decision == "" {
			t.Fatal("no decision")
		}
	}
	slices.Sort(lat)
	p95, p99, worst := lat[len(lat)*95/100], lat[len(lat)*99/100], lat[len(lat)-1]
	t.Logf("moderation pipeline on 200 chars: p50 %v, p95 %v, p99 %v, max %v (budget p95 %v, ceiling %v)", lat[len(lat)/2], p95, p99,
		worst, p95Budget, ceiling)
	if p95 > p95Budget {
		t.Fatalf("p95 %v > %v", p95, p95Budget)
	}
	// The ceiling is checked on p99: a single GC pause or a loaded CI machine must not fail the build.
	if p99 > ceiling {
		t.Fatalf("p99 %v > ceiling %v", p99, ceiling)
	}
	if !strings.Contains(msgs[3], "zq") {
		t.Fatal("the adversarial message lost its placeholders")
	}
}

func BenchmarkPipeline(b *testing.B) {
	engine := moderationcfg.MustEngine()
	compiled, _ := catalog.Compile(engine.Normalizer(), fixture())
	sw := moderation.Effective{Platform: moderation.AllOn(), At: time.Now()}
	msgs := messages()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = engine.Evaluate(moderation.Input{View: engine.View(msgs[i%len(msgs)]), Switches: sw, Categories: cats{},
			Catalog: []*catalog.Compiled{compiled}, Spam: &moderation.SpamCounts{}})
		i++
	}
}
