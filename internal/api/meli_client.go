package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
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
	Status       string  `json:"status"`
	LinkVenda    string  `json:"link_venda,omitempty"` // campo extra para link de venda (pode ser o mesmo que Permalink ou diferente se quisermos usar um link de afiliado)
}

type searchResponse struct {
	Results []SearchItem `json:"results"`
}

// ProductPrice holds the best price and details for a product item.
type ProductPrice struct {
	Price     float64
	ItemID    string
	Title     string
	Permalink string // May be empty; use ItemID to search for item page
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
	endpoint := fmt.Sprintf("%s/highlights/%s/category/%s", c.baseURL, defaultSiteID, categoryID)

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

	var highlights HighlightResponse
	if err := json.NewDecoder(resp.Body).Decode(&highlights); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(highlights.Content))
	items := make([]SearchItem, 0, len(highlights.Content))

	for _, highlight := range highlights.Content {
		ids = append(ids, highlight.ID)
		item, err := c.GetHighlightDetail(ctx, highlight.ID, highlight.Type)

		if err != nil {
			log.Printf("[ERROR] Failed to get detail for highlight %s: %v", highlight.ID, err)
			continue
		}
		productPrice, err := c.GetProductBestPriceWithLink(ctx, item.ID)
		if err != nil {
			log.Printf("[ERROR] Failed to get best price for item %s: %v", item.ID, err)
			continue
		}
		item.Price = productPrice.Price
		item.LinkVenda = productPrice.Permalink
		if err != nil {
			log.Printf("[ERROR] Failed to get best price for item %s: %v", item.ID, err)
			continue
		}
		items = append(items, *item)
	}

	return items, nil
}
func (c *MeliClient) GetHighlightDetail(ctx context.Context, highlightID string, highlightType string) (*SearchItem, error) {
	var endpoint string
	if highlightType == "PRODUCT" {
		endpoint = fmt.Sprintf("%s/products/%s", c.baseURL, highlightID)
	} else {
		endpoint = fmt.Sprintf("%s/items/%s", c.baseURL, highlightID)
	}

	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("meli %s: status=%d - %s", highlightType, resp.StatusCode, string(body))
	}

	// Decodificar dependendo do tipo
	// Ler corpo inteiro para melhorar mensagens de erro de decodificação
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[ERROR] Failed to read response body for %s: %v - body: %s", highlightID, err, string(body))
		return nil, err
	}

	if highlightType == "PRODUCT" {
		var product Product
		if err := json.Unmarshal(bodyBytes, &product); err != nil {
			return nil, fmt.Errorf("json decode product: %w - body: %s", err, string(bodyBytes))
		}
		return mapProductToSearchItem(product), nil
	} else {
		var item Item
		if err := json.Unmarshal(bodyBytes, &item); err != nil {
			return nil, fmt.Errorf("json decode item: %w - body: %s", err, string(bodyBytes))
		}
		return mapItemToSearchItem(item), nil
	}
}

func (c *MeliClient) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Headers básicos (sempre presentes)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")
	req.Header.Set("Referer", "https://www.mercadolivre.com.br/")

	// Se tiver token, adiciona Authorization
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	return req, nil
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

func mapProductToSearchItem(p Product) *SearchItem {
	return &SearchItem{
		ID:         p.ID,
		Title:      p.Name,
		CategoryID: p.DomainID,
		Price:      0, // precisa buscar em /products/{id}/items
		Thumbnail:  firstProductPicture(p.Pictures),
		Permalink:  p.Permalink,
		Status:     p.Status,
	}
}

func mapItemToSearchItem(i Item) *SearchItem {
	return &SearchItem{
		ID:         i.ID,
		Title:      i.Title,
		CategoryID: i.CategoryID,
		Price:      i.Price,
		Thumbnail:  i.Thumbnail,
		Permalink:  i.Permalink,
		Status:     i.Status,
	}
}

func firstProductPicture(pics []ProductPicture) string {
	if len(pics) > 0 {
		return pics[0].URL
	}
	return ""
}

// GetProductBestPrice fetches `/products/{id}/items` and returns the lowest
// item price for the given product. Supports several response formats including
// {"items": [...]}, plain []Item, and paged {"paging", "results"}.
func (c *MeliClient) GetProductBestPrice(ctx context.Context, productID string) (float64, error) {
	endpoint := fmt.Sprintf("%s/products/%s/items", c.baseURL, productID)

	// paging info used when API returns {paging, results}
	type pagingInfo struct {
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}

	type highlightPage struct {
		Paging  pagingInfo `json:"paging"`
		Results []struct {
			ItemID     string  `json:"item_id"`
			Price      float64 `json:"price"`
			Condition  string  `json:"condition"`
			CurrencyID string  `json:"currency_id"`
		} `json:"results"`
	}

	min := math.MaxFloat64
	found := false

	// processBody tries several possible response formats and updates min/found
	processBody := func(body []byte) (pagingInfo, error) {
		// 1) Try {"items": [...]}
		type itemsWrapper struct {
			Items []Item `json:"items"`
		}
		var iw itemsWrapper
		if err := json.Unmarshal(body, &iw); err == nil && len(iw.Items) > 0 {
			for _, it := range iw.Items {
				if it.Price <= 0 {
					continue
				}
				if it.Status != "active" {
					continue
				}
				if it.Price < min {
					min = it.Price
					found = true
				}
			}
			return pagingInfo{}, nil
		}

		// 2) Try plain []Item
		var items []Item
		if err := json.Unmarshal(body, &items); err == nil && len(items) > 0 {
			for _, it := range items {
				if it.Price <= 0 {
					continue
				}
				if it.Status != "active" {
					continue
				}
				if it.Price < min {
					min = it.Price
					found = true
				}
			}
			return pagingInfo{}, nil
		}

		// 3) Try paged highlight response {paging, results}
		var hp highlightPage
		if err := json.Unmarshal(body, &hp); err == nil && len(hp.Results) > 0 {
			log.Printf("[DEBUG] Paged response: total=%d, offset=%d, limit=%d, results=%d", hp.Paging.Total, hp.Paging.Offset, hp.Paging.Limit, len(hp.Results))
			for _, r := range hp.Results {
				if r.Price <= 0 {
					log.Printf("[DEBUG] Skipping item %s: price=%.2f (invalid)", r.ItemID, r.Price)
					continue
				}
				log.Printf("[DEBUG] Found item in paged results: ItemID=%s, Price=%.2f, Condition=%s", r.ItemID, r.Price, r.Condition)
				if r.Price < min {
					min = r.Price
					found = true
					log.Printf("[DEBUG] New best price: %.2f from item %s (condition: %s)", min, r.ItemID, r.Condition)
				}
			}
			return hp.Paging, nil
		}

		return pagingInfo{}, fmt.Errorf("unknown items response format")
	}

	// initial request
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("meli product items: status=%d - %s", resp.StatusCode, string(b))
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	paging, err := processBody(bodyBytes)
	if err != nil {
		return 0, fmt.Errorf("json decode product items: %w - body: %s", err, string(bodyBytes))
	}

	// If paging indicates more results, iterate pages
	if paging.Total > 0 && paging.Limit > 0 {
		for offset := paging.Offset + paging.Limit; offset < paging.Total; offset += paging.Limit {
			u, err := url.Parse(endpoint)
			if err != nil {
				break
			}
			q := u.Query()
			q.Set("offset", fmt.Sprintf("%d", offset))
			q.Set("limit", fmt.Sprintf("%d", paging.Limit))
			u.RawQuery = q.Encode()

			req, err := c.newRequest(ctx, http.MethodGet, u.String(), nil)
			if err != nil {
				return 0, err
			}
			resp, err := c.httpClient.Do(req)
			if err != nil {
				return 0, err
			}
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return 0, fmt.Errorf("meli product items (paged): status=%d - %s", resp.StatusCode, string(b))
			}
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return 0, err
			}
			if _, err := processBody(b); err != nil {
				return 0, fmt.Errorf("json decode product items (paged): %w - body: %s", err, string(b))
			}
		}
	}

	if !found {
		return 0, fmt.Errorf("no active items with price for product %s", productID)
	}
	return min, nil
}

// GetProductBestPriceWithLink fetches `/products/{id}/items` and returns the
// lowest price item with its link/URL. Supports paged and non-paged formats.
func (c *MeliClient) GetProductBestPriceWithLink(ctx context.Context, productID string) (*ProductPrice, error) {
	endpoint := fmt.Sprintf("%s/products/%s/items", c.baseURL, productID)
	shouldLog := productID == "MLB36931922"

	type pagingInfo struct {
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}

	type highlightPage struct {
		Paging  pagingInfo `json:"paging"`
		Results []struct {
			ItemID    string  `json:"item_id"`
			Price     float64 `json:"price"`
			Condition string  `json:"condition"`
		} `json:"results"`
	}

	var bestPrice *ProductPrice
	minPrice := math.MaxFloat64

	// processBody tries several possible response formats and updates bestPrice
	processBody := func(body []byte) (pagingInfo, error) {
		// 1) Try {"items": [...]}
		type itemsWrapper struct {
			Items []Item `json:"items"`
		}
		var iw itemsWrapper
		if err := json.Unmarshal(body, &iw); err == nil && len(iw.Items) > 0 {
			for _, it := range iw.Items {
				if it.Price <= 0 {
					continue
				}
				if it.Status != "active" {
					continue
				}
				if shouldLog {
					log.Printf("[DEBUG] [%s] Found item in wrapper: ID=%s, Price=%.2f, Status=%s", productID, it.ID, it.Price, it.Status)
				}
				if it.Price < minPrice {
					minPrice = it.Price
					bestPrice = &ProductPrice{
						Price:     it.Price,
						ItemID:    it.ID,
						Title:     it.Title,
						Permalink: it.Permalink,
					}
					if shouldLog {
						log.Printf("[DEBUG] [%s] New best price: %.2f from item %s", productID, minPrice, it.ID)
					}
				}
			}
			return pagingInfo{}, nil
		}

		// 2) Try plain []Item
		var items []Item
		if err := json.Unmarshal(body, &items); err == nil && len(items) > 0 {
			for _, it := range items {
				if it.Price <= 0 {
					continue
				}
				if it.Status != "active" {
					continue
				}
				if shouldLog {
					log.Printf("[DEBUG] [%s] Found item in array: ID=%s, Price=%.2f, Status=%s", productID, it.ID, it.Price, it.Status)
				}
				if it.Price < minPrice {
					minPrice = it.Price
					bestPrice = &ProductPrice{
						Price:     it.Price,
						ItemID:    it.ID,
						Title:     it.Title,
						Permalink: it.Permalink,
					}
					if shouldLog {
						log.Printf("[DEBUG] [%s] New best price: %.2f from item %s", productID, minPrice, it.ID)
					}
				}
			}
			return pagingInfo{}, nil
		}

		// 3) Try paged highlight response {paging, results}
		var hp highlightPage
		if err := json.Unmarshal(body, &hp); err == nil && len(hp.Results) > 0 {
			if shouldLog {
				log.Printf("[DEBUG] [%s] Paged response: total=%d, offset=%d, limit=%d, results=%d", productID, hp.Paging.Total, hp.Paging.Offset, hp.Paging.Limit, len(hp.Results))
			}
			for _, r := range hp.Results {
				if r.Price <= 0 {
					if shouldLog {
						log.Printf("[DEBUG] [%s] Skipping item %s: price=%.2f (invalid)", productID, r.ItemID, r.Price)
					}
					continue
				}
				if shouldLog {
					log.Printf("[DEBUG] [%s] Found item in paged results: ItemID=%s, Price=%.2f, Condition=%s", productID, r.ItemID, r.Price, r.Condition)
				}
				if r.Price < minPrice {
					minPrice = r.Price
					bestPrice = &ProductPrice{
						Price:     r.Price,
						ItemID:    r.ItemID,
						Title:     "",
						Permalink: "",
					}
					if shouldLog {
						log.Printf("[DEBUG] [%s] New best price: %.2f from item %s (condition: %s)", productID, minPrice, r.ItemID, r.Condition)
					}
				}
			}
			return hp.Paging, nil
		}

		return pagingInfo{}, fmt.Errorf("unknown items response format")
	}

	// initial request
	req, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("meli product items: status=%d - %s", resp.StatusCode, string(b))
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	paging, err := processBody(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("json decode product items: %w - body: %s", err, string(bodyBytes))
	}

	// If paging indicates more results, iterate pages
	if paging.Total > 0 && paging.Limit > 0 {
		for offset := paging.Offset + paging.Limit; offset < paging.Total; offset += paging.Limit {
			u, err := url.Parse(endpoint)
			if err != nil {
				break
			}
			q := u.Query()
			q.Set("offset", fmt.Sprintf("%d", offset))
			q.Set("limit", fmt.Sprintf("%d", paging.Limit))
			u.RawQuery = q.Encode()

			req, err := c.newRequest(ctx, http.MethodGet, u.String(), nil)
			if err != nil {
				return nil, err
			}
			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("meli product items (paged): status=%d - %s", resp.StatusCode, string(b))
			}
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, err
			}
			if _, err := processBody(b); err != nil {
				return nil, fmt.Errorf("json decode product items (paged): %w - body: %s", err, string(b))
			}
		}
	}

	if bestPrice == nil {
		return nil, fmt.Errorf("no active items with price for product %s", productID)
	}
	if shouldLog {
		log.Printf("[DEBUG] [%s] Before validation: Price=%.2f, ItemID=%s", productID, bestPrice.Price, bestPrice.ItemID)
	}

	// Validate that the best price item is actually active on Mercado Livre
	if bestPrice.ItemID != "" {
		itemEndpoint := fmt.Sprintf("%s/items/%s", c.baseURL, bestPrice.ItemID)
		req, err := c.newRequest(ctx, http.MethodGet, itemEndpoint, nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				var validateItem Item
				if json.Unmarshal(bodyBytes, &validateItem) == nil {
					if validateItem.Status != "active" {
						if shouldLog {
							log.Printf("[DEBUG] [%s] Item %s is NOT active (status=%s), rejecting", productID, bestPrice.ItemID, validateItem.Status)
						}
						// Item is not active, return error - we don't have a valid backup
						return nil, fmt.Errorf("best price item %s is not active (status=%s)", bestPrice.ItemID, validateItem.Status)
					}
					if shouldLog {
						log.Printf("[DEBUG] [%s] Item %s validated as ACTIVE", productID, bestPrice.ItemID)
					}
				}
			} else {
				resp.Body.Close()
			}
		}
	}

	if shouldLog {
		log.Printf("[DEBUG] [%s] FINAL RESULT: Price=%.2f, ItemID=%s", productID, bestPrice.Price, bestPrice.ItemID)
	}
	return bestPrice, nil
}
