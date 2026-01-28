package outbound

import "context"

type StockCheckItem struct {
	ItemID   string
	Quantity int32
}

type UnavailableItem struct {
	ItemID            string
	ItemName          string
	RequestedQuantity int32
	AvailableQuantity int32
}

type InventoryClient interface {
	CheckStockAvailability(ctx context.Context, items []StockCheckItem) ([]UnavailableItem, error)
	UpdateStock(ctx context.Context, items []StockCheckItem, transactionType, referenceType, referenceID, notes string) error
}
