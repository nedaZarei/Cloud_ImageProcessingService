package models

type Request struct {
	ID           int        `json:"id" form:"id" db:"id"`
	Email        string     `json:"email" form:"email" db:"email"`
	Status       TaskStatus `json:"status" form:"status" db:"status"`
	ImageCaption string     `json:"image_caption" form:"image_caption" db:"image_caption"`
	NewImageURL  string     `json:"new_image_url" form:"new_image_url" db:"new_image_url"`
}

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskReady     TaskStatus = "ready"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)
