// Package dto holds the inputs/outputs of the sample use cases with Validate() (ADR-0041).
package dto

import (
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Length limits mirror the ent schema (MaxLen) and the OpenAPI maxLength.
const (
	MaxCode = 32
	MaxName = 128
)

// CreateWidget is the input of the CreateWidget command.
type CreateWidget struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Validate returns 422 VALIDATION_FAILED with MAX_LENGTH/REQUIRED per field.
func (in CreateWidget) Validate() error {
	var pe *problem.Error
	add := func(field, sub, msg string) {
		if pe == nil {
			pe = problem.Invalid(field, sub, msg)
			return
		}
		pe.AddField(field, sub, msg)
	}
	check := func(field, v string, maxLen int) {
		switch {
		case v == "":
			add(field, problem.SubRequired, "required")
		case utf8.RuneCountInString(v) > maxLen:
			add(field, problem.SubMaxLength, "too long")
		}
	}
	check("code", in.Code, MaxCode)
	check("name", in.Name, MaxName)
	if pe == nil {
		return nil
	}
	return pe
}

// RenameWidget renames a widget with optimistic concurrency.
type RenameWidget struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Version int64     `json:"version"`
}

// Validate checks the name.
func (in RenameWidget) Validate() error {
	if in.Name == "" {
		return problem.Invalid("name", problem.SubRequired, "required")
	}
	if utf8.RuneCountInString(in.Name) > MaxName {
		return problem.Invalid("name", problem.SubMaxLength, "too long")
	}
	return nil
}

// WidgetView is the read model row.
type WidgetView struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

// Created is the response of CreateWidget.
type Created struct {
	ID uuid.UUID `json:"id"`
}
