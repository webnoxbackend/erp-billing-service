package grpc

import (
	"example-service/internal/application/dto"
	"example-service/internal/domain"
	"example-service/internal/ports/services"
	proto "example-service/example-service/proto"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handler implements the gRPC ExampleService server
type Handler struct {
	proto.UnimplementedExampleServiceServer
	exampleService services.ExampleService
}

// NewHandler creates a new gRPC handler
func NewHandler(exampleService services.ExampleService) *Handler {
	return &Handler{
		exampleService: exampleService,
	}
}

// CreateExample handles example creation
func (h *Handler) CreateExample(ctx context.Context, req *proto.CreateExampleRequest) (*proto.CreateExampleResponse, error) {
	// Map proto request to DTO
	createReq := &dto.CreateExampleRequest{
		Name: req.Name,
	}

	// Validate request
	if createReq.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}

	// Call service
	resp, err := h.exampleService.CreateExample(createReq)
	if err != nil {
		return nil, h.mapError(err)
	}

	// Map DTO to proto response
	return &proto.CreateExampleResponse{
		Id:        resp.ID,
		Name:      resp.Name,
		Status:    resp.Status,
		CreatedAt: resp.CreatedAt,
		UpdatedAt: resp.UpdatedAt,
	}, nil
}

// GetExample handles example retrieval
func (h *Handler) GetExample(ctx context.Context, req *proto.GetExampleRequest) (*proto.GetExampleResponse, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	resp, err := h.exampleService.GetExample(req.Id)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &proto.GetExampleResponse{
		Id:        resp.ID,
		Name:      resp.Name,
		Status:    resp.Status,
		CreatedAt: resp.CreatedAt,
		UpdatedAt: resp.UpdatedAt,
	}, nil
}

// ListExamples handles listing all examples
func (h *Handler) ListExamples(ctx context.Context, req *proto.ListExamplesRequest) (*proto.ListExamplesResponse, error) {
	examples, err := h.exampleService.ListExamples()
	if err != nil {
		return nil, h.mapError(err)
	}

	protoExamples := make([]*proto.ExampleResponse, len(examples))
	for i, ex := range examples {
		protoExamples[i] = &proto.ExampleResponse{
			Id:        ex.ID,
			Name:      ex.Name,
			Status:    ex.Status,
			CreatedAt: ex.CreatedAt,
			UpdatedAt: ex.UpdatedAt,
		}
	}

	return &proto.ListExamplesResponse{
		Examples: protoExamples,
	}, nil
}

// UpdateExample handles example update
func (h *Handler) UpdateExample(ctx context.Context, req *proto.UpdateExampleRequest) (*proto.UpdateExampleResponse, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	updateReq := &dto.UpdateExampleRequest{
		Name:   req.Name,
		Status: req.Status,
	}

	resp, err := h.exampleService.UpdateExample(req.Id, updateReq)
	if err != nil {
		return nil, h.mapError(err)
	}

	return &proto.UpdateExampleResponse{
		Id:        resp.ID,
		Name:      resp.Name,
		Status:    resp.Status,
		CreatedAt: resp.CreatedAt,
		UpdatedAt: resp.UpdatedAt,
	}, nil
}

// DeleteExample handles example deletion
func (h *Handler) DeleteExample(ctx context.Context, req *proto.DeleteExampleRequest) (*proto.DeleteExampleResponse, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	if err := h.exampleService.DeleteExample(req.Id); err != nil {
		return nil, h.mapError(err)
	}

	return &proto.DeleteExampleResponse{
		Success: true,
		Message: "Example deleted successfully",
	}, nil
}

// mapError maps domain errors to gRPC status errors
func (h *Handler) mapError(err error) error {
	switch err {
	case domain.ErrExampleNotFound:
		return status.Errorf(codes.NotFound, "example not found")
	case domain.ErrExampleAlreadyExists:
		return status.Errorf(codes.AlreadyExists, "example already exists")
	case domain.ErrInvalidInput:
		return status.Errorf(codes.InvalidArgument, "invalid input")
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

