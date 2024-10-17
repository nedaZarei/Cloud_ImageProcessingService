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
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mailersend/mailersend-go"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/nedaZarei/Cloud_ImageProcessingService/CaptionToImageService/config"
	"github.com/nedaZarei/Cloud_ImageProcessingService/CaptionToImageService/pkg/db"
)

type Service struct {
	cfg             *config.Config
	RequestDatabase db.RequestDatabase
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
	s.RequestDatabase = db.NewRequestDatabase(dB)

	//minio init
	s.minioClient, err = minio.New(s.cfg.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.cfg.Minio.AccessKey, s.cfg.Minio.SecretKey, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("failed to init Minio client: %v", err)
	}
	log.Println("connected to minio")

	s.StartScheduledImageProcessing()
	return nil
}

func (s *Service) StartScheduledImageProcessing() {
	//creating a ticker that triggers every 5 sec starting a goroutine that runs ImageProcessingJob
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	//when sth writes to this channel it will stop the loop
	quit := make(chan struct{})

	for {
		select {
		case <-ticker.C:
			go s.ImageProcessingJob()
		case <-quit:
			return
		}
	}
}

func (s *Service) ImageProcessingJob() {
	ctx := context.Background()
	requests, err := s.RequestDatabase.FetchReadyRequests(ctx)
	if err != nil {
		log.Printf("failed to get ready requests: %v", err)
		return
	}

	for _, req := range requests {
		//sending the caption to HuggingFace API and getting the image
		imageBytes, err := s.generateImageFromCaption(req.ImageCaption)
		if err != nil || len(imageBytes) == 0 {
			log.Printf("failed to generate image for request %d: %v", req.ID, err)
			err = s.RequestDatabase.UpdateImageURL(ctx, strconv.Itoa(req.ID), "")
			if err != nil {
				log.Printf("failed to update image URL for request %d: %v", req.ID, err)
				continue
			}
			continue
		}

		imageURL, err := s.uploadImageToMinIO(ctx, req.ID, imageBytes)
		if err != nil {
			log.Printf("failed to upload image for request %d: %v", req.ID, err)
			continue
		}

		err = s.RequestDatabase.UpdateImageURL(ctx, strconv.Itoa(req.ID), imageURL)
		if err != nil {
			log.Printf("failed to update image URL for request %d: %v", req.ID, err)
			continue
		}

		err = s.sendEmail("image generated", "your image has been generated --> "+imageURL, "<h1>your image has been generated --> "+imageURL+"</h1>", "CaptionToImageService", s.cfg.Email.From, req.Email)
		if err != nil {
			log.Printf("failed to send email for request %d: %v", req.ID, err)
		}

		log.Printf("successfully processed request %d, image URL: %s", req.ID, imageURL)
	}
}

func (s *Service) generateImageFromCaption(caption string) ([]byte, error) {
	// preparing the request payload
	payload := map[string]string{
		"inputs": caption,
	}

	//making the request to HuggingFace API
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.HuggingFace.URL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create Hugging Face request: %w", err)
	}

	//setting the headers
	req.Header.Set("Authorization", "Bearer "+s.cfg.HuggingFace.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send Hugging Face request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Hugging Face response: %w", err)
		}
		log.Printf("Hugging Face response: %s", string(body))
		return nil, fmt.Errorf("received non-200 response from Hugging Face: %d", resp.StatusCode)
	}

	//reading the response body (which contains the image)
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Hugging Face response: %w", err)
	}

	return imageBytes, nil
}

func (s *Service) sendEmail(subject, text, html, fromName, fromEmail, toEmail string) error {
	ms := mailersend.NewMailersend(s.cfg.Email.APIKey)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	from := mailersend.From{
		Name:  fromName,
		Email: fromEmail,
	}

	recipients := []mailersend.Recipient{
		{
			Email: toEmail,
		},
	}

	message := ms.Email.NewMessage()

	message.SetFrom(from)
	message.SetRecipients(recipients)
	message.SetSubject(subject)
	message.SetHTML(html)
	message.SetText(text)

	_, err := ms.Email.Send(ctx, message)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) uploadImageToMinIO(ctx context.Context, id int, imageBytes []byte) (string, error) {
	_, err := s.minioClient.PutObject(ctx, s.cfg.Minio.Bucket, strconv.Itoa(id)+time.Now().Format("2006-01-02"), bytes.NewReader(imageBytes), int64(len(imageBytes)), minio.PutObjectOptions{ContentType: "image/png"})
	if err != nil {
		return "", fmt.Errorf("failed to upload image to Minio: %w", err)
	}
	//generating the image URL
	imageURL := fmt.Sprintf("https://%s/%s/%s", s.cfg.Minio.Endpoint, s.cfg.Minio.Bucket, strconv.Itoa(id)+time.Now().Format("2006-01-02"))
	return imageURL, nil
}
