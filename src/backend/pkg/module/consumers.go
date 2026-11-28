package module

import (
	"sort"
	"strings"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/reconcile"
)

// ConsumerInfo is the source and the event types of one declared outbox consumer (F134: listProcessedEventIds answers
// which consumers of a module receive the events of a source module). It is recorded in every binary from the options
// the factory builds without JetStream; a factory that cannot build them is recorded without types (Source "").
type ConsumerInfo struct {
	Module     string   // receiving module
	Name       string   // <module>.<handler> (the consumer column of infra.processed_events / dead_letters)
	Source     string   // producing module (stream EVT_<SOURCE>)
	EventTypes []string // event types of the subjects; ["*"] for a wildcard subject
}

// recordConsumer builds the options of the factory once with a nil JetStream to read its stream and subjects.
func (r *Registry) recordConsumer(module, name string, build ConsumerFactory) {
	info := ConsumerInfo{Module: module, Name: name}
	if base, _, ok := strings.Cut(name, "@"); ok {
		info.Name = base
	}
	func() {
		defer func() { _ = recover() }() // a factory that needs JetStream leaves the info without source
		o := build(nil)
		info.Source, info.EventTypes = streamSource(o)
	}()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.res.consumerInfos = append(r.res.consumerInfos, info)
	if r.res.factories == nil {
		r.res.factories = map[string]ConsumerFactory{}
	}
	r.res.factories[info.Name] = build
}

// ConsumerOptions builds the options of the declared consumer name without JetStream (nil): its handler, unit of work and
// subjects, for `worker projection rebuild`, which applies the snapshot of the source through the same handler (F003
// Clarifications 2026-11-28). ok is false for a consumer that is not declared or whose factory needs JetStream.
func (r *Registry) ConsumerOptions(name string) (o outbox.ConsumerOptions, ok bool) {
	r.mu.Lock()
	build := r.res.factories[name]
	r.mu.Unlock()
	if build == nil {
		return o, false
	}
	defer func() {
		if recover() != nil {
			o, ok = outbox.ConsumerOptions{}, false
		}
	}()
	o = build(nil)
	return o, o.Handler != nil && o.UoW != nil
}

// streamSource maps EVT_<SOURCE> and ew.evt.<source>.<event_type> subjects to the source and event types.
func streamSource(o outbox.ConsumerOptions) (string, []string) {
	source := strings.ToLower(strings.TrimPrefix(o.Stream, "EVT_"))
	if source == "" || !strings.HasPrefix(o.Stream, "EVT_") {
		return "", nil
	}
	prefix := "ew.evt." + source + "."
	var types []string
	seen := map[string]bool{}
	for _, s := range o.Subjects {
		t := strings.TrimPrefix(s, prefix)
		if t == s {
			continue
		}
		if t == ">" || strings.Contains(t, "*") || strings.Contains(t, ">") {
			return source, []string{"*"}
		}
		if !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}
	sort.Strings(types)
	return source, types
}

// ConsumerInfos returns the declared consumers of module (every one when module is empty), sorted by name.
func (r *Registry) ConsumerInfos(module string) []ConsumerInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ConsumerInfo
	for _, c := range r.res.consumerInfos {
		if module == "" || c.Module == module {
			c.EventTypes = append([]string(nil), c.EventTypes...)
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ReconcileSource returns the query side of listOutboxEventIds / listProcessedEventIds of module (F134) over its
// platform pool and its declared consumers; nil when the module has no database core in this binary.
func (r *Registry) ReconcileSource(module string) *reconcile.Source {
	c, err := r.Core(module)
	if err != nil || c.Platform == nil {
		return nil
	}
	return &reconcile.Source{Module: module, Store: outbox.NewReader(c.Platform), Consumers: func() []reconcile.ConsumerDecl {
		infos := r.ConsumerInfos(module)
		out := make([]reconcile.ConsumerDecl, 0, len(infos))
		for _, i := range infos {
			out = append(out, reconcile.ConsumerDecl{Name: i.Name, Source: i.Source, EventTypes: i.EventTypes})
		}
		return out
	}}
}
