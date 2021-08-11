package config

import (
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
)

func TestInvalidEnvFilePassed(t *testing.T) {
	err := godotenv.Load("")
	assert.Error(t, err)
}

func TestGet(t *testing.T) {
	err := godotenv.Load("../../.env")
	assert.NoError(t, err)

	conf, err := newConfig()
	assert.NoError(t, err)
	assert.NotNil(t, conf)

	c2bShortcode, err := strconv.Atoi(os.Getenv("MPESA_C2B_SHORTCODE"))
	assert.NoError(t, err)
	assert.Equal(t, conf.MpesaC2B.Shortcode, uint(c2bShortcode))
}
