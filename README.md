# mpesa-golang-sdk

mpesa-golang-sdk is and SDK for intergrating M-pesa into your golang project. The package currently includes the Lipa Na
Mpesa integration. More API will be added soon.

## Getting Started

In order to use the package, you will need to first create an account at [Daraja](https://developer.safaricom.co.ke).
Once your account has been created, create a test app or switch to a company and select the app to use. Update
your `.env` with the credentials of the selected app.

Create `.env` file if none exists

```bash
  touch .env
```

Fill in the environment variables.

```
MPESA_C2B_CONSUMER_KEY=
MPESA_C2B_CONSUMER_SECRET=
MPESA_C2B_SHORTCODE=
MPESA_C2B_PASSKEY=
```

## Installation

```bash 
  go get github.com/jwambugu/mpesa-golang-sdk
```

## Usage/Examples

```go
package main

import (
	"github.com/jwambugu/mpesa-golang-sdk"
	"github.com/jwambugu/mpesa-golang-sdk/pkg/config"
	"log"
)

func main() {
	// Get the mpesa configuration
	conf, err := config.Get()

	if err != nil {
		log.Fatalln(err)
	}

	// Initialize a new mpesa app
	mpesaApp := mpesa.Init(conf.MpesaC2B.Credentials, false)

	// Make the Lipa na Mpesa online request
	response, err := mpesaApp.LipaNaMpesaOnline(&mpesa.STKPushRequest{
		Shortcode:     conf.MpesaC2B.Shortcode.Shortcode,
		PartyB:        conf.MpesaC2B.Shortcode.Shortcode,
		Passkey:       conf.MpesaC2B.Shortcode.Passkey,
		Amount:        200,
		PhoneNumber:   254700000000,
		ReferenceCode: "nullable",
		CallbackURL:   "https://local.test", // Add your callback URL here
	})

	if err != nil {
		log.Fatalln(err)
	}

	// Check if the request was successful
	if response.IsSuccessful {
		// Handle your successful logic here 
	}
}
```

  