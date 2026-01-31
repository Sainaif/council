package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/middleware"
	"github.com/sainaif/council/internal/services/council"
)

type CouncilHandler struct {
	orchestrator *council.Orchestrator
	db           *database.DB
}

func NewCouncilHandler(orchestrator *council.Orchestrator, db *database.DB) *CouncilHandler {
	return &CouncilHandler{orchestrator: orchestrator, db: db}
}

type StartCouncilRequest struct {
	Question        string   `json:"question"`
	Models          []string `json:"models"`
	Mode            string   `json:"mode"`
	CategoryID      *int64   `json:"category_id,omitempty"`
	ChairpersonID   *string  `json:"chairperson_id,omitempty"`
	DebateRounds    int      `json:"debate_rounds,omitempty"`
	EnableDevil     bool     `json:"enable_devil_advocate,omitempty"`
	EnableMystery   bool     `json:"enable_mystery_judge,omitempty"`
	ResponseTimeout int      `json:"response_timeout,omitempty"`
}

func (h *CouncilHandler) Start(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims == nil {
		log.Printf("[COUNCIL] Start request rejected - no claims")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	if claims.AccessToken == "" {
		log.Printf("[COUNCIL] Start request rejected - no access token for user: %s", claims.UserID)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "GitHub Copilot access required. Please log out and log in again.",
		})
	}

	var req StartCouncilRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("[COUNCIL] Start request rejected - invalid body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	log.Printf("[COUNCIL] Starting session - user: %s, mode: %s, models: %v, question: %.50s...",
		claims.UserID, req.Mode, req.Models, req.Question)

	// Map to internal request
	startReq := council.StartRequest{
		Question:        req.Question,
		Models:          req.Models,
		Mode:            council.Mode(req.Mode),
		CategoryID:      req.CategoryID,
		ChairpersonID:   req.ChairpersonID,
		DebateRounds:    req.DebateRounds,
		EnableDevil:     req.EnableDevil,
		EnableMystery:   req.EnableMystery,
		ResponseTimeout: req.ResponseTimeout,
	}

	session, err := h.orchestrator.StartSession(c.Context(), claims.UserID, claims.AccessToken, startReq)
	if err != nil {
		log.Printf("[COUNCIL] Failed to start session for user %s: %v", claims.UserID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	log.Printf("[COUNCIL] Session started successfully - id: %s, status: %s", session.ID, session.Status)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"session_id": session.ID,
		"status":     session.Status,
		"ws_url":     "/ws/council/" + session.ID,
	})
}

func (h *CouncilHandler) Get(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Session ID required",
		})
	}

	session, err := h.orchestrator.GetSession(c.Context(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Session not found",
		})
	}

	return c.JSON(session)
}

type VoteRequest struct {
	RankedResponses []string `json:"ranked_responses"`
}

func (h *CouncilHandler) Vote(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	userID := middleware.GetUserID(c)

	var req VoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if len(req.RankedResponses) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Ranked responses required",
		})
	}

	if err := h.orchestrator.SubmitUserVote(c.Context(), sessionID, userID, req.RankedResponses); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to submit vote",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Vote submitted",
	})
}

func (h *CouncilHandler) Appeal(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	userID := middleware.GetUserID(c)

	// Get original session
	session, err := h.orchestrator.GetSession(c.Context(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Session not found",
		})
	}

	// Verify ownership
	if session.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":   true,
			"message": "Cannot appeal another user's session",
		})
	}

	// TODO: Create new appeal session with different models
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Appeal feature coming soon",
	})
}

func (h *CouncilHandler) Cancel(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	userID := middleware.GetUserID(c)

	// Get session
	session, err := h.orchestrator.GetSession(c.Context(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Session not found",
		})
	}

	// Verify ownership
	if session.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":   true,
			"message": "Cannot cancel another user's session",
		})
	}

	if err := h.orchestrator.CancelSession(c.Context(), sessionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to cancel session",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Session cancelled",
	})
}

// History returns the user's session history
func (h *CouncilHandler) History(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	limit := c.QueryInt("limit", 20)
	if limit > 100 {
		limit = 100
	}

	sessions := make([]map[string]interface{}, 0)
	rows, err := h.db.Query(`
		SELECT id, question, mode, status, created_at, completed_at
		FROM sessions
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		log.Printf("[COUNCIL] Failed to fetch history for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to fetch history",
		})
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, question, mode, status string
		var createdAt string
		var completedAt *string
		if err := rows.Scan(&id, &question, &mode, &status, &createdAt, &completedAt); err != nil {
			continue
		}

		// Get response count for this session
		var responseCount int
		_ = h.db.QueryRow(`SELECT COUNT(*) FROM responses WHERE session_id = ?`, id).Scan(&responseCount)

		sessions = append(sessions, map[string]interface{}{
			"id":             id,
			"question":       question,
			"mode":           mode,
			"status":         status,
			"created_at":     createdAt,
			"completed_at":   completedAt,
			"response_count": responseCount,
		})
	}

	log.Printf("[COUNCIL] Fetched %d sessions for user %s", len(sessions), userID)
	return c.JSON(sessions)
}
