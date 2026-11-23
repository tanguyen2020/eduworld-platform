package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// EconomyQueries are the economy read use cases (query.Economy, F075).
type EconomyQueries interface {
	GetWallet(ctx context.Context) (query.WalletView, error)
	ListWalletLedger(ctx context.Context, limit int, cursor, lang string) (query.Page[query.LedgerView], error)
	ListShopItems(ctx context.Context, kind string, limit int, cursor string) (query.Page[query.ShopItemView], error)
	ListInventory(ctx context.Context, kind string, limit int, cursor string) (query.Page[query.InventoryView], error)
	ListInventoryHistory(ctx context.Context, limit int, cursor, lang string) (query.Page[query.ItemHistoryView], error)
	ListCollections(ctx context.Context, limit int, cursor string) (query.Page[query.CollectionView], error)
	GetCollection(ctx context.Context, collection uuid.UUID, lang string) (query.CollectionDetailView, error)
	ListAchievements(ctx context.Context, category string, limit int, cursor string) (query.Page[query.AchievementView], error)
	ListAdjustments(ctx context.Context, in query.AdjustmentsInput) (query.Page[read.AdjustmentRecord], error)
}

// EconomyCommands are the economy write use cases (command.Economy, F075).
type EconomyCommands interface {
	Purchase(ctx context.Context, shopItem uuid.UUID, quantity, expectedPrice int) (command.PurchaseResult, error)
	CreateAdjustment(ctx context.Context, in command.AdjustmentInput) (command.AdjustmentView, error)
	ApproveAdjustment(ctx context.Context, id uuid.UUID) (command.AdjustmentView, error)
	RejectAdjustment(ctx context.Context, id uuid.UUID, reason string) (command.AdjustmentView, error)
}

// EconomyOperations are the player routes of F075 (F075-AC-12: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY). No route
// adds an item to an inventory or trades items between students (F075-AC-16, AC-17).
var EconomyOperations = []string{"getWallet", "listWalletLedger", "listShopItems", "purchaseShopItem", "listInventory", "listInventoryHistory",
	"listCollections", "getCollection", "listAchievements"}

// EconomyAdminOperations are the four-eyes adjustment routes (internal gateway, portal client, platform role,
// game.economy.adjust — F075-AC-25…AC-27).
var EconomyAdminOperations = []string{"listEconomyAdjustments", "createEconomyAdjustment", "approveEconomyAdjustment", "rejectEconomyAdjustment"}

// economyOperations are the F075 operations (method, path, X-Tenant-Id mode, Idempotency-Key, success status and guard of
// the spec; the contract test checks them against specs/openapi/eduworld-api.v1.yaml). /admin/ routes also get
// portal_staff + platform role from the route group (httpx effectiveGuard).
var economyOperations = []httpx.Operation{
	{ID: "getWallet", Module: "game", Method: http.MethodGet, Pattern: "/game/wallet", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listWalletLedger", Module: "game", Method: http.MethodGet, Pattern: "/game/wallet/ledger", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listShopItems", Module: "game", Method: http.MethodGet, Pattern: "/game/shop/items", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "purchaseShopItem", Module: "game", Method: http.MethodPost, Pattern: "/game/shop/items/{shop_item_id}:purchase", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listInventory", Module: "game", Method: http.MethodGet, Pattern: "/game/inventory", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listInventoryHistory", Module: "game", Method: http.MethodGet, Pattern: "/game/inventory/history", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listCollections", Module: "game", Method: http.MethodGet, Pattern: "/game/collections", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "getCollection", Module: "game", Method: http.MethodGet, Pattern: "/game/collections/{collection_id}", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listAchievements", Module: "game", Method: http.MethodGet, Pattern: "/game/achievements", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardGamePlayer},
	{ID: "listEconomyAdjustments", Module: "game", Method: http.MethodGet, Pattern: "/admin/economy/adjustments", Tenant: httpx.TenantNone,
		Idempotency: httpx.IdempotencyNone, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "createEconomyAdjustment", Module: "game", Method: http.MethodPost, Pattern: "/admin/economy/adjustments", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusAccepted, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "approveEconomyAdjustment", Module: "game", Method: http.MethodPost, Pattern: "/admin/economy/adjustments/{adjustment_id}:approve", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
	{ID: "rejectEconomyAdjustment", Module: "game", Method: http.MethodPost, Pattern: "/admin/economy/adjustments/{adjustment_id}:reject", Tenant: httpx.TenantRequired,
		Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK, Guard: httpx.GuardPortalStaff | httpx.GuardPlatformRole},
}

func init() {
	for _, o := range economyOperations {
		Operations[o.ID] = o
	}
	PlayerOperations = append(PlayerOperations, EconomyOperations...)
}

func (h Handlers) economyRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"getWallet":                h.getWallet,
		"listWalletLedger":         h.listWalletLedger,
		"listShopItems":            h.listShopItems,
		"purchaseShopItem":         h.purchaseShopItem,
		"listInventory":            h.listInventory,
		"listInventoryHistory":     h.listInventoryHistory,
		"listCollections":          h.listCollections,
		"getCollection":            h.getCollection,
		"listAchievements":         h.listAchievements,
		"listEconomyAdjustments":   h.listEconomyAdjustments,
		"createEconomyAdjustment":  h.createEconomyAdjustment,
		"approveEconomyAdjustment": h.approveEconomyAdjustment,
		"rejectEconomyAdjustment":  h.rejectEconomyAdjustment,
	}
}

func limitParam(r *http.Request) (int, error) {
	s := r.URL.Query().Get("limit")
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, problem.Invalid("limit", problem.SubInvalid, "an integer")
	}
	return n, nil
}

func (h Handlers) getWallet(w http.ResponseWriter, r *http.Request) {
	out, err := h.EconomyQueries.GetWallet(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listWalletLedger(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	l := lang(r)
	out, err := h.EconomyQueries.ListWalletLedger(r.Context(), limit, r.URL.Query().Get("cursor"), l)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, l)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listShopItems(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.EconomyQueries.ListShopItems(r.Context(), q.Get("kind"), limit, q.Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) purchaseShopItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "shop_item_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ExpectedPrice int `json:"expected_price"`
		Quantity      int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	out, err := h.EconomyCommands.Purchase(r.Context(), id, b.Quantity, b.ExpectedPrice)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listInventory(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.EconomyQueries.ListInventory(r.Context(), q.Get("kind"), limit, q.Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listInventoryHistory(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	l := lang(r)
	out, err := h.EconomyQueries.ListInventoryHistory(r.Context(), limit, r.URL.Query().Get("cursor"), l)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, l)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listCollections(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.EconomyQueries.ListCollections(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getCollection(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "collection_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	l := lang(r)
	out, err := h.EconomyQueries.GetCollection(r.Context(), id, l)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, l)
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) listAchievements(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.EconomyQueries.ListAchievements(r.Context(), q.Get("category"), limit, q.Get("cursor"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

// adjustmentJSON is EconomyAdjustment of the contract.
func adjustmentJSON(a read.AdjustmentRecord) command.AdjustmentView {
	return command.AdjustmentView{AdjustmentID: a.ID, TenantID: a.TenantID, StudentID: a.StudentID, Kind: a.Kind, CurrencyCode: a.Currency,
		ItemID: a.ItemID, Delta: a.Delta, Reason: a.Reason, TicketRef: a.TicketRef, Status: a.Status, CreatedBy: a.CreatedBy, ApprovedBy: a.ApprovedBy,
		RejectedBy: a.RejectedBy, RejectReason: a.RejectReason, CreatedAt: a.CreatedAt, AppliedAt: a.AppliedAt, RejectedAt: a.RejectedAt}
}

func (h Handlers) listEconomyAdjustments(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	in := query.AdjustmentsInput{Status: q.Get("status"), Limit: limit, Cursor: q.Get("cursor")}
	if v := q.Get("student_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(w, r, problem.Invalid("student_id", problem.SubInvalid, "must be a UUID"))
			return
		}
		in.StudentID = &id
	}
	p, err := h.EconomyQueries.ListAdjustments(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]command.AdjustmentView, 0, len(p.Data))
	for _, a := range p.Data {
		data = append(data, adjustmentJSON(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h Handlers) createEconomyAdjustment(w http.ResponseWriter, r *http.Request) {
	var b struct {
		TenantID     uuid.UUID `json:"tenant_id"`
		StudentID    uuid.UUID `json:"student_id"`
		Kind         string    `json:"kind"`
		CurrencyCode *string   `json:"currency_code"`
		ItemID       *string   `json:"item_id"`
		Delta        int64     `json:"delta"`
		Reason       string    `json:"reason"`
		TicketRef    *string   `json:"ticket_ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	out, err := h.EconomyCommands.CreateAdjustment(r.Context(), command.AdjustmentInput{TenantID: b.TenantID, StudentID: b.StudentID, Kind: b.Kind,
		Currency: b.CurrencyCode, ItemID: b.ItemID, Delta: b.Delta, Reason: b.Reason, TicketRef: b.TicketRef})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h Handlers) approveEconomyAdjustment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "adjustment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.EconomyCommands.ApproveAdjustment(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) rejectEconomyAdjustment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "adjustment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	out, err := h.EconomyCommands.RejectAdjustment(r.Context(), id, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
