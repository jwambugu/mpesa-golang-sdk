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

	txnStatusRes, err := mpesaApp.GetTransactionStatus(ctx, "Safaricom999!*!", mpesa.TransactionStatusRequest{
		IdentifierType:           mpesa.ShortcodeIdentifierType,
		Initiator:                "YOUR_INITIATOR_NAME",
		Occasion:                 "Test occassion",
		OriginatorConversationID: "",
		PartyA:                   600426,
		QueueTimeOutURL:          "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
		Remarks:                  "Test remarks",
		ResultURL:                "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
		TransactionID:            "SB162HIYLY",
	})

	if err != nil {
		log.Fatalf("txn status: %v\n", err)
	}

	log.Printf("txn status: %+v\n", txnStatusRes)

	b2cRes, err := mpesaApp.B2C(ctx, "Safaricom999!*!", mpesa.B2CRequest{
		InitiatorName:   "YOUR_INITIATOR_NAME",
		CommandID:       mpesa.BusinessPaymentCommandID,
		Amount:          10,
		PartyA:          600426,
		PartyB:          254708374149,
		Remarks:         "This is a remark",
		QueueTimeOutURL: "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
		ResultURL:       "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
		Occasion:        "Test Occasion",
	})

	if err != nil {
		log.Fatalf("b2c: %v\n", err)
	}

	log.Printf("b2c: %+v\n", b2cRes)

	accBal, err := mpesaApp.GetAccountBalance(ctx, "Safaricom999!*!", mpesa.AccountBalanceRequest{
		Initiator:       "YOUR_INITIATOR_NAME",
		PartyA:          600981,
		QueueTimeOutURL: "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
		Remarks:         "Test Local",
		ResultURL:       "https://webhook.site/62daf156-31dc-4b07-ac41-698dbfadaa4b",
	})

	if err != nil {
		log.Fatalf("account balance: %v\n", err)
	}

	log.Printf("account balance: %+v\n", accBal)
}
