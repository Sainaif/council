package routes

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/sainaif/council/internal/handlers"
	"github.com/sainaif/council/internal/middleware"
	ws "github.com/sainaif/council/internal/websocket"
)

type Handlers struct {
	Auth      *handlers.AuthHandler
	Council   *handlers.CouncilHandler
	Model     *handlers.ModelHandler
	Ranking   *handlers.RankingHandler
	Analytics *handlers.AnalyticsHandler
	Settings  *handlers.SettingsHandler
}

func Setup(app *fiber.App, h Handlers, authMw *middleware.AuthMiddleware, wsHub *ws.Hub) {
	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Auth routes (no auth required)
	auth := app.Group("/auth")
	auth.Get("/github", h.Auth.InitiateOAuth)
	auth.Get("/callback", h.Auth.Callback)
	auth.Get("/logout", h.Auth.Logout)
	auth.Get("/me", authMw.Required(), h.Auth.Me)

	// API routes
	api := app.Group("/api", authMw.Required())

	// Council routes
	council := api.Group("/council")
	council.Post("/start", h.Council.Start)
	council.Get("/:id", h.Council.Get)
	council.Post("/:id/vote", h.Council.Vote)
	council.Post("/:id/appeal", h.Council.Appeal)
	council.Post("/:id/cancel", h.Council.Cancel)

	// Model routes
	models := api.Group("/models")
	models.Get("/", h.Model.List)
	models.Get("/:id", h.Model.Get)
	models.Get("/:id/history", h.Model.History)

	// Ranking routes
	rankings := api.Group("/rankings")
	rankings.Get("/", h.Ranking.Global)
	rankings.Get("/:category", h.Ranking.ByCategory)

	// Matchup routes
	api.Get("/matchups/:modelA/:modelB", h.Ranking.HeadToHead)

	// Analytics routes
	analytics := api.Group("/analytics")
	analytics.Get("/overview", h.Analytics.Overview)
	analytics.Get("/user-bias", h.Analytics.UserBias)
	analytics.Get("/costs", h.Analytics.Costs)

	// Settings routes
	settings := api.Group("/settings")
	settings.Get("/", h.Settings.Get)
	settings.Put("/", h.Settings.Update)

	// WebSocket route for real-time updates
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/council/:id", authMw.Optional(), websocket.New(func(c *websocket.Conn) {
		sessionID := c.Params("id")
		wsHub.HandleConnection(c, sessionID)
	}))
}
