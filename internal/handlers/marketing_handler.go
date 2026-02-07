package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"melibot/internal/service"
)

type MarketingHandler struct {
	svc *service.MarketingService
}

func NewMarketingHandler(svc *service.MarketingService) *MarketingHandler {
	return &MarketingHandler{svc: svc}
}

// RegisterRoutes wires marketing-related routes into the given router group.
func (h *MarketingHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/categories", h.GetCategories)
		api.GET("/trends", h.GetTopTrends)
		api.GET("/category_suggest", h.SuggestCategory)
	}
}

// GetCategories returns the root categories for MLB.
func (h *MarketingHandler) GetCategories(c *gin.Context) {
	ctx := c.Request.Context()

	cats, err := h.svc.RootCategories(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cats)
}

// GetTopTrends returns the top sold products for a given category.
func (h *MarketingHandler) GetTopTrends(c *gin.Context) {
	ctx := c.Request.Context()
	categoryID := c.Query("category_id")
	if categoryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id is required"})
		return
	}

	items, err := h.svc.TopTrendsByCategory(ctx, categoryID, 10)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

// SuggestCategory uses the category predictor to suggest categories from free text.
func (h *MarketingHandler) SuggestCategory(c *gin.Context) {
	ctx := c.Request.Context()
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	preds, err := h.svc.SuggestCategories(ctx, query)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preds)
}

