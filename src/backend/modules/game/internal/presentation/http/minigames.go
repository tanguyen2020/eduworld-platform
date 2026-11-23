package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// MiniGameCommands are the mini-game use cases of a student (command.MiniGames, F073).
type MiniGameCommands interface {
	Start(ctx context.Context, in command.StartMiniGameInput) (command.MiniGameSession, error)
}

// MiniGameOperations are the player routes of F073 (F073-AC-13: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var MiniGameOperations = []string{"startMiniGame"}

func init() {
	Operations["startMiniGame"] = op("startMiniGame", http.MethodPost, "/game/minigames/{minigame_config_id}:start", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusCreated, httpx.GuardGamePlayer)
	PlayerOperations = append(PlayerOperations, MiniGameOperations...)
}

func (h Handlers) miniGameRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{"startMiniGame": h.startMiniGame}
}

// startMiniGame implements POST /game/minigames/{minigame_config_id}:start (F073-AC-06, AC-12).
func (h Handlers) startMiniGame(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "minigame_config_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ContextType string     `json:"context_type"`
		ContextID   *uuid.UUID `json:"context_id"`
	}
	if err := optionalBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.MiniGames.Start(r.Context(), command.StartMiniGameInput{ConfigID: id, ContextType: b.ContextType, ContextID: b.ContextID})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
