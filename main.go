package main

import (
	"fmt"
	"gitlab.com/jwambugu/go-mpesa/pkg/config"
	"log"
)

func main() {
	conf, err := config.Get()

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(conf)
}
