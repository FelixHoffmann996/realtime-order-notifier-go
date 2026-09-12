package orders

import "testing"

func TestDecideOrderNotification(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantEvent string
		wantSend  bool
	}{
		{name: "checkout becomes visible", status: "checkout_completed", wantEvent: "order.checkout_completed", wantSend: true},
		{name: "fulfillment becomes visible", status: "fulfillment_started", wantEvent: "order.fulfillment_started", wantSend: true},
		{name: "receipt becomes visible", status: "receipt_ready", wantEvent: "order.receipt_ready", wantSend: true},
		{name: "delivery becomes visible", status: "delivered", wantEvent: "order.delivered", wantSend: true},
		{name: "internal transition stays quiet", status: "inventory_reserved", wantSend: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification, send := Decide(Update{OrderID: "ord_42", AccountID: "acct_7", Status: tt.status})
			if send != tt.wantSend {
				t.Fatalf("send = %v, want %v", send, tt.wantSend)
			}
			if send && notification.Event != tt.wantEvent {
				t.Fatalf("event = %q, want %q", notification.Event, tt.wantEvent)
			}
			if send && notification.Channel != "account:acct_7" {
				t.Fatalf("channel = %q", notification.Channel)
			}
		})
	}
}
