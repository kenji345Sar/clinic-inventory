package organization

import "github.com/google/uuid"

// CorporationModel はCorporationの永続化用モデル（gorm）。
type CorporationModel struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"not null"`
}

func (CorporationModel) TableName() string { return "corporations" }

// FacilityModel はFacilityの永続化用モデル（gorm）。
type FacilityModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name          string     `gorm:"not null"`
	FacilityType  string     `gorm:"column:facility_type;not null"`
	CorporationID uuid.UUID  `gorm:"type:uuid;index;not null"`
	GroupID       *uuid.UUID `gorm:"type:uuid"`
	// NotificationEmail は発注確定のお知らせの宛先。空文字は「送らない」。
	NotificationEmail string `gorm:"not null;default:''"`
}

func (FacilityModel) TableName() string { return "facilities" }
