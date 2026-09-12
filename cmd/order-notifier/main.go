package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"example.com/realtime-order-notifier/internal/infrai"
	"example.com/realtime-order-notifier/internal/orders"
)

type server struct {
	realtime *infrai.Client
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &server{realtime: infrai.NewClient(key)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /order-updates", s.receiveOrderUpdate)
	mux.HandleFunc("POST /realtime-token", s.issueRealtimeToken)
	log.Printf("order notifier listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) receiveOrderUpdate(w http.ResponseWriter, r *http.Request) {
	var update orders.Update
	if err := decodeJSON(r, &update); err != nil || update.OrderID == "" || update.AccountID == "" || update.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "order_id, account_id, and status are required"})
		return
	}
	notification, send := orders.Decide(update)
	if !send {
		writeJSON(w, http.StatusAccepted, map[string]any{"published": false})
		return
	}
	if err := s.realtime.CreateChannel(r.Context(), notification.Channel); err != nil {
		writeUpstreamError(w, err)
		return
	}
	idempotencyKey := fmt.Sprintf("order:%s:%s", update.OrderID, update.Status)
	if err := s.realtime.Publish(r.Context(), notification.Channel, notification.Event, notification.Data, notification.AccountID, idempotencyKey); err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"published": true, "channel": notification.Channel, "event": notification.Event})
}

func (s *server) issueRealtimeToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ClientID  string `json:"client_id"`
		AccountID string `json:"account_id"`
	}
	if err := decodeJSON(r, &input); err != nil || input.ClientID == "" || input.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "client_id and account_id are required"})
		return
	}
	data, err := s.realtime.IssueToken(r.Context(), input.ClientID, []string{"account:" + input.AccountID})
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]any{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]any{"error": "notification delivery failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
