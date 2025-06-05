package entity

import "github.com/google/uuid"

type File struct {
	ID        uuid.UUID `json:"id" form:"id" gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Filename  string    `json:"filename" form:"filename"`
	Path      string    `json:"path" form:"path"`
	Size      int64     `json:"size" form:"size"`
	MimeType  string    `json:"mime_type" form:"mime_type"`
	Shareable *bool     `json:"shareable" form:"shareable" gorm:"default:false"`

	UserID uuid.UUID `json:"user_id" form:"user_id" gorm:"type:uuid;not null"`
	User   User      `json:"user" gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Timestamp
}
