package mpesa

import (
	"github.com/stretchr/testify/assert"
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

			assert.NotNil(t, environment)
			assert.Equal(t, tc.expectedEnvironment, environment)
		})
	}
}
