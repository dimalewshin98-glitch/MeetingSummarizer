package service

import (
	"context"
)

type ServiceInterface interface {
	Ping(ctx context.Context) error
}
