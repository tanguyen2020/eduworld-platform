// Package command holds write-context use cases (ADR-0087): load aggregate → domain method →
// persist with UnitOfWork + outbox in one transaction. Calls authz.Require first.
package command
