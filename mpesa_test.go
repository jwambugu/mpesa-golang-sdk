package mpesa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"
)

const (
	testConsumerKey    = "W6vRBOiKRSYZpXABQjXf9W3+KR+tGWGKTrOpOhnfig"
	testConsumerSecret = "MmE8/5EW3XXBIKg4qpDJ8g"
)

func TestMpesa_GenerateAccessToken(t *testing.T) {
	tests := []struct {
		name string
		mock func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient)
	}{
		{
			name: "it generates and caches an access token successfully",
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusOK, `
						{
						"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
						"expires_in": "3599"
						}`
				})

				token, err := app.GenerateAccessToken(ctx)
				require.NoError(t, err)
				require.NotEmpty(t, token)
				require.Equal(t, token, app.cache[testConsumerKey].AccessToken)

				// Make subsequent call to get the token from the cache
				token, err = app.GenerateAccessToken(ctx)
				require.NoError(t, err)
				require.Equal(t, token, app.cache[testConsumerKey].AccessToken)
			},
		},
		{
			name: "it fails to generate an access token",
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusBadRequest, ``
				})

				token, err := app.GenerateAccessToken(ctx)
				require.NotNil(t, err)
				require.Empty(t, token)
			},
		},
		{
			name: "it flushes and generates a new access token successfully",
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient) {
				oldToken := "0A0v8OgxqqoocblflR58m9chMdnU"

				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusOK, `
						{
						"access_token": "` + oldToken + `",
						"expires_in": "3599"
						}`
				})

				token, err := app.GenerateAccessToken(ctx)
				require.NoError(t, err)
				require.NotEmpty(t, token)

				gotCachedData := app.cache[testConsumerKey]
				require.Equal(t, token, gotCachedData.AccessToken)

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
				require.NoError(t, err)
				require.Equal(t, token, app.cache[testConsumerKey].AccessToken)
				require.NotEqual(t, oldToken, app.cache[testConsumerKey].AccessToken)
			},
		},
		{
			name: "it fails with 404 if invalid url is passed",
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.stkPushURL, func() (status int, body string) {
					return http.StatusNotFound, ``
				})

				token, err := app.GenerateAccessToken(ctx)
				require.NotNil(t, err)
				require.Empty(t, token)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cl := newMockHttpClient()
			app := NewApp(cl, testConsumerKey, testConsumerSecret, Sandbox)

			ctx := context.Background()
			tc.mock(t, ctx, app, cl)
		})
	}
}

func TestMpesa_LipaNaMpesaOnline(t *testing.T) {
	tests := []struct {
		name   string
		stkReq STKPushRequest
		mock   func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, stkReq STKPushRequest)
	}{
		{
			name: "it makes stk push request successfully",
			stkReq: STKPushRequest{
				BusinessShortCode: "174379",
				TransactionType:   "CustomerPayBillOnline",
				Amount:            10,
				PartyA:            254708374149,
				PartyB:            "174379",
				PhoneNumber:       254708374149,
				CallBackURL:       "https://example.com",
				AccountReference:  "Test",
				TransactionDesc:   "Test",
			},
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, stkReq STKPushRequest) {
				passkey := "passkey"

				c.MockRequest(app.stkPushURL, func() (status int, body string) {
					req := c.requests[1]

					require.Equal(t, "application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					require.Equal(t, wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams STKPushRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					require.NoError(t, err)

					timestamp := time.Now().Format("20060102150405")
					wantPassword := fmt.Sprintf("%s%s%s", stkReq.BusinessShortCode, passkey, timestamp)

					gotPassword := make([]byte, base64.StdEncoding.DecodedLen(len(reqParams.Password)))
					n, err := base64.StdEncoding.Decode(gotPassword, []byte(reqParams.Password))
					require.NoError(t, err)
					require.Equal(t, wantPassword, string(gotPassword[:n]))

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

				require.NoError(t, err)
				require.NotNil(t, res)
			},
		},
		{
			name: "request fails with an error code",
			stkReq: STKPushRequest{
				BusinessShortCode: "",
				TransactionType:   "CustomerPayBillOnline",
				Amount:            10,
				PartyA:            254708374149,
				PartyB:            "174379",
				PhoneNumber:       254708374149,
				CallBackURL:       "https://example.com",
				AccountReference:  "Test",
				TransactionDesc:   "Test",
			},
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, stkReq STKPushRequest) {
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
				require.Error(t, err)
				require.Nil(t, res)
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

			ctx := context.Background()
			tc.mock(t, ctx, app, cl, tc.stkReq)

			_, err := app.GenerateAccessToken(ctx)
			require.NoError(t, err)
			require.Len(t, cl.requests, 2)
		})
	}
}

func TestUnmarshalSTKPushCallback(t *testing.T) {
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
			require.NoError(t, err)
			require.NotNil(t, callback)
			require.Equal(t, "ws_CO_191220191020363925", callback.Body.STKCallback.CheckoutRequestID)
		})
	}
}

func TestMpesa_B2C(t *testing.T) {
	tests := []struct {
		name   string
		b2cReq B2CRequest
		env    Environment
		mock   func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest)
	}{
		{
			name: "it makes a b2c request on sandbox successfully",
			b2cReq: B2CRequest{
				InitiatorName:      "TestG2Init",
				SecurityCredential: "BusinessPayment",
				CommandID:          "",
				Amount:             10,
				PartyA:             "600123",
				PartyB:             "254728762287",
				Remarks:            "This is a remark",
				QueueTimeOutURL:    "https://example.com",
				ResultURL:          "https://example.com",
				Occasion:           "Test Occasion",
			},
			env: Sandbox,
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest) {
				c.MockRequest(app.b2cURL, func() (status int, body string) {
					req := c.requests[1]

					require.Equal(t, "application/json", req.Header.Get("Content-Type"))
					wantAuthorizationHeader := `Bearer ` + app.cache[testConsumerKey].AccessToken
					require.Equal(t, wantAuthorizationHeader, req.Header.Get("Authorization"))

					var reqParams B2CRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					require.NoError(t, err)
					require.NotEmpty(t, reqParams.SecurityCredential)
					require.Equal(t, b2cReq.InitiatorName, reqParams.InitiatorName)

					return http.StatusOK, `
					{    
					 "ConversationID": "AG_20191219_00005797af5d7d75f652",    
					 "OriginatorConversationID": "16740-34861180-1",    
					 "ResponseCode": "0",    
					 "ResponseDescription": "Accept the service request successfully."
					}`
				})

				res, err := app.B2C(ctx, "random-string", b2cReq)
				require.NoError(t, err)
				require.NotNil(t, res)
			},
		},
		{
			name: "it makes a b2c request on production successfully",
			b2cReq: B2CRequest{
				InitiatorName:      "TestG2Init",
				SecurityCredential: "BusinessPayment",
				CommandID:          "",
				Amount:             10,
				PartyA:             "600123",
				PartyB:             "254728762287",
				Remarks:            "This is a remark",
				QueueTimeOutURL:    "https://example.com",
				ResultURL:          "https://example.com",
				Occasion:           "Test Occasion",
			},
			env: Production,
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest) {
				c.MockRequest(app.b2cURL, func() (status int, body string) {
					req := c.requests[1]

					var reqParams B2CRequest
					err := json.NewDecoder(req.Body).Decode(&reqParams)
					require.NoError(t, err)
					require.NotEmpty(t, reqParams.SecurityCredential)

					return http.StatusOK, `
					{    
					 "ConversationID": "AG_20191219_00005797af5d7d75f652",    
					 "OriginatorConversationID": "16740-34861180-1",    
					 "ResponseCode": "0",    
					 "ResponseDescription": "Accept the service request successfully."
					}`
				})

				res, err := app.B2C(ctx, "random-string", b2cReq)
				require.NoError(t, err)
				require.NotNil(t, res)
			},
		},
		{
			name: "request fails with an error code",
			b2cReq: B2CRequest{
				InitiatorName:      "",
				SecurityCredential: "BusinessPayment",
				CommandID:          "",
				Amount:             10,
				PartyA:             "600123",
				PartyB:             "254728762287",
				Remarks:            "This is a remark",
				QueueTimeOutURL:    "https://example.com",
				ResultURL:          "https://example.com",
				Occasion:           "Test Occasion",
			},
			env: Production,
			mock: func(t *testing.T, ctx context.Context, app *Mpesa, c *mockHttpClient, b2cReq B2CRequest) {
				c.MockRequest(app.b2cURL, func() (status int, body string) {
					return http.StatusBadRequest, `
					{    
					   "requestId": "11728-2929992-1",
					   "errorCode": "401.002.01",
					   "errorMessage": "Error Occurred - Invalid Access Token - BJGFGOXv5aZnw90KkA4TDtu4Xdyf"
					}`
				})

				res, err := app.B2C(ctx, "random-string", b2cReq)
				require.Error(t, err)
				require.Nil(t, res)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cl := newMockHttpClient()
			app := NewApp(cl, testConsumerKey, testConsumerSecret, tc.env)

			cl.MockRequest(app.authURL, func() (status int, body string) {
				return http.StatusOK, `
						{
						"access_token": "0A0v8OgxqqoocblflR58m9chMdnU",
						"expires_in": "3599"
						}`
			})

			ctx := context.Background()
			tc.mock(t, ctx, app, cl, tc.b2cReq)

			_, err := app.GenerateAccessToken(ctx)
			require.NoError(t, err)
			require.Len(t, cl.requests, 2)
		})
	}
}

func TestUnmarshalB2CCallback(t *testing.T) {
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
			require.NoError(t, err)
			require.NotNil(t, callback)
			require.Equal(t, "AG_20191219_00004e48cf7e3533f581", callback.Result.ConversationID)
			require.Equal(t, "QueueTimeoutURL", callback.Result.ReferenceData.ReferenceItem.Key)
		})
	}
}
