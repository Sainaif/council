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
	userID := middleware.GetUserID(c)
	if userID == "" {
		log.Printf("[COUNCIL] Start request rejected - no user ID")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
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
		userID, req.Mode, req.Models, req.Question)

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

	session, err := h.orchestrator.StartSession(c.Context(), userID, startReq)
	if err != nil {
		log.Printf("[COUNCIL] Failed to start session for user %s: %v", userID, err)
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
