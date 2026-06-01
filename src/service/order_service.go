package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/E-cercise/E-cercise/src/data/request"
	"github.com/E-cercise/E-cercise/src/data/response"
	"github.com/E-cercise/E-cercise/src/enum"
	"github.com/E-cercise/E-cercise/src/helper"
	"github.com/E-cercise/E-cercise/src/logger"
	"github.com/E-cercise/E-cercise/src/model"
	"github.com/E-cercise/E-cercise/src/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderService interface {
	CreateOrder(req request.PlaceOrderCartRequest, user *model.User) (*uuid.UUID, error)
	GetOrderDetail(orderID uuid.UUID, user *model.User) (*response.OrderDetailResponse, error)
	UpdateOrderStatus(orderID uuid.UUID, user *model.User) error
	GetMyOrders(userID uuid.UUID, orderStatus enum.OrderStatus) (*response.MyOrderResponse, error)
	GetOrderList(q request.OrderListRequest) (*response.OrderListResponse, error)
}

type orderService struct {
	db            *gorm.DB
	cartRepo      repository.CartRepository
	equipmentRepo repository.EquipmentRepository
	orderRepo     repository.OrderRepository
}

func NewOrderService(db *gorm.DB, cartRepo repository.CartRepository, equipmentRepo repository.EquipmentRepository, orderRepo repository.OrderRepository) OrderService {
	return &orderService{db: db, cartRepo: cartRepo, equipmentRepo: equipmentRepo, orderRepo: orderRepo}
}

func (s *orderService) CreateOrder(req request.PlaceOrderCartRequest, user *model.User) (*uuid.UUID, error) {
	logger.Log.Info("Starting order creation process", map[string]interface{}{"user_id": user.ID})

	tx := s.db.Begin()

	defer func() {
		if r := recover(); r != nil {
			logger.Log.WithError(fmt.Errorf("%v", r)).Error("Panic recovered during order creation")
			tx.Rollback()
		}
	}()

	cart, err := s.cartRepo.GetCart(user.ID)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get cart for user", map[string]interface{}{"user_id": user.ID})
		tx.Rollback()
		return nil, fmt.Errorf("failed to get cart for user %s: %w", user.ID, err)
	}

	cartItemsMap := make(map[uuid.UUID]model.LineEquipment)
	for _, item := range cart.LineEquipments {
		cartItemsMap[item.ID] = item
	}

	order := &model.Order{
		UserID:          user.ID,
		DeliveryAddress: req.Address,
		LineEquipments:  []model.LineEquipment{},
		OrderStatus:     enum.OrderPlaced,
		PaymentType:     req.PaymentType,
	}

	if order.PaymentType == enum.PaymentTypeCreditOrDebitCard {
		order.OrderStatus = enum.OrderPaid
	}

	if err := s.orderRepo.CreateOrder(tx, order); err != nil {
		tx.Rollback()
		logger.Log.WithError(err).Error("Failed to create order", map[string]interface{}{"order_id": order.ID})
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	var totalPrice float64
	for _, lineEquipmentID := range req.LineEquipments {

		cartItem, exists := cartItemsMap[lineEquipmentID]
		if !exists {
			errMsg := fmt.Sprintf("Line item not found in cart: %s", lineEquipmentID)
			logger.Log.Error(errMsg, map[string]interface{}{"user_id": user.ID, "line_equipment_id": lineEquipmentID})
			tx.Rollback()
			return nil, errors.New(errMsg)
		}

		equipmentOption, err := s.equipmentRepo.FindOptionByID(cartItem.EquipmentOptionID)
		if err != nil {
			logger.Log.WithError(err).Error("Failed to fetch equipment option", map[string]interface{}{"equipment_option_id": cartItem.EquipmentOptionID})
			tx.Rollback()
			return nil, fmt.Errorf("failed to fetch equipment option %s: %w", cartItem.EquipmentOptionID, err)
		}

		// Atomic, contention-safe decrement. The WHERE guards against two
		// concurrent checkouts both passing a read-time stock check and
		// overselling; RowsAffected == 0 means the stock ran out in between.
		result := tx.Model(&model.EquipmentOption{}).
			Where("id = ? AND remaining_products >= ?", cartItem.EquipmentOptionID, cartItem.Quantity).
			UpdateColumn("remaining_products", gorm.Expr("remaining_products - ?", cartItem.Quantity))
		if result.Error != nil {
			logger.Log.WithError(result.Error).Error("Failed to update inventory", map[string]interface{}{"equipment_option_id": cartItem.EquipmentOptionID})
			tx.Rollback()
			return nil, fmt.Errorf("failed to update inventory: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			tx.Rollback()
			return nil, fmt.Errorf("insufficient stock for option %s", cartItem.EquipmentOptionID)
		}

		totalPrice += float64(cartItem.Quantity) * equipmentOption.Price
		cartItem.CartID = nil
		cartItem.OrderID = &order.ID
		if err := tx.Save(&cartItem).Error; err != nil {
			logger.Log.WithError(err).Error("Failed to update cart item to order item", map[string]interface{}{"cart_item_id": cartItem.ID})
			tx.Rollback()
			return nil, fmt.Errorf("failed to update cart item to order item: %w", err)
		}
		logger.Log.Info("Updated cart item to be part of the order", map[string]interface{}{"cart_item_id": cartItem.ID, "order_id": order.ID})
	}

	order.TotalPrice = totalPrice
	if err := s.orderRepo.SaveOrder(tx, order); err != nil {
		tx.Rollback()
		logger.Log.WithError(err).Error("Failed to save order", map[string]interface{}{"order_id": order.ID})
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	logger.Log.Info("Order created successfully", map[string]interface{}{"order_id": order.ID, "total_price": totalPrice})

	if err := tx.Commit().Error; err != nil {
		logger.Log.WithError(err).Error("error committing transaction")
		tx.Rollback()
		return nil, err
	}
	return &order.ID, nil

}

func (s *orderService) GetOrderDetail(orderID uuid.UUID, user *model.User) (*response.OrderDetailResponse, error) {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		logger.Log.WithError(err).Error("Falied to get order detail")
		return nil, fmt.Errorf("failed to get order detail")
	}

	if user.Role == enum.RoleUser && order.UserID != user.ID {
		return nil, fmt.Errorf("order not found")
	}

	var resp response.OrderDetailResponse

	// Build the recipient block from the ORDER OWNER's profile (order.User), not the
	// caller — otherwise an admin viewing another user's order sees their own address.
	// TODO: snapshot name/address/phone onto the order row at checkout so this returns
	// the address-at-purchase rather than the owner's current profile (deferred).
	owner := order.User
	address := response.Address{
		FullName:    fmt.Sprintf("%s %s", owner.FirstName, owner.LastName),
		AddressLine: owner.Address,
		PhoneNumber: owner.PhoneNumber,
	}

	var orders []response.LineEquipment

	for _, lineEquipment := range order.LineEquipments {
		equipment, err := s.equipmentRepo.FindByID(lineEquipment.EquipmentID)
		if err != nil {
			logger.Log.WithError(err).Error("error during find equipment with this id", lineEquipment.EquipmentID)
			return nil, err
		}

		equipmentOption, err := s.equipmentRepo.FindOptionByID(lineEquipment.EquipmentOptionID)
		if err != nil {
			logger.Log.WithError(err).Error("error during find equipment option with this id", lineEquipment.EquipmentOptionID)
			return nil, err
		}

		var imagePath string
		if primaryImage := helper.FindPrimaryImageFromEquipment(*equipment); primaryImage != nil {
			imagePath = primaryImage.CloudinaryPath
		} else {
			imagePath = fmt.Sprintf("https://placehold.co/600x400?text=%s/png", strings.ReplaceAll(equipment.Name, " ", "+"))
		}

		lineEquipment := response.LineEquipment{
			LineEquipmentID: lineEquipment.ID.String(),
			EquipmentName:   equipment.Name,
			ImgUrl:          imagePath,
			Quantity:        lineEquipment.Quantity,
			PerUnitPrice:    equipmentOption.Price,
			Total:           float64(lineEquipment.Quantity) * equipmentOption.Price,
		}
		orders = append(orders, lineEquipment)
	}

	resp = response.OrderDetailResponse{
		ID:          order.ID,
		OrderStatus: order.OrderStatus,
		Address:     address,
		Orders:      orders,
		NetPrice:    order.TotalPrice,
	}

	return &resp, nil
}

func (s *orderService) UpdateOrderStatus(orderID uuid.UUID, user *model.User) error {
	order, err := s.orderRepo.FindByID(orderID)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get order detail")
		return fmt.Errorf("failed to get order detail")
	}

	switch user.Role {
	case enum.RoleUser:
		if order.UserID != user.ID {
			logger.Log.Errorf("User %v tried to update order %v not owned by them", user.ID, order.ID)
			return fmt.Errorf("you are not allowed to update this order")
		}
		if order.OrderStatus != enum.OrderToReceive {
			logger.Log.Errorf("User %v not allowed to update order in status: %v", user.ID, order.OrderStatus)
			return fmt.Errorf("you can only update order if status is 'ToReceive'")
		}
	case enum.RoleAdmin:
		// Admin advances Placed->Paid (confirm cash payment), Paid->Shipped out, and
		// Shipped out->To Receive. (USER may only confirm receipt: To Receive->Received.)
		// Placed->Paid MUST be allowed for some role or every order is stuck at Placed.
		if order.OrderStatus != enum.OrderPlaced && order.OrderStatus != enum.OrderPaid && order.OrderStatus != enum.OrderShipped {
			logger.Log.Errorf("Admin not allowed to update order in status: %v", order.OrderStatus)
			return fmt.Errorf("admin can only update order if status is 'Placed', 'Paid' or 'Shipped'")
		}
	default:
		logger.Log.Errorf("Unauthorized role: %v", user.Role)
		return fmt.Errorf("unauthorized role")
	}

	statusTransaction := map[enum.OrderStatus]enum.OrderStatus{
		enum.OrderPlaced:    enum.OrderPaid,
		enum.OrderPaid:      enum.OrderShipped,
		enum.OrderShipped:   enum.OrderToReceive,
		enum.OrderToReceive: enum.OrderReceived,
	}

	nextStatus, ok := statusTransaction[order.OrderStatus]
	if !ok {
		if order.OrderStatus == enum.OrderReceived {
			logger.Log.Error("Order has already been received, no further status update possible")
			return fmt.Errorf("order has already been received, no further status update possible")
		}
		logger.Log.Errorf("Invalid order status: %v", order.OrderStatus)
		return fmt.Errorf("invalid order status: %v", order.OrderStatus)
	}

	if err := s.orderRepo.UpdateOrderStatusByID(order.ID, nextStatus); err != nil {
		logger.Log.WithError(err).Errorf("Cannot update order status with ID: %v", orderID)
		return err
	}

	return nil
}

// buildOrderSummary fetches the equipment and option for the first line item of
// an order and returns the abbreviated display name and primary image URL.
// Centralises the nil-image guard for GetMyOrders and GetOrderList.
func (s *orderService) buildOrderSummary(order model.Order) (name string, imgURL string, err error) {
	if len(order.LineEquipments) == 0 {
		return "", "", fmt.Errorf("order %s has no line items", order.ID)
	}
	line := order.LineEquipments[0]

	equipment, err := s.equipmentRepo.FindByID(line.EquipmentID)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get equipment")
		return "", "", fmt.Errorf("failed to get equipment")
	}

	equipmentOpt, err := s.equipmentRepo.FindOptionByID(line.EquipmentOptionID)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get equipment option")
		return "", "", fmt.Errorf("failed to get equipment option")
	}

	fallback := fmt.Sprintf("https://placehold.co/600x400?text=%s/png",
		strings.ReplaceAll(equipment.Name, " ", "+"))
	imgURL = helper.PrimaryImageURL(*equipmentOpt, fallback)
	name = helper.AbbreviateEquipmentName(equipment.Name, equipmentOpt.Name)
	return name, imgURL, nil
}

func (s *orderService) GetMyOrders(userID uuid.UUID, orderStatus enum.OrderStatus) (*response.MyOrderResponse, error) {
	orders, err := s.orderRepo.FindByStatus(userID, orderStatus)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get orders", "userID", userID)
		return nil, fmt.Errorf("failed to get orders")
	}

	var resp response.MyOrderResponse

	if len(orders) == 0 {
		return &resp, nil
	}

	for _, order := range orders {
		logger.Log.Info(fmt.Sprintf("Order with ID: %s with Status: %s", order.ID, order.OrderStatus))
		if len(order.LineEquipments) == 0 {
			logger.Log.Warn(fmt.Sprintf("Order ID %s has no line items, skipping", order.ID))
			continue
		}

		name, imgURL, err := s.buildOrderSummary(order)
		if err != nil {
			return nil, err
		}

		orderResp := response.Order{
			CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05"),
			FirstLineEquipment: response.FirstLineEquipment{
				ImgURL: imgURL,
				Name:   name,
			},
			ID:          order.ID,
			OrderStatus: order.OrderStatus,
			PaymentType: order.PaymentType,
			TotalPrice:  order.TotalPrice,
			UpdatedAt:   order.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		resp.Orders = append(resp.Orders, orderResp)
	}

	return &resp, nil
}

func (s *orderService) GetOrderList(q request.OrderListRequest) (*response.OrderListResponse, error) {
	orders, err := s.orderRepo.FindOrderList(q)
	if err != nil {
		logger.Log.WithError(err).Error("Failed to get orders List")
		return nil, fmt.Errorf("failed to get orders")
	}

	var resp response.OrderListResponse

	if len(orders) == 0 {
		return &resp, nil
	}

	for _, order := range orders {
		logger.Log.Info(fmt.Sprintf("Order with ID: %s with Status: %s", order.ID, order.OrderStatus))

		if len(order.LineEquipments) == 0 {
			logger.Log.Warn(fmt.Sprintf("Order ID %s has no line items", order.ID))
			continue
		}

		name, imgURL, err := s.buildOrderSummary(order)
		if err != nil {
			return nil, err
		}

		orderResp := response.OrderList{
			CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05"),
			FirstLineEquipment: response.FirstLineEquipment{
				ImgURL: imgURL,
				Name:   name,
			},
			ID:          order.ID,
			UserID:      order.UserID,
			OrderStatus: order.OrderStatus,
			PaymentType: order.PaymentType,
			TotalPrice:  order.TotalPrice,
			UpdatedAt:   order.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		resp.Orders = append(resp.Orders, orderResp)
	}

	return &resp, nil
}
