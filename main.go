package main

import (
	"fmt"
	"gitlab.com/jwambugu/go-mpesa/pkg/config"
	"gitlab.com/jwambugu/go-mpesa/pkg/mpesa"
	"log"
)

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

	response, err := mpesaService.LipaNaMpesaOnline(&mpesa.STKPushRequest{
		Shortcode:     conf.MpesaC2B.Shortcode.Shortcode,
		PartyB:        conf.MpesaC2B.Shortcode.Shortcode,
		Passkey:       conf.MpesaC2B.Shortcode.Passkey,
		Amount:        2,
		PhoneNumber:   254708666389,
		ReferenceCode: "nullable",
		CallbackURL:   "https://local.test",
	})

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(fmt.Printf("%+v", response))

}
