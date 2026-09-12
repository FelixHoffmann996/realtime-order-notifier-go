package orders

import "fmt"

type Update struct {
	OrderID   string `json:"order_id"`
	AccountID string `json:"account_id"`
	Status    string `json:"status"`
}

type Notification struct {
	Channel   string         `json:"channel"`
	Event     string         `json:"event"`
	Data      map[string]any `json:"data"`
	AccountID string         `json:"account_id"`
}

func Decide(update Update) (Notification, bool) {
	events := map[string]string{
		"checkout_completed":  "order.checkout_completed",
		"fulfillment_started": "order.fulfillment_started",
		"receipt_ready":       "order.receipt_ready",
		"delivered":           "order.delivered",
	}
	event, ok := events[update.Status]
	if !ok {
		return Notification{}, false
	}
	return Notification{
		Channel:   fmt.Sprintf("account:%s", update.AccountID),
		Event:     event,
		Data:      map[string]any{"order_id": update.OrderID, "status": update.Status},
		AccountID: update.AccountID,
	}, true
}
