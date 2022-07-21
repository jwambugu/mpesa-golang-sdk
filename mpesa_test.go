package mpesa

import (
	"context"
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
					return http.StatusOK, `
						{
						  "MerchantRequestID": "29115-34620561-1",
						  "CheckoutRequestID": "ws_CO_191220191020363925",
						  "ResponseCode": "0",
						  "ResponseDescription": "Success. Request accepted for processing",
						  "CustomerMessage": "Success. Request accepted for processing"
						}`
				})

				res, err := app.STKPush(ctx, passkey, &stkReq)

				require.NoError(t, err)
				require.NotNil(t, res)
			},
		},
		{
			name: "request fails with an error code",
			stkReq: STKPushRequest{
				TransactionType:  "CustomerPayBillOnline",
				Amount:           10,
				PartyA:           254708374149,
				PartyB:           "174379",
				PhoneNumber:      254708374149,
				CallBackURL:      "https://example.com",
				AccountReference: "Test",
				TransactionDesc:  "Test",
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

				res, err := app.STKPush(ctx, passkey, &stkReq)
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
		name      string
		input     interface{}
		wantError bool
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
			name: "it can unmarshal a unsuccessful transaction callback struct",
			input: STKPushCallback{
				Body: STKPushCallbackBody{
					StkCallback: StkCallback{
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
			if tc.wantError {
				require.Error(t, err)
				require.Nil(t, callback)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, callback)
			require.Equal(t, "ws_CO_191220191020363925", callback.Body.StkCallback.CheckoutRequestID)
		})
	}
}
