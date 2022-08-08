# About Mpesa Golang SDK

Mpesa Golang SDK facilitates in integrating M-pesa APIS into your go project. The following APIs are currently supported:

| API                                                                                       | Description                                                                   |
|-------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| [Authorization](https://developer.safaricom.co.ke/APIs/Authorization)                     | Generates an access token for authenticating APIs                             |
| [Lipa Na M-Pesa Online API](https://developer.safaricom.co.ke/APIs/MpesaExpressSimulate)  | Initiates online payment on behalf of a customer.                             |
| [Business To Customer  (B2C) ](https://developer.safaricom.co.ke/APIs/BusinessToCustomer) | Transact between an M-Pesa short code to a phone number registered on M-Pesa. |
| [M-Pesa Express Query](https://sandbox.safaricom.co.ke/mpesa/stkpushquery/v1/query)       | Check the status of a Lipa Na M-Pesa Online Payment.                          |

## Getting Started

To use the APIs, follow these steps:

1. Register or login to your account on [Daraja](https://developer.safaricom.co.ke/)
2. Create a new or view existing apps [here](https://developer.safaricom.co.ke/MyApps)
3. Copy the app credentials. To prevent exposing you API keys, you can store them on configuration file such as `.env` or `config.yml`.


## Installation

```bash 
  go get github.com/jwambugu/mpesa-golang-sdk
```

## Usage/Examples

### Environments
The SDK supports the following environments:

1. `mpesa.Sandbox` for test environment.
2. `mpesa.Production` for production environment once you go live.

### Examples

```go
package main

import (
    "context"
    "github.com/jwambugu/mpesa-golang-sdk"
    "log"
    "net/http"
    "time"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    mpesaApp := mpesa.NewApp(http.DefaultClient, "CONSUMER_KEY_GOES_HERE", "CONSUMER_SECRET_GOES_HERE", mpesa.Sandbox)
    
    stkPushRes, err := mpesaApp.STKPush(ctx, "PASSKEY_GOES_HERE", mpesa.STKPushRequest{
        BusinessShortCode: 174379,
        TransactionType:   "CustomerPayBillOnline",
        Amount:            10,
        PartyA:            254708374149,
        PartyB:            174379,
        PhoneNumber:       254708374149,
        CallBackURL:       "https://example.com",
        AccountReference:  "Test",
        TransactionDesc:   "Test Request",
    })
    
    if err != nil {
        log.Fatalln(err)
    }
    
    log.Printf("%+v", stkPushRes)
    
    b2cRes, err := mpesaApp.B2C(ctx, "INITIATOR_PASSWORD_GOES_HERE", mpesa.B2CRequest{
        InitiatorName:   "TestG2Init",
        CommandID:       "BusinessPayment",
        Amount:          10,
        PartyA:          600123,
        PartyB:          254728762287,
        Remarks:         "This is a remark",
        QueueTimeOutURL: "https://example.com",
        ResultURL:       "https://example.com",
        Occasion:        "Test Occasion",
    })
    
    if err != nil {
        log.Fatalln(err)
    }
    
    log.Printf("%+v", b2cRes)

    stkQueryRes, err := mpesaApp.STKPushQuery(ctx, "PASSKEY_GOES_HERE", mpesa.STKPushQueryRequest{
        BusinessShortCode: 174379,
        CheckoutRequestID: "ws_CO_03082022131319635708374149", // STK PUSH CheckoutRequestID,
    })
    
    if err != nil {
        log.Fatalln(err)
    }
    
    log.Printf("%+v", stkQueryRes)
}
```

### Processing Callbacks
The SDK adds a helper functions to decode callbacks. These are:
1. `mpesa.UnmarshalSTKPushCallback(v)`
2. `mpesa.UnmarshalB2CCallback(v)`

The following types are supported, any other type will be decoded using `json.Unmarshal`

| Supported Types |
|-----------------|
| string          |
| *http.Request   |

```go
mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
    callback, err := mpesa.UnmarshalB2CCallback(r)
    if err != nil {
        log.Fatalln(err)
    }
    
    log.Printf("%+v", callback)
})


callback, err := mpesa.UnmarshalSTKPushCallback(`
{    
   "Body": {
      "stkCallback": {
         "MerchantRequestID": "29115-34620561-1",
         "CheckoutRequestID": "ws_CO_191220191020363925",
         "ResultCode": 1032,
         "ResultDesc": "Request cancelled by user."
      }
   }
}`)

if err != nil {
    log.Fatalln(err)
}

log.Printf("%+v", callback)
```
