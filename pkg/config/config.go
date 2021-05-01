package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
)

type (
	// Credentials contains the keys to be used to authenticate the app on M-Pesa
	Credentials struct {
		ConsumerKey    string `json:"consumer_key"`
		ConsumerSecret string `json:"consumer_secret"`
	}

	// Shortcode contains the data for either a C2B or B2C shortcode
	Shortcode struct {
		Shortcode string
		Passkey   string
	}

	MpesaC2B struct {
		Credentials *Credentials
		Shortcode   *Shortcode
	}

	// Config stores the configuration keys we need to run the app
	Config struct {
		// MpesaC2B is the shortcode used for C2B transactions
		MpesaC2B *MpesaC2B
	}
)

// newConfig creates and returns a new Config
func newConfig() *Config {
	return &Config{
		MpesaC2B: &MpesaC2B{
			Credentials: &Credentials{
				ConsumerKey:    os.Getenv("MPESA_C2B_CONSUMER_KEY"),
				ConsumerSecret: os.Getenv("MPESA_C2B_CONSUMER_SECRET"),
			},
			Shortcode: &Shortcode{
				Shortcode: os.Getenv("MPESA_C2B_SHORTCODE"),
				Passkey:   os.Getenv("MPESA_C2B_PASSKEY"),
			},
		},
	}

}

// Get reads from the .env file and create a new Config
func Get() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config.Get.LoadEnvFile:: %v", err)
	}

	conf := newConfig()

	return conf, nil
}
