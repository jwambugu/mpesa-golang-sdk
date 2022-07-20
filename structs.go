package mpesa

import "time"

type (
	// AuthorizationResponse is returned when trying to authenticate the app using provided credentials
	AuthorizationResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`

		// Internal fields
		setAt time.Time
	}

	// STKPushRequest represents the data to be provided by the user for LipaNaMpesaOnlineRequestParameters
	STKPushRequest struct {
		// BusinessShortCode is organizations shortcode (Paybill or Buy goods - A 5 to 7-digit account number) used to
		// identify an organization and receive the transaction.
		BusinessShortCode string `json:"BusinessShortCode"`

		// Password is a base64 encoded string used for encrypting the request sent which is a combination of
		// BusinessShortCode + Passkey + Timestamp
		Password string `json:"Password"`

		// Timestamp of the transaction in the format of YEAR+MONTH+DATE+HOUR+MINUTE+SECOND (YYYYMMDDHHmmss).
		// Each part should be at least two digits apart from the year which takes four digits.
		// Example 20060102150405
		Timestamp string `json:"Timestamp"`

		// TransactionType identifies the transaction when sending the request to M-Pesa. Expects CustomerPayBillOnline
		// or CustomerBuyGoodsOnline
		TransactionType string `json:"TransactionType"`

		// Amount to be transacted which will be deducted from the customer.
		Amount uint64 `json:"Amount,omitempty"`

		// PartyA is phone number sending money. The parameter expected is a valid Safaricom Mobile Number that is
		// M-Pesa registered in the format 2547XXXXXXXX
		PartyA uint64 `json:"PartyA"`

		// PartyB is the organization receiving the funds. The parameter expected is a 5 to 7 digit as defined on
		// the Shortcode description which can also be the same as BusinessShortCode value.
		PartyB string `json:"PartyB,omitempty"`

		// PhoneNumber to receive the STK Pin Prompt which can be same as PartyA value.
		PhoneNumber uint64 `json:"PhoneNumber,omitempty"`

		// CallbackURL is a valid secure URL that is used to receive notifications from M-Pesa API. It is the endpoint
		// to which the results will be sent by M-Pesa API.
		CallBackURL string `json:"CallBackURL"`

		// AccountReference is parameter that is defined by your system as an identifier of the transaction for
		// CustomerPayBillOnline transaction type. Along with the business name, this value is also displayed to the
		// customer in the STK Pin Prompt message and must be maximum of 12 characters.
		AccountReference string `json:"AccountReference"`

		// TransactionDesc is any additional information/comment that can be sent along with the request from your
		// system with a maximum of 13 Characters.
		TransactionDesc string `json:"TransactionDesc"`
	}

	// STKPushRequestResponse is the response sent back by mpesa after initiating making STKPushRequest.
	STKPushRequestResponse struct {
		// MerchantRequestID is a global unique Identifier for any submitted payment request. Example: 16813-1590513-1
		MerchantRequestID string `json:"MerchantRequestID,omitempty"`

		// CheckoutRequestID is a global unique identifier of the processed checkout transaction request.
		// Example: ws_CO_DMZ_12321_23423476
		CheckoutRequestID string `json:"CheckoutRequestID,omitempty"`

		// ResponseDescription is an acknowledgment message from the API that gives the status of the request submission.
		// It usually maps to a specific ResponseCode value which can be a success message or an error description.
		ResponseDescription string `json:"ResponseDescription,omitempty"`

		// ResponseCode is a numeric status code that indicates the status of the transaction submission.
		// 0 means successful submission and any other code means an error occurred.
		ResponseCode string `json:"ResponseCode,omitempty"`

		// CustomerMessage is a message that your system can display to the Customer as an acknowledgement of the
		// payment request submission. Example: Success. Request accepted for processing.
		CustomerMessage string `json:"CustomerMessage,omitempty"`

		// Error Responses:

		// RequestID is a unique request ID for the payment request
		RequestID string `json:"requestId,omitempty"`

		// ErrorCode is a predefined code that indicates the reason for request failure that is defined in the
		// ErrorMessage. The error codes maps to specific error message.
		ErrorCode string `json:"errorCode,omitempty"`

		// ErrorMessage is a short descriptive message of the failure reason.
		ErrorMessage string `json:"errorMessage,omitempty"`
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
)
