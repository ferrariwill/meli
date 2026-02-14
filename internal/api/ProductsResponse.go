package api

import "encoding/json"

type Product struct {
	ID               string `json:"id"`
	CatalogProductID string `json:"catalog_product_id"`
	Status           string `json:"status"`
	DomainID         string `json:"domain_id"`
	Permalink        string `json:"permalink"`
	Name             string `json:"name"`
	FamilyName       string `json:"family_name"`
	Type             string `json:"type"`

	Brand string `json:"brand"`
	Model string `json:"model"`
	Color string `json:"color"`

	Thumbnail string           `json:"thumbnail"`
	Pictures  []ProductPicture `json:"pictures"`

	ShortDescription json.RawMessage `json:"short_description"`

	CreatedAt string `json:"date_created"`
	UpdatedAt string `json:"last_updated"`
}

type ProductPicture struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	MaxWidth  int    `json:"max_width"`
	MaxHeight int    `json:"max_height"`
}

// ShortDescriptionText tenta extrair um texto legível de ShortDescription,
// que pode ser uma string ou um objeto com campos como `plain_text` ou `blocks`.
func ShortDescriptionText(b json.RawMessage) string {
	if len(b) == 0 {
		return ""
	}

	// Tenta como string direta
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		return s
	}

	// Tenta como objeto
	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err == nil {
		if v, ok := obj["plain_text"].(string); ok {
			return v
		}
		if v, ok := obj["text"].(string); ok {
			return v
		}
		if blocks, ok := obj["blocks"].([]interface{}); ok {
			out := ""
			for _, blk := range blocks {
				if m, ok := blk.(map[string]interface{}); ok {
					if t, ok := m["text"].(string); ok {
						if out != "" {
							out += "\n"
						}
						out += t
					}
				}
			}
			if out != "" {
				return out
			}
		}
		// fallback: return the raw JSON
		return string(b)
	}

	return string(b)
}
