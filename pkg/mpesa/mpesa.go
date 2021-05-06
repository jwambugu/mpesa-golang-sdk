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

	// mpesaAccessTokenResponse is the response sent back by Safaricom when we make a request to generate a token
	// for a specific app
	mpesaAccessTokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    string `json:"expires_in"`
		RequestID    string `json:"requestId"`
		ErrorCode    string `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}

	// LipaNaMpesaOnlineRequestParameters has the parameters used initiate online payment on behalf of a customer.
	LipaNaMpesaOnlineRequestParameters struct {
		// This is organizations shortcode (Paybill or Buygoods - A 5 to 7 digit account number) used to identify
		// an organization and receive the transaction.
		// Example Shortcode (5 to 7 digits) e.g. 654321
		BusinessShortCode uint
		// 	This is the password used for encrypting the request sent: A base64 encoded string.
		//	The base64 string is a combination of Shortcode+Passkey+Timestamp
		Password string
		// This is the Timestamp of the transaction in the format of YEAR+MONTH+DATE+HOUR+MINUTE+SECOND (YYYYMMDDHHmmss).
		// Each part should be at least two digits apart from the year which takes four digits.
		// Example 20060102150405
		Timestamp string
		// This is the transaction type that is used to identify the transaction when sending the request to M-Pesa.
		// The transaction type for M-Pesa Express is "CustomerPayBillOnline"
		// Accepted values are CustomerPayBillOnline or CustomerBuyGoodsOnline
		TransactionType string
		// This is the Amount transacted normally a numeric value.
		// Money that customer pays to the Shortcode. Only whole numbers are supported.
		Amount uint64
		// The phone number sending money. The parameter expected is a Valid Safaricom Mobile Number that is M-Pesa
		// registered in the format 254XXXXXXXXX
		PartyA uint64
		// The organization receiving the funds. The parameter expected is a 5 to 7 digit as defined on the Shortcode
		// description above. This can be the same as BusinessShortCode value above.
		PartyB uint
		// The Mobile Number to receive the STK Pin Prompt. This number can be the same as PartyA value above.
		// Expected format is 254XXXXXXXXX
		PhoneNumber uint64
		// A CallBack URL is a valid secure URL that is used to receive notifications from M-Pesa API.
		// It is the endpoint to which the results will be sent by M-Pesa API.
		// Example https://ip or domain:port/path (https://mydomain.com/path, https://0.0.0.0:9090/path)
		CallBackURL string
		// This is an Alpha-Numeric parameter that is defined by your system as an Identifier of the transaction for
		// CustomerPayBillOnline transaction type. Along with the business name, this value is also displayed to the
		// customer in the STK Pin Prompt message. Maximum of 12 characters.
		AccountReference string
		// This is any additional information/comment that can be sent along with the request from your system.
		// Maximum of 13 Characters.
		TransactionDesc string
	}

	// LipaNaMpesaOnlineRequestResponse is the response sent back by mpesa after initiating an STK push request.
	LipaNaMpesaOnlineRequestResponse struct {
		// This is a global unique Identifier for any submitted payment request.
		// Sample value 16813-1590513-1
		MerchantRequestID string `json:"MerchantRequestID"`
		// This is a global unique identifier of the processed checkout transaction request.
		// Sample value ws_CO_DMZ_12321_23423476
		CheckoutRequestID string `json:"CheckoutRequestID"`
		// Response description is an acknowledgment message from the API that gives the status of the request.
		// It usually maps to a specific ResponseCode value.
		// It can be a Success submission message or an error description.
		// Examples are :
		// - The service request has failed
		// - The service request has been accepted successfully.
		// - Invalid Access Token.
		ResponseDescription string `json:"ResponseDescription"`
		// This is a Numeric status code that indicates the status of the transaction submission.
		// 0 means successful submission and any other code means an error occurred.
		ResponseCode uint `json:"ResponseCode"`
		// This is a message that your system can display to the Customer as an acknowledgement of the payment
		// request submission. Example Success. Request accepted for processing.
		CustomerMessage string `json:"CustomerMessage"`
		// This is a unique requestID for the payment request
		RequestID string `json:"requestId"`
		// This is a predefined code that indicates the reason for request failure.
		// This is defined in the Response Error Details below.
		// The error codes maps to specific error message as illustrated in the Response Error Details below.
		ErrorCode string `json:"errorCode"`
		// This is a short descriptive message of the failure reason.
		ErrorMessage string `json:"errorMessage"`
	}

	// STKPushRequest represents the data to be provided by the user for LipaNaMpesaOnlineRequestParameters
	STKPushRequest struct {
		// Paybill for the organisation
		Shortcode uint
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

func (m *Mpesa) LipaNaMpesaOnline(s STKPushRequest) (LipaNaMpesaOnlineRequestResponse, error) {
	return LipaNaMpesaOnlineRequestResponse{}, nil
}
