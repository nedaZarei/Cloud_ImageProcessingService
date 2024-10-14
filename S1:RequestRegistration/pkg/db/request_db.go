package db

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/nedaZarei/Cloud_ImageProcessingService/RequestRegistration/pkg/models"
)

const (
	CREATE_REQUEST_TABLE = `CREATE TABLE IF NOT EXISTS requests(
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) NOT NULL,
		status VARCHAR(255) NOT NULL,
		image_caption VARCHAR(255) NOT NULL,
		new_image_url VARCHAR(255) NOT NULL
	);`
)

type RequestDatabase interface {
	CreateRequest(ctx context.Context, email string) (int, error)
	GetRequestByID(ctx context.Context, id int) (*models.Request, error)
}

type RequestDatabaseImpl struct {
	db *sqlx.DB
}

func NewRequestDatabase(autoCreate bool, db *sqlx.DB) (*RequestDatabaseImpl, error) {
	if autoCreate {
		if _, err := db.Exec(CREATE_REQUEST_TABLE); err != nil {
			return nil, err
		}
	}
	return &RequestDatabaseImpl{db: db}, nil
}

func (r *RequestDatabaseImpl) CreateRequest(ctx context.Context, email string) (int, error) {
	var id int
	err := r.db.QueryRow("INSERT INTO requests(email, status, image_caption, new_image_url) VALUES($1, $2, $3, $4) RETURNING id",
		email, models.TaskPending, "", "").Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *RequestDatabaseImpl) GetRequestByID(ctx context.Context, id int) (*models.Request, error) {
	request := &models.Request{}
	err := r.db.Get(request, "SELECT * FROM requests WHERE id=$1", id)
	if err != nil {
		return nil, err
	}
	return request, nil
}
