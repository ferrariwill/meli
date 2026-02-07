package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL     = "https://api.mercadolibre.com"
	defaultSiteID      = "MLB"
	defaultHTTPTimeout = 10 * time.Second
)

// MeliClient is a small HTTP client to talk to Mercado Livre public APIs.
type MeliClient struct {
	httpClient  *http.Client
	baseURL     string
	accessToken string
	clientID    string
}

func NewMeliClient(accessToken string, clientID string) *MeliClient {
	return &MeliClient{
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		baseURL:     defaultBaseURL,
		accessToken: accessToken,
		clientID:    clientID,
	}
}

// SearchItem represents a subset of fields from the search API.
type SearchItem struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Price        float64 `json:"price"`
	Thumbnail    string  `json:"thumbnail"`
	SoldQuantity int     `json:"sold_quantity"`
	Health       string  `json:"health"`
	CategoryID   string  `json:"category_id"`
	Permalink    string  `json:"permalink"`
}

type searchResponse struct {
	Results []SearchItem `json:"results"`
}

// Category represents a Mercado Livre category.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CategoryPrediction is a simplified view of category predictor output.
type CategoryPrediction struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Prob float64 `json:"prediction_probability"`
}

type categoryPredictorResponse struct {
	Predictions []CategoryPrediction `json:"predictions"`
}

// TopSoldByCategory fetches the top N sold products for a given category.
// This endpoint now requires authentication due to PolicyAgent restrictions.
func (c *MeliClient) TopSoldByCategory(ctx context.Context, categoryID string, limit int) ([]SearchItem, error) {
	if limit <= 0 {
		limit = 10
	}

	endpoint := fmt.Sprintf("%s/highlights/%s/category/%s", c.baseURL, defaultSiteID, categoryID)

	// q := url.Values{}
	// q.Set("category", categoryID)
	// q.Set("sort", "sold_quantity")
	// q.Set("limit", fmt.Sprintf("%d", limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Debug: log token status
	if c.accessToken == "" {
		log.Println("[DEBUG] Warning: accessToken is empty for TopSoldByCategory")
	} else {
		// Set headers
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
		req.Header.Set("Referer", "https://www.mercadolivre.com.br/")

		// Add Authorization header if token is available
		if c.accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.accessToken)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read full error body for better debugging
		errorBody, _ := io.ReadAll(resp.Body)
		// log.Printf("[ERROR] TopSoldByCategory failed: status=%d, response=%s", resp.StatusCode, string(errorBody))
		return nil, fmt.Errorf("meli search: unexpected status %d - %s", resp.StatusCode, string(errorBody))
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	if len(sr.Results) > limit {
		return sr.Results[:limit], nil
	}
	return sr.Results, nil
}

// RootCategories returns the main categories for the site.
// This endpoint now requires authentication due to PolicyAgent restrictions.
func (c *MeliClient) RootCategories(ctx context.Context) ([]Category, error) {
	endpoint := fmt.Sprintf("%s/sites/%s/categories", c.baseURL, defaultSiteID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	req.Header.Set("Referer", "https://www.mercadolivre.com.br/")

	// Add Authorization header if token is available
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read full error body for better debugging
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("meli categories: unexpected status %d - %s", resp.StatusCode, string(errorBody))
	}

	var cats []Category
	if err := json.NewDecoder(resp.Body).Decode(&cats); err != nil {
		return nil, err
	}
	return cats, nil
}

// PredictCategory suggests categories for a free-text query using Mercado Livre's
// category predictor API. This endpoint may require authentication.
func (c *MeliClient) PredictCategory(ctx context.Context, query string) ([]CategoryPrediction, error) {
	endpoint := fmt.Sprintf("%s/sites/%s/category_predictor/predict", c.baseURL, defaultSiteID)

	q := url.Values{}
	q.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	req.Header.Set("Referer", "https://www.mercadolivre.com.br/")

	// Add Authorization header if token is available
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read full error body for better debugging
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("meli category predictor: unexpected status %d - %s", resp.StatusCode, string(errorBody))
	}

	var pr categoryPredictorResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	return pr.Predictions, nil
}

func (c *MeliClient) applyAuth(req *http.Request) {
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
}
