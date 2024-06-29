package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

type TokenVerificationResponse struct {
	Valid  bool   `json:"valid"`
	UserID int    `json:"user_id,omitempty"`
	Name   string `json:"name,omitempty"`
	Email  string `json:"email,omitempty"`
}

type TokenVerificationRequest struct {
	Token string `json:"token"`
}

type Group struct {
	ID        uint      `json:"id" gorm:"primary_key"`
	Name      string    `json:"name"`
	ToDos     []ToDo    `json:"todos" gorm:"foreignkey:GroupID;constraint:OnDelete:CASCADE;"`
	CreatedAt time.Time `json:"created_at"`
}

type ToDo struct {
	ID        uint      `json:"id" gorm:"primary_key"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	GroupID   uint      `json:"group_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate hook sets CreatedAt timestamp before creating record
func (g *Group) BeforeCreate(scope *gorm.Scope) error {
	g.CreatedAt = time.Now()
	return nil
}

// BeforeCreate hook sets CreatedAt timestamp before creating record
func (t *ToDo) BeforeCreate(scope *gorm.Scope) error {
	t.CreatedAt = time.Now()
	return nil
}

func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(&Group{}, &ToDo{})
}
