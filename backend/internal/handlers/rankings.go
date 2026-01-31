package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/database"
)

type RankingHandler struct {
	db *database.DB
}

func NewRankingHandler(db *database.DB) *RankingHandler {
	return &RankingHandler{db: db}
}

type RankingEntry struct {
	Rank        int     `json:"rank"`
	ModelID     string  `json:"model_id"`
	DisplayName string  `json:"display_name"`
	Provider    string  `json:"provider"`
	Rating      int     `json:"rating"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	Draws       int     `json:"draws"`
	WinRate     float64 `json:"win_rate"`
	GamesPlayed int     `json:"games_played"`
	Trend       int     `json:"trend"` // Recent rating change
}

func (h *RankingHandler) Global(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}

	var rankings []RankingEntry
	rows, err := h.db.Query(`
		SELECT
			m.id, m.display_name, m.provider,
			COALESCE(AVG(mr.rating), 1500) as avg_rating,
			COALESCE(SUM(mr.wins), 0) as wins,
			COALESCE(SUM(mr.losses), 0) as losses,
			COALESCE(SUM(mr.draws), 0) as draws
		FROM models m
		LEFT JOIN model_ratings mr ON m.id = mr.model_id
		WHERE m.is_active = 1
		GROUP BY m.id
		ORDER BY avg_rating DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get rankings",
		})
	}
	defer func() { _ = rows.Close() }()

	rank := 1
	for rows.Next() {
		var e RankingEntry
		var avgRating float64
		_ = rows.Scan(&e.ModelID, &e.DisplayName, &e.Provider, &avgRating, &e.Wins, &e.Losses, &e.Draws)
		e.Rating = int(avgRating)
		e.Rank = rank
		e.GamesPlayed = e.Wins + e.Losses + e.Draws
		if e.GamesPlayed > 0 {
			e.WinRate = float64(e.Wins) / float64(e.GamesPlayed)
		}

		// Get recent trend
		var recentChange sql.NullInt64
		h.db.QueryRow(`
			SELECT SUM(change) FROM elo_history
			WHERE model_id = ? AND created_at > datetime('now', '-7 days')
		`, e.ModelID).Scan(&recentChange)
		if recentChange.Valid {
			e.Trend = int(recentChange.Int64)
		}

		rankings = append(rankings, e)
		rank++
	}

	return c.JSON(rankings)
}

func (h *RankingHandler) ByCategory(c *fiber.Ctx) error {
	category := c.Params("category")
	if category == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Category required",
		})
	}

	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}

	// Get category ID
	var categoryID int64
	err := h.db.QueryRow(`SELECT id FROM categories WHERE name = ?`, category).Scan(&categoryID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Category not found",
		})
	}

	var rankings []RankingEntry
	rows, err := h.db.Query(`
		SELECT
			m.id, m.display_name, m.provider,
			COALESCE(mr.rating, 1500),
			COALESCE(mr.wins, 0),
			COALESCE(mr.losses, 0),
			COALESCE(mr.draws, 0)
		FROM models m
		LEFT JOIN model_ratings mr ON m.id = mr.model_id AND mr.category_id = ?
		WHERE m.is_active = 1
		ORDER BY COALESCE(mr.rating, 1500) DESC
		LIMIT ?
	`, categoryID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get rankings",
		})
	}
	defer rows.Close()

	rank := 1
	for rows.Next() {
		var e RankingEntry
		rows.Scan(&e.ModelID, &e.DisplayName, &e.Provider, &e.Rating, &e.Wins, &e.Losses, &e.Draws)
		e.Rank = rank
		e.GamesPlayed = e.Wins + e.Losses + e.Draws
		if e.GamesPlayed > 0 {
			e.WinRate = float64(e.Wins) / float64(e.GamesPlayed)
		}
		rankings = append(rankings, e)
		rank++
	}

	return c.JSON(fiber.Map{
		"category":    category,
		"category_id": categoryID,
		"rankings":    rankings,
	})
}

func (h *RankingHandler) HeadToHead(c *fiber.Ctx) error {
	modelA := c.Params("modelA")
	modelB := c.Params("modelB")

	if modelA == "" || modelB == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Both model IDs required",
		})
	}

	// Ensure consistent ordering
	if modelA > modelB {
		modelA, modelB = modelB, modelA
	}

	type MatchupStats struct {
		CategoryID   *int64  `json:"category_id,omitempty"`
		CategoryName *string `json:"category_name,omitempty"`
		ModelAWins   int     `json:"model_a_wins"`
		ModelBWins   int     `json:"model_b_wins"`
		Draws        int     `json:"draws"`
		TotalGames   int     `json:"total_games"`
	}

	var overall MatchupStats
	var byCategory []MatchupStats

	// Get overall matchup
	err := h.db.QueryRow(`
		SELECT COALESCE(SUM(model_a_wins), 0), COALESCE(SUM(model_b_wins), 0), COALESCE(SUM(draws), 0)
		FROM matchups
		WHERE model_a_id = ? AND model_b_id = ?
	`, modelA, modelB).Scan(&overall.ModelAWins, &overall.ModelBWins, &overall.Draws)
	if err != nil && err != sql.ErrNoRows {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get matchup data",
		})
	}
	overall.TotalGames = overall.ModelAWins + overall.ModelBWins + overall.Draws

	// Get matchups by category
	rows, err := h.db.Query(`
		SELECT c.id, c.name, COALESCE(m.model_a_wins, 0), COALESCE(m.model_b_wins, 0), COALESCE(m.draws, 0)
		FROM categories c
		LEFT JOIN matchups m ON c.id = m.category_id AND m.model_a_id = ? AND m.model_b_id = ?
	`, modelA, modelB)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ms MatchupStats
			var catID int64
			var catName string
			rows.Scan(&catID, &catName, &ms.ModelAWins, &ms.ModelBWins, &ms.Draws)
			ms.CategoryID = &catID
			ms.CategoryName = &catName
			ms.TotalGames = ms.ModelAWins + ms.ModelBWins + ms.Draws
			byCategory = append(byCategory, ms)
		}
	}

	// Get model info
	type ModelInfo struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Provider    string `json:"provider"`
		Rating      int    `json:"rating"`
	}

	var infoA, infoB ModelInfo
	h.db.QueryRow(`
		SELECT m.id, m.display_name, m.provider, COALESCE(AVG(mr.rating), 1500)
		FROM models m LEFT JOIN model_ratings mr ON m.id = mr.model_id
		WHERE m.id = ? GROUP BY m.id
	`, modelA).Scan(&infoA.ID, &infoA.DisplayName, &infoA.Provider, &infoA.Rating)

	h.db.QueryRow(`
		SELECT m.id, m.display_name, m.provider, COALESCE(AVG(mr.rating), 1500)
		FROM models m LEFT JOIN model_ratings mr ON m.id = mr.model_id
		WHERE m.id = ? GROUP BY m.id
	`, modelB).Scan(&infoB.ID, &infoB.DisplayName, &infoB.Provider, &infoB.Rating)

	return c.JSON(fiber.Map{
		"model_a":     infoA,
		"model_b":     infoB,
		"overall":     overall,
		"by_category": byCategory,
	})
}
