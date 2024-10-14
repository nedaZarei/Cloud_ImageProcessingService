package main

import (
	"fmt"
	"log"

	"github.com/nedaZarei/Cloud_ImageProcessingService/ImageCaptioner/config"
	"github.com/nedaZarei/Cloud_ImageProcessingService/ImageCaptioner/service"
)

func main() {
	cfg, err := config.InitConfig("./config/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	fmt.Println(cfg)

	//creating a new instance of service 2
	imageCaptioningService := service.NewService(cfg)

	if err := imageCaptioningService.StartService(); err != nil {
		log.Fatalf("failed to start service one: %v", err)
	}
}
