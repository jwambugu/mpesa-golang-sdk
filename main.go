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
}
