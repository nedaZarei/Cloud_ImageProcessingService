package db

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/nedaZarei/Cloud_ImageProcessingService/ImageCaptioner/pkg/models"
)

// interface for updating image request status
type ImageRequestDatabase interface {
	SetRequestReady(ctx context.Context, requestID int, caption string) error
}

type ImageRequestDBImpl struct {
	db *sqlx.DB
}

func NewImageRequestDB(db *sqlx.DB) *ImageRequestDBImpl {
	return &ImageRequestDBImpl{db: db}
}

// updates the request status to ready and adds the caption
func (repo *ImageRequestDBImpl) SetRequestReady(ctx context.Context, requestID int, caption string) error {
	_, err := repo.db.Exec("UPDATE image_requests SET status=$1, caption=$2 WHERE request_id=$3", models.TaskReady, caption, requestID)
	if err != nil {
		return err
	}
	return nil
}
