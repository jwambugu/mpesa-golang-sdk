package mpesa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testConsumerKey    = "W6vRBOiKRSYZpXABQjXf9W3+KR+tGWGKTrOpOhnfig"
	testConsumerSecret = "MmE8/5EW3XXBIKg4qpDJ8g"
)

func TestMpesa_GenerateAccessToken(t *testing.T) {
	var (
		asserts = assert.New(t)
		ctx     = context.Background()
	)

	tests := []struct {
		name string
		mock func(t *testing.T, app *Mpesa, c *mockHttpClient)
	}{
		{
			name: "it generates and caches an access token successfully",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusOK, `
						{
						"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
						"expires_in": "3599"
						}`
				})

				token, err := app.GenerateAccessToken(ctx)
				asserts.NoError(err)
				asserts.NotEmpty(token)
				asserts.Equal(token, app.cache[testConsumerKey].AccessToken)

				// Make subsequent call to get the token from the cache
				token, err = app.GenerateAccessToken(ctx)
				asserts.NoError(err)
				asserts.Equal(token, app.cache[testConsumerKey].AccessToken)
			},
		},
		{
			name: "it fails to generate an access token",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusBadRequest, ``
				})

				token, err := app.GenerateAccessToken(ctx)
				asserts.NotNil(err)
				asserts.Empty(token)
			},
		},
		{
			name: "it flushes and generates a new access token successfully",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient) {
				oldToken := "0A0v8OgxqqoocblflR58m9chMdnU"

				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusOK, `
					{
						"access_token": "` + oldToken + `",
						"expires_in": "3599"
					}`
				})

				token, err := app.GenerateAccessToken(ctx)
				asserts.NoError(err)
				asserts.NotEmpty(t, token)

				gotCachedData := app.cache[testConsumerKey]
				asserts.Equal(token, gotCachedData.AccessToken)

				// Alter the time the cache was set to simulate an expired cache
				gotCachedData.setAt = time.Now().Add(-1 * time.Hour)
				app.cache[testConsumerKey] = gotCachedData

				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusOK, `
					{
						"access_token": "R58m9chMdnU0A0v8Ogxqqoocblfl",
						"expires_in": "3599"
					}`
				})

				// Make subsequent call to get the token from the cache
				token, err = app.GenerateAccessToken(ctx)
				asserts.NoError(err)
				asserts.Equal(token, app.cache[testConsumerKey].AccessToken)
				asserts.NotEqual(t, oldToken, app.cache[testConsumerKey].AccessToken)
			},
		},
		{
			name: "it fails with 404 if invalid url is passed",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.stkPushURL, func() (status int, body string) {
					return http.StatusNotFound, ``
				})

				token, err := app.GenerateAccessToken(ctx)
				asserts.NotNil(err)
				asserts.Empty(token)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				cl  = newMockHttpClient()
				app = NewApp(cl, testConsumerKey, testConsumerSecret, Sandbox)
			)

			tc.mock(t, app, cl)
		})
	}
}

func TestMpesa_STKPush(t *testing.T) {
	var (
		asserts = assert.New(t)
		ctx     = context.Background()
	)

	tests := []struct {
		name   string
		stkReq STKPushRequest
		mock   func(t *testing.T, app *Mpesa, c *mockHttpClient, stkReq STKPushRequest)
	}{
		{
			name: "it makes stk push request successfully",
			stkReq: STKPushRequest{
				BusinessShortCode: 174379,
				TransactionType:   "CustomerPayBillOnline",
				Amount:            10,
				PartyA:            254708374149,
				PartyB:            174379,
				PhoneNumber:       254708374149,
				CallBackURL:       "https://example.com",
				AccountReference:  "Test",
				TransactionDesc:   "Test",
			},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, stkReq STKPushRequest) {
				passkey := "passkey"

				c.MockRequest(app.stkPushURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams STKPushRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					asserts.NoError(err)

					timestamp := time.Now().Format("20060102150405")
					wantPassword := fmt.Sprintf("%d%s%s", stkReq.BusinessShortCode, passkey, timestamp)

					gotPassword := make([]byte, base64.StdEncoding.DecodedLen(len(reqParams.Password)))
					n, err := base64.StdEncoding.Decode(gotPassword, []byte(reqParams.Password))
					asserts.NoError(err)
					asserts.Equal(wantPassword, string(gotPassword[:n]))

					return http.StatusOK, `
						{
						  "MerchantRequestID": "29115-34620561-1",
						  "CheckoutRequestID": "ws_CO_191220191020363925",
						  "ResponseCode": "0",
						  "ResponseDescription": "Success. Request accepted for processing",
						  "CustomerMessage": "Success. Request accepted for processing"
						}`
				})

				res, err := app.STKPush(ctx, passkey, stkReq)
				asserts.NoError(err)
				asserts.NotNil(res)
				asserts.Contains(res.CustomerMessage, "Success. Request accepted for processing")
			},
		},
		{
			name: "request fails with an error code",
			stkReq: STKPushRequest{
				BusinessShortCode: 0,
				TransactionType:   "CustomerPayBillOnline",
				Amount:            10,
				PartyA:            254708374149,
				PartyB:            174379,
				PhoneNumber:       254708374149,
				CallBackURL:       "https://example.com",
				AccountReference:  "Test",
				TransactionDesc:   "Test",
			},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, stkReq STKPushRequest) {
				passkey := "passkey"

				c.MockRequest(app.stkPushURL, func() (status int, body string) {
					return http.StatusBadRequest, `
						{
							"requestId": "4788-81090592-4",
							"errorCode": "400.002.02",
							"errorMessage": "Bad Request - Invalid BusinessShortCode"
						}`
				})

				res, err := app.STKPush(ctx, passkey, stkReq)
				asserts.Error(err)
				asserts.Nil(res)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				cl  = newMockHttpClient()
				app = NewApp(cl, testConsumerKey, testConsumerSecret, Sandbox)
			)

			cl.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
				{
					"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
					"expires_in": "3599"
				}`
			})

			tc.mock(t, app, cl, tc.stkReq)

			_, err := app.GenerateAccessToken(ctx)
			asserts.NoError(err)
			asserts.Len(cl.requests, 2)
		})
	}
}

func TestUnmarshalSTKPushCallback(t *testing.T) {
	var (
		asserts = assert.New(t)
	)

	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name: "it can unmarshal a successful transaction callback string",
			input: `{
			   "Body":{
				  "stkCallback":{
					 "MerchantRequestID":"29115-34620561-1",
					 "CheckoutRequestID":"ws_CO_191220191020363925",
					 "ResultCode":0,
					 "ResultDesc":"The service request is processed successfully.",
					 "CallbackMetadata":{
						"Item":[
						   {
							  "Name":"Amount",
							  "Value":1.00
						   },
						   {
							  "Name":"MpesaReceiptNumber",
							  "Value":"NLJ7RT61SV"
						   },
						   {
							  "Name":"TransactionDate",
							  "Value":20191219102115
						   },
						   {
							  "Name":"PhoneNumber",
							  "Value":254708374149
						   }
						]
					 }
				  }
			   }
			}`,
		},
		{
			name: "it can unmarshal an unsuccessful transaction callback struct",
			input: STKPushCallback{
				Body: STKPushCallbackBody{
					STKCallback: STKCallback{
						MerchantRequestID: "29115-34620561-1",
						CheckoutRequestID: "ws_CO_191220191020363925",
						ResultCode:        1032,
						ResultDesc:        "Request cancelled by user.",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			callback, err := UnmarshalSTKPushCallback(tc.input)
			asserts.NoError(err)
			asserts.NotNil(callback)
			asserts.Equal("ws_CO_191220191020363925", callback.Body.STKCallback.CheckoutRequestID)
		})
	}
}

func TestMpesa_B2C(t *testing.T) {
	var (
		asserts = assert.New(t)
		ctx     = context.Background()
	)

	tests := []struct {
		name   string
		b2cReq B2CRequest
		env    Environment
		mock   func(t *testing.T, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest)
	}{
		{
			name: "it makes a b2c request on sandbox successfully",
			b2cReq: B2CRequest{
				InitiatorName:   "TestG2Init",
				CommandID:       "BusinessPayment",
				Amount:          10,
				PartyA:          600123,
				PartyB:          254728762287,
				Remarks:         "This is a remark",
				QueueTimeOutURL: "https://example.com",
				ResultURL:       "https://example.com",
				Occasion:        "Test Occasion",
			},
			env: Sandbox,
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest) {
				c.MockRequest(app.b2cURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams B2CRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					asserts.NoError(err)
					asserts.NotEmpty(reqParams.SecurityCredential)
					asserts.Equal(b2cReq.InitiatorName, reqParams.InitiatorName)

					return http.StatusOK, `
					{    
					 "ConversationID": "AG_20191219_00005797af5d7d75f652",    
					 "OriginatorConversationID": "16740-34861180-1",    
					 "ResponseCode": "0",    
					 "ResponseDescription": "Accept the service request successfully."
					}`
				})

				res, err := app.B2C(ctx, "random-string", b2cReq)
				asserts.NoError(err)
				asserts.NotNil(res)
				asserts.Contains(res.ResponseDescription, "Accept the service request successfully")
			},
		},
		{
			name: "it makes a b2c request on production successfully",
			b2cReq: B2CRequest{
				InitiatorName:   "TestG2Init",
				CommandID:       "BusinessPayment",
				Amount:          10,
				PartyA:          600123,
				PartyB:          254728762287,
				Remarks:         "This is a remark",
				QueueTimeOutURL: "https://example.com",
				ResultURL:       "https://example.com",
				Occasion:        "Test Occasion",
			},
			env: Production,
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest) {
				c.MockRequest(app.b2cURL, func() (status int, body string) {
					req := c.requests[1]

					var reqParams B2CRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					asserts.NoError(err)
					asserts.NotEmpty(t, reqParams.SecurityCredential)

					return http.StatusOK, `
					{    
					 "ConversationID": "AG_20191219_00005797af5d7d75f652",    
					 "OriginatorConversationID": "16740-34861180-1",    
					 "ResponseCode": "0",    
					 "ResponseDescription": "Accept the service request successfully."
					}`
				})

				res, err := app.B2C(ctx, "random-string", b2cReq)
				asserts.NoError(err)
				asserts.NotNil(res)
				asserts.Contains(res.ResponseDescription, "Accept the service request successfully")
			},
		},
		{
			name: "request fails with an error code",
			b2cReq: B2CRequest{
				InitiatorName:   "",
				CommandID:       "BusinessPayment",
				Amount:          10,
				PartyA:          600123,
				PartyB:          254728762287,
				Remarks:         "This is a remark",
				QueueTimeOutURL: "https://example.com",
				ResultURL:       "https://example.com",
				Occasion:        "Test Occasion",
			},
			env: Production,
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest) {
				c.MockRequest(app.b2cURL, func() (status int, body string) {
					return http.StatusBadRequest, `
					{    
					   "requestId": "11728-2929992-1",
					   "errorCode": "401.002.01",
					   "errorMessage": "Error Occurred - Invalid Access Token - BJGFGOXv5aZnw90KkA4TDtu4Xdyf"
					}`
				})

				res, err := app.B2C(ctx, "random-string", b2cReq)
				asserts.Error(err)
				asserts.Contains(err.Error(), "Invalid Access Token")
				asserts.Nil(res)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				cl  = newMockHttpClient()
				app = NewApp(cl, testConsumerKey, testConsumerSecret, tc.env)
			)

			cl.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
				{
					"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
					"expires_in": "3599"
				}`
			})

			tc.mock(t, app, cl, tc.b2cReq)

			_, err := app.GenerateAccessToken(ctx)
			asserts.NoError(err)
			asserts.Len(cl.requests, 2)
		})
	}
}

func TestUnmarshalB2CCallback(t *testing.T) {
	asserts := assert.New(t)

	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name: "it can unmarshal a successful transaction callback string",
			input: `
			{    
			   "Result": {
				  "ResultType": 0,
				  "ResultCode": 0,
				  "ResultDesc": "The service request is processed successfully.", 
				  "OriginatorConversationID": "10571-7910404-1",
				  "ConversationID": "AG_20191219_00004e48cf7e3533f581",
				  "TransactionID": "NLJ41HAY6Q",
				  "ResultParameters": {
					 "ResultParameter": [
					  {
						 "Key": "TransactionAmount",
						 "Value": 10
					  },
					  {
						 "Key": "TransactionReceipt",
						 "Value": "NLJ41HAY6Q"
					  },
					  {
						 "Key": "B2CRecipientIsRegisteredCustomer",
						 "Value": "Y"
					  },
					  {
						 "Key": "B2CChargesPaidAccountAvailableFunds",
						 "Value": -4510.00
					  },
					  {
						 "Key": "ReceiverPartyPublicName",
						 "Value": "254708374149 - John Doe"
					  },
					  {
						 "Key": "TransactionCompletedDateTime",
						 "Value": "19.12.2019 11:45:50"
					  },
					  {
						 "Key": "B2CUtilityAccountAvailableFunds",
						 "Value": 10116.00
					  },
					  {
						 "Key": "B2CWorkingAccountAvailableFunds",
						 "Value": 900000.00
					  }
					]
				  },
				  "ReferenceData": {
					 "ReferenceItem": {
						"Key": "QueueTimeoutURL",
						"Value": "https:\/\/internalsandbox.safaricom.co.ke\/mpesa\/b2cresults\/v1\/submit"
					  }
				  }
			   }
			}`,
		},
		{
			name: "it can unmarshal an unsuccessful transaction callback struct",
			input: B2CCallback{
				Result: B2CCallbackResult{
					ResultType:               0,
					ResultCode:               0,
					ResultDesc:               "The initiator information is invalid.",
					OriginatorConversationID: "29112-34801843-1",
					ConversationID:           "AG_20191219_00004e48cf7e3533f581",
					TransactionID:            "NLJ41HAY6Q",
					ReferenceData: B2CReferenceData{
						ReferenceItem: B2CReferenceItem{
							Key:   "QueueTimeoutURL",
							Value: "https:\\/\\/internalsandbox.safaricom.co.ke\\/mpesa\\/b2cresults\\/v1\\/submit",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			callback, err := UnmarshalB2CCallback(tc.input)
			asserts.NoError(err)
			asserts.NotNil(callback)
			asserts.Equal("AG_20191219_00004e48cf7e3533f581", callback.Result.ConversationID)
			asserts.Equal("QueueTimeoutURL", callback.Result.ReferenceData.ReferenceItem.Key)
		})
	}
}

func TestMpesa_STKPushQuery(t *testing.T) {
	var (
		asserts = assert.New(t)
		ctx     = context.Background()
	)

	tests := []struct {
		name string
		mock func(t *testing.T, app *Mpesa, c *mockHttpClient, stkReq STKQueryRequest)
	}{
		{
			name: "it makes an stk push query request successfully",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, stkReq STKQueryRequest) {
				passkey := "passkey"

				c.MockRequest(app.stkPushQueryURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams STKQueryRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					asserts.NoError(err)

					timestamp := time.Now().Format("20060102150405")
					wantPassword := fmt.Sprintf("%d%s%s", stkReq.BusinessShortCode, passkey, timestamp)

					gotPassword := make([]byte, base64.StdEncoding.DecodedLen(len(reqParams.Password)))
					n, err := base64.StdEncoding.Decode(gotPassword, []byte(reqParams.Password))
					asserts.NoError(err)
					asserts.Equal(wantPassword, string(gotPassword[:n]))

					return http.StatusOK, `
						{
						  "ResponseCode": "0",
						  "MerchantRequestID": "8773-65037085-1",
						  "CheckoutRequestID": "ws_CO_03082022131319635708374149",
						  "ResultCode": "0",
                          "ResultDesc": "Success. Request accepted for processing",
						  "CustomerMessage": "Success. Request accepted for processing"
						}`
				})

				res, err := app.STKQuery(ctx, passkey, stkReq)
				asserts.NoError(err)
				asserts.NotNil(res)
				asserts.Contains(res.CustomerMessage, "Request accepted")
			},
		},
		{
			name: "the request fails if the transaction is being processed",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, stkReq STKQueryRequest) {
				passkey := "passkey"

				c.MockRequest(app.stkPushQueryURL, func() (status int, body string) {
					return http.StatusInternalServerError, `
						{
						  "RequestID": "ws_CO_03082022131319635708374149",
						  "ErrorCode": "500.001.1001",
						  "ErrorMessage": "The transaction is being processed"
						}`
				})

				res, err := app.STKQuery(ctx, passkey, stkReq)
				asserts.Error(err)
				asserts.Contains(err.Error(), "error code 500.001.1001:The transaction is being processed")
				asserts.Nil(res)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cl := newMockHttpClient()
			app := NewApp(cl, testConsumerKey, testConsumerSecret, Sandbox)

			cl.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
				{
					"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
					"expires_in": "3599"
				}`
			})

			_, err := app.GenerateAccessToken(ctx)
			asserts.NoError(err)

			tc.mock(t, app, cl, STKQueryRequest{
				BusinessShortCode: 174379,
				CheckoutRequestID: "ws_CO_03082022131319635708374149",
			})
		})
	}
}

func Test_RegisterC2BURL(t *testing.T) {
	var (
		asserts = assert.New(t)
		ctx     = context.Background()
	)

	tests := []struct {
		name       string
		env        Environment
		mock       func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, c2bRequest RegisterC2BURLRequest)
		c2bRequest RegisterC2BURLRequest
	}{
		{
			name: "it should register URLs in sanbox",
			env:  Sandbox,
			c2bRequest: RegisterC2BURLRequest{
				ShortCode:       600638,
				ResponseType:    "Completed",
				ValidationURL:   "http://example.com/validate",
				ConfirmationURL: "http://example.com/confirm",
			},
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, c2bRequest RegisterC2BURLRequest) {
				c.MockRequest(app.c2bURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams RegisterC2BURLRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					asserts.NoError(err)

					return http.StatusOK, `
						{
						  "OriginatorCoversationID": "7619-37765134-1",
						  "ResponseCode": "0",
						  "ResponseDescription": "success"
						}`
				})

				res, err := app.RegisterC2BURL(ctx, c2bRequest)
				asserts.NoError(err)
				asserts.NotNil(res)
				asserts.Equal(res.ResponseDescription, "success")
			},
		},
		{
			name: "it should register URLs in production",
			env:  Production,
			c2bRequest: RegisterC2BURLRequest{
				ShortCode:       200200,
				ResponseType:    "Canceled",
				ValidationURL:   "http://example.com/validate",
				ConfirmationURL: "http://example.com/confirm",
			},
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, c2bRequest RegisterC2BURLRequest) {
				c.MockRequest(app.c2bURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams RegisterC2BURLRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					asserts.NoError(err)

					return http.StatusOK, `
						{
						  "OriginatorCoversationID": "7619-37765134-1",
						  "ResponseCode": "0",
						  "ResponseDescription": "success"
						}`
				})

				res, err := app.RegisterC2BURL(ctx, c2bRequest)
				asserts.NoError(err)
				asserts.NotNil(res)
				asserts.Equal(res.ResponseDescription, "success")
			},
		},
		{
			name: "fail with invalid response type",
			c2bRequest: RegisterC2BURLRequest{
				ResponseType: "Foo",
			},
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, c2bRequest RegisterC2BURLRequest) {
				res, err := app.RegisterC2BURL(ctx, c2bRequest)
				asserts.Error(err)
				asserts.Equal(err.Error(), "mpesa: the provided ResponseType [Foo] is not valid")
				asserts.Nil(res)
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				client = newMockHttpClient()
				app    = NewApp(client, testConsumerKey, testConsumerSecret, tc.env)
			)

			client.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
				{
					"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
					"expires_in": "3599"
				}`
			})

			tc.mock(t, ctx, app, client, tc.c2bRequest)
		})
	}
}

func TestMpesa_DynamicQR(t *testing.T) {
	var (
		ctx     = context.Background()
		asserts = assert.New(t)
	)

	tests := []struct {
		name string
		mock func(app *Mpesa, c *mockHttpClient, qrReq DynamicQRRequest)
	}{
		{
			name: "it makes a request and generates a qr code",
			mock: func(app *Mpesa, c *mockHttpClient, qrReq DynamicQRRequest) {
				c.MockRequest(app.dynamicQRURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					return http.StatusOK, `
						{
							"ResponseCode": "00",
							"ResponseDescription": "The service request is processed successfully.",
							"QRCode": "iVBORw0KGgoAAAANSUhEUgAAASwAAAEsCAYAAAB5fY51AAAJXElEQVR42u3dyZLbMAwFwPn/n55cU1nsIQXQANivSqfYMhewLSmy5utbRKRJvgyBiABLRARYIgIsERFgiYgAS0SAJSICLBERYIkIsOTPwfr6+msT4ybAsvCMmwiwLDzjJsCy8MS4CbAEWCLAEmAJsG5eeD9ZmKv//r/F/a/XrYDw6j07nx/1WU/2lbHPJ/MBbmC1A+vda56+fxetlf38vr+dz9z9rBMwRLUNWMBqeWrz5Nt6d/9RCz9ii/ysbBiqtkuAVRas1dOPHYR2X7vT3hVko5CJBCtyPIEFrOvBimhj5AI7CUjVNgELWKPAenf9Z/XbPOJaFrCABSxgvXzdKlgdLhIDC1jAGgjWyili1AV3YAELWMAqBZZrWMACljwq3ncYAQtYwALWKLB2Pr/TKWHkbQ07t1qcGM/ddgmwyoC1+7pp17BO3/SaPZ6R7RJgjQAre7FWAmt1HiogAyxgtQUr87pGpR/rZv9cKXMMqu9LgHUULDHnAqwyhat4Z8wrrIA1HirFO39+zTmwgCVtwRJguYYh5dESYImIAEtEgFX2usK0LWp8Ou7nZG2ow56nwcACFrDUIbCABSxgAQtYCgVYwAIWsIAFLHUILGABC1jAAhawgAUsYAELWMBSh8ACFrCABaxLweqYjgu7I8TqcG6/gAUsYKlDYCkUYAFLHQJLoQBLHQLLgAILWOoQWAoFWMCyvoClUIClDoFlQIEFLHUILIUCLGBZX8AqsWhv7tfJNqvDeV8MwAIWsNQhsBQKsIClDoGlUIAFLGABC1jAUofAUijAAhawgKVQgAUsYAELWMBSh8BSKMACFrCApVD0C1jAAhawgKUOgaVQ9AtYwAKWQtEvYAELWMAClvkClkLRL2ABC1gKRb+ABSxgAQtY5gtYCkW/gAUsYCmUqXeo37wfYAELWMACFrCABSxgqUNgKRRgAQtYwAIWsIAFLGABC1jAApZCARawgAUsYAELWMACFrCABSxgNRzQTv3q+NjiqV8M1hew9AtYwAKWAQUWsNQhsBQKsIBlfQFLv4AFLGAZUGABSx0CS6EAC1jWF7D0C1jAApYBBRaw1CGwLtw6Lkh3qN9Zh8BSKIAAFrCABSz7ARawgAUsYAELWAoFEMACFrCAZT/AAhawgAUsYAFLoQACWMACFrDsRx0CSxoFWNKuZg0BsIAlwBJgAUuAJcACFrAEWMASYAmwgCXAEmABC1gCLGAJsARYwJJ5YE2969ed3Hfe7T31lxLAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxglZrgap9VrZg6juHNi3/644+BBSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAsmg/vGg7LpKpOAqwgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAetasBRlnTGs9iWkVucCCixgAUutAksRAAtYahVYigBYahVYigBYwFKrwFIEwAKWWgWWIgCWWgWWIgAWsNQqsBQBsIClVoFV4hG3HYGYOobaAyxgAQtY2gMsiw1YgAAWsIAFLO0BFrCABSztAZbFBixAAAtYwAKW9gALWMACFrCABSxjCAhgNQKrGo4n99OxXx3hm1ob0x/HDCxgAQtYwAIWsEADLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAuhQsxXSmPTfDV22cp9/FDixgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAuhSsmzFyl3avRwlPRRZYwAIWsIAFLGAZQ2ABC1jAAhawgGWxAQtYwAIWsIwhsIAFLGABC1jAstiABSxgAQtYxhBYwOrXuYZ3cnu0sc2d7sACFrBswAIWsIAFLGABC1g2YAELWMACFrCABSxgAQtYwAKWDVjAAhawgAUsYAELWMACloQDYXx6fXGad2BZkMYHWMASYBkfYAFLgAUsYIkFaXyABSwBFrCABSwLEljAApZYkMYHWMASYAELWCPBchfynYhMfWzx1F9TAAtYwAIWsIAFLHMKLGABC1jAAhawgAUsYAELWMAyp8ACFrCABSxgAQtYwAIWsIAFLGABC1jHinvqou0IxFSw1DOwgAUsYAELWMACFrCABSxgAUs9AwtYwAIWsIAFLGABC1gmGFjAUs/AAhawgEUiYAELWMACVqMJnnrn9Mm+d2zPzb+CmI4jsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWNeB5VHLd/5aoOMXFbCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAGtavqUBM/YVD6y8sYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsCwBYwAIWsIAFLGABC1jAMl/AAlabwb74UctTkZ0678ACFrCABSxgAQtYwAIWsBQusIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAKWeQdWqQmeunUEa+zi8EsAYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsEREgCUiAiwRAZaICLBERIAlIsASEQGWiAiwRARYIiLAEhEBlogAS0QEWCICLBERYImIAEtEgCUiAiwREWBNnJQHz9rOfoZ31rPoP9Gv255RDyw5BlVnsCLaEtmv7n+IAVgCrMQjpEpgTfnrMcCSMmC9WlxZKD6F8N17s0/LnoC18u8CLFlYhJ/YZ/aRSBZYu58DLGBJAbAijq5OtesTR1eOsoAlxcFafU8XsE68ToAlSQsn4mL5qXZl9QtYwJLGYK2eCj5tV8a1sJV9AAtY0gCsyKOrTLCy+/WT17u9AVjSDKyoi9c7R1jVbmIFFrCkAFg7p4KnTlWfABgJJ7CAJYfBOnkUcuI6VNSp26v3AwtYUgisjCOQCm3MGncBlhwA6/TvEU+BlXna5ugKWFIIrNPtivifu8h+Rdz2IMCS79iLwVWOQqo9w8rFdmBJE7A+ceQXDUTWaTKogCUfAit7EZ5+PE10v0DVN78A4PhWMY/tjp0AAAAASUVORK5CYII="
						}`
				})

				resp, err := app.DynamicQR(ctx, qrReq, PayMerchantBuyGoods, false)
				asserts.NoError(err)
				asserts.NotNil(resp)
				asserts.Equal("00", resp.ResponseCode)
			},
		},
		{
			name: "it makes a request and generates a qr code with the decode image",
			mock: func(app *Mpesa, c *mockHttpClient, qrReq DynamicQRRequest) {
				c.MockRequest(app.dynamicQRURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					return http.StatusOK, `
						{
							"ResponseCode": "00",
							"ResponseDescription": "The service request is processed successfully.",
							"QRCode": "iVBORw0KGgoAAAANSUhEUgAAASwAAAEsCAYAAAB5fY51AAAJXElEQVR42u3dyZLbMAwFwPn/n55cU1nsIQXQANivSqfYMhewLSmy5utbRKRJvgyBiABLRARYIgIsERFgiYgAS0SAJSICLBERYIkIsOTPwfr6+msT4ybAsvCMmwiwLDzjJsCy8MS4CbAEWCLAEmAJsG5eeD9ZmKv//r/F/a/XrYDw6j07nx/1WU/2lbHPJ/MBbmC1A+vda56+fxetlf38vr+dz9z9rBMwRLUNWMBqeWrz5Nt6d/9RCz9ii/ysbBiqtkuAVRas1dOPHYR2X7vT3hVko5CJBCtyPIEFrOvBimhj5AI7CUjVNgELWKPAenf9Z/XbPOJaFrCABSxgvXzdKlgdLhIDC1jAGgjWyili1AV3YAELWMAqBZZrWMACljwq3ncYAQtYwALWKLB2Pr/TKWHkbQ07t1qcGM/ddgmwyoC1+7pp17BO3/SaPZ6R7RJgjQAre7FWAmt1HiogAyxgtQUr87pGpR/rZv9cKXMMqu9LgHUULDHnAqwyhat4Z8wrrIA1HirFO39+zTmwgCVtwRJguYYh5dESYImIAEtEgFX2usK0LWp8Ou7nZG2ow56nwcACFrDUIbCABSxgAQtYCgVYwAIWsIAFLHUILGABC1jAAhawgAUsYAELWMBSh8ACFrCABaxLweqYjgu7I8TqcG6/gAUsYKlDYCkUYAFLHQJLoQBLHQLLgAILWOoQWAoFWMCyvoClUIClDoFlQIEFLHUILIUCLGBZX8AqsWhv7tfJNqvDeV8MwAIWsNQhsBQKsIClDoGlUIAFLGABC1jAUofAUijAAhawgKVQgAUsYAELWMBSh8BSKMACFrCApVD0C1jAAhawgKUOgaVQ9AtYwAKWQtEvYAELWMAClvkClkLRL2ABC1gKRb+ABSxgAQtY5gtYCkW/gAUsYCmUqXeo37wfYAELWMACFrCABSxgqUNgKRRgAQtYwAIWsIAFLGABC1jAApZCARawgAUsYAELWMACFrCABSxgNRzQTv3q+NjiqV8M1hew9AtYwAKWAQUWsNQhsBQKsIBlfQFLv4AFLGAZUGABSx0CS6EAC1jWF7D0C1jAApYBBRaw1CGwLtw6Lkh3qN9Zh8BSKIAAFrCABSz7ARawgAUsYAELWAoFEMACFrCAZT/AAhawgAUsYAFLoQACWMACFrDsRx0CSxoFWNKuZg0BsIAlwBJgAUuAJcACFrAEWMASYAmwgCXAEmABC1gCLGAJsARYwJJ5YE2969ed3Hfe7T31lxLAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxglZrgap9VrZg6juHNi3/644+BBSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAsmg/vGg7LpKpOAqwgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAetasBRlnTGs9iWkVucCCixgAUutAksRAAtYahVYigBYahVYigBYwFKrwFIEwAKWWgWWIgCWWgWWIgAWsNQqsBQBsIClVoFV4hG3HYGYOobaAyxgAQtY2gMsiw1YgAAWsIAFLO0BFrCABSztAZbFBixAAAtYwAKW9gALWMACFrCABSxjCAhgNQKrGo4n99OxXx3hm1ob0x/HDCxgAQtYwAIWsEADLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAuhQsxXSmPTfDV22cp9/FDixgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAuhSsmzFyl3avRwlPRRZYwAIWsIAFLGAZQ2ABC1jAAhawgGWxAQtYwAIWsIwhsIAFLGABC1jAstiABSxgAQtYxhBYwOrXuYZ3cnu0sc2d7sACFrBswAIWsIAFLGABC1g2YAELWMACFrCABSxgAQtYwAKWDVjAAhawgAUsYAELWMACloQDYXx6fXGad2BZkMYHWMASYBkfYAFLgAUsYIkFaXyABSwBFrCABSwLEljAApZYkMYHWMASYAELWCPBchfynYhMfWzx1F9TAAtYwAIWsIAFLHMKLGABC1jAAhawgAUsYAELWMAyp8ACFrCABSxgAQtYwAIWsIAFLGABC1jHinvqou0IxFSw1DOwgAUsYAELWMACFrCABSxgAUs9AwtYwAIWsIAFLGABC1gmGFjAUs/AAhawgEUiYAELWMACVqMJnnrn9Mm+d2zPzb+CmI4jsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWNeB5VHLd/5aoOMXFbCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAGtavqUBM/YVD6y8sYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsCwBYwAIWsIAFLGABC1jAMl/AAlabwb74UctTkZ0678ACFrCABSxgAQtYwAIWsBQusIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAKWeQdWqQmeunUEa+zi8EsAYAELWMACFrCABSxgAQtYwAIWsIAFLGABC1jAAhawgAUsYAELWMACFrCABSxgAQtYwAIWsEREgCUiAiwRAZaICLBERIAlIsASEQGWiAiwRARYIiLAEhEBlogAS0QEWCICLBERYImIAEtEgCUiAiwREWBNnJQHz9rOfoZ31rPoP9Gv255RDyw5BlVnsCLaEtmv7n+IAVgCrMQjpEpgTfnrMcCSMmC9WlxZKD6F8N17s0/LnoC18u8CLFlYhJ/YZ/aRSBZYu58DLGBJAbAijq5OtesTR1eOsoAlxcFafU8XsE68ToAlSQsn4mL5qXZl9QtYwJLGYK2eCj5tV8a1sJV9AAtY0gCsyKOrTLCy+/WT17u9AVjSDKyoi9c7R1jVbmIFFrCkAFg7p4KnTlWfABgJJ7CAJYfBOnkUcuI6VNSp26v3AwtYUgisjCOQCm3MGncBlhwA6/TvEU+BlXna5ugKWFIIrNPtivifu8h+Rdz2IMCS79iLwVWOQqo9w8rFdmBJE7A+ceQXDUTWaTKogCUfAit7EZ5+PE10v0DVN78A4PhWMY/tjp0AAAAASUVORK5CYII="
						}`
				})

				resp, err := app.DynamicQR(ctx, qrReq, PayMerchantBuyGoods, true)
				asserts.NoError(err)
				asserts.NotNil(resp)

				defer func() {
					err = os.Remove(resp.ImagePath)
					asserts.NoError(err)
				}()

				asserts.Equal("00", resp.ResponseCode)
				asserts.NotEmpty(resp.ImagePath)

				wd, err := os.Getwd()
				asserts.NoError(err)

				imagesDir := filepath.Join(wd, "storage", "images")
				amountStr := strconv.Itoa(int(qrReq.Amount))

				wantFilename := qrReq.MerchantName + "_" + amountStr + "_" + qrReq.CreditPartyIdentifier + ".png"
				wantFilename = imagesDir + "/" + strings.ReplaceAll(wantFilename, " ", "_")

				asserts.Equal(wantFilename, resp.ImagePath)

				_, err = os.Stat(resp.ImagePath)
				asserts.NoError(err)
			},
		},
		{
			name: "request fails if an invalid trasaction type is passed",
			mock: func(app *Mpesa, c *mockHttpClient, qrReq DynamicQRRequest) {
				c.MockRequest(app.dynamicQRURL, func() (status int, body string) {
					req := c.requests[1]

					asserts.Equal("application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					asserts.Equal(wantAuthorizationHeader, req.Header.Get("Authorization"))

					return http.StatusBadRequest, `
						{
							"requestId": "42579-78118541-4",
							"errorCode": "400",
							"errorMessage": "Bad Request - Invalid TrxCode"
						}`
				})

				resp, err := app.DynamicQR(ctx, qrReq, "PayMerchantBuyGoods", true)
				asserts.Error(err)
				asserts.Nil(resp)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				cl  = newMockHttpClient()
				app = NewApp(cl, testConsumerKey, testConsumerSecret, Sandbox)
			)

			cl.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
				{
					"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
					"expires_in": "3599"
				}`
			})

			_, err := app.GenerateAccessToken(ctx)
			asserts.NoError(err)

			tc.mock(app, cl, DynamicQRRequest{
				Amount:                10,
				CreditPartyIdentifier: "111222",
				MerchantName:          "jwambugu",
				ReferenceNo:           "NULLABLE",
				Size:                  "500",
			})
		})
	}
}

func TestMpesa_GetTransactionStatus(t *testing.T) {
	var (
		ctx              = context.Background()
		initatorPassword = "random-string"
	)

	tests := []struct {
		name          string
		txnStatusReq  TransactionStatusRequest
		env           Environment
		mock          func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest)
		requestsCount int
	}{
		{
			name: "it generates valid security credentials and makes the request successfully on sandbox",
			env:  Sandbox,
			txnStatusReq: TransactionStatusRequest{
				Initiator:       "testapi",
				Occasion:        "Test",
				PartyA:          "600426",
				QueueTimeOutURL: "https://example.com/",
				Remarks:         "Test remarks",
				ResultURL:       "https://example.com/",
				TransactionID:   "SAM62HFIRW",
			},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest) {
				c.MockRequest(app.txnStatusURL, func() (status int, body string) {
					req := c.requests[1]

					require.Equal(t, "application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					require.Equal(t, wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams TransactionStatusRequest

					err := json.NewDecoder(req.Body).Decode(&reqParams)
					require.NoError(t, err)
					require.NotEmpty(t, reqParams.SecurityCredential) // TODO: verify the security credential

					return http.StatusOK, `{
						"OriginatorConversationID": "2ba8-4165-beca-292db11f9ef878061",
						"ConversationID": "AG_20240122_2010332bae9191b3d522",
						"ResponseCode": "0",
						"ResponseDescription": "Accept the service request successfully."
					}`
				})

				res, err := app.GetTransactionStatus(ctx, initatorPassword, txnStatusReq)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Contains(t, res.ResponseDescription, "Accept the service request successfully")
			},
			requestsCount: 2,
		},
		{
			name: "it generates valid security credentials and makes the request successfully on production",
			env:  Production,
			txnStatusReq: TransactionStatusRequest{
				Initiator:       "testapi",
				Occasion:        "Test",
				PartyA:          "600426",
				QueueTimeOutURL: "https://example.com/",
				Remarks:         "Test remarks",
				ResultURL:       "https://example.com/",
				TransactionID:   "SAM62HFIRW",
			},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest) {
				c.MockRequest(app.txnStatusURL, func() (status int, body string) {
					req := c.requests[1]

					require.Equal(t, "application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					require.Equal(t, wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams TransactionStatusRequest

					err := json.NewDecoder(req.Body).Decode(&reqParams)
					require.NoError(t, err)
					require.NotEmpty(t, reqParams.SecurityCredential) // TODO: verify the security credential

					return http.StatusOK, `{
						"OriginatorConversationID": "2ba8-4165-beca-292db11f9ef878061",
						"ConversationID": "AG_20240122_2010332bae9191b3d522",
						"ResponseCode": "0",
						"ResponseDescription": "Accept the service request successfully."
					}`
				})

				res, err := app.GetTransactionStatus(ctx, initatorPassword, txnStatusReq)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Contains(t, res.ResponseDescription, "Accept the service request successfully")
			},
			requestsCount: 2,
		},
		{
			name: "request fails if no initiator password is provided",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest) {
				res, err := app.GetTransactionStatus(ctx, "", txnStatusReq)
				require.NotNil(t, err)
				require.EqualError(t, err, ErrInvalidInitiatorPassword.Error())
				require.Nil(t, res)
			},
			requestsCount: 1,
		},
		{
			name:         "request fails if invalid queue timeout URL is passed",
			txnStatusReq: TransactionStatusRequest{QueueTimeOutURL: "http://example.com"},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest) {
				res, err := app.GetTransactionStatus(ctx, initatorPassword, txnStatusReq)
				require.NotNil(t, err)
				require.Contains(t, err.Error(), "must use \"https\"")
				require.Nil(t, res)
			},
			requestsCount: 1,
		},
		{
			name: "request fails if invalid queue timeout URL is passed",
			txnStatusReq: TransactionStatusRequest{
				QueueTimeOutURL: "https://example.com",
				ResultURL:       "http://example.com",
			},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest) {
				res, err := app.GetTransactionStatus(ctx, initatorPassword, txnStatusReq)
				require.NotNil(t, err)
				require.Contains(t, err.Error(), "must use \"https\"")
				require.Nil(t, res)
			},
			requestsCount: 1,
		},
		{
			name: "request fails with an error code",
			txnStatusReq: TransactionStatusRequest{
				Initiator:       "testapi",
				Occasion:        "Test",
				PartyA:          "600426",
				QueueTimeOutURL: "https://example.com/",
				Remarks:         "Test remarks",
				ResultURL:       "https://example.com/",
				TransactionID:   "SAM62HFIRW",
			},
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient, txnStatusReq TransactionStatusRequest) {
				c.MockRequest(app.txnStatusURL, func() (status int, body string) {
					return http.StatusBadRequest, `
					{    
					   "requestId": "11728-2929992-1",
					   "errorCode": "401.002.01",
					   "errorMessage": "Error Occurred - Invalid Access Token - BJGFGOXv5aZnw90KkA4TDtu4Xdyf"
					}`
				})

				res, err := app.GetTransactionStatus(ctx, initatorPassword, txnStatusReq)
				require.NotNil(t, err)
				require.Nil(t, res)
				require.Contains(t, err.Error(), "401.002.01")
			},
			requestsCount: 2,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				cl  = newMockHttpClient()
				app = NewApp(cl, testConsumerKey, testConsumerSecret, tc.env)
			)

			cl.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
				{
					"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
					"expires_in": "3599"
				}`
			})

			tc.mock(t, app, cl, tc.txnStatusReq)
			_, err := app.GenerateAccessToken(ctx)
			require.NoError(t, err)
			require.Len(t, cl.requests, tc.requestsCount)
		})
	}
}
