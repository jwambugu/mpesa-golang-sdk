# About Mpesa Golang SDK

Mpesa golang sdk is an SDK for integrating M-pesa APIS into your golang project. The package currently includes the Lipa
Na Mpesa integration and B2C APIs. More APIs will be added soon.

## Getting Started

In order to use the package, you will need to first create an account at [Daraja](https://developer.safaricom.co.ke).
Once your account has been created, create a test app or switch to a company and select the app to use. Update
your `.env` with the credentials of the selected app.

## Environment Variables

Create `.env` file if none exists

```bash
  touch .env
```

Add the following environment variables to your .env file

`MPESA_C2B_CONSUMER_KEY`

`MPESA_C2B_CONSUMER_SECRET`

`MPESA_C2B_SHORTCODE`

`MPESA_C2B_PASSKEY`

`MPESA_B2C_CONSUMER_KEY`

`MPESA_B2C_CONSUMER_SECRET`

`MPESA_B2C_SHORTCODE`

`MPESA_B2C_INITIATOR_NAME`

`MPESA_B2C_INITIATOR_PASSWORD`

## Installation

```bash 
  go get github.com/jwambugu/mpesa-golang-sdk
```

## Usage/Examples

### Lipa na M-Pesa Online Payment (STK Push)

Lipa na M-Pesa Online Payment API is used to initiate a M-Pesa transaction on behalf of a customer using STK Push.

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
		Shortcode:       conf.MpesaC2B.Shortcode,
		PartyB:          conf.MpesaC2B.Shortcode,
		Passkey:         conf.MpesaC2B.Passkey,
		Amount:          200,
		PhoneNumber:     254700000000,
		ReferenceCode:   "nullable",
		CallbackURL:     "https://local.test", // Add your callback URL here
		TransactionType: "",                   // CustomerPayBillOnline or CustomerBuyGoodsOnline 
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

### Using B2C API

This API enables Business to Customer (B2C) transactions between a company and customers who are the end-users of its
products or services.

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
	mpesaApp := mpesa.Init(conf.MpesaB2C.Credentials, true)

	b2c := conf.MpesaB2C

	response, err := mpesaApp.B2CPayment(&mpesa.B2CPaymentRequest{
		InitiatorName:     b2c.InitiatorName,
		InitiatorPassword: b2c.InitiatorPassword,
		CommandID:         "BusinessPayment", // SalaryPayment or BusinessPayment or PromotionPayment
		Amount:            200,
		Shortcode:         b2c.Shortcode,
		PhoneNumber:       254700000000,
		Remarks:           "Test",
		QueueTimeOutURL:   "https://local.test",
		ResultURL:         "https://local.test",
		Occasion:          "Test",
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

  