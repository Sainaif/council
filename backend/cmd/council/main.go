package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/sainaif/council/internal/config"
	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/handlers"
	"github.com/sainaif/council/internal/middleware"
	"github.com/sainaif/council/internal/routes"
	"github.com/sainaif/council/internal/services/auth"
	"github.com/sainaif/council/internal/services/copilot"
	"github.com/sainaif/council/internal/services/council"
	"github.com/sainaif/council/internal/services/elo"
	"github.com/sainaif/council/internal/websocket"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed")

	// Initialize services
	authService := auth.NewGitHubAuth(cfg)
	copilotService := copilot.NewService()
	eloService := elo.NewCalculator(db)
	wsHub := websocket.NewHub()
	councilService := council.NewOrchestrator(db, copilotService, eloService, wsHub)

	// Start WebSocket hub
	go wsHub.Run()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, db, cfg)
	councilHandler := handlers.NewCouncilHandler(councilService, db)
	modelHandler := handlers.NewModelHandler(db, copilotService)
	rankingHandler := handlers.NewRankingHandler(db)
	analyticsHandler := handlers.NewAnalyticsHandler(db)
	settingsHandler := handlers.NewSettingsHandler(db)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:               "Council Arena",
		DisableStartupMessage: false,
		ErrorHandler:          errorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))

	// CORS configuration
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.FrontendURL,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	// Auth middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.SessionSecret)

	// Setup routes
	routes.Setup(app, routes.Handlers{
		Auth:      authHandler,
		Council:   councilHandler,
		Model:     modelHandler,
		Ranking:   rankingHandler,
		Analytics: analyticsHandler,
		Settings:  settingsHandler,
	}, authMiddleware, wsHub)

	// Serve static frontend files in production
	if !cfg.IsDev {
		app.Static("/", "./static")
		// SPA fallback
		app.Get("/*", func(c *fiber.Ctx) error {
			return c.SendFile("./static/index.html")
		})
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down gracefully...")

		// Stop WebSocket hub
		wsHub.Shutdown()

		// Close Copilot sessions
		copilotService.Shutdown()

		// Shutdown server with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := app.ShutdownWithContext(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Starting Council Arena on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error":   true,
		"message": message,
	})
}
