package mpesa

import (
	"encoding/json"
	"fmt"
	"github.com/patrickmn/go-cache"
	"gitlab.com/jwambugu/go-mpesa/pkg/config"
	"io/ioutil"
	"net/http"
	"time"
)

type (
	// Mpesa is an app to make a transaction
	Mpesa struct {
		ConsumerKey    string
		ConsumerSecret string
		BaseURL        string
		IsOnProduction bool
		Cache          *cache.Cache
	}

	// accessTokenResponse is the response sent back by Safaricom when we make a request to generate a token
	// for a specific app
	accessTokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    string `json:"expires_in"`
		RequestID    string `json:"requestId"`
		ErrorCode    string `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}
)

// Init initializes a new Mpesa app that will be used to perform C2B or B2C transaction
func Init(c *config.Credentials, isOnProduction bool) *Mpesa {
	baseUrl := "https://sandbox.safaricom.co.ke"

	if isOnProduction {
		baseUrl = "https://api.safaricom.co.ke"
	}

	newCache := cache.New(55*time.Minute, 10*time.Minute)

	return &Mpesa{
		ConsumerKey:    c.ConsumerKey,
		ConsumerSecret: c.ConsumerSecret,
		BaseURL:        baseUrl,
		IsOnProduction: isOnProduction,
		Cache:          newCache,
	}
}

// makeRequest performs all the http requests to MPesa
func makeRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("Accept", "application/json")

	var client http.Client

	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("mpesa.MakeRequest:: %v", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("mpesa.ReadBody:: %v", err)
	}

	fmt.Println(fmt.Sprintf("[*] Response Body:: %s", string(body)))
	return body, nil
}

// cachedAccessToken returns the cached access token
func (m *Mpesa) cachedAccessToken() (interface{}, bool) {
	return m.Cache.Get(m.ConsumerKey)
}

// GetAccessToken returns a token to be used to authenticate an app.
// This token should be used in all other subsequent requests to the APIs
// GetAccessToken will also cache the access token for 55 minutes.
func (m *Mpesa) GetAccessToken() (string, error) {
	cachedToken, exists := m.cachedAccessToken()

	if exists {
		return cachedToken.(string), nil
	}

	url := fmt.Sprintf("%s/oauth/v1/generate?grant_type=client_credentials", m.BaseURL)

	// Create a new http request
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return "", fmt.Errorf("mpesa.GetAccessToken.NewRequest:: %v", err)
	}

	// Set the basic auth header
	req.SetBasicAuth(m.ConsumerKey, m.ConsumerSecret)

	resp, err := makeRequest(req)

	if err != nil {
		return "", err
	}

	var response accessTokenResponse

	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("mpesa.GetAccessToken.UnmarshalResponse:: %v", err)
	}

	// Check if the authentication passed. If it did, we won't have any error code
	if response.ErrorCode != "" {
		return "", fmt.Errorf("mpesa.GetAccessToken.MpesaResponse:: %v", response.ErrorMessage)
	}

	token := response.AccessToken

	m.Cache.Set(m.ConsumerKey, token, 55*time.Minute)

	return token, nil
}

// Environment returns the current environment the app is running on.
// It will return either production or sandbox
func (m *Mpesa) Environment() string {
	environment := "production"

	if !m.IsOnProduction {
		environment = "sandbox"
	}

	return environment
}
