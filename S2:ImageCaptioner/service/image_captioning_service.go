package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/nedaZarei/Cloud_ImageProcessingService/ImageCaptioner/config"
	"github.com/nedaZarei/Cloud_ImageProcessingService/ImageCaptioner/pkg/db"
)

type Service struct {
	cfg             *config.Config
	RequestDatabase db.ImageRequestDatabase
	rabbitMQClient  *amqp.Channel
	minioClient     *minio.Client
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		cfg: cfg}
}

func (s *Service) StartService() error {
	//db init
	dB, err := sqlx.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		s.cfg.Postgres.Host, s.cfg.Postgres.Port, s.cfg.Postgres.Username, s.cfg.Postgres.Password, s.cfg.Postgres.Database))
	if err != nil {
		return fmt.Errorf("failed to connect to Postgres: %v", err)
	}
	log.Println("connected to Postgres")
	s.RequestDatabase = db.NewImageRequestDB(dB)

	//rabbitMQ init
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/",
		s.cfg.RabbitMQ.Username, s.cfg.RabbitMQ.Password, s.cfg.RabbitMQ.Host, s.cfg.RabbitMQ.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}
	log.Println("Connected to RabbitMQ")
	s.rabbitMQClient, err = conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %v", err)
	}

	//minio init
	s.minioClient, err = minio.New(s.cfg.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.cfg.Minio.AccessKey, s.cfg.Minio.SecretKey, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("failed to init Minio client: %v", err)
	}
	log.Println("connected to Minio")

	//start consuming messages from RabbitMQ
	s.consumeImageRequests()

	return nil
}

func (s *Service) consumeImageRequests() {
	msgs, err := s.rabbitMQClient.Consume(
		"image_requests", // queue
		"",               // consumer
		true,             // auto-ack
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		log.Fatalf("failed to register a consumer: %v", err)
	}
	go func() {
		for d := range msgs {
			log.Printf("received a message: %s", d.MessageId)
			id, err := strconv.Atoi(d.MessageId)
			if err != nil {
				log.Printf("failed to convert message id to int: %v", err)
				continue
			}
			caption, err := s.processImage(id)
			if err != nil {
				log.Printf("failed to generate caption: %v", err)
				continue
			}
			//updating request status
			err = s.RequestDatabase.SetRequestReady(context.Background(), id, caption)
			if err != nil {
				log.Printf("failed to update request status: %v", err)
				continue
			}
		}
	}()
}

func (s *Service) processImage(imageID int) (string, error) {

	//retrievign the image from Minio
	imageData, err := s.minioClient.GetObject(context.Background(), s.cfg.Minio.Bucket, strconv.Itoa(imageID), minio.GetObjectOptions{})
	if err != nil {
		return "", err

	}
	defer imageData.Close()

	// converting image to byte array
	imageBytes := new(bytes.Buffer)
	if _, err := io.Copy(imageBytes, imageData); err != nil {
		log.Printf("Failed to read image data: %v", err)
		return "", err
	}

	//equesting a caption from Hugging Face API
	caption, err := s.getCaption(imageBytes.Bytes())
	if err != nil {
		log.Printf("Failed to generate caption: %v", err)
		return "", err
	}

	err = s.RequestDatabase.SetRequestReady(context.Background(), imageID, caption)
	if err != nil {
		log.Printf("failed to update request status: %v", err)
		return "", err
	}

	return caption, nil
}

func (s *Service) getCaption(imageData []byte) (string, error) {
	req, err := http.NewRequest("POST", s.cfg.HuggingFace.URL, bytes.NewBuffer(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.HuggingFace.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API responded with status: %d", resp.StatusCode)
	}

	captionResponse := struct {
		Caption string `json:"generated_caption"`
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&captionResponse); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(captionResponse.Caption) > 0 {
		return captionResponse.Caption, nil
	}
	return "", fmt.Errorf("no caption received")
}
