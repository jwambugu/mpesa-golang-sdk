package mpesa

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	b2cURL     string
}

var (
	ErrInvalidBusinessShortCode      = errors.New("mpesa: business shortcode must be 5 or more digits")
	ErrInvalidPartyB                 = errors.New("mpesa: party B must be must be 5 or more digits")
	ErrInvalidPasskey                = errors.New("mpesa: passkey cannot be empty")
	ErrInvalidAmount                 = errors.New("mpesa: amount must be greater than 0")
	ErrInvalidPhoneNumber            = errors.New("mpesa: phone number must be 12 digits and must be in international format")
	ErrInvalidCallbackURL            = errors.New("mpesa: callback URL must be a valid URL or IP")
	ErrInvalidReferenceCode          = errors.New("mpesa: reference code cannot be more than 13 characters")
	ErrInvalidTransactionDescription = errors.New("mpesa: transaction description cannot be more than 13 characters")
	ErrInvalidTransactionType        = errors.New("mpesa: invalid transaction type provided")
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

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("mpesa.ReadBody:: %v", err)
	}

	//fmt.Println(fmt.Sprintf("[*] Response Body:: %s", string(body)))
	return body, nil
}

// GenerateAccessToken returns a time bound access token to call allowed APIs.
// This token should be used in all other subsequent requests to the APIs
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
// It will return either production or sandbox
func (m *Mpesa) Environment() Environment {
	return m.environment
}

// isValidURL attempt to check if the value passed is a valid url or string
func isValidURL(s string) (bool, error) {
	rawURL, err := url.ParseRequestURI(s)

	if err != nil {
		return false, err
	}

	address := net.ParseIP(rawURL.Host)

	if address == nil {
		return strings.Contains(rawURL.Host, "."), nil
	}

	return true, nil
}

// validateSTKPushRequest attempt to validate the request before processing it
func (s *STKPushRequest) validateSTKPushRequest() error {
	shortcode := strconv.Itoa(int(s.Shortcode))

	if len(shortcode) < 5 {
		return ErrInvalidBusinessShortCode
	}

	partyB := strconv.Itoa(int(s.PartyB))

	if len(partyB) < 5 {
		return ErrInvalidPartyB
	}

	if len(s.Passkey) == 0 {
		return ErrInvalidPasskey
	}

	if s.Amount <= 0 {
		return ErrInvalidAmount
	}

	phoneNumber := strconv.Itoa(int(s.PhoneNumber))

	if len(phoneNumber) != 12 || phoneNumber[:3] != "254" {
		return ErrInvalidPhoneNumber
	}

	if len(s.CallbackURL) == 0 {
		return ErrInvalidCallbackURL
	}

	// Attempt to validate the callback URL
	isValidURL, err := isValidURL(s.CallbackURL)

	if err != nil {
		return err
	}

	if !isValidURL {
		return ErrInvalidCallbackURL
	}

	if len(s.ReferenceCode) > 13 {
		return ErrInvalidReferenceCode
	}

	if len(s.TransactionDescription) > 13 {
		return ErrInvalidTransactionDescription
	}

	if len(s.TransactionDescription) == 0 {
		s.TransactionDescription = "STK Push Transaction"
	}

	transactionType := s.TransactionType

	if transactionType != "CustomerPayBillOnline" && transactionType != "CustomerBuyGoodsOnline" {
		return ErrInvalidTransactionType
	}

	return nil
}

// generateSTKPushRequestPasswordAndTimestamp returns a base64 encoded password and the current timestamp
func generateSTKPushRequestPasswordAndTimestamp(shortcode uint, passkey string) (string, string) {
	timestamp := time.Now().Format("20060102150405")

	passwordToEncode := fmt.Sprintf("%d%s%s", shortcode, passkey, timestamp)

	encodedPassword := base64.StdEncoding.EncodeToString([]byte(passwordToEncode))

	return encodedPassword, timestamp
}

// getTheTransactionType determines the current transaction type.
// This is made by checking if the shortcode matches the partyB, then this is a paybill transaction.
// If they don't match, then this is a buy goods/till number transaction
//func getTheTransactionType(shortcode, partyB uint) string {
//	if shortcode != partyB {
//		return "CustomerBuyGoodsOnline"
//	}
//
//	return "CustomerPayBillOnline"
//}

// lipaNaMpesaOnlineRequestBody creates the request payload
func (s *STKPushRequest) lipaNaMpesaOnlineRequestBody() ([]byte, error) {
	shortcode := s.Shortcode
	partyB := s.PartyB

	password, timestamp := generateSTKPushRequestPasswordAndTimestamp(shortcode, s.Passkey)

	params := lipaNaMpesaOnlineRequestParameters{
		BusinessShortCode: shortcode,
		Password:          password,
		Timestamp:         timestamp,
		TransactionType:   s.TransactionType,
		Amount:            s.Amount,
		PartyA:            s.PhoneNumber,
		PartyB:            partyB,
		PhoneNumber:       s.PhoneNumber,
		CallBackURL:       s.CallbackURL,
		AccountReference:  s.ReferenceCode,
		TransactionDesc:   s.TransactionDescription,
	}

	requestBody, err := json.Marshal(params)

	if err != nil {
		return nil, err
	}

	return requestBody, nil
}

// LipaNaMpesaOnline makes a request to pay via STk push.
// Returns LipaNaMpesaOnlineRequestResponse and an error if any occurs
// To check if the transaction was successful, use LipaNaMpesaOnlineRequestResponse.IsSuccessful
func (m *Mpesa) LipaNaMpesaOnline(s *STKPushRequest) (*LipaNaMpesaOnlineRequestResponse, error) {
	if err := s.validateSTKPushRequest(); err != nil {
		return nil, err
	}

	requestBody, err := s.lipaNaMpesaOnlineRequestBody()

	if err != nil {
		return nil, fmt.Errorf("mpesa.LipaNaMpesaOnline.CreateRequestBody:: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, m.stkPushURL, bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, fmt.Errorf("mpesa.LipaNaMpesaOnline.CreateNewRequest:: %v", err)
	}

	accessToken, err := m.GenerateAccessToken()

	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Make the request
	resp, err := makeRequest(req)

	if err != nil {
		return nil, err
	}

	var response *LipaNaMpesaOnlineRequestResponse

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("mpesa.LipaNaMpesaOnline.UnmarshalResponse:: %v", err)
	}

	// Set the request as successful by default
	response.IsSuccessful = true

	// Check if the request was processed successfully.
	// We'll determine this if there's no error code
	if response.ErrorCode != "" {
		response.IsSuccessful = false
	}

	return response, nil
}

// certificatePath returns the path to the certificate to be used to encrypt the security credential
func certificatePath(isOnProduction bool) string {
	if !isOnProduction {
		return "./certificates/sandbox.cer"
	}

	return "./certificates/production.cer"
}

// getSecurityCredentials return Base64 encoded string of the B2C short code and password
// which is encrypted using M-Pesa public key and validates the transaction on M-Pesa Core system.
func (b *B2CPaymentRequest) getSecurityCredentials(isOnProduction bool) (string, error) {
	certificate := certificatePath(isOnProduction)

	file, err := os.Open(certificate)

	if err != nil {
		return "", fmt.Errorf("mpesa.getSecurityCredentials.OpenCertificate:: %v", err)
	}

	publicKey, err := ioutil.ReadAll(file)

	if err != nil {
		return "", fmt.Errorf("mpesa.getSecurityCredentials.ReadCertificateContents:: %v", err)
	}

	block, _ := pem.Decode(publicKey)

	var cert *x509.Certificate

	cert, err = x509.ParseCertificate(block.Bytes)

	if err != nil {
		return "", fmt.Errorf("mpesa.getSecurityCredentials.ParseCertificate:: %v", err)
	}

	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)

	reader := rand.Reader

	encrypted, err := rsa.EncryptPKCS1v15(reader, rsaPublicKey, []byte(b.InitiatorPassword))

	if err != nil {
		return "", fmt.Errorf("mpesa.getSecurityCredentials.EncryptCredentials:: %v", err)
	}

	securityCredentials := base64.StdEncoding.EncodeToString(encrypted)
	return securityCredentials, nil
}

// b2cPaymentRequestBody returns payload to be used as the request body
func (b *B2CPaymentRequest) b2cPaymentRequestBody(isOnProduction bool) ([]byte, error) {
	securityCredentials, err := b.getSecurityCredentials(isOnProduction)

	if err != nil {
		return nil, err
	}

	params := b2cPaymentRequestRequestParameters{
		InitiatorName:      b.InitiatorName,
		SecurityCredential: securityCredentials,
		CommandID:          b.CommandID,
		Amount:             b.Amount,
		PartyA:             b.Shortcode,
		PartyB:             b.PhoneNumber,
		Remarks:            b.Remarks,
		QueueTimeOutURL:    b.QueueTimeOutURL,
		ResultURL:          b.ResultURL,
		Occassion:          b.Occasion,
	}

	requestBody, err := json.Marshal(params)

	if err != nil {
		return nil, fmt.Errorf("mpesa.b2cPaymentRequestBody.MarshalRequestBody:: %v", err)
	}

	return requestBody, nil
}

// B2CPayment initiates a B2C request to be used to send money the customer
func (m *Mpesa) B2CPayment(b *B2CPaymentRequest) (*B2CPaymentRequestResponse, error) {
	requestBody, err := b.b2cPaymentRequestBody(m.environment.IsProduction())

	if err != nil {
		return nil, err
	}

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, m.b2cURL, bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, fmt.Errorf("mpesa.B2CPayment.CreateNewRequest:: %v", err)
	}

	// Generate an access token
	accessToken, err := m.GenerateAccessToken()

	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := makeRequest(req)

	if err != nil {
		return nil, err
	}

	response := &B2CPaymentRequestResponse{}

	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("mpesa.B2CPayment.UnmarshalResponse:: %v", err)
	}

	// Set the request as successful by default
	response.IsSuccessful = true

	// Check if the request was processed successfully.
	// We'll determine this if there's no error code
	if response.ErrorCode != "" {
		response.IsSuccessful = false
	}

	return response, nil
}
