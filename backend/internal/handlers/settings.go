package handlers

import (
	"database/sql"
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/middleware"
)

type SettingsHandler struct {
	db *database.DB
}

func NewSettingsHandler(db *database.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

type UserSettings struct {
	DefaultModels        []string `json:"default_models"`
	PreferredCategories  []string `json:"preferred_categories"`
	UIDensity            string   `json:"ui_density"`
	Language             string   `json:"language"`
	AutoSaveSessions     bool     `json:"auto_save_sessions"`
	UserFeedbackWeight   float64  `json:"user_feedback_weight"`
}

func (h *SettingsHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var settings UserSettings
	var defaultModels, preferredCategories sql.NullString
	var autoSave sql.NullBool
	var feedbackWeight sql.NullFloat64

	err := h.db.QueryRow(`
		SELECT default_models, preferred_categories, ui_density, language,
			   auto_save_sessions, user_feedback_weight
		FROM user_preferences WHERE user_id = ?
	`, userID).Scan(
		&defaultModels, &preferredCategories, &settings.UIDensity,
		&settings.Language, &autoSave, &feedbackWeight,
	)

	if err == sql.ErrNoRows {
		// Return defaults
		return c.JSON(UserSettings{
			DefaultModels:       []string{},
			PreferredCategories: []string{},
			UIDensity:           "comfortable",
			Language:            "en",
			AutoSaveSessions:    true,
			UserFeedbackWeight:  0.5,
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get settings",
		})
	}

	if defaultModels.Valid {
		json.Unmarshal([]byte(defaultModels.String), &settings.DefaultModels)
	}
	if preferredCategories.Valid {
		json.Unmarshal([]byte(preferredCategories.String), &settings.PreferredCategories)
	}
	if autoSave.Valid {
		settings.AutoSaveSessions = autoSave.Bool
	} else {
		settings.AutoSaveSessions = true
	}
	if feedbackWeight.Valid {
		settings.UserFeedbackWeight = feedbackWeight.Float64
	} else {
		settings.UserFeedbackWeight = 0.5
	}

	return c.JSON(settings)
}

type UpdateSettingsRequest struct {
	DefaultModels        *[]string `json:"default_models,omitempty"`
	PreferredCategories  *[]string `json:"preferred_categories,omitempty"`
	UIDensity            *string   `json:"ui_density,omitempty"`
	Language             *string   `json:"language,omitempty"`
	AutoSaveSessions     *bool     `json:"auto_save_sessions,omitempty"`
	UserFeedbackWeight   *float64  `json:"user_feedback_weight,omitempty"`
}

func (h *SettingsHandler) Update(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	username := middleware.GetUsername(c)

	var req UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	// Validate ui_density
	if req.UIDensity != nil {
		if *req.UIDensity != "compact" && *req.UIDensity != "comfortable" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "ui_density must be 'compact' or 'comfortable'",
			})
		}
	}

	// Validate language
	if req.Language != nil {
		validLangs := map[string]bool{"en": true, "pl": true}
		if !validLangs[*req.Language] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid language. Supported: en, pl",
			})
		}
	}

	// Validate user_feedback_weight
	if req.UserFeedbackWeight != nil {
		if *req.UserFeedbackWeight < 0 || *req.UserFeedbackWeight > 1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "user_feedback_weight must be between 0 and 1",
			})
		}
	}

	// Ensure user exists in preferences
	_, err := h.db.Exec(`
		INSERT INTO user_preferences (user_id, github_username)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO NOTHING
	`, userID, username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to initialize settings",
		})
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}

	if req.DefaultModels != nil {
		modelsJSON, _ := json.Marshal(req.DefaultModels)
		updates = append(updates, "default_models = ?")
		args = append(args, string(modelsJSON))
	}
	if req.PreferredCategories != nil {
		catsJSON, _ := json.Marshal(req.PreferredCategories)
		updates = append(updates, "preferred_categories = ?")
		args = append(args, string(catsJSON))
	}
	if req.UIDensity != nil {
		updates = append(updates, "ui_density = ?")
		args = append(args, *req.UIDensity)
	}
	if req.Language != nil {
		updates = append(updates, "language = ?")
		args = append(args, *req.Language)
	}
	if req.AutoSaveSessions != nil {
		updates = append(updates, "auto_save_sessions = ?")
		args = append(args, *req.AutoSaveSessions)
	}
	if req.UserFeedbackWeight != nil {
		updates = append(updates, "user_feedback_weight = ?")
		args = append(args, *req.UserFeedbackWeight)
	}

	if len(updates) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No changes",
		})
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")

	query := "UPDATE user_preferences SET "
	for i, u := range updates {
		if i > 0 {
			query += ", "
		}
		query += u
	}
	query += " WHERE user_id = ?"
	args = append(args, userID)

	_, err = h.db.Exec(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to update settings",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Settings updated",
	})
}
