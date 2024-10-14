package main

import (
	"fmt"
	"log"

	"github.com/nedaZarei/Cloud_ImageProcessingService/RequestRegistration/config"
	"github.com/nedaZarei/Cloud_ImageProcessingService/RequestRegistration/service"
)

func main() {
	cfg, err := config.InitConfig("./config/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	fmt.Println(cfg)

	//creating a new instance of service 1
	imageGettingService := service.NewService(cfg)

	if err := imageGettingService.StartService(); err != nil {
		log.Fatalf("failed to start service one: %v", err)
	}
}
