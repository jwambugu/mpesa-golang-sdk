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
		// request submission. Example Success. MockRequest accepted for processing.
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

	// B2CPaymentRequest is the data to be used to make B2C Payment MockRequest
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
