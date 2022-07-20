package mpesa

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Environment indicates the current mode the application is running on. Either Sandbox or Production.
type Environment string

// cache stores the AuthorizationResponse for the specified accessTokenTTL
type cache map[string]AuthorizationResponse

const (
	Sandbox    Environment = "sandbox"
	Production Environment = "production"
)

var accessTokenTTL = 55 * time.Minute

// IsProduction returns true if the current env is set to production.
func (e Environment) IsProduction() bool {
	return e == Production
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Mpesa is an app to make a transaction
type Mpesa struct {
	client      HttpClient
	environment Environment
	mu          sync.Mutex
	cache       cache

	consumerKey    string
	consumerSecret string

	authURL    string
	stkPushURL string
}

var (
	ErrInvalidPasskey = errors.New("mpesa: passkey cannot be empty")
)

const (
	sandboxBaseURL    = "https://sandbox.safaricom.co.ke"
	productionBaseURL = "https://api.safaricom.co.ke"
)

// NewApp initializes a new Mpesa app that will be used to perform C2B or B2C transaction
func NewApp(c HttpClient, consumerKey, consumerSecret string, env Environment) *Mpesa {
	if c == nil {
		c = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	baseUrl := sandboxBaseURL
	if env == Production {
		baseUrl = productionBaseURL
	}

	return &Mpesa{
		client:         c,
		environment:    env,
		cache:          make(cache),
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		authURL:        baseUrl + `/oauth/v1/generate?grant_type=client_credentials`,
		stkPushURL:     baseUrl + `/mpesa/stkpush/v1/processrequest`,
	}
}

// GenerateAccessToken returns a time bound access token to call allowed APIs.
// This token should be used in all other subsequent responses to the APIs
// GenerateAccessToken will also cache the access token for the specified refresh after period
func (m *Mpesa) GenerateAccessToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cachedData, ok := m.cache[m.consumerKey]; ok {
		if cachedData.setAt.Add(accessTokenTTL).After(time.Now()) {
			return cachedData.AccessToken, nil
		}
	}

	req, err := http.NewRequest(http.MethodGet, m.authURL, nil)
	if err != nil {
		return "", fmt.Errorf("mpesa: error creating authorization http request - %v", err)
	}

	req.SetBasicAuth(m.consumerKey, m.consumerSecret)

	res, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mpesa: error making authorization http request - %v", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mpesa: authorization request failed with status - %v", res.Status)
	}

	var response AuthorizationResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("mpesa: error decoding authorization response - %v", err.Error())
	}

	response.setAt = time.Now()
	m.cache[m.consumerKey] = response
	return m.cache[m.consumerKey].AccessToken, nil
}

// Environment returns the current environment the app is running on.
func (m *Mpesa) Environment() Environment {
	return m.environment
}

// LipaNaMpesaOnline initiates online payment on behalf of a customer using STKPush.
func (m *Mpesa) LipaNaMpesaOnline(
	ctx context.Context,
	passkey string,
	stkReq *STKPushRequest,
) (*STKPushRequestResponse, error) {
	if passkey == "" {
		return nil, ErrInvalidPasskey
	}

	timestamp := time.Now().Format("20060102150405")
	password := fmt.Sprintf("%s%s%s", stkReq.BusinessShortCode, passkey, timestamp)
	stkReq.Password = base64.StdEncoding.EncodeToString([]byte(password))

	reqBody, err := json.Marshal(stkReq)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error marshling stk push request payload - %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.stkPushURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("mpesa: error creating stk push request - %v", err)
	}

	accessToken, err := m.GenerateAccessToken()
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", `Bearer `+accessToken)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error making stk push request - %v", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp STKPushRequestResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding stk push request response - %v", err.Error())
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: stk push request ID %v failed with error code %v:%v",
			resp.RequestID,
			resp.ErrorCode,
			resp.ErrorMessage,
		)
	}

	return &resp, nil
}
