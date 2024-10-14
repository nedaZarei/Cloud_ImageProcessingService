package main

import (
	"fmt"
	"log"

	"github.com/nedaZarei/Cloud_ImageProcessingService/CaptionToImageService/config"
	"github.com/nedaZarei/Cloud_ImageProcessingService/CaptionToImageService/service"
)

func main() {
	cfg, err := config.InitConfig("./config/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	fmt.Println(cfg)

	//creating a new instance of service 3
	ImageGenerationService := service.NewService(cfg)

	if err := ImageGenerationService.StartService(); err != nil {
		log.Fatalf("failed to start service one: %v", err)
	}
}
