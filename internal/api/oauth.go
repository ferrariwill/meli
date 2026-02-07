package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// OAuth base URLs - Mercado Livre uses different endpoints
	oauthAuthURL  = "https://auth.mercadolivre.com.br/authorization"
	oauthTokenURL = "https://api.mercadolibre.com/oauth/token"
)

// OAuthClient handles OAuth 2.0 flow for Mercado Livre
type OAuthClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

func NewOAuthClient(clientID, clientSecret, redirectURI string) *OAuthClient {
	return &OAuthClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetAuthorizationURL returns the URL to redirect the user for OAuth authorization
func (o *OAuthClient) GetAuthorizationURL() string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", o.clientID)
	params.Set("redirect_uri", o.redirectURI)
	// Note: redirect_uri must match exactly what's configured in Mercado Livre DevCenter
	return oauthAuthURL + "?" + params.Encode()
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	UserID       int    `json:"user_id"`
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (o *OAuthClient) ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("client_id", o.clientID)
	params.Set("client_secret", o.clientSecret)
	params.Set("code", code)
	params.Set("redirect_uri", o.redirectURI)

	// For POST requests, params must be in the body
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oauth token exchange failed: status %d - %s", resp.StatusCode, string(errorBody))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}
