package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/middleware"
	"github.com/sainaif/council/internal/services/copilot"
)

type ModelHandler struct {
	db      *database.DB
	copilot *copilot.Service
}

func NewModelHandler(db *database.DB, copilot *copilot.Service) *ModelHandler {
	return &ModelHandler{db: db, copilot: copilot}
}

type ModelResponse struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Provider     string   `json:"provider"`
	Rating       int      `json:"rating"`
	Wins         int      `json:"wins"`
	Losses       int      `json:"losses"`
	Draws        int      `json:"draws"`
	WinRate      float64  `json:"win_rate"`
	GamesPlayed  int      `json:"games_played"`
	Capabilities []string `json:"capabilities,omitempty"`
}

func (h *ModelHandler) List(c *fiber.Ctx) error {
	// Get user's access token from JWT claims
	claims := middleware.GetClaims(c)
	if claims == nil || claims.AccessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Authentication required for model access",
		})
	}

	// Get models from Copilot service using user's token
	models, err := h.copilot.ListModels(c.Context(), claims.UserID, claims.AccessToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to list models: " + err.Error(),
		})
	}

	// Enrich with ratings from database
	var response []ModelResponse
	for _, m := range models {
		mr := ModelResponse{
			ID:           m.ID,
			DisplayName:  m.DisplayName,
			Provider:     m.Provider,
			Rating:       1500, // Default
			Capabilities: m.Capabilities,
		}

		// Get aggregated stats
		var wins, losses, draws int
		err := h.db.QueryRow(`
			SELECT COALESCE(SUM(wins), 0), COALESCE(SUM(losses), 0), COALESCE(SUM(draws), 0)
			FROM model_ratings WHERE model_id = ?
		`, m.ID).Scan(&wins, &losses, &draws)
		if err == nil {
			mr.Wins = wins
			mr.Losses = losses
			mr.Draws = draws
			mr.GamesPlayed = wins + losses + draws
			if mr.GamesPlayed > 0 {
				mr.WinRate = float64(wins) / float64(mr.GamesPlayed)
			}
		}

		// Get average rating
		var avgRating sql.NullFloat64
		_ = h.db.QueryRow(`
			SELECT AVG(rating) FROM model_ratings WHERE model_id = ?
		`, m.ID).Scan(&avgRating)
		if avgRating.Valid {
			mr.Rating = int(avgRating.Float64)
		}

		response = append(response, mr)
	}

	return c.JSON(response)
}

func (h *ModelHandler) Get(c *fiber.Ctx) error {
	modelID := c.Params("id")
	if modelID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Model ID required",
		})
	}

	// Get user's access token from JWT claims
	claims := middleware.GetClaims(c)
	if claims == nil || claims.AccessToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Authentication required",
		})
	}

	// Get model from Copilot service
	model, err := h.copilot.GetModel(c.Context(), claims.UserID, claims.AccessToken, modelID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Model not found",
		})
	}

	mr := ModelResponse{
		ID:           model.ID,
		DisplayName:  model.DisplayName,
		Provider:     model.Provider,
		Rating:       1500,
		Capabilities: model.Capabilities,
	}

	// Get stats per category
	type CategoryStats struct {
		CategoryID   int     `json:"category_id"`
		CategoryName string  `json:"category_name"`
		Rating       int     `json:"rating"`
		Wins         int     `json:"wins"`
		Losses       int     `json:"losses"`
		Draws        int     `json:"draws"`
		WinRate      float64 `json:"win_rate"`
	}

	var categoryStats []CategoryStats
	rows, err := h.db.Query(`
		SELECT c.id, c.name, COALESCE(mr.rating, 1500), COALESCE(mr.wins, 0),
			   COALESCE(mr.losses, 0), COALESCE(mr.draws, 0)
		FROM categories c
		LEFT JOIN model_ratings mr ON c.id = mr.category_id AND mr.model_id = ?
	`, modelID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var cs CategoryStats
			_ = rows.Scan(&cs.CategoryID, &cs.CategoryName, &cs.Rating, &cs.Wins, &cs.Losses, &cs.Draws)
			total := cs.Wins + cs.Losses + cs.Draws
			if total > 0 {
				cs.WinRate = float64(cs.Wins) / float64(total)
			}
			categoryStats = append(categoryStats, cs)
		}
	}

	// Get overall stats
	var wins, losses, draws int
	_ = h.db.QueryRow(`
		SELECT COALESCE(SUM(wins), 0), COALESCE(SUM(losses), 0), COALESCE(SUM(draws), 0)
		FROM model_ratings WHERE model_id = ?
	`, modelID).Scan(&wins, &losses, &draws)

	mr.Wins = wins
	mr.Losses = losses
	mr.Draws = draws
	mr.GamesPlayed = wins + losses + draws
	if mr.GamesPlayed > 0 {
		mr.WinRate = float64(wins) / float64(mr.GamesPlayed)
	}

	var avgRating sql.NullFloat64
	_ = h.db.QueryRow(`SELECT AVG(rating) FROM model_ratings WHERE model_id = ?`, modelID).Scan(&avgRating)
	if avgRating.Valid {
		mr.Rating = int(avgRating.Float64)
	}

	return c.JSON(fiber.Map{
		"model":          mr,
		"category_stats": categoryStats,
	})
}

func (h *ModelHandler) History(c *fiber.Ctx) error {
	modelID := c.Params("id")
	if modelID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Model ID required",
		})
	}

	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100
	}

	type HistoryEntry struct {
		SessionID  string `json:"session_id"`
		CategoryID *int64 `json:"category_id,omitempty"`
		OldRating  int    `json:"old_rating"`
		NewRating  int    `json:"new_rating"`
		Change     int    `json:"change"`
		Reason     string `json:"reason"`
		CreatedAt  string `json:"created_at"`
	}

	var history []HistoryEntry
	rows, err := h.db.Query(`
		SELECT session_id, category_id, old_rating, new_rating, change, reason, created_at
		FROM elo_history
		WHERE model_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, modelID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get history",
		})
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var h HistoryEntry
		var categoryID sql.NullInt64
		var sessionID sql.NullString
		_ = rows.Scan(&sessionID, &categoryID, &h.OldRating, &h.NewRating, &h.Change, &h.Reason, &h.CreatedAt)
		if sessionID.Valid {
			h.SessionID = sessionID.String
		}
		if categoryID.Valid {
			h.CategoryID = &categoryID.Int64
		}
		history = append(history, h)
	}

	return c.JSON(history)
}
