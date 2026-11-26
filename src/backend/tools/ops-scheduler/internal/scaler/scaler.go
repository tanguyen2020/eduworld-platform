// Package scaler runs one reconcile tick of ops-scheduler: plan (schedule) → capacity check → scale through the
// Docker API proxy → metric (F130-AC-22, AC-23). Scaling never follows load (ADR-0024 mục 4).
package scaler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/capacity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/dockerapi"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/ops-scheduler/internal/schedule"
)

// Docker is the port of the Docker Engine API (implemented by dockerapi.Client).
type Docker interface {
	ListServices(ctx context.Context, stack string) ([]dockerapi.Service, error)
	ListNodes(ctx context.Context) ([]dockerapi.Node, error)
	ListRunningTasks(ctx context.Context) ([]dockerapi.Task, error)
	ScaleService(ctx context.Context, s dockerapi.Service, replicas uint64) error
}

// Scaler reconciles the replica count of scheduled services.
type Scaler struct {
	Stack   string
	Planner *schedule.Planner
	Docker  Docker
	Metrics *metrics.Metrics
	Log     *slog.Logger
}

// Outcome is the result of one service in a tick (returned for tests and logs).
type Outcome struct {
	Service, Action, Result string
	From, To                uint64
	Reason                  string
}

// Tick runs one reconcile pass at now.
func (s *Scaler) Tick(ctx context.Context, now time.Time) ([]Outcome, error) {
	services, err := s.Docker.ListServices(ctx, s.Stack)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	byName := map[string]dockerapi.Service{}
	for _, svc := range services {
		byName[svc.Name] = svc
	}
	var ledger *capacity.Ledger
	var outcomes []Outcome
	for _, d := range s.Planner.Next(now) {
		full := s.Stack + "_" + d.Service
		svc, ok := byName[full]
		target := uint64(d.Replicas)
		if !ok || svc.Replicas == nil {
			outcomes = append(outcomes, s.record(Outcome{Service: d.Service, Action: metrics.ActionScaleUp, Result: metrics.ResultError,
				To: target, Reason: "service missing or not replicated"}))
			continue
		}
		current := *svc.Replicas
		if current == target {
			continue
		}
		o := Outcome{Service: d.Service, From: current, To: target, Action: metrics.ActionScaleDown}
		if target > current {
			o.Action = metrics.ActionScaleUp
			if ledger == nil {
				if ledger, err = s.ledger(ctx); err != nil {
					o.Result, o.Reason = metrics.ResultError, err.Error()
					outcomes = append(outcomes, s.record(o))
					ledger = nil
					continue
				}
			}
			if err := ledger.Reserve(svc, current, target); err != nil {
				o.Result, o.Reason = metrics.ResultRejected, err.Error()
				outcomes = append(outcomes, s.record(o))
				continue
			}
		}
		if err := s.Docker.ScaleService(ctx, svc, target); err != nil {
			o.Result, o.Reason = metrics.ResultError, err.Error()
		} else {
			o.Result = metrics.ResultOK
		}
		outcomes = append(outcomes, s.record(o))
	}
	if s.Metrics != nil {
		s.Metrics.LastTick.Set(float64(now.Unix()))
	}
	return outcomes, nil
}

func (s *Scaler) ledger(ctx context.Context) (*capacity.Ledger, error) {
	nodes, err := s.Docker.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	tasks, err := s.Docker.ListRunningTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return capacity.NewLedger(capacity.Free(nodes, tasks)), nil
}

func (s *Scaler) record(o Outcome) Outcome {
	if s.Metrics != nil {
		s.Metrics.ScaleActions.WithLabelValues(o.Service, o.Action, o.Result).Inc()
	}
	if s.Log != nil {
		lvl := slog.LevelInfo
		if o.Result != metrics.ResultOK {
			lvl = slog.LevelWarn
		}
		s.Log.Log(context.Background(), lvl, "scheduled scale", "service", o.Service, "action", o.Action, "result", o.Result,
			"from", o.From, "to", o.To, "reason", o.Reason)
	}
	return o
}
