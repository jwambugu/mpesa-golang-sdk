package mpesa

import "time"

// DynamicQRTransactionType represents the supported transaction types for the Dynamic QR API
type DynamicQRTransactionType string

const (
	PayMerchantBuyGoods      DynamicQRTransactionType = "BG"
	PaybillOrBusinessNumber  DynamicQRTransactionType = "PB"
	SendMoneyViaMobileNumber DynamicQRTransactionType = "SM"
	SentToBusiness           DynamicQRTransactionType = "SB"
	WithdrawCashAtAgentTill  DynamicQRTransactionType = "WA"
)

// CommandID is a unique command that specifies B2C transaction type.
type CommandID string

const (
	// AccountBalanceCommandID is applied when getting the account balance of a shortcode
	AccountBalanceCommandID CommandID = "AccountBalance"

	// BusinessPayBillCommandID is applied for BusinessPayBillRequest
	BusinessPayBillCommandID CommandID = "BusinessPayBill"

	// BusinessPaymentCommandID is a normal business to customer payment, supports only M-PESA registered customers.
	BusinessPaymentCommandID CommandID = "BusinessPayment"

	// PromotionPaymentCommandID is a promotional payment to customers. The M-PESA notification message is a congratulatory
	// message. Supports only M-PESA registered customers.
	PromotionPaymentCommandID CommandID = "PromotionPayment"

	// SalaryPaymentCommandID is used for sending money to both registered and unregistered M-Pesa customers.
	SalaryPaymentCommandID CommandID = "SalaryPayment"

	// TransactionStatusQueryCommandID is applied when getting the status of a transaction.
	TransactionStatusQueryCommandID CommandID = "TransactionStatusQuery"
)

// IdentifierType is the type of organization receiving the transaction
type IdentifierType uint8

const ShortcodeIdentifierType IdentifierType = 4

// TransactionType is used ti identify the type of the transaction being made.
type TransactionType string

const (
	// CustomerBuyGoodsOnlineTransactionType us used to STK push requests for till numbers.
	CustomerBuyGoodsOnlineTransactionType = "CustomerBuyGoodsOnline"

	// CustomerPayBillOnlineTransactionType us used to STK push requests for paybill numbers.
	CustomerPayBillOnlineTransactionType = "CustomerPayBillOnline"
)

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
		BusinessShortCode uint `json:"BusinessShortCode"`

		// Password is a base64 encoded string used for encrypting the request sent which is a combination of
		// BusinessShortCode + Passkey + Timestamp
		Password string `json:"Password"`

		// Timestamp of the transaction in the format of YEAR+MONTH+DATE+HOUR+MINUTE+SECOND (YYYYMMDDHHmmss).
		// Each part should be at least two digits apart from the year which takes four digits.
		// Example 20060102150405
		Timestamp string `json:"Timestamp"`

		// TransactionType identifies the transaction when sending the request to M-Pesa. Expects CustomerPayBillOnline
		// or CustomerBuyGoodsOnline
		TransactionType TransactionType `json:"TransactionType"`

		// Amount to be transacted which will be deducted from the customer.
		Amount uint `json:"Amount,omitempty"`

		// PartyA is phone number sending money. The parameter expected is a valid Safaricom Mobile Number that is
		// M-Pesa registered in the format 2547XXXXXXXX
		PartyA uint `json:"PartyA"`

		// PartyB is the organization receiving the funds. The parameter expected is a 5 to 7 digit as defined on
		// the Shortcode description which can also be the same as BusinessShortCode value.
		PartyB uint `json:"PartyB"`

		// PhoneNumber to receive the STK Pin Prompt which can be same as PartyA value.
		PhoneNumber uint64 `json:"PhoneNumber"`

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

	// Response is the response sent back by mpesa after initiating a request.
	Response struct {
		// CheckoutRequestID is a global unique identifier of the processed checkout transaction request.
		// Example: ws_CO_DMZ_12321_23423476
		CheckoutRequestID string `json:"CheckoutRequestID,omitempty"`

		// ConversationID is a global unique identifier for the transaction request returned by the M-Pesa upon successful
		// request submission.
		ConversationID string `json:"ConversationID,omitempty"`

		// CustomerMessage is a message that your system can display to the Customer as an acknowledgement of the
		// payment request submission. Example: Success. Request accepted for processing.
		CustomerMessage string `json:"CustomerMessage,omitempty"`

		// ErrorCode is a predefined code that indicates the reason for request failure that is defined in the
		// ErrorMessage. The error codes maps to specific error message.
		ErrorCode string `json:"errorCode,omitempty"`

		// ErrorMessage is a short descriptive message of the failure reason.
		ErrorMessage string `json:"errorMessage,omitempty"`

		// MerchantRequestID is a global unique Identifier for any submitted payment request. Example: 16813-1590513-1
		MerchantRequestID string `json:"MerchantRequestID,omitempty"`

		// OriginatorConversationID is a global unique identifier for the transaction request returned by the API proxy
		// upon successful request submission.
		OriginatorConversationID string `json:"OriginatorConversationID,omitempty"`

		// ResponseCode is a numeric status code that indicates the status of the transaction submission.
		// 0 means successful submission and any other code means an error occurred.
		ResponseCode string `json:"ResponseCode,omitempty"`

		// ResponseDescription is an acknowledgment message from the API that gives the status of the request submission.
		// It usually maps to a specific ResponseCode value which can be a success message or an error description.
		ResponseDescription string `json:"ResponseDescription,omitempty"`

		// ResultCode is a numeric status code that indicates the status of the transaction processing.
		// 0 means successful processing and any other code means an error occured or the transaction failed.
		ResultCode string `json:"ResultCode,omitempty"`

		// ResultDesc description is a message from the API that gives the status of the request processing, usualy maps
		// to a specific ResultCode value. It can be a success or an error description message.
		ResultDesc string `json:"ResultDesc,omitempty"`

		// RequestID is a unique request ID for the payment request
		RequestID string `json:"requestId,omitempty"`
	}

	STKCallbackItem struct {
		Name  string      `json:"Name"`
		Value interface{} `json:"Value,omitempty"`
	}

	STKCallbackMetadata struct {
		// Item is a JSON Array, within the CallbackMetadata, that holds additional transaction details in
		// JSON objects. It is only returned for Successful transaction as part of CallbackMetadata
		Item []STKCallbackItem `json:"Item"`
	}

	STKCallback struct {
		// MerchantRequestID is a global unique Identifier for any submitted payment request. It is the same
		// value returned to the acknowledgement message on the Response.
		MerchantRequestID string `json:"MerchantRequestID"`

		// CheckoutRequestID is a global unique identifier of the processed checkout transaction request.
		// It is the same value returned to the acknowledgement message on the Response.
		CheckoutRequestID string `json:"CheckoutRequestID"`

		// ResultCode is a numeric status code that indicates the status of the transaction processing.
		// 0 means successful processing and any other code means an error occurred or the transaction failed.
		ResultCode int `json:"ResultCode"`

		// ResultDesc is a message from the API that gives the status of the request processing. It usually maps
		// to a specific ResultCode value. It can be a success or an error description message.
		ResultDesc string `json:"ResultDesc"`

		// CallbackMetadata is the JSON object that holds more details for the transaction.
		// It is only returned for successful transaction.
		//
		// Successful transaction contains this sample payload:
		//
		// {
		//   "CallbackMetadata":{
		//      "Item":[
		//         {
		//            "Name":"Amount",
		//            "Value":1.00
		//         },
		//         {
		//            "Name":"MpesaReceiptNumber",
		//            "Value":"NLJ7RT61SV"
		//         },
		//         {
		//            "Name":"TransactionDate",
		//            "Value":20191219102115
		//         },
		//         {
		//            "Name":"PhoneNumber",
		//            "Value":254708374149
		//         }
		//      ]
		//   }
		// }
		//
		CallbackMetadata STKCallbackMetadata `json:"CallbackMetadata"`
	}

	STKPushCallbackBody struct {
		// STKCallback stores the data related to the request.
		STKCallback STKCallback `json:"stkCallback"`
	}

	// STKPushCallback is the response sent back sent to the callback URL after making the STKPushRequest
	STKPushCallback struct {
		// Body is the root key for the entire callback message.
		Body STKPushCallbackBody `json:"Body"`
	}

	B2CRequest struct {
		// InitiatorName is the username of the M-Pesa B2C account API operator. The access channel for this operator
		// must be API and the account must be in active status.
		InitiatorName string `json:"InitiatorName"`

		// SecurityCredential is the value obtained after encrypting the API initiator password.
		SecurityCredential string `json:"SecurityCredential"`

		/*
			CommandID is a unique command that specifies B2C transaction type.
				- SalaryPaymentCommandID: This supports sending money to both registered and unregistered M-Pesa customers.
				- BusinessPaymentCommandID: This is a normal business to customer payment,supports only M-Pesa registered customers.
				- PromotionPaymentCommandID: This is a promotional payment to customers. The M-Pesa notification message is a
				congratulatory message and supports only M-Pesa registered customers.
		*/
		CommandID CommandID `json:"CommandID"`

		// Amount to be sent to the customer.
		Amount uint `json:"Amount"`

		// PartyA is the B2C organization shortcode from which the money is to be from.
		PartyA uint `json:"PartyA"`

		// PartyB is the customer mobile number to receive the amount which should have the country code (254).
		PartyB uint64 `json:"PartyB"`

		// Remarks represents any additional information to be associated with the transaction.
		Remarks string `json:"Remarks"`

		// QueueTimeOutURL is the URL to be specified in your request that will be used by API Proxy to send
		// notification in-case the payment request is timed out while awaiting processing in the queue.
		QueueTimeOutURL string `json:"QueueTimeOutURL"`

		// ResultURL is the URL to be specified in your request that will be used by M-Pesa to send notification upon
		// processing of the payment request.
		ResultURL string `json:"ResultURL"`

		// Occasion is any additional information to be associated with the transaction.
		Occasion string `json:"Occasion"`
	}

	// ResultParameter holds additional transaction details.
	// Details available:
	// 1:
	ResultParameter struct {
		Key   string      `json:"Key"`
		Value interface{} `json:"Value"`
	}

	ResultParameters struct {
		// ResultParameter is a JSON array within the ResultParameters.
		ResultParameter []ResultParameter `json:"ResultParameter"`
	}

	ReferenceItem struct {
		Key   string `json:"Key"`
		Value string `json:"Value"`
	}

	ReferenceData struct {
		ReferenceItem ReferenceItem `json:"ReferenceItem"`
	}

	CallbackResult struct {
		// ConversationID is a global unique identifier for the transaction request returned by the M-Pesa
		// upon successful request submission.
		ConversationID string `json:"ConversationID"`

		// OriginatorConversationID is a global unique identifier for the transaction request returned by the API
		// proxy upon successful request submission.
		OriginatorConversationID string `json:"OriginatorConversationID"`

		ReferenceData ReferenceData `json:"ReferenceData"`

		// ResultCode is a numeric status code that indicates the status of the transaction processing.
		// 0 means success and any other code means an error occurred or the transaction failed.
		ResultCode int `json:"ResultCode"`

		// ResultDesc is a message from the API that gives the status of the request processing and usually maps to
		// a specific ResultCode value.
		ResultDesc string `json:"ResultDesc"`

		// ResultParameters is a JSON object that holds more details for the transaction.
		ResultParameters ResultParameters `json:"ResultParameters"`

		// ResultType is a status code that indicates whether the transaction was already sent to your listener.
		// Usual value is 0.
		ResultType int `json:"ResultType"`

		// TransactionID is a unique M-PESA transaction ID for every payment request. Same value is sent to customer
		// over SMS upon successful processing.
		TransactionID string `json:"TransactionID"`
	}

	Callback struct {
		// Result is the root parameter that encloses the entire result message.
		Result CallbackResult `json:"Result"`
	}

	STKQueryRequest struct {
		// BusinessShortCode is organizations shortcode (Paybill or Buy goods - A 5 to 7-digit account number) used to
		// identify an organization and receive the transaction.
		BusinessShortCode uint `json:"BusinessShortCode"`

		// CheckoutRequestID is a global unique identifier of the processed checkout transaction request.
		CheckoutRequestID string `json:"CheckoutRequestID"`

		// Password is a base64 encoded string used for encrypting the request sent which is a combination of
		// BusinessShortCode + Passkey + Timestamp
		Password string `json:"Password"`

		// Timestamp of the transaction in the format of YEAR+MONTH+DATE+HOUR+MINUTE+SECOND (YYYYMMDDHHmmss).
		// Each part should be at least two digits apart from the year which takes four digits.
		// Example 20060102150405
		Timestamp string `json:"Timestamp"`
	}

	RegisterC2BURLRequest struct {
		// ShortCode is usually, a unique number is tagged to an M-PESA pay bill/till number of the organization.
		ShortCode uint `json:"ShortCode"`

		// ResponseType This parameter specifies what is to happen if for any reason the validation URL is not reachable.
		// Note that, this is the default action value that determines what M-PESA will do in the scenario that
		//your endpoint is unreachable or is unable to respond on time. Only two values are allowed: Completed or Cancelled.
		// Completed means M-PESA will automatically complete your transaction, whereas Cancelled means M-PESA will
		// automatically cancel the transaction, in the event M-PESA is unable to reach your Validation URL.
		ResponseType ResponseType `json:"ResponseType"`

		// ConfirmationURL is the URL that receives the confirmation request from API upon payment completion.
		ConfirmationURL string `json:"ConfirmationURL"`

		// ValidationURL is the URL that receives the validation request from the API upon payment submission.
		// The validation URL is only called if the external validation on the registered shortcode is enabled.
		// (By default External Validation is disabled).
		ValidationURL string `json:"ValidationURL"`
	}

	DynamicQRRequest struct {
		// Total Amount for the sale or transaction
		Amount uint `json:"Amount"`

		// CreditPartyIdentifier can be a Mobile Number, Business Number, Agent Till, Paybill or Business number, or Merchant Buy Goods.
		CreditPartyIdentifier string `json:"CPI"`

		// MerchantName is the name of the Company/M-Pesa merchant
		MerchantName string `json:"MerchantName"`

		// ReferenceNo is the transaction reference number.
		ReferenceNo string `json:"RefNo"`

		// Size of the QR code image in pixels. QR code image will always be a square image.
		Size string `json:"Size"`

		/*
			TransactionType represents the type of transaction being made.
				- PayMerchantBuyGoods: Pay Merchant (Buy Goods).
				- WithdrawCashAtAgentTill: Withdraw Cash at Agent Till.
				- PaybillOrBusinessNumber: Paybill or Business number.
				- SendMoneyViaMobileNumber: Send Money(Mobile number)
				- SentToBusiness: Sent to Business. Business number CPI in MSISDN format.
		*/
		TransactionType DynamicQRTransactionType `json:"TrxCode"`
	}

	DynamicQRResponse struct {
		// ImagePath is the absolute path to the decoded base64 image
		ImagePath string `json:"qr_path,omitempty"`

		// ErrorCode is a predefined code that indicates the reason for request failure that is defined in the
		// ErrorMessage. The error codes maps to specific error message.
		ErrorCode string `json:"errorCode,omitempty"`

		// ErrorMessage is a short descriptive message of the failure reason.
		ErrorMessage string `json:"errorMessage,omitempty"`

		// QRCode Image/Data/String.
		QRCode string `json:"QRCode,omitempty"`

		// RequestID represents the ID for the request
		RequestID string `json:"requestId,omitempty"`

		// ResponseCode is a numeric status code that indicates the status of the transaction submission.
		ResponseCode string `json:"ResponseCode,omitempty"`

		// ResponseDescription is a response describing the status of the transaction.
		ResponseDescription string `json:"ResponseDescription,omitempty"`
	}

	TransactionStatusRequest struct {
		// The CommandID for the request - TransactionStatusQueryCommandID
		CommandID CommandID `json:"CommandID"`

		// IdentifierType is the type of organization receiving the transaction
		IdentifierType IdentifierType `json:"IdentifierType"`

		// Initiator is the credential/username used to authenticate the transaction request.
		Initiator string `json:"Initiator"`

		// Occasion is an optional paramater that is a sequence of characters up to 100
		Occasion string `json:"Occasion"`

		// OriginatorConversationID is a global unique identifier for the transaction request returned by the API proxy
		// upon successful request submission. If you don’t have the M-PESA transaction ID you can use this to query.
		OriginatorConversationID string `json:"OriginatorConversationID,omitempty"`

		// PartyA is an Organization/MSISDN receiving the transaction. Shortcode (6-9 digits) MSISDN (12 Digits)
		PartyA uint `json:"PartyA"`

		// QueueTimeOutURL is the endpoint that will be used by API Proxy to send notification incase the request is timed
		// out while awaiting processing in the queue. Must be served via https.
		QueueTimeOutURL string `json:"QueueTimeOutURL"`

		// Remarks are comments that are sent along with the transaction. They are a sequence of characters up to 100
		Remarks string `json:"Remarks"`

		// ResultURL is the endpoint that will be used by M-PESA to send notification upon processing of the request.
		// Must be served via https.
		ResultURL string `json:"ResultURL"`

		// SecurityCredential is an encrypted password for the initiator to authenticate the transaction request
		SecurityCredential string `json:"SecurityCredential"`

		// TransactionID is a unique identifier to identify a transaction on Mpesa
		TransactionID string `json:"TransactionID"`
	}

	AccountBalanceRequest struct {
		// The CommandID for the request - AccountBalanceCommandID
		CommandID CommandID `json:"CommandID"`

		// IdentifierType is the type of organization fetching the balance - Shortcode (Till Number Organization shortcode)
		IdentifierType IdentifierType `json:"IdentifierType"`

		// Initiator is the credential/username used to authenticate the request.
		Initiator string `json:"Initiator"`

		// PartyA is the shortcode of the organization querying for the account balance.
		PartyA int `json:"PartyA"`

		// QueueTimeOutURL is the endpoint that will be used by API Proxy to send notification incase the request is timed
		// out while awaiting processing in the queue. Must be served via https.
		QueueTimeOutURL string `json:"QueueTimeOutURL"`

		// Remarks are comments that are sent along with the transaction. They are a sequence of characters up to 100
		Remarks string `json:"Remarks"`

		// ResultURL is the endpoint that will be used by M-PESA to send notification upon processing of the request.
		// Must be served via https.
		ResultURL string `json:"ResultURL"`

		// SecurityCredential is an encrypted password for the initiator to authenticate the request
		SecurityCredential string `json:"SecurityCredential"`
	}

	BusinessPayBillRequest struct {
		// AccountReference is account number to be associated with the payment. Up to 13 characters.
		AccountReference string `json:"AccountReference"`

		// Amount is the transaction amount.
		Amount uint `json:"Amount"`

		// The CommandID for the request - BusinessPayBillCommandID
		CommandID CommandID `json:"CommandID"`

		// Initiator is the credential/username used to authenticate the request.
		Initiator string `json:"Initiator"`

		// Occasion is an optional paramater that is a sequence of characters up to 100
		Occasion string `json:"Occasion"`

		// PartyA is your shortcode. The shortcode from which money will be deducted.
		PartyA uint `json:"PartyA"`

		// PartyB is the shortcode to which money will be moved to.
		PartyB uint `json:"PartyB"`

		// QueueTimeOutURL is the endpoint that will be used by API Proxy to send notification incase the request is
		//timed out while awaiting processing in the queue. Must be served via https.
		QueueTimeOutURL string `json:"QueueTimeOutURL"`

		// RecieverIdentifierType is the type of shortcode to which money is credited. This API supports type
		// ShortcodeIdentifierType only
		RecieverIdentifierType IdentifierType `json:"RecieverIdentifierType"`

		// Remarks are comments that are sent along with the transaction. They are a sequence of characters up to 100
		Remarks string `json:"Remarks"`

		// Optional. Requester is the consumer’s mobile number on behalf of whom you are paying.
		Requester int64 `json:"Requester"`

		// ResultURL is the endpoint that will be used by M-PESA to send notification upon processing of the request.
		// Must be served via https.
		ResultURL string `json:"ResultURL"`

		// SecurityCredential is an encrypted password for the initiator to authenticate the request
		SecurityCredential string `json:"SecurityCredential"`

		// SenderIdentifierType is the type of shortcode from which money is deducted.
		// For this API, only ShortcodeIdentifierType is allowed
		SenderIdentifierType IdentifierType `json:"SenderIdentifierType"`
	}
)
