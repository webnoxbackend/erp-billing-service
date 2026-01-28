package grpc

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/ports/outbound"
	proto "erp-billing-service/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InventoryClient struct {
	client proto.ServiceAndPartsServiceClient
	conn   *grpc.ClientConn
}

func NewInventoryClient(address string) (*InventoryClient, error) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inventory service: %w", err)
	}

	client := proto.NewServiceAndPartsServiceClient(conn)
	return &InventoryClient{
		client: client,
		conn:   conn,
	}, nil
}

func (c *InventoryClient) Close() error {
	return c.conn.Close()
}

func (c *InventoryClient) CheckStockAvailability(ctx context.Context, items []outbound.StockCheckItem) ([]outbound.UnavailableItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqItems := make([]*proto.StockItemRequest, len(items))
	for i, item := range items {
		reqItems[i] = &proto.StockItemRequest{
			ItemId:   item.ItemID,
			Quantity: item.Quantity,
		}
	}

	resp, err := c.client.CheckStockAvailability(ctx, &proto.CheckStockAvailabilityRequest{
		Items: reqItems,
	})
	if err != nil {
		return nil, fmt.Errorf("remote call failed: %w", err)
	}

	if resp.Available {
		return nil, nil
	}

	unavailable := make([]outbound.UnavailableItem, len(resp.UnavailableItems))
	for i, item := range resp.UnavailableItems {
		unavailable[i] = outbound.UnavailableItem{
			ItemID:            item.ItemId,
			ItemName:          item.ItemName,
			RequestedQuantity: item.RequestedQuantity,
			AvailableQuantity: item.AvailableQuantity,
		}
	}

	return unavailable, nil
}

func (c *InventoryClient) UpdateStock(ctx context.Context, items []outbound.StockCheckItem, transactionType, referenceType, referenceID, notes string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqItems := make([]*proto.StockItemRequest, len(items))
	for i, item := range items {
		reqItems[i] = &proto.StockItemRequest{
			ItemId:   item.ItemID,
			Quantity: item.Quantity,
		}
	}

	resp, err := c.client.UpdateStock(ctx, &proto.UpdateStockRequest{
		Items:           reqItems,
		TransactionType: transactionType,
		ReferenceType:   referenceType,
		ReferenceId:     referenceID,
		Notes:           notes,
	})
	if err != nil {
		return fmt.Errorf("remote call failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("stock update failed: %s", resp.Message)
	}

	return nil
}
