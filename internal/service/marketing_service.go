package service

import (
	"context"

	"melibot/internal/api"
	"melibot/internal/repository"
)

// MarketingService encapsulates business logic for marketing/sales analysis.
type MarketingService struct {
	meliClient *api.MeliClient
	trendRepo  *repository.TrendRepository
}

func NewMarketingService(meliClient *api.MeliClient, trendRepo *repository.TrendRepository) *MarketingService {
	return &MarketingService{
		meliClient: meliClient,
		trendRepo:  trendRepo,
	}
}

// TopTrendsByCategory returns the top N sold products for a category
// and stores their metrics for trend analysis.
func (s *MarketingService) TopTrendsByCategory(ctx context.Context, categoryID string, limit int) ([]api.SearchItem, error) {
	ids, err := s.meliClient.TopSoldByCategory(ctx, categoryID, limit)
	if err != nil {
		return nil, err
	}
	items := make([]api.SearchItem, 0, len(ids))

	for _, id := range ids {
		items = append(items, api.SearchItem{
			ID:           id.ID,
			Title:        id.Title, // preencher depois com dados do /items/{id}
			Price:        id.Price, // idem
			Thumbnail:    id.Thumbnail,
			SoldQuantity: id.SoldQuantity,
			Health:       id.Health,
			CategoryID:   id.CategoryID, // cuidado: aqui não é o mesmo que ProductID
			Permalink:    id.Permalink,
		})
	}

	/*trends := make([]repository.ProductTrend, 0, len(items))
	for _, it := range items {
		trends = append(trends, repository.ProductTrend{
			ProductID:    it.ID,
			Title:        it.Title,
			CategoryID:   it.CategoryID,
			SoldQuantity: it.SoldQuantity,
			Health:       it.Health,
			Price:        it.Price,
			Thumbnail:    it.Thumbnail,
			Permalink:    it.Permalink,
		})
	}*/

	/*// Persist trend data (best-effort; surface error to caller).
	if err := s.trendRepo.SaveProductTrends(ctx, trends); err != nil {
		return nil, err
	}*/

	return items, nil
}

// RootCategories lists the main Mercado Livre categories for MLB.
func (s *MarketingService) RootCategories(ctx context.Context) ([]api.Category, error) {
	return s.meliClient.RootCategories(ctx)
}

// SuggestCategories uses the Mercado Livre category predictor to suggest
// categories based on a free-text query.
func (s *MarketingService) SuggestCategories(ctx context.Context, query string) ([]api.CategoryPrediction, error) {
	return s.meliClient.PredictCategory(ctx, query)
}
