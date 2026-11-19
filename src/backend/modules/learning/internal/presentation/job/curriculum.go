// Package job holds the job entry points of module learning (worker profile jobs, seed): thin runners over the use
// cases; scheduling belongs to the binary that wires them (F020-3).
package job

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// ImportRunner runs queued content import jobs (command.Curriculum).
type ImportRunner interface {
	RunPendingImports(ctx context.Context, limit int) (int, error)
}

// ImportWorker is the import job of the worker profile jobs (F021 T032): every tick it runs up to Batch queued jobs.
type ImportWorker struct {
	Runner ImportRunner
	Batch  int
}

// RunOnce runs one batch and returns how many jobs ran.
func (w ImportWorker) RunOnce(ctx context.Context) (int, error) {
	batch := w.Batch
	if batch <= 0 {
		batch = 10
	}
	return w.Runner.RunPendingImports(ctx, batch)
}

// TranslationRunner runs queued translation jobs (command.Translations, F025 US5).
type TranslationRunner interface {
	RunPendingTranslations(ctx context.Context, limit int) (int, error)
}

// TranslationWorker is the translation-jobs job of the worker profile jobs (F025 T041): every tick it runs up to Batch
// queued jobs.
type TranslationWorker struct {
	Runner TranslationRunner
	Batch  int
}

// RunOnce runs one batch and returns how many jobs ran.
func (w TranslationWorker) RunOnce(ctx context.Context) (int, error) {
	batch := w.Batch
	if batch <= 0 {
		batch = 10
	}
	return w.Runner.RunPendingTranslations(ctx, batch)
}

// Seeder is what the seed needs of the curriculum use cases (command.Curriculum).
type Seeder interface {
	SeedSubjects(ctx context.Context, subjects []command.SubjectInput) (int, error)
	SeedCurriculum(ctx context.Context, kind string, r io.Reader, at time.Time) (command.SeedResult, error)
}

type subjectsFile struct {
	Subjects []struct {
		Code        string            `yaml:"code"`
		Name        map[string]string `yaml:"name"`
		GradeLevels []int             `yaml:"grade_levels"`
	} `yaml:"subjects"`
}

// SeedResult is what one seed run did per curriculum file.
type SeedResult struct {
	SubjectsCreated int
	Curricula       map[string]command.SeedResult
}

// Seed loads dir/subjects.yaml, then every other *.yaml of dir through the import use case and publishes it
// (F021-AC-23): a second run creates nothing.
func Seed(ctx context.Context, h Seeder, dir string) (SeedResult, error) {
	out := SeedResult{Curricula: map[string]command.SeedResult{}}
	b, err := os.ReadFile(filepath.Join(dir, "subjects.yaml")) //nolint:gosec // seed directory of the repository
	if err != nil {
		return out, err
	}
	var sf subjectsFile
	if err := yaml.Unmarshal(b, &sf); err != nil {
		return out, fmt.Errorf("subjects.yaml: %w", err)
	}
	subjects := make([]command.SubjectInput, len(sf.Subjects))
	for i, s := range sf.Subjects {
		subjects[i] = command.SubjectInput{Code: s.Code, Name: s.Name, GradeLevels: s.GradeLevels}
	}
	if out.SubjectsCreated, err = h.SeedSubjects(ctx, subjects); err != nil {
		return out, fmt.Errorf("subjects: %w", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return out, err
	}
	sort.Strings(files)
	for _, f := range files {
		name := filepath.Base(f)
		if name == "subjects.yaml" {
			continue
		}
		r, err := os.Open(f) //nolint:gosec // seed directory of the repository
		if err != nil {
			return out, err
		}
		res, err := h.SeedCurriculum(ctx, port.KindCurriculumYAML, r, time.Now())
		_ = r.Close()
		if err != nil {
			return out, fmt.Errorf("%s: %w", name, err)
		}
		if len(res.Errors) > 0 {
			return out, fmt.Errorf("%s: %d rejected rows, first: row %d %s %s", name, len(res.Errors), res.Errors[0].Row, res.Errors[0].Field,
				res.Errors[0].Code)
		}
		out.Curricula[name] = res
		slog.InfoContext(ctx, "learning seed: curriculum", "file", name, "curriculum_id", res.CurriculumID, "created", res.Created,
			"already_published", res.Skipped)
	}
	return out, nil
}
