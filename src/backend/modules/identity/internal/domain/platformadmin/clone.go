package platformadmin

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// MaxCloneResources is the limit of resource_ids per clone job (TenantCloneRequest.maxItems, F045-AC-16).
const MaxCloneResources = 500

// MaxPlanSteps bounds the number of steps of a clone plan (the saga definition has that many slots).
const MaxPlanSteps = 8

// Clone job statuses (platform.tenant_clone_jobs.status).
const (
	CloneQueued          = "queued"
	CloneRunning         = "running"
	CloneSucceeded       = "succeeded"
	ClonePartiallyFailed = "partially_failed"
	CloneFailed          = "failed"
)

// Validation errors of a clone request (422 VALIDATION_FAILED with the field).
var (
	ErrCloneSameTenant    = errors.New("platformadmin: the source and target tenants must differ")
	ErrCloneNotAllowed    = errors.New("platformadmin: resource_type cannot be cloned")
	ErrCloneNoResources   = errors.New("platformadmin: resource_ids is empty")
	ErrCloneTooManyIDs    = errors.New("platformadmin: resource_ids has more than 500 items")
	ErrCloneDuplicateIDs  = errors.New("platformadmin: resource_ids has duplicates")
	ErrCloneTenantMissing = errors.New("platformadmin: the tenant does not exist or is not active")
)

// CloneStep is one step of a clone plan: one module clones one resource type, receiving the id_map of the
// previous steps (dependencies first, F045-AC-13).
type CloneStep struct {
	Module       string `json:"module"`
	ResourceType string `json:"resource_type"`
	// Root is true for the requested type (its resource_ids are the request's); dependencies receive the ids
	// the module discovers itself (no dependency type is planned in Release 1.0).
	Root bool `json:"root"`
}

// dependencies lists, per type, the types cloned before it by another module (cross-module references, e.g.
// quest → question_set). Same-module dependencies are cloned by the owning module in its own step.
var dependencies = map[string][]string{}

// PlanClone validates a clone request and returns its ordered steps.
func PlanClone(source, target uuid.UUID, rt string, ids []uuid.UUID) ([]CloneStep, error) {
	if source == target {
		return nil, ErrCloneSameTenant
	}
	if !Clonable(rt) {
		return nil, fmt.Errorf("%w: %q", ErrCloneNotAllowed, rt)
	}
	switch {
	case len(ids) == 0:
		return nil, ErrCloneNoResources
	case len(ids) > MaxCloneResources:
		return nil, ErrCloneTooManyIDs
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			return nil, ErrCloneDuplicateIDs
		}
		seen[id] = true
	}
	var steps []CloneStep
	for _, dep := range dependencies[rt] {
		m, err := Owner(dep)
		if err != nil {
			return nil, err
		}
		steps = append(steps, CloneStep{Module: m, ResourceType: dep})
	}
	m, err := Owner(rt)
	if err != nil {
		return nil, err
	}
	steps = append(steps, CloneStep{Module: m, ResourceType: rt, Root: true})
	if len(steps) > MaxPlanSteps {
		return nil, fmt.Errorf("platformadmin: clone plan of %q has %d steps", rt, len(steps))
	}
	return steps, nil
}

// ClonePair maps one source record to its copy.
type ClonePair struct {
	ResourceType string    `json:"resource_type"`
	SourceID     uuid.UUID `json:"source_id"`
	CloneID      uuid.UUID `json:"clone_id"`
}

// CloneStepOutcome is what a module answered for one step (saga_steps.response; the pairs themselves are
// appended to the id_map of the job, which may hold far more than a step response).
type CloneStepOutcome struct {
	StepNo       int    `json:"step_no"`
	Module       string `json:"module"`
	ResourceType string `json:"resource_type"`
	Status       string `json:"status"` // succeeded | partially_failed | failed (module status); "" = not run
	Cloned       int    `json:"cloned"`
	Skipped      int    `json:"skipped"`
	Error        string `json:"error,omitempty"`
}

// CloneResult is the final state of a job computed from its steps (the id_map stays as the steps wrote it).
type CloneResult struct {
	Status string
	Count  int
	Stats  map[string]int
	Errors []string
}

// SummarizeClone folds the step outcomes into the job result: every step succeeded → succeeded; some copies
// made but a step failed or skipped records → partially_failed; nothing copied and a step failed → failed.
func SummarizeClone(outcomes []CloneStepOutcome, sagaFailed bool, sagaError string) CloneResult {
	res := CloneResult{Status: CloneSucceeded, Stats: map[string]int{}, Errors: []string{}}
	problems := false
	for _, o := range outcomes {
		res.Count += o.Cloned
		res.Stats[o.ResourceType] += o.Cloned
		if o.Skipped > 0 {
			res.Stats[o.ResourceType+"_skipped"] += o.Skipped
		}
		if o.Status != CloneSucceeded {
			problems = true
		}
		if o.Error != "" {
			res.Errors = append(res.Errors, StepError(o.Module, o.StepNo, o.Error))
		}
	}
	if sagaFailed {
		problems = true
		if sagaError != "" && len(res.Errors) == 0 {
			res.Errors = append(res.Errors, Bound(sagaError, 255))
		}
	}
	switch {
	case !problems:
	case res.Count > 0:
		res.Status = ClonePartiallyFailed
	default:
		res.Status = CloneFailed
	}
	return res
}

// StepError formats the error of a saga step for the API (TenantCloneJob.errors, maxLength 255).
func StepError(module string, step int, msg string) string {
	return Bound(fmt.Sprintf("%s step %d: %s", module, step, msg), 255)
}

// Bound cuts s to n bytes on a rune boundary.
func Bound(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && (s[n]&0xC0) == 0x80 {
		n--
	}
	return s[:n]
}
