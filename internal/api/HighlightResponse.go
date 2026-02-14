package api

type HighlightResponse struct {
	QueryData struct {
		HighlightType string `json:"highlight_type"`
		Criteria      string `json:"criteria"`
		ID            string `json:"id"`
	} `json:"query_data"`
	Content []struct {
		ID       string `json:"id"`
		Position int    `json:"position"`
		Type     string `json:"type"`
	} `json:"content"`
}
