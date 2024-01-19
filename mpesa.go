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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Environment indicates the current mode the application is running on. Either Sandbox or Production.
type Environment uint8

// cache stores the AuthorizationResponse for the specified accessTokenTTL
type cache map[string]AuthorizationResponse

const (
	Sandbox Environment = iota
	Production

	ResponseTypeComplete string = "Completed"
	ResponseTypeCanceled string = "Canceled"
)

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
	client      HttpClient
	environment Environment
	mu          sync.Mutex
	cache       cache

	consumerKey    string
	consumerSecret string

	authURL         string
	b2cURL          string
	c2bURL          string
	dynamicQRURL    string
	stkPushQueryURL string
	stkPushURL      string
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

	baseUrl := sandboxBaseURL
	if env == Production {
		baseUrl = productionBaseURL
	}

	return &Mpesa{
		client:      c,
		environment: env,
		cache:       make(cache),

		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,

		authURL:         baseUrl + `/oauth/v1/generate?grant_type=client_credentials`,
		b2cURL:          baseUrl + `/mpesa/b2c/v1/paymentrequest`,
		c2bURL:          baseUrl + `/mpesa/c2b/v1/registerurl`,
		dynamicQRURL:    baseUrl + `/mpesa/qrcode/v1/generate`,
		stkPushQueryURL: baseUrl + `/mpesa/stkpushquery/v1/query`,
		stkPushURL:      baseUrl + `/mpesa/stkpush/v1/processrequest`,
	}
}

func generateTimestampAndPassword(shortcode uint, passkey string) (string, string) {
	timestamp := time.Now().Format("20060102150405")
	password := fmt.Sprintf("%d%s%s", shortcode, passkey, timestamp)
	return timestamp, base64.StdEncoding.EncodeToString([]byte(password))
}

func (m *Mpesa) makeHttpRequestWithToken(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error marshling request payload - %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("mpesa: error creating request - %v", err)
	}

	accessToken, err := m.GenerateAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", `Bearer `+accessToken)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error making request - %v", err)
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
func (m *Mpesa) STKPush(ctx context.Context, passkey string, req STKPushRequest) (*GeneralRequestResponse, error) {
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

	var resp GeneralRequestResponse
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding stk push request response - %v", err)
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
func (m *Mpesa) B2C(ctx context.Context, initiatorPwd string, req B2CRequest) (*GeneralRequestResponse, error) {
	if initiatorPwd == "" {
		return nil, ErrInvalidInitiatorPassword
	}

	certPath := "certs/sandbox.cer"
	if m.Environment().IsProduction() {
		certPath = "certs/production.cer"
	}

	publicKey, err := certFS.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error opening cert %v - %v", certPath, err)
	}

	block, _ := pem.Decode(publicKey)

	var cert *x509.Certificate
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("mpesa: error parsing cert %v - %v", certPath, err)
	}

	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	reader := rand.Reader
	signature, err := rsa.EncryptPKCS1v15(reader, rsaPublicKey, []byte(initiatorPwd))
	if err != nil {
		return nil, fmt.Errorf("mpesa: error encrypting initiator password: %v", err)
	}

	req.SecurityCredential = base64.StdEncoding.EncodeToString(signature)

	res, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.b2cURL, req)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer res.Body.Close()

	var resp GeneralRequestResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding b2c request response - %v", err)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: b2c request ID %v failed with error code %v:%v",
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

// STKQuery checks the status of an STKPush payment.
func (m *Mpesa) STKQuery(ctx context.Context, passkey string, req STKQueryRequest) (*GeneralRequestResponse, error) {
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

	var resp GeneralRequestResponse
	if err = json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("mpesa: error decoding stk push query request response - %v", err)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf(
			"mpesa: stk push query request ID %v failed with error code %v:%v",
			resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
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
func (m *Mpesa) RegisterC2BURL(ctx context.Context, req RegisterC2BURLRequest) (*GeneralRequestResponse, error) {
	switch req.ResponseType {
	case ResponseTypeComplete, ResponseTypeCanceled:
		response, err := m.makeHttpRequestWithToken(ctx, http.MethodPost, m.c2bURL, req)
		if err != nil {
			return nil, errors.Join(errors.New("mpesa: c2b url validation failed"), err)
		}
		defer func(body io.ReadCloser) {
			_ = body.Close()
		}(response.Body)

		var result GeneralRequestResponse
		err = json.NewDecoder(response.Body).Decode(&result)
		if err != nil {
			return nil, errors.Join(errors.New("mpesa: could not unmarshall c2b response body"), err)
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
		return nil, fmt.Errorf("mpesa: error decoding dynamic QR response - %v", err)
	}

	if resp.ErrorCode != "" {
		return nil, fmt.Errorf("mpesa: dynamic QR request ID %v failed with error code %v:%v",
			resp.RequestID, resp.ErrorCode, resp.ErrorMessage,
		)
	}

	if !decodeImage {
		return resp, nil
	}

	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(resp.QRCode))

	image, err := png.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("mpesa: failed to decode png: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("mpesa: failed to get working dir: %v", err)
	}

	imagesDir := filepath.Join(wd, "storage", "images")
	if _, err := os.Stat(imagesDir); os.IsNotExist(err) {
		if err = os.Mkdir(imagesDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("mpesa: failed to create images dir: %v", err)
		}
	}

	amountStr := strconv.Itoa(int(req.Amount))
	filename := req.MerchantName + "_" + amountStr + "_" + req.CreditPartyIdentifier + ".png"
	filename = imagesDir + "/" + strings.ReplaceAll(filename, " ", "_")

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("mpesa: failed to open png file: %v", err)

	}

	if err = png.Encode(f, image); err != nil {
		return nil, fmt.Errorf("mpesa: failed to encode png: %v", err)
	}

	resp.ImagePath = filename
	return resp, nil
}
