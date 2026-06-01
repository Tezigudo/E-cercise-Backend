package repository

import (
	"github.com/E-cercise/E-cercise/src/data/request"
	"github.com/E-cercise/E-cercise/src/enum"
	"github.com/E-cercise/E-cercise/src/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderRepository interface {
	CreateOrder(tx *gorm.DB, order *model.Order) error
	SaveOrder(tx *gorm.DB, order *model.Order) error
	FindByID(orderID uuid.UUID) (*model.Order, error)
	UpdateOrderStatusByID(orderID uuid.UUID, orderStatus enum.OrderStatus) error
	FindByStatus(userID uuid.UUID, orderStatus enum.OrderStatus) ([]model.Order, error)
	FindOrderList(q request.OrderListRequest) ([]model.Order, error)
}

type orderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) CreateOrder(tx *gorm.DB, order *model.Order) error {
	return tx.Create(order).Error
}

func (r *orderRepository) SaveOrder(tx *gorm.DB, order *model.Order) error {
	return tx.Save(order).Error
}

func (r *orderRepository) FindByID(orderID uuid.UUID) (*model.Order, error) {
	var order model.Order
	// First (not Find): a missing order must surface gorm.ErrRecordNotFound so
	// callers can return 404. Find leaves a zero-value Order with a nil error,
	// which leaks an empty 200 stub for non-existent IDs.
	err := r.db.Preload("LineEquipments").First(&order, "id = ?", orderID).Error
	return &order, err
}

func (r *orderRepository) UpdateOrderStatusByID(orderID uuid.UUID, orderStatus enum.OrderStatus) error {
	var order model.Order
	return r.db.Model(&order).Where("id = ?", orderID).Update("order_status", orderStatus).Error
}

func (r *orderRepository) FindByStatus(userID uuid.UUID, orderStatus enum.OrderStatus) ([]model.Order, error) {
	var orders []model.Order
	query := r.db.
		Preload("LineEquipments").
		Preload("LineEquipments.EquipmentOption").
		Where("user_id = ?", userID)

	// Only filter by status when one is actually requested. An empty
	// OrderStatus ("") is NOT a valid value for the Postgres order_status enum,
	// so applying it unconditionally throws SQLSTATE 22P02 — the GET /order/me
	// 500 that fires whenever the "all orders" view sends no status filter.
	// Mirrors the nil-guard the admin FindOrderList already has.
	if orderStatus != "" {
		query = query.Where("order_status = ?", orderStatus)
	}

	if err := query.Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *orderRepository) FindOrderList(q request.OrderListRequest) ([]model.Order, error) {
	var orders []model.Order

	query := r.db.Model(&model.Order{})

	if q.OrderStatus != nil {
		query = query.Where("order_status = ?", *q.OrderStatus)
	}

	if q.OrderID != nil {
		query = query.Where("id = ?", *q.OrderID)
	}

	if q.PaymentType != nil {
		query = query.Where("payment_type = ?", *q.PaymentType)
	}

	if q.UserID != nil {
		query = query.Where("user_id = ?", *q.UserID)
	}

	err := query.
		Preload("LineEquipments").
		Preload("LineEquipments.EquipmentOption").
		Find(&orders).Error

	if err != nil {
		return nil, err
	}

	return orders, nil
}
