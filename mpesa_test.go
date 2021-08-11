package mpesa

import (
	"encoding/json"
	"github.com/jwambugu/mpesa-golang-sdk/pkg/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInit(t *testing.T) {
	testCases := []struct {
		name           string
		want           string
		isOnProduction bool
	}{
		{
			name:           "SandboxConfig",
			want:           SandboxBaseURL,
			isOnProduction: false,
		},
		{
			name:           "ProductionConfig",
			want:           ProductionBaseURL,
			isOnProduction: true,
		},
	}

	conf := newTestConfig(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newMpesa := Init(conf.MpesaC2B.Credentials, tc.isOnProduction)

			assert.NotNil(t, newMpesa)
			assert.Equal(t, tc.want, newMpesa.BaseURL)
			assert.Equal(t, tc.isOnProduction, newMpesa.IsOnProduction)
			assert.NotNil(t, newMpesa.ConsumerKey)
			assert.NotNil(t, newMpesa.ConsumerSecret)
			assert.NotNil(t, newMpesa.Cache)
		})
	}
}

func TestMpesa_Environment(t *testing.T) {
	testCases := []struct {
		name                string
		expectedEnvironment string
		isOnProduction      bool
	}{
		{
			name:                "IsOnSandboxEnvironment",
			expectedEnvironment: "sandbox",
			isOnProduction:      false,
		},
		{
			name:                "IsOnProductionEnvironment",
			expectedEnvironment: "production",
			isOnProduction:      true,
		},
	}

	conf := newTestConfig(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newMpesa := Init(conf.MpesaC2B.Credentials, tc.isOnProduction)
			assert.NotNil(t, newMpesa)
			assert.Equal(t, tc.isOnProduction, newMpesa.IsOnProduction)

			environment := newMpesa.Environment()

			assert.NotEmpty(t, environment)
			assert.Equal(t, tc.expectedEnvironment, environment)
		})
	}
}

func TestIsValidURL(t *testing.T) {
	testCases := []struct {
		url     string
		isValid bool
	}{
		{url: SandboxBaseURL, isValid: true},
		{url: ProductionBaseURL, isValid: true},
		{url: "localhost", isValid: false},
		{url: "mpesa.test", isValid: false},
		{url: "https://jwambugu.com:9340", isValid: true},
	}

	for _, tc := range testCases {
		isValid, err := isValidURL(tc.url)

		if !tc.isValid {
			assert.Error(t, err)
			assert.Equal(t, tc.isValid, isValid)
			assert.False(t, isValid)
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, tc.isValid, isValid)
		assert.True(t, isValid)
	}
}

func TestMakeRequest(t *testing.T) {
	expected := map[string]string{
		"name": "test",
	}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(expected)
	}))

	req, err := http.NewRequest(http.MethodPost, svr.URL, nil)

	response, err := makeRequest(req)
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.Equal(t, "application/json", req.Header.Get("Accept"))

	var responseBody map[string]string

	err = json.Unmarshal(response, &responseBody)
	assert.NoError(t, err)
	assert.Equal(t, expected, responseBody)
}

func TestMpesa_getAccessToken(t *testing.T) {
	conf := newTestConfig(t)

	testCases := []struct {
		name        string
		credentials *config.Credentials
		isValid     bool
	}{
		{
			name:        "HasValidCredentials",
			credentials: conf.MpesaC2B.Credentials,
			isValid:     true,
		},
		{
			name: "HasInvalidCredentials",
			credentials: &config.Credentials{
				ConsumerKey:    "",
				ConsumerSecret: "",
			},
			isValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := Init(tc.credentials, false)

			token, err := m.getAccessToken()

			cachedToken, exists := m.cachedAccessToken()

			if !tc.isValid {
				assert.Error(t, err)
				assert.Empty(t, token)

				assert.False(t, exists)
				assert.Empty(t, cachedToken)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, token)
			assert.True(t, exists)
			assert.NotEmpty(t, cachedToken)
		})
	}
}
