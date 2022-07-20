package mpesa

import (
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

				token, err := app.GenerateAccessToken()
				require.NoError(t, err)
				require.NotEmpty(t, token)
				require.Equal(t, token, app.cache[testConsumerKey].AccessToken)

				// Make subsequent call to get the token from the cache
				token, err = app.GenerateAccessToken()
				require.NoError(t, err)
				require.Equal(t, token, app.cache[testConsumerKey].AccessToken)
			},
		},
		{
			name: "it fails to generate an access token",
			mock: func(t *testing.T, app *Mpesa, c *mockHttpClient) {
				c.MockRequest(app.authURL, func() (status int, body string) {
					return http.StatusBadRequest, ``
				})

				token, err := app.GenerateAccessToken()
				require.NotNil(t, err)
				require.Empty(t, token)
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

				token, err := app.GenerateAccessToken()
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
				token, err = app.GenerateAccessToken()
				require.NoError(t, err)
				require.Equal(t, token, app.cache[testConsumerKey].AccessToken)
				require.NotEqual(t, oldToken, app.cache[testConsumerKey].AccessToken)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cl := newMockHttpClient()
			app := NewApp(cl, testConsumerKey, testConsumerSecret, Sandbox)

			tc.mock(t, app, cl)
		})
	}
}
