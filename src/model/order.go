package model

import (
	"github.com/E-cercise/E-cercise/src/enum"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Order struct {
	ID              uuid.UUID        `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID          uuid.UUID        `gorm:"type:uuid;not null" json:"user_id"`
	User            User             `gorm:"foreignKey:UserID" json:"user"`
	LineEquipments  []LineEquipment  `gorm:"foreignKey:OrderID" json:"line_equipments"`
	DeliveryAddress string           `gorm:"type:text;not null" json:"delivery_address"`
	// Recipient snapshot — captured at checkout so order detail shows the
	// name/phone/address AT PURCHASE TIME, independent of later profile edits.
	RecipientName    string           `gorm:"type:varchar(255)" json:"recipient_name"`
	RecipientPhone   string           `gorm:"type:varchar(50)" json:"recipient_phone"`
	RecipientAddress string           `gorm:"type:text" json:"recipient_address"`
	PaymentType      enum.PaymentType `gorm:"type:payment_type" json:"payment_type"`
	TotalPrice      float64          `gorm:"type:decimal(10,2);not null" json:"total_price"`
	OrderStatus     enum.OrderStatus `gorm:"type:order_status;default:'Placed';not null" json:"order_status"`
	CreatedAt       time.Time        `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time        `gorm:"default:CURRENT_TIMESTAMP"`
}

func (o *Order) BeforeUpdate(tx *gorm.DB) error {
	o.UpdatedAt = time.Now()
	return nil
}
