package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/middleware"
)

type AnalyticsHandler struct {
	db *database.DB
}

func NewAnalyticsHandler(db *database.DB) *AnalyticsHandler {
	return &AnalyticsHandler{db: db}
}

func (h *AnalyticsHandler) Overview(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	type Overview struct {
		TotalSessions    int     `json:"total_sessions"`
		CompletedCount   int     `json:"completed_count"`
		AverageModels    float64 `json:"average_models_per_session"`
		MostUsedModel    string  `json:"most_used_model"`
		TopPerformer     string  `json:"top_performer"`
		TotalVotes       int     `json:"total_votes"`
		SessionsToday    int     `json:"sessions_today"`
		SessionsThisWeek int     `json:"sessions_this_week"`
	}

	var overview Overview

	// Total sessions for user
	_ = h.db.QueryRow(`
		SELECT COUNT(*) FROM sessions WHERE user_id = ?
	`, userID).Scan(&overview.TotalSessions)

	// Completed sessions
	_ = h.db.QueryRow(`
		SELECT COUNT(*) FROM sessions WHERE user_id = ? AND status = 'completed'
	`, userID).Scan(&overview.CompletedCount)

	// Sessions today
	_ = h.db.QueryRow(`
		SELECT COUNT(*) FROM sessions
		WHERE user_id = ? AND date(created_at) = date('now')
	`, userID).Scan(&overview.SessionsToday)

	// Sessions this week
	_ = h.db.QueryRow(`
		SELECT COUNT(*) FROM sessions
		WHERE user_id = ? AND created_at > datetime('now', '-7 days')
	`, userID).Scan(&overview.SessionsThisWeek)

	// Average models per session
	_ = h.db.QueryRow(`
		SELECT COALESCE(AVG(model_count), 0) FROM (
			SELECT session_id, COUNT(DISTINCT model_id) as model_count
			FROM responses r
			JOIN sessions s ON r.session_id = s.id
			WHERE s.user_id = ?
			GROUP BY session_id
		)
	`, userID).Scan(&overview.AverageModels)

	// Most used model
	var mostUsed sql.NullString
	_ = h.db.QueryRow(`
		SELECT model_id FROM responses r
		JOIN sessions s ON r.session_id = s.id
		WHERE s.user_id = ?
		GROUP BY model_id
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, userID).Scan(&mostUsed)
	if mostUsed.Valid {
		overview.MostUsedModel = mostUsed.String
	}

	// Top performer (highest ELO)
	var topPerformer sql.NullString
	_ = h.db.QueryRow(`
		SELECT m.id FROM models m
		LEFT JOIN model_ratings mr ON m.id = mr.model_id
		GROUP BY m.id
		ORDER BY COALESCE(AVG(mr.rating), 1500) DESC
		LIMIT 1
	`).Scan(&topPerformer)
	if topPerformer.Valid {
		overview.TopPerformer = topPerformer.String
	}

	// Total votes by user
	_ = h.db.QueryRow(`
		SELECT COUNT(*) FROM votes WHERE voter_type = 'user' AND voter_id = ?
	`, userID).Scan(&overview.TotalVotes)

	// Model performance trends
	type ModelTrend struct {
		ModelID     string  `json:"model_id"`
		DisplayName string  `json:"display_name"`
		Rating      int     `json:"rating"`
		Trend7d     int     `json:"trend_7d"`
		WinRate     float64 `json:"win_rate"`
	}

	var trends []ModelTrend
	rows, err := h.db.Query(`
		SELECT
			m.id, m.display_name,
			COALESCE(AVG(mr.rating), 1500) as rating,
			COALESCE(SUM(mr.wins), 0) as wins,
			COALESCE(SUM(mr.losses), 0) as losses
		FROM models m
		LEFT JOIN model_ratings mr ON m.id = mr.model_id
		WHERE m.is_active = 1
		GROUP BY m.id
		ORDER BY rating DESC
		LIMIT 10
	`)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var t ModelTrend
			var wins, losses int
			_ = rows.Scan(&t.ModelID, &t.DisplayName, &t.Rating, &wins, &losses)
			if wins+losses > 0 {
				t.WinRate = float64(wins) / float64(wins+losses)
			}

			// Get 7-day trend
			var trend sql.NullInt64
			_ = h.db.QueryRow(`
				SELECT SUM(change) FROM elo_history
				WHERE model_id = ? AND created_at > datetime('now', '-7 days')
			`, t.ModelID).Scan(&trend)
			if trend.Valid {
				t.Trend7d = int(trend.Int64)
			}

			trends = append(trends, t)
		}
	}

	// Category distribution
	type CategoryDist struct {
		CategoryID   int    `json:"category_id"`
		CategoryName string `json:"category_name"`
		SessionCount int    `json:"session_count"`
	}

	var categoryDist []CategoryDist
	catRows, err := h.db.Query(`
		SELECT c.id, c.name, COUNT(s.id) as session_count
		FROM categories c
		LEFT JOIN sessions s ON c.id = s.category_id AND s.user_id = ?
		GROUP BY c.id
		ORDER BY session_count DESC
	`, userID)
	if err == nil {
		defer func() { _ = catRows.Close() }()
		for catRows.Next() {
			var cd CategoryDist
			_ = catRows.Scan(&cd.CategoryID, &cd.CategoryName, &cd.SessionCount)
			categoryDist = append(categoryDist, cd)
		}
	}

	return c.JSON(fiber.Map{
		"overview":              overview,
		"model_trends":          trends,
		"category_distribution": categoryDist,
	})
}

func (h *AnalyticsHandler) UserBias(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	type ModelPreference struct {
		ModelID       string  `json:"model_id"`
		DisplayName   string  `json:"display_name"`
		TimesVotedFor int     `json:"times_voted_for"`
		TotalVotes    int     `json:"total_votes"`
		Preference    float64 `json:"preference_rate"`
	}

	var preferences []ModelPreference
	rows, err := h.db.Query(`
		WITH user_votes AS (
			SELECT ranked_responses FROM votes
			WHERE voter_type = 'user' AND voter_id = ?
		),
		vote_counts AS (
			SELECT
				r.model_id,
				COUNT(*) as times_voted_for
			FROM user_votes uv, responses r
			WHERE r.anonymous_label = (
				SELECT json_extract(uv.ranked_responses, '$[0]')
			)
			GROUP BY r.model_id
		)
		SELECT
			m.id, m.display_name,
			COALESCE(vc.times_voted_for, 0),
			(SELECT COUNT(*) FROM user_votes) as total
		FROM models m
		LEFT JOIN vote_counts vc ON m.id = vc.model_id
		WHERE m.is_active = 1
		ORDER BY times_voted_for DESC
	`, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to analyze user bias",
		})
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var p ModelPreference
		_ = rows.Scan(&p.ModelID, &p.DisplayName, &p.TimesVotedFor, &p.TotalVotes)
		if p.TotalVotes > 0 {
			p.Preference = float64(p.TimesVotedFor) / float64(p.TotalVotes)
		}
		preferences = append(preferences, p)
	}

	// Detect potential bias
	var biasWarning string
	if len(preferences) > 0 && preferences[0].TotalVotes >= 10 {
		if preferences[0].Preference > 0.5 {
			biasWarning = "You may have a preference for " + preferences[0].DisplayName + ". Consider trying other models."
		}
	}

	return c.JSON(fiber.Map{
		"preferences":  preferences,
		"bias_warning": biasWarning,
	})
}

func (h *AnalyticsHandler) Costs(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	type CostSummary struct {
		TotalTokens      int     `json:"total_tokens"`
		TotalSessions    int     `json:"total_sessions"`
		AvgTokensSession float64 `json:"avg_tokens_per_session"`
		TokensToday      int     `json:"tokens_today"`
		TokensThisWeek   int     `json:"tokens_this_week"`
		TokensThisMonth  int     `json:"tokens_this_month"`
	}

	var summary CostSummary

	// Total tokens
	_ = h.db.QueryRow(`
		SELECT COALESCE(SUM(r.token_count), 0), COUNT(DISTINCT r.session_id)
		FROM responses r
		JOIN sessions s ON r.session_id = s.id
		WHERE s.user_id = ?
	`, userID).Scan(&summary.TotalTokens, &summary.TotalSessions)

	if summary.TotalSessions > 0 {
		summary.AvgTokensSession = float64(summary.TotalTokens) / float64(summary.TotalSessions)
	}

	// Tokens today
	_ = h.db.QueryRow(`
		SELECT COALESCE(SUM(r.token_count), 0)
		FROM responses r
		JOIN sessions s ON r.session_id = s.id
		WHERE s.user_id = ? AND date(s.created_at) = date('now')
	`, userID).Scan(&summary.TokensToday)

	// Tokens this week
	_ = h.db.QueryRow(`
		SELECT COALESCE(SUM(r.token_count), 0)
		FROM responses r
		JOIN sessions s ON r.session_id = s.id
		WHERE s.user_id = ? AND s.created_at > datetime('now', '-7 days')
	`, userID).Scan(&summary.TokensThisWeek)

	// Tokens this month
	_ = h.db.QueryRow(`
		SELECT COALESCE(SUM(r.token_count), 0)
		FROM responses r
		JOIN sessions s ON r.session_id = s.id
		WHERE s.user_id = ? AND s.created_at > datetime('now', '-30 days')
	`, userID).Scan(&summary.TokensThisMonth)

	// Usage by model
	type ModelUsage struct {
		ModelID     string `json:"model_id"`
		DisplayName string `json:"display_name"`
		TokenCount  int    `json:"token_count"`
		Requests    int    `json:"requests"`
	}

	var modelUsage []ModelUsage
	rows, err := h.db.Query(`
		SELECT r.model_id, m.display_name, COALESCE(SUM(r.token_count), 0), COUNT(*)
		FROM responses r
		JOIN sessions s ON r.session_id = s.id
		JOIN models m ON r.model_id = m.id
		WHERE s.user_id = ?
		GROUP BY r.model_id
		ORDER BY SUM(r.token_count) DESC
	`, userID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var mu ModelUsage
			_ = rows.Scan(&mu.ModelID, &mu.DisplayName, &mu.TokenCount, &mu.Requests)
			modelUsage = append(modelUsage, mu)
		}
	}

	// Daily usage for the past 30 days
	type DailyUsage struct {
		Date       string `json:"date"`
		TokenCount int    `json:"token_count"`
		Sessions   int    `json:"sessions"`
	}

	var dailyUsage []DailyUsage
	dailyRows, err := h.db.Query(`
		SELECT date(s.created_at) as day, COALESCE(SUM(r.token_count), 0), COUNT(DISTINCT s.id)
		FROM sessions s
		LEFT JOIN responses r ON s.id = r.session_id
		WHERE s.user_id = ? AND s.created_at > datetime('now', '-30 days')
		GROUP BY day
		ORDER BY day DESC
	`, userID)
	if err == nil {
		defer func() { _ = dailyRows.Close() }()
		for dailyRows.Next() {
			var du DailyUsage
			_ = dailyRows.Scan(&du.Date, &du.TokenCount, &du.Sessions)
			dailyUsage = append(dailyUsage, du)
		}
	}

	return c.JSON(fiber.Map{
		"summary":     summary,
		"by_model":    modelUsage,
		"daily_usage": dailyUsage,
	})
}
