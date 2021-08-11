package mpesa

import (
	"github.com/jwambugu/mpesa-golang-sdk/pkg/config"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func newTestConfig(t *testing.T) *config.Config {
	// Get the mpesa configuration
	conf, err := config.Get()
	assert.NoError(t, err)

	assert.NotNil(t, conf)
	return conf
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
