package http

import (
	"context"
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// DocxImportCommands is the Word import of F028 (command.DocxImport).
type DocxImportCommands interface {
	CreateDraft(ctx context.Context, in command.CreateDraftInput) (dto.ImportItem, error)
}

func (h Handlers) listContentImportItems(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "import_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.ListImportItems(r.Context(), query.ImportItemsInput{ImportID: id, Status: r.URL.Query().Get("status"), Page: pageIn(r)})
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, dto.ImportItemJSON)
}

func (h Handlers) createDraftFromImportItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "import_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	no, err := pathInt(r, "item_no")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		MathAssets []command.DraftMathAsset `json:"math_assets"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	if h.DocxImports == nil {
		fail(w, r, problem.New(problem.Unavailable, "Word imports are not enabled"))
		return
	}
	it, err := h.DocxImports.CreateDraft(r.Context(), command.CreateDraftInput{ImportID: id, ItemNo: no, MathAssets: b.MathAssets})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.ImportItemJSON(it))
}
