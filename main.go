package main

import (
	"encoding/json"
	"fmt"
	"github.com/jwambugu/go-mpesa/pkg/config"
	"github.com/jwambugu/go-mpesa/pkg/mpesa"

	"io/ioutil"
	"log"
	"net/http"
)

func stkPushCallback(w http.ResponseWriter, r *http.Request) {
	var c *mpesa.LipaNaMpesaOnlineCallback

	payload, err := ioutil.ReadAll(r.Body)

	if err != nil {
		log.Fatalln(err)
	}

	if err := json.Unmarshal(payload, &c); err != nil {
		log.Fatalln(err)
	}

	fmt.Println(fmt.Sprintf("%+v", c))
}

func main() {
	conf, err := config.Get()

	if err != nil {
		log.Fatalln(err)
	}

	//fmt.Println(fmt.Printf("%+v", conf.MpesaC2B.Credentials))

	mpesaService := mpesa.Init(conf.MpesaC2B.Credentials, false)

	//fmt.Println(fmt.Printf("%+v", mpesaService))

	token, err := mpesaService.GetAccessToken()

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(token)

	//response, err := mpesaService.LipaNaMpesaOnline(&mpesa.STKPushRequest{
	//	Shortcode:     conf.MpesaC2B.Shortcode.Shortcode,
	//	PartyB:        conf.MpesaC2B.Shortcode.Shortcode,
	//	Passkey:       conf.MpesaC2B.Shortcode.Passkey,
	//	Amount:        2,
	//	PhoneNumber:   254708666389,
	//	ReferenceCode: "nullable",
	//	CallbackURL:   "https://local.test",
	//})
	//
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	//fmt.Println(fmt.Printf("%+v", response))

	http.HandleFunc("/stk-callback", stkPushCallback)
	log.Fatalln(http.ListenAndServe(":3000", nil))
}
