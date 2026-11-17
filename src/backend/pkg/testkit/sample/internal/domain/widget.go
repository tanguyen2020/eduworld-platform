// Package domain holds the Widget aggregate of the sample module (standard library only, R13).
package domain

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidWidget is returned for a rule violation of the aggregate.
var ErrInvalidWidget = errors.New("widget: invalid")

// Widget is the aggregate.
type Widget struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Code     string
	Name     string
	Version  int64
}

// NewWidget creates a widget; the id and tenant are assigned by persistence.
func NewWidget(code, name string) (*Widget, error) {
	if code == "" || name == "" {
		return nil, ErrInvalidWidget
	}
	return &Widget{Code: code, Name: name, Version: 1}, nil
}

// Rename changes the name.
func (w *Widget) Rename(name string) error {
	if name == "" {
		return ErrInvalidWidget
	}
	w.Name = name
	return nil
}
