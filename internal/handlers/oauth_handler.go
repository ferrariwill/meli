package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"

	"melibot/internal/api"
)

var (
	// Global token storage (in production, use Redis or database)
	currentToken string
	tokenMutex   sync.RWMutex
	oauthClient  *api.OAuthClient
)

// InitializeOAuth configures OAuth client with credentials from environment
// This should be called AFTER godotenv.Load() in main.go
func InitializeOAuth() {
	clientID := os.Getenv("ML_CLIENT_ID")
	clientSecret := os.Getenv("ML_CLIENT_SECRET")
	redirectURI := os.Getenv("ML_REDIRECT_URI")

	if clientID == "" || clientSecret == "" || redirectURI == "" {
		log.Println("[WARN] OAuth credentials not fully configured. ML_CLIENT_ID, ML_CLIENT_SECRET, and ML_REDIRECT_URI are required.")
		return
	}

	oauthClient = api.NewOAuthClient(clientID, clientSecret, redirectURI)
	log.Printf("[INFO] OAuth initialized successfully with client_id: %s", clientID)
}

// GetCurrentToken returns the current access token (thread-safe)
func GetCurrentToken() string {
	tokenMutex.RLock()
	defer tokenMutex.RUnlock()
	return currentToken
}

// SetCurrentToken sets the current access token (thread-safe)
func SetCurrentToken(token string) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	currentToken = token
}

// GetTokenFromContext tries to get the access token from:
// 1. Memory (currentToken)
// 2. Cookie (ml_access_token)
// 3. Environment variable fallback
func GetTokenFromContext(c *gin.Context) string {
	// Try to get from memory first
	if token := GetCurrentToken(); token != "" {
		log.Printf("[DEBUG] Token found in MEMORY: first 20 chars: %s...", token[:20])
		return token
	}
	log.Println("[DEBUG] Token NOT in memory, checking cookie...")

	// Try to get from cookie
	if cookie, err := c.Cookie("ml_access_token"); err == nil && cookie != "" {
		log.Printf("[DEBUG] Token found in COOKIE: first 20 chars: %s...", cookie[:20])
		// Update in-memory token for future requests
		SetCurrentToken(cookie)
		return cookie
	}
	log.Println("[DEBUG] Token NOT in cookie, using .env fallback")

	// Fallback to environment variable
	envToken := os.Getenv("ML_ACCESS_TOKEN")
	if envToken != "" {
		log.Printf("[DEBUG] Using .env token: first 20 chars: %s...", envToken[:20])
	}
	return envToken
}

// RegisterOAuthRoutes registers OAuth-related routes
func RegisterOAuthRoutes(r *gin.Engine) {
	r.GET("/auth/login", HandleLogin)
	r.GET("/callback", HandleCallback)
	r.GET("/auth/status", HandleAuthStatus)
	r.GET("/auth/logout", HandleLogout)
	r.GET("/auth/debug", HandleAuthDebug)
}

// HandleLogin redirects user to Mercado Livre authorization page
func HandleLogin(c *gin.Context) {
	if oauthClient == nil {
		// Redirect to help page
		c.Redirect(http.StatusFound, "/oauth-help")
		return
	}

	authURL := oauthClient.GetAuthorizationURL()

	// Log the URL for debugging
	log.Printf("Redirecting to OAuth URL: %s", authURL)
	log.Printf("Redirect URI configured: %s", os.Getenv("ML_REDIRECT_URI"))

	// Try redirect
	c.Redirect(http.StatusFound, authURL)
}

// HandleCallback handles the OAuth callback from Mercado Livre
func HandleCallback(c *gin.Context) {
	log.Println("[DEBUG] HandleCallback called!")

	if oauthClient == nil {
		log.Println("[ERROR] oauthClient is nil!")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "OAuth not configured",
		})
		return
	}

	code := c.Query("code")

	if code == "" {
		errorParam := c.Query("error")
		errorDesc := c.Query("error_description")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "Authorization failed",
			"error_code":        errorParam,
			"error_description": errorDesc,
		})
		return
	}

	ctx := c.Request.Context()
	tokenResp, err := oauthClient.ExchangeCodeForToken(ctx, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to exchange code for token: " + err.Error(),
		})
		return
	}

	// Store the access token in memory
	SetCurrentToken(tokenResp.AccessToken)

	// Also store the token in an HTTP-only secure cookie for persistence
	// maxAge: 86400 = 1 day (adjust as needed for your token expiration)
	c.SetCookie("ml_access_token", tokenResp.AccessToken, 86400, "/", "", false, true)
	c.SetCookie("ml_user_id", fmt.Sprintf("%d", tokenResp.UserID), 86400, "/", "", false, true)

	// Redirect to dashboard with success message
	c.Redirect(http.StatusFound, "/?auth=success&user_id="+fmt.Sprintf("%d", tokenResp.UserID))
}

// HandleAuthStatus returns the current authentication status
func HandleAuthStatus(c *gin.Context) {
	token := GetCurrentToken()
	if token == "" {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
			"message":       "Not authenticated. Visit /auth/login to authenticate",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"message":       "Authenticated successfully",
	})
}

// HandleLogout clears the authentication tokens
func HandleLogout(c *gin.Context) {
	// Clear in-memory token
	SetCurrentToken("")

	// Clear cookies
	c.SetCookie("ml_access_token", "", -1, "/", "", false, true)
	c.SetCookie("ml_user_id", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// HandleAuthDebug shows OAuth configuration for debugging
func HandleAuthDebug(c *gin.Context) {
	clientID := os.Getenv("ML_CLIENT_ID")
	redirectURI := os.Getenv("ML_REDIRECT_URI")
	hasSecret := os.Getenv("ML_CLIENT_SECRET") != ""

	var authURL string
	if oauthClient != nil {
		authURL = oauthClient.GetAuthorizationURL()
	}

	c.JSON(http.StatusOK, gin.H{
		"configured":   oauthClient != nil,
		"client_id":    clientID,
		"redirect_uri": redirectURI,
		"has_secret":   hasSecret,
		"auth_url":     authURL,
	})
}
