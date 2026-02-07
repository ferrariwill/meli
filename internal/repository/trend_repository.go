package repository

import (
	"context"
	"time"

	"melibot/database"

	"gorm.io/gorm"
)

// ProductTrend stores minimal data for trend analysis.
type ProductTrend struct {
	ID           uint    `gorm:"primaryKey"`
	ProductID    string  `gorm:"index;not null"`
	Title        string  `gorm:"not null"`
	CategoryID   string  `gorm:"index;not null"`
	SoldQuantity int     `gorm:"not null"`
	Health       string  `gorm:"size:64"`
	Price        float64 `gorm:"not null"`
	Thumbnail    string  `gorm:"size:512"`
	Permalink    string  `gorm:"size:512"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type TrendRepository struct {
	db *gorm.DB
}

func NewTrendRepository() *TrendRepository {
	return &TrendRepository{
		db: database.DB,
	}
}

// AutoMigrate ensures DB schema is up to date for this repository.
func AutoMigrate() error {
	return database.DB.AutoMigrate(&ProductTrend{})
}

// SaveProductTrends persists a batch of product trend records.
func (r *TrendRepository) SaveProductTrends(ctx context.Context, items []ProductTrend) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}
