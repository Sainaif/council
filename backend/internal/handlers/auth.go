package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/config"
	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/middleware"
	"github.com/sainaif/council/internal/services/auth"
)

type AuthHandler struct {
	auth *auth.GitHubAuth
	db   *database.DB
	cfg  *config.Config
}

func NewAuthHandler(auth *auth.GitHubAuth, db *database.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{auth: auth, db: db, cfg: cfg}
}

func (h *AuthHandler) InitiateOAuth(c *fiber.Ctx) error {
	state := h.auth.GenerateState()

	// Store state in cookie for CSRF protection
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		Secure:   !h.cfg.IsDev,
		SameSite: "Lax",
	})

	authURL := h.auth.GetAuthURL(state)
	return c.Redirect(authURL)
}

func (h *AuthHandler) Callback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	storedState := c.Cookies("oauth_state")

	// Validate state
	if state == "" || state != storedState {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid OAuth state",
		})
	}

	// Clear state cookie
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
	})

	// Exchange code for token
	token, err := h.auth.Exchange(c.Context(), code)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to exchange OAuth code",
		})
	}

	// Get user info
	user, err := h.auth.GetUser(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get user info",
		})
	}

	// Create or update user preferences
	// Use fmt.Sprintf to convert int64 to string to match JWT token's UserID format
	userID := fmt.Sprintf("%d", user.ID)
	_, err = h.db.Exec(`
		INSERT INTO user_preferences (user_id, github_username, github_avatar_url, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			github_username = ?,
			github_avatar_url = ?,
			updated_at = CURRENT_TIMESTAMP
	`, userID, user.Login, user.AvatarURL, user.Login, user.AvatarURL)
	if err != nil {
		// Log but don't fail - user can still use the app
		log.Printf("Failed to update user preferences: %v", err)
	}

	// Create JWT token
	jwtToken, err := h.auth.CreateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create session",
		})
	}

	// Set token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "council_token",
		Value:    jwtToken,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   !h.cfg.IsDev,
		SameSite: "Lax",
		Path:     "/",
	})

	// Redirect to frontend
	redirectURL := h.cfg.FrontendURL + "/arena"
	return c.Redirect(redirectURL)
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// Clear token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "council_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: true,
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logged out successfully",
	})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Not authenticated",
		})
	}

	// Get additional user info from database
	var language, uiDensity string
	err := h.db.QueryRow(`
		SELECT COALESCE(language, 'en'), COALESCE(ui_density, 'comfortable')
		FROM user_preferences WHERE user_id = ?
	`, claims.UserID).Scan(&language, &uiDensity)
	if err != nil {
		language = "en"
		uiDensity = "comfortable"
	}

	return c.JSON(fiber.Map{
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"avatar_url": claims.AvatarURL,
		"language":   language,
		"ui_density": uiDensity,
	})
}
