package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strconv"
)

type (
	// Credentials contains the keys to be used to authenticate the app on M-Pesa
	Credentials struct {
		ConsumerKey    string `json:"consumer_key"`
		ConsumerSecret string `json:"consumer_secret"`
	}

	// Shortcode contains the data for either a C2B or B2C shortcode
	Shortcode struct {
		Shortcode         uint
		Passkey           string
		Credentials       *Credentials
		InitiatorName     string
		InitiatorPassword string
	}

	// Config stores the configuration keys we need to run the app
	Config struct {
		// MpesaC2B is the shortcode used for C2B transactions
		MpesaC2B *Shortcode
		// MpesaB2C is the shortcode used for C2B transactions
		MpesaB2C *Shortcode
	}
)

// newConfig creates and returns a new Config
func newConfig() (*Config, error) {
	c2bShortcode, err := strconv.Atoi(os.Getenv("MPESA_C2B_SHORTCODE"))

	if err != nil {
		return nil, fmt.Errorf("config.Get.InvalidC2BShortcode:: %v", err)
	}

	b2CShortcode, err := strconv.Atoi(os.Getenv("MPESA_B2C_SHORTCODE"))

	if err != nil {
		return nil, fmt.Errorf("config.Get.InvalidB2CShortcode:: %v", err)
	}

	conf := &Config{
		MpesaC2B: &Shortcode{
			Shortcode: uint(c2bShortcode),
			Passkey:   os.Getenv("MPESA_C2B_PASSKEY"),
			Credentials: &Credentials{
				ConsumerKey:    os.Getenv("MPESA_C2B_CONSUMER_KEY"),
				ConsumerSecret: os.Getenv("MPESA_C2B_CONSUMER_SECRET"),
			},
		},
		MpesaB2C: &Shortcode{
			Shortcode: uint(b2CShortcode),
			Credentials: &Credentials{
				ConsumerKey:    os.Getenv("MPESA_B2C_CONSUMER_KEY"),
				ConsumerSecret: os.Getenv("MPESA_B2C_CONSUMER_SECRET"),
			},
			InitiatorName:     os.Getenv("MPESA_B2C_INITIATOR_NAME"),
			InitiatorPassword: os.Getenv("MPESA_B2C_INITIATOR_PASSWORD"),
		},
	}

	return conf, nil
}

// Get reads from the .env file and create a new Config
func Get() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config.Get.LoadEnvFile:: %v", err)
	}

	conf, err := newConfig()

	if err != nil {
		return nil, err
	}

	return conf, nil
}
