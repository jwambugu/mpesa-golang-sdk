package main

import (
	"fmt"
	"gitlab.com/jwambugu/go-mpesa/pkg/config"
	"gitlab.com/jwambugu/go-mpesa/pkg/mpesa"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
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

	fmt.Println(mpesaService.Environment())

	s := strconv.Itoa(254708666389)
	fmt.Println(s[:3])

	u, err := url.ParseRequestURI("https://127.0.0.1:4040")

	if err != nil {
		log.Fatalln(err)
	}

	address := net.ParseIP(u.Host)

	log.Println("url-info", "host", address)

	if address == nil {
		log.Println("url-info", "host", u.Host)

		fmt.Println(strings.Contains(u.Host, "."))
	}

	fmt.Println(u)

	response, err := mpesaService.LipaNaMpesaOnline(&mpesa.STKPushRequest{
		Shortcode:     0,
		Passkey:       "",
		Amount:        2,
		PhoneNumber:   0,
		ReferenceCode: "",
		CallbackURL:   "",
	})

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(fmt.Printf("%+v", response))

}
