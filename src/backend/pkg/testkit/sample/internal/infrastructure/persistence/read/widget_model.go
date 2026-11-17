// Package read implements the read ports of the sample module in entx.ReadSession (READ ONLY).
package read

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/ent/widget"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit/sample/internal/application/dto"
)

// WidgetModel implements read.WidgetReadModel.
type WidgetModel struct{ Session *entx.ReadSession }

// List returns the widgets of the scope ordered by code.
func (m WidgetModel) List(ctx context.Context) (out []dto.WidgetView, err error) {
	err = m.Session.Read(ctx, func(ctx context.Context) error {
		rows, err := ent.NewClient(ent.Driver(entx.Driver(ctx))).Widget.Query().Order(ent.Asc(widget.FieldCode)).All(ctx)
		if err != nil {
			return err
		}
		out = make([]dto.WidgetView, 0, len(rows))
		for _, r := range rows {
			out = append(out, dto.WidgetView{ID: r.ID, Code: r.Code, Name: r.Name})
		}
		return nil
	})
	return out, err
}
