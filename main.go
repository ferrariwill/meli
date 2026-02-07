package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"melibot/database"
	"melibot/internal/api"
	"melibot/internal/handlers"
	"melibot/internal/repository"
	"melibot/internal/service"
)

func main() {
	// Load .env file (if present)
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found or error loading .env, continuing with existing environment variables")
	}

	// Initialize OAuth client with loaded environment variables
	handlers.InitializeOAuth()

	// Initialize database connection
	database.Connect()

	// Run repository migrations
	if err := repository.AutoMigrate(); err != nil {
		log.Fatalf("failed to run repository migrations: %v", err)
	}

	// Wire dependencies
	meliClientID := os.Getenv("ML_CLIENT_ID")
	trendRepo := repository.NewTrendRepository()

	// Setup Gin router
	router := gin.Default()

	// Simple health check route
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// OAuth routes (must be registered before API routes)
	handlers.RegisterOAuthRoutes(router)

	// Create middleware to validate token for protected routes
	requireAuth := func(c *gin.Context) {
		token := handlers.GetTokenFromContext(c)
		if token == "" {
			c.JSON(401, gin.H{"error": "Autenticação necessária. Por favor, faça login primeiro."})
			c.Abort()
			return
		}
		c.Next()
	}

	// Create a function to get fresh client/service/handler with current token
	getMarketingHandler := func(c *gin.Context) *handlers.MarketingHandler {
		meliAccessToken := handlers.GetTokenFromContext(c)
		if meliAccessToken == "" {
			meliAccessToken = os.Getenv("ML_ACCESS_TOKEN") // fallback to env
			if meliAccessToken == "" {
				log.Println("[DEBUG] Warning: No token found in context or .env for API request")
			}
		}
		log.Printf("[DEBUG] Creating handler with token (first 20 chars): %s...", meliAccessToken[:20])
		meliClient := api.NewMeliClient(meliAccessToken, meliClientID)
		marketingService := service.NewMarketingService(meliClient, trendRepo)
		return handlers.NewMarketingHandler(marketingService)
	}

	// API routes with dynamic token refresh
	apiGroup := router.Group("/api")
	{
		// Categories - can work without auth for public data
		apiGroup.GET("/categories", func(c *gin.Context) {
			getMarketingHandler(c).GetCategories(c)
		})
		// Trends - requires authentication
		apiGroup.GET("/trends", requireAuth, func(c *gin.Context) {
			getMarketingHandler(c).GetTopTrends(c)
		})
		// Category suggest - requires authentication
		apiGroup.GET("/category_suggest", requireAuth, func(c *gin.Context) {
			getMarketingHandler(c).SuggestCategory(c)
		})
	}

	// Static dashboard
	router.Static("/static", "./web")
	router.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})
	router.GET("/oauth-help", func(c *gin.Context) {
		c.File("./web/oauth_help.html")
	})

	// Determine server port from env, default 8080
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Servidor iniciado na porta", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
