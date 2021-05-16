package mpesa

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jwambugu/mpesa-golang-sdk/pkg/config"

	"github.com/patrickmn/go-cache"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
		// IsSuccessful custom field to determine if the request went through
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
	}

	// LipaNaMpesaOnlineCallback is the response sent back sent to the callback URL after making an STKPush request
	LipaNaMpesaOnlineCallback struct {
		Body struct {
			StkCallback struct {
				MerchantRequestID string `json:"MerchantRequestID"`
				CheckoutRequestID string `json:"CheckoutRequestID"`
				ResultCode        int    `json:"ResultCode"`
				ResultDesc        string `json:"ResultDesc"`
				CallbackMetadata  struct {
					Item []struct {
						Name  string      `json:"Name"`
						Value interface{} `json:"Value,omitempty"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
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

	endpoint := fmt.Sprintf("%s/oauth/v1/generate?grant_type=client_credentials", m.BaseURL)

	// Create a new http request
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)

	if err != nil {
		return "", fmt.Errorf("mpesa.GetAccessToken.NewRequest:: %v", err)
	}

	// Set the basic auth header
	req.SetBasicAuth(m.ConsumerKey, m.ConsumerSecret)

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
func getTheTransactionType(shortcode, partyB uint) string {
	if shortcode != partyB {
		return "CustomerBuyGoodsOnline"
	}

	return "CustomerPayBillOnline"
}

// lipaNaMpesaOnlineRequestBody creates the request payload
func (s *STKPushRequest) lipaNaMpesaOnlineRequestBody() ([]byte, error) {
	shortcode := s.Shortcode
	partyB := s.PartyB

	password, timestamp := generateSTKPushRequestPasswordAndTimestamp(shortcode, s.Passkey)

	transactionType := getTheTransactionType(shortcode, partyB)

	params := lipaNaMpesaOnlineRequestParameters{
		BusinessShortCode: shortcode,
		Password:          password,
		Timestamp:         timestamp,
		TransactionType:   transactionType,
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

	endpoint := fmt.Sprintf("%s/mpesa/stkpush/v1/processrequest", m.BaseURL)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(requestBody))

	if err != nil {
		return nil, fmt.Errorf("mpesa.LipaNaMpesaOnline.CreateNewRequest:: %v", err)
	}

	accessToken, err := m.GetAccessToken()

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

	// Set the transaction as successful by default
	response.IsSuccessful = true

	// Check if the request was processed successfully.
	// We'll determine this if there's no error code
	if response.ErrorCode != "" {
		response.IsSuccessful = false
	}

	return response, nil
}
