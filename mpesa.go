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
	"github.com/patrickmn/go-cache"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Environment string

const (
	Sandbox    Environment = "sandbox"
	Production Environment = "production"
)

// IsProduction returns true if the current env is set to production.
func (e Environment) IsProduction() bool {
	return e == Production
}

type (
	// Mpesa is an app to make a transaction
	Mpesa struct {
		consumerKey    string
		consumerSecret string
		baseURL        string
		environment    Environment
		cache          *cache.Cache

		client http.Client
	}

	// mpesaAccessTokenResponse is the response sent back by Safaricom when we make a request to generate a token
	// for a specific app
	mpesaAccessTokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    string `json:"expires_in"`
		RequestID    string `json:"requestId"`
		ErrorCode    string `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}

	// lipaNaMpesaOnlineRequestParameters has the parameters used initiate online payment on behalf of a customer.
	lipaNaMpesaOnlineRequestParameters struct {
		// This is organizations shortcode (Paybill or Buygoods - A 5 to 7 digit account number) used to identify
		// an organization and receive the transaction.
		// Example Shortcode (5 to 7 digits) e.g. 654321
		BusinessShortCode uint `json:"BusinessShortCode"`
		// 	This is the password used for encrypting the request sent: A base64 encoded string.
		//	The base64 string is a combination of Shortcode+Passkey+Timestamp
		Password string `json:"Password"`
		// This is the Timestamp of the transaction in the format of YEAR+MONTH+DATE+HOUR+MINUTE+SECOND (YYYYMMDDHHmmss).
		// Each part should be at least two digits apart from the year which takes four digits.
		// Example 20060102150405
		Timestamp string `json:"Timestamp"`
		// This is the transaction type that is used to identify the transaction when sending the request to M-Pesa.
		// The transaction type for M-Pesa Express is "CustomerPayBillOnline"
		// Accepted values are CustomerPayBillOnline or CustomerBuyGoodsOnline
		TransactionType string `json:"TransactionType"`
		// This is the Amount transacted normally a numeric value.
		// Money that customer pays to the Shortcode. Only whole numbers are supported.
		Amount uint64 `json:"Amount"`
		// The phone number sending money. The parameter expected is a Valid Safaricom Mobile Number that is M-Pesa
		// registered in the format 254XXXXXXXXX
		PartyA uint64 `json:"PartyA"`
		// The organization receiving the funds. The parameter expected is a 5 to 7 digit as defined on the Shortcode
		// description above. This can be the same as BusinessShortCode value above.
		PartyB uint `json:"PartyB"`
		// The Mobile Number to receive the STK Pin Prompt. This number can be the same as PartyA value above.
		// Expected format is 254XXXXXXXXX
		PhoneNumber uint64 `json:"PhoneNumber"`
		// A CallBack URL is a valid secure URL that is used to receive notifications from M-Pesa API.
		// It is the endpoint to which the results will be sent by M-Pesa API.
		// Example https://ip or domain:port/path (https://mydomain.com/path, https://0.0.0.0:9090/path)
		CallBackURL string `json:"CallBackURL"`
		// This is an Alpha-Numeric parameter that is defined by your system as an Identifier of the transaction for
		// CustomerPayBillOnline transaction type. Along with the business name, this value is also displayed to the
		// customer in the STK Pin Prompt message. Maximum of 12 characters.
		AccountReference string `json:"AccountReference"`
		// This is any additional information/comment that can be sent along with the request from your system.
		// Maximum of 13 Characters.
		TransactionDesc string `json:"TransactionDesc"`
	}

	// LipaNaMpesaOnlineRequestResponse is the response sent back by mpesa after initiating an STK push request.
	LipaNaMpesaOnlineRequestResponse struct {
		// This is a global unique Identifier for any submitted payment request.
		// Sample value 16813-1590513-1
		MerchantRequestID string `json:"MerchantRequestID,omitempty"`
		// This is a global unique identifier of the processed checkout transaction request.
		// Sample value ws_CO_DMZ_12321_23423476
		CheckoutRequestID string `json:"CheckoutRequestID,omitempty"`
		// Response description is an acknowledgment message from the API that gives the status of the request.
		// It usually maps to a specific ResponseCode value.
		// It can be a Success submission message or an error description.
		// Examples are :
		// - The service request has failed
		// - The service request has been accepted successfully.
		// - Invalid Access Token.
		ResponseDescription string `json:"ResponseDescription,omitempty"`
		// This is a Numeric status code that indicates the status of the transaction submission.
		// 0 means successful submission and any other code means an error occurred.
		ResponseCode string `json:"ResponseCode,omitempty"`
		// This is a message that your system can display to the Customer as an acknowledgement of the payment
		// request submission. Example Success. Request accepted for processing.
		CustomerMessage string `json:"CustomerMessage,omitempty"`
		// This is a unique requestID for the payment request
		RequestID string `json:"requestId,omitempty"`
		// This is a predefined code that indicates the reason for request failure.
		// This is defined in the Response Error Details below.
		// The error codes maps to specific error message as illustrated in the Response Error Details below.
		ErrorCode string `json:"errorCode,omitempty"`
		// This is a short descriptive message of the failure reason.
		ErrorMessage string `json:"errorMessage,omitempty"`
		// IsSuccessful custom field to determine if the request was processed successfully
		// without any errors
		IsSuccessful bool
	}

	// STKPushRequest represents the data to be provided by the user for LipaNaMpesaOnlineRequestParameters
	STKPushRequest struct {
		// Paybill for the organisation
		Shortcode uint
		// Organization receiving funds. Can be same as Shortcode but different in case of till numbers.
		PartyB uint
		// This is key shared by safaricom after going live.
		Passkey string
		// Amount to be transacted. This is will be deducted from the customer.
		Amount uint64
		// The PhoneNumber to receive the STK Pin Prompt
		PhoneNumber uint64
		// An identifier for the transaction.
		ReferenceCode string
		// Endpoint to send the payment notifications.
		CallbackURL string
		// Any additional information/comment that can be sent along with the request.
		TransactionDescription string
		// This is the type of transaction to be performed. Expects CustomerPayBillOnline or CustomerBuyGoodsOnline
		TransactionType string
	}

	// LipaNaMpesaOnlineCallback is the response sent back sent to the callback URL after making an STKPush request
	LipaNaMpesaOnlineCallback struct {
		// This is the root key for the entire Callback message.
		Body struct {
			// This is the first child of the Body.
			StkCallback struct {
				// This is a global unique Identifier for any submitted payment request.
				// This is the same value returned in the acknowledgement message of the initial request.
				MerchantRequestID string `json:"MerchantRequestID"`
				// This is a global unique identifier of the processed checkout transaction request.
				// This is the same value returned in the acknowledgement message of the initial request.
				CheckoutRequestID string `json:"CheckoutRequestID"`
				// This is a numeric status code that indicates the status of the transaction processing.
				// 0 means successful processing and any other code means an error occurred or the transaction failed.
				ResultCode int `json:"ResultCode"`
				// Result description is a message from the API that gives the status of the request processing,
				// usually maps to a specific ResultCode value.
				// It can be a Success processing message or an error description message.
				ResultDesc string `json:"ResultDesc"`
				// This is the JSON object that holds more details for the transaction.
				// It is only returned for Successful transaction.
				CallbackMetadata struct {
					// This is a JSON Array, within the CallbackMetadata, that holds additional transaction details in
					// JSON objects. Since this array is returned under the CallbackMetadata,
					// it is only returned for Successful transaction.
					Item []struct {
						Name  string      `json:"Name"`
						Value interface{} `json:"Value,omitempty"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}

	// b2cPaymentRequestRequestParameters are the parameters for the B2C API used to transact between an M-Pesa short
	// code to a phone number registered on M-Pesa.
	b2cPaymentRequestRequestParameters struct {
		// The username of the M-Pesa B2C account API operator.
		InitiatorName string `json:"InitiatorName"`
		// This is the value obtained after encrypting the API initiator password.
		SecurityCredential string `json:"SecurityCredential"`
		// This is a unique command that specifies B2C transaction type.
		// Possible values are:
		// 1. SalaryPayment: This supports sending money to both registered and unregistered M-Pesa customers.
		//2. BusinessPayment: This is a normal business to customer payment,  supports only M-Pesa registered customers.
		//3. PromotionPayment: This is a promotional payment to customers.
		// The M-Pesa notification message is a congratulatory message. Supports only M-Pesa registered customers.
		CommandID string `json:"CommandID"`
		// The amount of money being sent to the customer.
		Amount uint64
		// This is the B2C organization shortcode from which the money is to be sent.
		PartyA uint
		// This is the customer mobile number  to receive the amount.
		// The number should have the country code (254) without the plus sign.
		PartyB uint64
		// Any additional information to be associated with the transaction. Sentence of upto 100 characters
		Remarks string
		// This is the URL to be specified in your request that will be used by API Proxy to send notification in case
		// the payment request is timed out while awaiting processing in the queue.
		QueueTimeOutURL string
		// This is the URL to be specified in your request that will be used by M-Pesa to send notification upon
		// processing of the payment request.
		ResultURL string
		// Any additional information to be associated with the transaction. Sentence of upto 100 characters
		Occassion string
	}

	// B2CPaymentRequest is the data to be used to make B2C Payment Request
	B2CPaymentRequest struct {
		// The username of the M-Pesa B2C account API operator.
		InitiatorName string
		// The password of the M-Pesa B2C account API operator.
		InitiatorPassword string
		// This is a unique command that specifies B2C transaction type.
		CommandID string
		// The amount of money being sent to the customer.
		Amount uint64
		// This is the B2C organization shortcode from which the money is to be sent.
		Shortcode uint
		// This is the customer mobile number  to receive the amount.
		// The number should have the country code (254) without the plus sign.
		PhoneNumber uint64
		// Any additional information to be associated with the transaction. Sentence of upto 100 characters
		Remarks string
		// This is the URL to be specified in your request that will be used by API Proxy to send notification in case
		// the payment request is timed out while awaiting processing in the queue.
		QueueTimeOutURL string
		// This is the URL to be specified in your request that will be used by M-Pesa to send notification upon
		// processing of the payment request.
		ResultURL string
		// Any additional information to be associated with the transaction. Sentence of upto 100 characters
		Occasion string
	}

	// B2CPaymentRequestResponse is the response sent back by mpesa after making a B2CPaymentRequest
	B2CPaymentRequestResponse struct {
		// This is a global unique identifier for the transaction request returned by the API
		// proxy upon successful request submission. Sample value AG_2376487236_126732989KJHJKH
		OriginatorConversationId string `json:"OriginatorConversationId,omitempty"`
		// This is a global unique identifier for the transaction request returned by the M-Pesa upon successful
		// request submission. Sample value 236543-276372-2
		ConversationId string `json:"ConversationId,omitempty"`
		// This is the description of the request submission status.
		// Sample - Accept the service request successfully
		ResponseDescription string `json:"ResponseDescription,omitempty"`
		// This is a unique requestID for the payment request
		RequestID string `json:"requestId,omitempty"`
		// This is a predefined code that indicates the reason for request failure.
		// This is defined in the Response Error Details below.
		// The error codes maps to specific error message as illustrated in the Response Error Details below.
		ErrorCode string `json:"errorCode,omitempty"`
		// This is a short descriptive message of the failure reason.
		ErrorMessage string `json:"errorMessage,omitempty"`
		// IsSuccessful custom field to determine if the request was processed successfully
		// without any errors
		IsSuccessful bool
	}

	// B2CPaymentRequestCallback this is a payload sent to the callback URL after making a B2CPaymentRequest
	B2CPaymentRequestCallback struct {
		// This is the root parameter that encloses the entire result message.
		Result struct {
			// This is a status code that indicates whether the transaction was already sent to your listener.
			// Usual value is 0.
			ResultType int `json:"ResultType"`
			// This is a numeric status code that indicates the status of the transaction processing.
			// 0 means success and any other code means an error occurred or the transaction failed.
			ResultCode int `json:"ResultCode"`
			// This is a message from the API that gives the status of the request processing and usually maps
			// to a specific result code value.
			// Samples are - Service request is has bee accepted successfully
			//				- Initiator information is invalid
			ResultDesc string `json:"ResultDesc"`
			// This is a global unique identifier for the transaction request returned by the API proxy upon
			// successful request submission.
			OriginatorConversationID string `json:"OriginatorConversationID"`
			// This is a global unique identifier for the transaction request returned by the M-Pesa upon
			// successful request submission.
			ConversationID string `json:"ConversationID"`
			// This is a unique M-PESA transaction ID for every payment request.
			// Same value is sent to customer over SMS upon successful processing.
			TransactionID string `json:"TransactionID"`
			// This is a JSON object that holds more details for the transaction.
			ResultParameters struct {
				// This is a JSON array within the ResultParameters that holds additional transaction details as
				// JSON objects.
				ResultParameter []struct {
					Key   string      `json:"Key"`
					Value interface{} `json:"Value"`
				} `json:"ResultParameter"`
			} `json:"ResultParameters"`
			ReferenceData struct {
				ReferenceItem struct {
					Key   string `json:"Key"`
					Value string `json:"Value"`
				} `json:"ReferenceItem"`
			} `json:"ReferenceData"`
		} `json:"Result"`
	}
)

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
	SandboxBaseURL    = "https://sandbox.safaricom.co.ke"
	ProductionBaseURL = "https://api.safaricom.co.ke"
)

// NewApp initializes a new Mpesa app that will be used to perform C2B or B2C transaction
func NewApp(c *http.Client, consumerKey, consumerSecret string, env Environment) *Mpesa {
	if c == nil {
		c = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	baseUrl := SandboxBaseURL
	if env == Production {
		baseUrl = ProductionBaseURL
	}

	newCache := cache.New(55*time.Minute, 10*time.Minute)

	return &Mpesa{
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		baseURL:        baseUrl,
		environment:    env,
		cache:          newCache,
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

// cachedAccessToken returns the cached access token
func (m *Mpesa) cachedAccessToken() (interface{}, bool) {
	return m.cache.Get(m.consumerKey)
}

// getAccessToken returns a token to be used to authenticate an app.
// This token should be used in all other subsequent requests to the APIs
// getAccessToken will also cache the access token for 55 minutes.
func (m *Mpesa) getAccessToken() (string, error) {
	cachedToken, exists := m.cachedAccessToken()

	if exists {
		return cachedToken.(string), nil
	}

	endpoint := fmt.Sprintf("%s/oauth/v1/generate?grant_type=client_credentials", m.baseURL)

	// Create a new http request
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)

	if err != nil {
		return "", fmt.Errorf("mpesa.GetAccessToken.NewRequest:: %v", err)
	}

	// Set the basic auth header
	req.SetBasicAuth(m.consumerKey, m.consumerSecret)

	resp, err := makeRequest(req)

	if err != nil {
		return "", err
	}

	var response mpesaAccessTokenResponse

	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("mpesa.GetAccessToken.UnmarshalResponse:: %v", err)
	}

	// Check if the authentication passed. If it did, we won't have any error code
	if response.ErrorCode != "" {
		return "", fmt.Errorf("mpesa.GetAccessToken.MpesaResponse:: %v", response.ErrorMessage)
	}

	token := response.AccessToken

	m.cache.Set(m.consumerKey, token, 55*time.Minute)

	return token, nil
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

	endpoint := fmt.Sprintf("%s/mpesa/stkpush/v1/processrequest", m.baseURL)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, fmt.Errorf("mpesa.LipaNaMpesaOnline.CreateNewRequest:: %v", err)
	}

	accessToken, err := m.getAccessToken()

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

	endpoint := fmt.Sprintf("%s/mpesa/b2c/v1/paymentrequest", m.baseURL)

	// Create a new request
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, fmt.Errorf("mpesa.B2CPayment.CreateNewRequest:: %v", err)
	}

	// Generate an access token
	accessToken, err := m.getAccessToken()

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
