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
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Environment indicates the current mode the application is running on. Either EnvironmentSandbox or EnvironmentProduction.
type Environment uint8

// cache stores the AuthorizationResponse for the specified accessTokenTTL
type cache map[string]AuthorizationResponse

const (
	EnvironmentSandbox Environment = iota
	EnvironmentProduction
)

type ResponseType string

const (
	ResponseTypeCanceled ResponseType = "Canceled"
	ResponseTypeComplete ResponseType = "Completed"
)

var accessTokenTTL = 55 * time.Minute

// requiredURLScheme present the required scheme for the callbacks
const requiredURLScheme = "https"

// IsProduction returns true if the current env is set to production.
func (e Environment) IsProduction() bool {
	return e == EnvironmentProduction
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

//go:embed certs
var certFS embed.FS

// Mpesa is an app to make a transaction
type Mpesa struct {
	client      HttpClient
	environment Environment
	mu          sync.Mutex
	cache       cache

	consumerKey    string
	consumerSecret string

	authURL           string
	accountBalanceURL string
	b2cURL            string
	c2bURL            string
	dynamicQRURL      string
	stkPushQueryURL   string
	stkPushURL        string
	txnStatusURL      string
}

var (
	// ErrInvalidPasskey indicates that no passkey was provided.
	ErrInvalidPasskey = errors.New("mpesa: passkey cannot be empty")

	// ErrInvalidInitiatorPassword indicates that no initiator password was provided.
	ErrInvalidInitiatorPassword = errors.New("mpesa: initiator password cannot be empty")
)

const (
	sandboxBaseURL    = "https://sandbox.safaricom.co.ke"
	productionBaseURL = "https://api.safaricom.co.ke"
)

// validateURL checks if the provided URL is valid and is being server via https
func validateURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("mpesa: %v", err)
	}

	if u.Scheme != requiredURLScheme {
		return fmt.Errorf("mpesa: %q must use %q", rawURL, requiredURLScheme)
	}

	return nil
}

// NewApp initializes a new Mpesa app that will be used to perform C2B or B2C transactions.
func NewApp(c HttpClient, consumerKey, consumerSecret string, env Environment) *Mpesa {
	if c == nil {
		c = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	baseUrl := sandboxBaseURL
	if env == EnvironmentProduction {
		baseUrl = productionBaseURL
	}

	return &Mpesa{
		client:      c,
		environment: env,
		cache:       make(cache),

		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,

		authURL:           baseUrl + `/oauth/v1/generate?grant_type=client_credentials`,
		accountBalanceURL: baseUrl + `/mpesa/accountbalance/v1/query`,
		b2cURL:            baseUrl + `/mpesa/b2c/v1/paymentrequest`,
		c2bURL:            baseUrl + `/mpesa/c2b/v1/registerurl`,
		dynamicQRURL:      baseUrl + `/mpesa/qrcode/v1/generate`,
		stkPushQueryURL:   baseUrl + `/mpesa/stkpushquery/v1/query`,
		stkPushURL:        baseUrl + `/mpesa/stkpush/v1/processrequest`,
		txnStatusURL:      baseUrl + `/mpesa/transactionstatus/v1/query`,
	}
}

// generateTimestampAndPassword returns the current timestamp in the format YYYYMMDDHHmmss and a base64 encoded
// password in the format shortcode+passkey+timestamp
func generateTimestampAndPassword(shortcode uint, passkey string) (string, string) {
	timestamp := time.Now().Format("20060102150405")
	password := fmt.Sprintf("%d%s%s", shortcode, passkey, timestamp)
	return timestamp, base64.StdEncoding.EncodeToString([]byte(password))
}

// makeHttpRequestWithToken makes an API call to the provided url using the provided http method.
func (m *Mpesa) makeHttpRequestWithToken(
	ctx context.Context, method, url string, body interface{},
) (*http.Response, error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mpesa: marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("mpesa: create request: %v", err)
	}

	accessToken, err := m.GenerateAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", `Bearer `+accessToken)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mpesa: make request: %v", err)
	}

	return res, nil
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
		return "", fmt.Errorf("mpesa: create auth request: %v", err)
	}

	req.SetBasicAuth(m.consumerKey, m.consumerSecret)

	res, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mpesa: make auth request: %v", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mpesa: auth failed with status: %v", res.Status)
	}

	var response AuthorizationResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("mpesa: decode auth response: %v", err)
	}

	response.setAt = time.Now()
	m.cache[m.consumerKey] = response
	return m.cache[m.consumerKey].AccessToken, nil
}

// STKPush initiates online payment on behalf of a customer using STKPush.
func (m *Mpesa) STKPush(ctx context.Context, passkey string, req STKPushRequest) (*Response, error) {
	if passkey == "" {
		return nil, ErrInvalidPasskey
	}

	req.Timestamp, req.Password = generateTimestampAndPassword(req.BusinessShortCode, passkey)

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.stkPushURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp Response
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: decode response : %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"mpesa: request %v failed with code %v: %v", resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	return &resp, nil
}

// UnmarshalSTKPushCallback decodes the provided value to STKPushCallback.
func UnmarshalSTKPushCallback(r io.Reader) (*STKPushCallback, error) {
	var callback STKPushCallback
	if err := json.NewDecoder(r).Decode(&callback); err != nil {
		return nil, fmt.Errorf("mpesa: decode: %v", err)
	}

	return &callback, nil
}

func (m *Mpesa) generateSecurityCredentials(initiatorPwd string) (string, error) {
	certPath := "certs/sandbox.cer"
	if m.Environment().IsProduction() {
		certPath = "certs/production.cer"
	}

	publicKey, err := certFS.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("mpesa: read cert: %v", err)
	}

	block, _ := pem.Decode(publicKey)

	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("mpesa:parse cert: %v", err)
	}

	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	reader := rand.Reader
	signature, err := rsa.EncryptPKCS1v15(reader, rsaPublicKey, []byte(initiatorPwd))
	if err != nil {
		return "", fmt.Errorf("mpesa: encrypt password: %v", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// B2C transacts between an M-Pesa short code to a phone number registered on M-Pesa
func (m *Mpesa) B2C(ctx context.Context, initiatorPwd string, req B2CRequest) (*Response, error) {
	if initiatorPwd == "" {
		return nil, ErrInvalidInitiatorPassword
	}

	securityCredential, err := m.generateSecurityCredentials(initiatorPwd)
	if err != nil {
		return nil, err
	}

	req.SecurityCredential = securityCredential

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.b2cURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp Response
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: decode response: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"mpesa: request %v failed with code %v: %v", resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	return &resp, nil
}

// UnmarshalCallback decodes the provided value to Callback
func UnmarshalCallback(r io.Reader) (*Callback, error) {
	var callback Callback
	if err := json.NewDecoder(r).Decode(&callback); err != nil {
		return nil, fmt.Errorf("mpesa: decode: %v", err)
	}

	return &callback, nil
}

// STKQuery checks the status of an STKPush payment.
func (m *Mpesa) STKQuery(ctx context.Context, passkey string, req STKQueryRequest) (*Response, error) {
	if passkey == "" {
		return nil, ErrInvalidPasskey
	}

	req.Timestamp, req.Password = generateTimestampAndPassword(req.BusinessShortCode, passkey)

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.stkPushQueryURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp Response
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: decode response: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"mpesa: request %v failed with code %v: %v", resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	return &resp, nil
}

// RegisterC2BURL API works hand in hand with Customer to Business (C2B) APIs and allows receiving payment notifications to your paybill.
// This API enables you to register the callback URLs via which you shall receive notifications for payments to your pay bill/till number.
// There are two URLs required for Register URL API: Validation URL and Confirmation URL.
// Validation URL: This is the URL that is only used when a Merchant (Partner) requires to validate the details of the payment before accepting.
// For example, a bank would want to verify if an account number exists in their platform before accepting a payment from the customer.
// Confirmation URL:  This is the URL that receives payment notification once payment has been completed successfully on M-PESA.
func (m *Mpesa) RegisterC2BURL(ctx context.Context, req RegisterC2BURLRequest) (*Response, error) {
	switch req.ResponseType {
	case ResponseTypeComplete, ResponseTypeCanceled:
		response, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.c2bURL, req)
		if err != nil {
			return nil, err
		}
		defer func(body io.ReadCloser) {
			_ = body.Close()
		}(response.Body)

		var result Response

		if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("mpesa: decode response: %v", err)
		}

		return &result, nil
	default:
		return nil, fmt.Errorf("mpesa: the provided ResponseType [%s] is not valid", req.ResponseType)
	}
}

// DynamicQR API is used to generate a Dynamic QR which enables Safaricom M-PESA customers who have My Safaricom App or
// M-PESA app, to scan a QR (Quick Response) code, to capture till number and amount then authorize to pay for goods and
// services at select LIPA NA M-PESA (LNM) merchant outlets. If the decodeImage parameter is set to true, the QR code
// will be decoded and a base url is set on the ImagePath field
func (m *Mpesa) DynamicQR(
	ctx context.Context, req DynamicQRRequest, transactionType DynamicQRTransactionType, decodeImage bool,
) (*DynamicQRResponse, error) {
	req.TransactionType = transactionType

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.dynamicQRURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp *DynamicQRResponse
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: decode response: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"mpesa: request %v failed with code %v: %v", resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	if !decodeImage {
		return resp, nil
	}

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(resp.QRCode))

	image, err := png.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("mpesa: decode png: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("mpesa: wd: %v", err)
	}

	imagesDir := filepath.Join(wd, "storage", "images")
	if _, err := os.Stat(imagesDir); os.IsNotExist(err) {
		if err = os.Mkdir(imagesDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("mpesa: create images dir: %v", err)
		}
	}

	amountStr := strconv.Itoa(int(req.Amount))
	filename := req.MerchantName + "_" + amountStr + "_" + req.CreditPartyIdentifier + ".png"
	filename = imagesDir + "/" + strings.ReplaceAll(filename, " ", "_")

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("mpesa: open png: %v", err)

	}

	if err = png.Encode(f, image); err != nil {
		return nil, fmt.Errorf("mpesa: encode png: %v", err)
	}

	resp.ImagePath = filename
	return resp, nil
}

// GetTransactionStatus checks the status of a transaction
func (m *Mpesa) GetTransactionStatus(
	ctx context.Context, initiatorPwd string, req TransactionStatusRequest,
) (*Response, error) {
	if initiatorPwd == "" {
		return nil, ErrInvalidInitiatorPassword
	}

	if err := validateURL(req.QueueTimeOutURL); err != nil {
		return nil, err
	}

	if err := validateURL(req.ResultURL); err != nil {
		return nil, err
	}

	securityCredential, err := m.generateSecurityCredentials(initiatorPwd)
	if err != nil {
		return nil, err
	}

	req.SecurityCredential = securityCredential
	req.CommandID = TransactionStatusQuery
	req.IdentifierType = Shortcode

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.txnStatusURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp Response
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: decode response: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"mpesa: request %v failed with code %v: %v", resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	return &resp, nil
}

// GetAccountBalance fetches the account balance of a short code. This can be used for both B2C, buy goods and pay bill
// accounts.
func (m *Mpesa) GetAccountBalance(
	ctx context.Context, initiatorPwd string, req AccountBalanceRequest,
) (*Response, error) {
	if initiatorPwd == "" {
		return nil, ErrInvalidInitiatorPassword
	}

	if err := validateURL(req.QueueTimeOutURL); err != nil {
		return nil, err
	}

	if err := validateURL(req.ResultURL); err != nil {
		return nil, err
	}

	securityCredential, err := m.generateSecurityCredentials(initiatorPwd)
	if err != nil {
		return nil, err
	}

	req.SecurityCredential = securityCredential
	req.CommandID = AccountBalance
	req.IdentifierType = Shortcode

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.accountBalanceURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp Response
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: decode response: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"mpesa: request %v failed with code %v: %v", resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	return &resp, nil
}
