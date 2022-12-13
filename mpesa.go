package mpesa

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Environment indicates the current mode the application is running on. Either Sandbox or Production.
type Environment uint8

// cache stores the AuthorizationResponse for the specified accessTokenTTL
type cache map[string]AuthorizationResponse

type identifier string

type IdentifierType uint8

// CommandID identifies the command to execute.
type CommandID string

const (
	Unknown Environment = iota
	// Sandbox represents the test Environment
	Sandbox
	// Production represent the live Environment
	Production
)

const (
	stkPush      identifier = "stk push"
	stkPushQuery identifier = "stk push query"
	b2c          identifier = "b2c"
	txnStatus    identifier = "txn status"
)

const (
	MSISDN     IdentifierType = 1
	TillNumber IdentifierType = 2
	ShortCode  IdentifierType = 4
)

const (
	TransactionStatusQuery CommandID = "TransactionStatusQuery"
)

const SuccessResultCode = "0"

var accessTokenTTL = 55 * time.Minute

// IsProduction returns true if the current env is set to production.
func (e Environment) IsProduction() bool {
	return e == Production
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

//go:embed certs
var certFS embed.FS

// Mpesa is an app to make a transaction
type Mpesa struct {
	mu    sync.Mutex
	cache cache

	client         HttpClient
	consumerKey    string
	consumerSecret string
	environment    Environment

	authURL         string
	b2cURL          string
	stkPushQueryURL string
	stkPushURL      string
	txnStatusURL    string
}

var (
	ErrInvalidPasskey           = errors.New("mpesa: passkey cannot be empty")
	ErrInvalidInitiatorPassword = errors.New("mpesa: initiator password cannot be empty")
)

const (
	sandboxBaseURL    = "https://sandbox.safaricom.co.ke"
	productionBaseURL = "https://api.safaricom.co.ke"
)

// NewApp initializes a new Mpesa app that will be used to perform C2B or B2C transactions.
func NewApp(c HttpClient, consumerKey, consumerSecret string, env Environment) *Mpesa {
	if c == nil {
		c = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	var baseUrl string
	switch env {
	case Production:
		baseUrl = productionBaseURL
	default:
		baseUrl = sandboxBaseURL
	}

	return &Mpesa{
		cache:          make(cache),
		client:         c,
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		environment:    env,

		authURL:         baseUrl + `/oauth/v1/generate?grant_type=client_credentials`,
		b2cURL:          baseUrl + `/mpesa/b2c/v1/paymentrequest`,
		stkPushQueryURL: baseUrl + `/mpesa/stkpushquery/v1/query`,
		stkPushURL:      baseUrl + `/mpesa/stkpush/v1/processrequest`,
		txnStatusURL:    baseUrl + `/mpesa/transactionstatus/v1/query`,
	}
}

func generateTimestampAndPassword(shortcode uint, passkey string) (string, string) {
	timestamp := time.Now().Format("20060102150405")
	password := fmt.Sprintf("%d%s%s", shortcode, passkey, timestamp)
	return timestamp, base64.StdEncoding.EncodeToString([]byte(password))
}

func (m *Mpesa) makeHttpRequestWithToken(
	ctx context.Context,
	method, url string,
	body interface{},
	identifier identifier,
) (*http.Response, error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error marshling %v request payload - %v", identifier, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("mpesa: error creating %v request - %v", identifier, err)
	}

	accessToken, err := m.GenerateAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", `Bearer `+accessToken)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error making %v request - %v", identifier, err)
	}

	return res, nil
}

func (m *Mpesa) getSecurityCredentials(password string) (string, error) {
	if password == "" {
		return "", ErrInvalidInitiatorPassword
	}

	certPath := "certs/sandbox.cer"
	if m.Environment().IsProduction() {
		certPath = "certs/production.cer"
	}

	publicKey, err := certFS.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("mpesa: error opening cert %v - %v", certPath, err)
	}

	block, _ := pem.Decode(publicKey)

	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("mpesa: error parsing cert %v - %v", certPath, err)
	}

	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	reader := rand.Reader
	signature, err := rsa.EncryptPKCS1v15(reader, rsaPublicKey, []byte(password))
	if err != nil {
		return "", fmt.Errorf("mpesa: error encrypting password: %v", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// Environment returns the current environment the app is running on.
func (m *Mpesa) Environment() Environment {
	return m.environment
}

// GenerateAccessToken returns a time bound access token to call allowed APIs.
// This token should be used in all other subsequent responses to the APIs
// GenerateAccessToken will also cache the access token for the specified refresh after period
func (m *Mpesa) GenerateAccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cachedData, ok := m.cache[m.consumerKey]; ok {
		if cachedData.setAt.Add(accessTokenTTL).After(time.Now()) {
			return cachedData.AccessToken, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.authURL, nil)
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
		return "", fmt.Errorf("mpesa: error decoding authorization response - %v", err)
	}

	response.setAt = time.Now()
	m.cache[m.consumerKey] = response
	return m.cache[m.consumerKey].AccessToken, nil
}

// STKPush initiates online payment on behalf of a customer using STKPush.
func (m *Mpesa) STKPush(ctx context.Context, passkey string, req STKPushRequest) (*GenericRequestResponse, error) {
	if passkey == "" {
		return nil, ErrInvalidPasskey
	}

	req.Timestamp, req.Password = generateTimestampAndPassword(req.BusinessShortCode, passkey)

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.stkPushURL, req, stkPush)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp GenericRequestResponse
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding stk push request response - %v", err)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: stk push request ID %v failed with error code %v - %v",
			resp.RequestID,
			resp.ErrorCode,
			resp.ErrorMessage,
		)
	}

	return &resp, nil
}

// UnmarshalSTKPushCallback decodes the provided value to STKPushCallback.
func UnmarshalSTKPushCallback(in interface{}) (*STKPushCallback, error) {
	b, err := toBytes(in)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error unmarshing input - %v", err)
	}

	var callback STKPushCallback
	if err := json.Unmarshal(b, &callback); err != nil {
		return nil, fmt.Errorf("mpesa: error unmarshling stk push callback - %v", err)
	}

	return &callback, nil
}

// B2C transacts between an M-Pesa short code to a phone number registered on M-Pesa
func (m *Mpesa) B2C(ctx context.Context, initiatorPwd string, req B2CRequest) (*GenericRequestResponse, error) {
	if initiatorPwd == "" {
		return nil, ErrInvalidInitiatorPassword
	}

	securityCredential, err := m.getSecurityCredentials(initiatorPwd)
	if err != nil {
		return nil, err
	}

	req.SecurityCredential = securityCredential

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.b2cURL, req, b2c)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp GenericRequestResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding b2c request response - %v", err)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: b2c request ID %v failed with error code %v - %v",
			resp.RequestID,
			resp.ErrorCode,
			resp.ErrorMessage,
		)
	}

	return &resp, nil
}

// UnmarshalB2CCallback decodes the provided value to B2CCallback
func UnmarshalB2CCallback(in interface{}) (*B2CCallback, error) {
	b, err := toBytes(in)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error unmarshing input - %v", err)
	}

	var callback B2CCallback
	if err := json.Unmarshal(b, &callback); err != nil {
		return nil, fmt.Errorf("mpesa: error unmarshling stk push callback - %v", err)
	}

	return &callback, nil
}

// STKPushQuery checks the status of an STKPush payment.
func (m *Mpesa) STKPushQuery(
	ctx context.Context,
	passkey string,
	req STKQueryRequest,
) (*GenericRequestResponse, error) {
	if passkey == "" {
		return nil, ErrInvalidPasskey
	}

	req.Timestamp, req.Password = generateTimestampAndPassword(req.BusinessShortCode, passkey)

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.stkPushQueryURL, req, stkPushQuery)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp GenericRequestResponse
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding stk push query request response - %v", err)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: stk push query request ID %v failed with error code %v - %v",
			resp.RequestID,
			resp.ErrorCode,
			resp.ErrorMessage,
		)
	}

	return &resp, nil
}

func (m *Mpesa) GetTxnStatus(
	ctx context.Context,
	initiatorPwd string,
	req TransactionStatusRequest,
) (*GenericRequestResponse, error) {
	if initiatorPwd == "" {
		return nil, ErrInvalidInitiatorPassword
	}

	securityCredential, err := m.getSecurityCredentials(initiatorPwd)
	if err != nil {
		return nil, err
	}

	req.SecurityCredential = securityCredential

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.txnStatusURL, req, txnStatus)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp GenericRequestResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding txn status request response - %v", err)
	}

	if resp.ResponseCode != SuccessResultCode {
		return nil, fmt.Errorf(
			"mpesa: txn status failed with response code %v - %v", resp.ResponseCode, resp.ResponseDescription,
		)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: txn status request ID %v failed with error code %v - %v",
			resp.RequestID,
			resp.ErrorCode,
			resp.ErrorMessage,
		)
	}

	return &resp, nil
}
