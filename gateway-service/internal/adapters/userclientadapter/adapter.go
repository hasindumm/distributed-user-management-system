package userclientadapter

import (
	"context"

	"gateway-service/internal/dto"
	"gateway-service/internal/ports"
	"user-service/pkg/userclient"
)

type Adapter struct {
	client *userclient.Client
}

func NewUserClientAdapter(client *userclient.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) CreateUser(ctx context.Context, req dto.CreateUserRequest) (dto.UserResponse, error) {
	user, err := a.client.CreateUser(ctx, userclient.CreateUserRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
		Age:       req.Age,
		Status:    req.Status,
	})
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponse(user), nil
}

func (a *Adapter) GetUserByID(ctx context.Context, id string) (dto.UserResponse, error) {
	user, err := a.client.GetUserByID(ctx, id)
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponse(user), nil
}

func (a *Adapter) GetUserByEmail(ctx context.Context, email string) (dto.UserResponse, error) {
	user, err := a.client.GetUserByEmail(ctx, email)
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponse(user), nil
}

func (a *Adapter) ListUsers(ctx context.Context, req dto.ListUsersRequest) ([]dto.UserResponse, error) {
	users, err := a.client.ListUsers(ctx, userclient.ListUsersRequest{
		Status: req.Status,
		Limit:  req.Limit,
		Offset: req.Offset,
	})
	if err != nil {
		return nil, err
	}

	responses := make([]dto.UserResponse, len(users))
	for i, u := range users {
		responses[i] = toUserResponse(u)
	}
	return responses, nil
}

func (a *Adapter) UpdateUser(ctx context.Context, id string, req dto.UpdateUserRequest) (dto.UserResponse, error) {
	user, err := a.client.UpdateUser(ctx, userclient.UpdateUserRequest{
		UserID:    id,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
		Age:       req.Age,
		Status:    req.Status,
	})
	if err != nil {
		return dto.UserResponse{}, err
	}
	return toUserResponse(user), nil
}

func (a *Adapter) DeleteUser(ctx context.Context, id string) error {
	return a.client.DeleteUser(ctx, id)
}

func (a *Adapter) Subscribe(ch chan<- userclient.Event) (ports.Subscription, error) {
	return a.client.Subscribe(ch)
}

func toUserResponse(u userclient.UserDTO) dto.UserResponse {
	return dto.UserResponse{
		UserID:    u.UserID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Phone:     u.Phone,
		Age:       u.Age,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
