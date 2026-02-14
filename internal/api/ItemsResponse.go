package api

type Item struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	CategoryID   string        `json:"category_id"`
	Price        float64       `json:"price"`
	CurrencyID   string        `json:"currency_id"`
	AvailableQty int           `json:"available_quantity"`
	SoldQty      int           `json:"sold_quantity"`
	Condition    string        `json:"condition"`
	Permalink    string        `json:"permalink"`
	Thumbnail    string        `json:"thumbnail"`
	Pictures     []ItemPicture `json:"pictures"`
	SellerID     int           `json:"seller_id"`
	Status       string        `json:"status"`
	Attributes   []Attribute   `json:"attributes"`
}

type ItemPicture struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type Attribute struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ValueID   string `json:"value_id"`
	ValueName string `json:"value_name"`
}
