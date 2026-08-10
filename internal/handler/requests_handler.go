package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/dimalewshin98-glitch/LTCalendar/internal/service"
)

type RequestsHandler struct {
	service service.ServiceInterface
}

func NewRequestsHandler(service service.ServiceInterface) *RequestsHandler {
	return &RequestsHandler{
		service: service,
	}
}

func (s *RequestsHandler) Ping(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusBadRequest)
		return
	}
	err := s.service.Ping(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
