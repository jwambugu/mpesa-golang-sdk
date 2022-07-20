package mpesa

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
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
			name: "it generates an access token successfully",
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
