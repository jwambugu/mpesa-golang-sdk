# [WIP]: About Mpesa Golang SDK

Mpesa Golang SDK facilitates in integrating M-pesa APIS into your go project. The following APIs are currently supported:

| API                                                                                       | Description                                                                                                                                                                                                                                                                              |
|-------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Authorization](https://developer.safaricom.co.ke/APIs/Authorization)                     | Generates an access token for authenticating APIs                                                                                                                                                                                                                                        |
| [Lipa Na M-Pesa Online API](https://developer.safaricom.co.ke/APIs/MpesaExpressSimulate)  | Initiates online payment on behalf of a customer.                                                                                                                                                                                                                                        |
| [Business To Customer  (B2C) ](https://developer.safaricom.co.ke/APIs/BusinessToCustomer) | Transact between an M-Pesa short code to a phone number registered on M-Pesa.                                                                                                                                                                                                            |
| [M-Pesa Express Query](https://developer.safaricom.co.ke/APIs/MpesaExpressQuery)          | Check the status of a Lipa Na M-Pesa Online Payment.                                                                                                                                                                                                                                     |
| [Dynamic QR](https://developer.safaricom.co.ke/APIs/DynamicQRCode)                        | Generates a dynamic M-PESA QR Code which enables Safaricom M-PESA customers who have My Safaricom App or M-PESA app, to scan a QR (Quick Response) code, to capture till number and amount then authorize to pay for goods and services at select LIPA NA M-PESA (LNM) merchant outlets. |
| [Transaction Status](https://developer.safaricom.co.ke/APIs/TransactionStatus)            | Check the status of a transaction.                                                                                                                                                                                                                                                       |
| [Account Balance](https://developer.safaricom.co.ke/APIs/AccountBalance)                  | Enquire the balance on an M-Pesa BuyGoods (Till Number).                                                                                                                                                                                                                                 |
| [Business Pay Bill](https://developer.safaricom.co.ke/APIs/BusinessPayBill)               | This API enables you to pay bills directly from your business account to a pay bill number, or a paybill store.                                                                                                                                                                          |

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

More examples can be found [here](https://github.com/jwambugu/mpesa-golang-sdk/tree/main/examples)

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

	mpesaApp := mpesa.NewApp(http.DefaultClient, "CONSUMER_KEY", "CONSUMER_SECRET", mpesa.EnvironmentSandbox)

	stkResp, err := mpesaApp.STKPush(ctx, "YOUR_PASSKEY", mpesa.STKPushRequest{
		BusinessShortCode: 174379,
		TransactionType:   mpesa.CustomerPayBillOnlineTransactionType,
		Amount:            10,
		PartyA:            254708374149,
		PartyB:            174379,
		PhoneNumber:       254708374149,
		CallBackURL:       "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
		AccountReference:  "Test reference",
		TransactionDesc:   "Test description",
	})

	if err != nil {
		log.Fatalf("stk: %v\n", err)
	}

	log.Printf("stk: %+v\n", stkResp)

	stkQueryRes, err := mpesaApp.STKQuery(ctx, "YOUR_PASSKEY", mpesa.STKQueryRequest{
		BusinessShortCode: 174379,
		CheckoutRequestID: "ws_CO_260520211133524545",
	})

	if err != nil {
		log.Fatalf("stk query: %v\n", err)
	}

	log.Printf("stk query %+v\n", stkQueryRes)

}
```

### Processing Callbacks
The SDK adds a helper functions to decode callbacks. These are:
1. `mpesa.UnmarshalSTKPushCallback(v)`
2. `mpesa.UnmarshalCallback(v)` for all other callbacks received

```go
mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
callback, err := mpesa.UnmarshalB2CCallback(r.Body)
    if err != nil {
        log.Fatalln(err)
    }
    
    log.Printf("%+v", callback)
})


data := strings.NewReader(`
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

callback, err := mpesa.UnmarshalSTKPushCallback(data)ack()

if err != nil {
    log.Fatalln(err)
}

log.Printf("%+v", callback)
```
