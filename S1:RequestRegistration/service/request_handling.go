package service

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/nedaZarei/Cloud_ImageProcessingService/RequestRegistration/config"
	"github.com/nedaZarei/Cloud_ImageProcessingService/RequestRegistration/pkg/db"
	"github.com/nedaZarei/Cloud_ImageProcessingService/RequestRegistration/pkg/models"
)

type Service struct {
	cfg             *config.Config
	e               *echo.Echo
	RequestDatabase db.RequestDatabase
	rabbitMQClient  *amqp.Channel
	minioClient     *minio.Client
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		e:   echo.New(),
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
	fmt.Println(s.cfg.Postgres)
	s.RequestDatabase, err = db.NewRequestDatabase(s.cfg.Postgres.AutoCreate, dB)
	if err != nil {
		return fmt.Errorf("failed to initialize request database: %v", err)
	}
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
		return fmt.Errorf("failed to initialize Minio client: %v", err)
	}
	log.Println("connected to Minio")

	//setting up echo server with middleware
	s.e.Use(middleware.Logger())
	s.e.Use(middleware.Recover())

	//api routes (for backward compatability)
	v1 := s.e.Group("/api/v1")
	v1.POST("/register", s.RegisterRequest)
	v1.GET("/request/:id", s.GetRequestStatus)

	if err := s.e.Start("localhost" + s.cfg.Server.Port); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	return nil
}

func (s *Service) RegisterRequest(c echo.Context) error {
	imageData, err := extractImageFromRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	//for email
	request := &struct {
		Email string `json:"email" form:"email"`
	}{}
	if err := c.Bind(request); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	//saving request and getting id
	id, err := s.RequestDatabase.CreateRequest(c.Request().Context(), request.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	//publishing to RabbitMQ
	err = s.rabbitMQClient.PublishWithContext(c.Request().Context(), "", "image_queue", false, false, amqp.Publishing{
		ContentType: "image/jpeg",
		MessageId:   strconv.Itoa(id),
		Body:        imageData,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	//uploading image to Minio
	_, err = s.minioClient.PutObject(c.Request().Context(), s.cfg.Minio.Bucket, strconv.Itoa(id), bytes.NewReader(imageData), int64(len(imageData)), minio.PutObjectOptions{ContentType: "image/jpeg"})
	if err != nil {
		fmt.Println(err)
		return c.JSON(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusCreated, request)
}
func extractImageFromRequest(c echo.Context) ([]byte, error) {
	c.Request().ParseMultipartForm(10 << 20) //max 10MB file size
	form, err := c.MultipartForm()
	if err != nil {
		return nil, err
	}

	file, exists := form.File["image"]
	if !exists || len(file) == 0 {
		return nil, fmt.Errorf("image file not found in the req")
	}

	src, err := file[0].Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	return io.ReadAll(src)
}

func (s *Service) GetRequestStatus(c echo.Context) error {
	id := c.Param("id")
	intID, err := strconv.Atoi(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	request, err := s.RequestDatabase.GetRequestByID(c.Request().Context(), intID)
	if err != nil {
		fmt.Println(err)
		return c.JSON(http.StatusInternalServerError, err)
	}

	if request.Status == models.TaskPending {
		return c.JSON(http.StatusOK, "req is in process (pending)")
	}

	return c.JSON(http.StatusOK, fmt.Sprintf("READY! now you can download the image from: %s", request.NewImageURL))
}
