package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nedaZarei/Cloud_ImageProcessingService/CaptionToImageService/pkg/models"
)

type RequestDatabase interface {
	UpdateImageURL(ctx context.Context, requestID string, imageURL string) error
	FetchReadyRequests(ctx context.Context) ([]models.Request, error)
}

type RequestDatabaseImpl struct {
	DB *sqlx.DB
}

func NewRequestDatabase(db *sqlx.DB) RequestDatabase {
	return &RequestDatabaseImpl{DB: db}
}

func (r *RequestDatabaseImpl) UpdateImageURL(ctx context.Context, requestID string, imageURL string) error {
	query := "UPDATE requests SET new_image_url = $1, status = $2 WHERE id = $3"
	var status string
	if imageURL == "" {
		status = string(models.TaskFailed)
		imageURL = "NULL"
	} else {
		status = string(models.TaskCompleted)
	}

	_, err := r.DB.ExecContext(ctx, query, imageURL, status, requestID) //executes a query without returning any rows
	if err != nil {
		fmt.Println("error updating image URL:", err)
		return err
	}
	return nil
}

// retrieves all requests with ready status
func (r *RequestDatabaseImpl) FetchReadyRequests(ctx context.Context) ([]models.Request, error) {
	var requests []models.Request
	query := "SELECT id, email, status, image_caption, new_image_url FROM requests WHERE status = $1 LIMIT 100"
	err := r.DB.SelectContext(ctx, &requests, query, models.TaskReady)
	if err != nil {
		return nil, err
	}
	return requests, nil
}
