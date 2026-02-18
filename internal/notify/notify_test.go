package notify

import (
	"errors"
	"testing"
)

// mockNotifier records calls and can be configured to return errors.
type mockNotifier struct {
	sent      []Notification
	available bool
	err       error
}

func (m *mockNotifier) Send(n Notification) error {
	m.sent = append(m.sent, n)
	return m.err
}

func (m *mockNotifier) IsAvailable() bool {
	return m.available
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Fatal("DefaultConfig should be enabled")
	}
	for _, e := range AllEvents() {
		if !cfg.EventFilter[e] {
			t.Errorf("event %s should be enabled in DefaultConfig", e)
		}
	}
}

func TestDefaultConfigContainsAllEvents(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.EventFilter) != len(AllEvents()) {
		t.Errorf("expected %d events in filter, got %d", len(AllEvents()), len(cfg.EventFilter))
	}
}

func TestManagerNotifyDisabled(t *testing.T) {
	mock := &mockNotifier{available: true}
	cfg := DefaultConfig()
	cfg.Enabled = false
	mgr := NewManager(cfg, mock)

	err := mgr.Notify(Notification{Event: EventServiceDiscovered, Title: "test", Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 0 {
		t.Error("should not send when disabled")
	}
}

func TestManagerNotifyFilteredOut(t *testing.T) {
	mock := &mockNotifier{available: true}
	cfg := DefaultConfig()
	cfg.EventFilter[EventServiceOffline] = false
	mgr := NewManager(cfg, mock)

	err := mgr.Notify(Notification{Event: EventServiceOffline, Title: "test", Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 0 {
		t.Error("should not send filtered-out event")
	}
}

func TestManagerNotifyAllowed(t *testing.T) {
	mock := &mockNotifier{available: true}
	cfg := DefaultConfig()
	mgr := NewManager(cfg, mock)

	n := Notification{Event: EventServiceDiscovered, Title: "hello", Message: "world"}
	err := mgr.Notify(n)
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.sent))
	}
	if mock.sent[0].Title != "hello" {
		t.Errorf("expected title 'hello', got %q", mock.sent[0].Title)
	}
}

func TestManagerNotifyPropagatesError(t *testing.T) {
	mock := &mockNotifier{available: true, err: errors.New("send failed")}
	cfg := DefaultConfig()
	mgr := NewManager(cfg, mock)

	err := mgr.Notify(Notification{Event: EventServiceDiscovered, Title: "test", Message: "test"})
	if err == nil {
		t.Fatal("expected error from notifier")
	}
}

func TestManagerNotifyUnknownEventAllowed(t *testing.T) {
	mock := &mockNotifier{available: true}
	cfg := DefaultConfig()
	mgr := NewManager(cfg, mock)

	// An event type not in the filter should still be sent (not explicitly blocked).
	err := mgr.Notify(Notification{Event: "unknown_event", Title: "test", Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 1 {
		t.Error("unknown event type should be allowed through (not in filter)")
	}
}

func TestServiceDiscovered(t *testing.T) {
	mock := &mockNotifier{available: true}
	mgr := NewManager(DefaultConfig(), mock)

	err := mgr.ServiceDiscovered("myapp", "3000")
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.sent))
	}
	n := mock.sent[0]
	if n.Event != EventServiceDiscovered {
		t.Errorf("expected event %s, got %s", EventServiceDiscovered, n.Event)
	}
	if n.URL != "http://myapp.localhost" {
		t.Errorf("unexpected URL: %s", n.URL)
	}
}

func TestServiceOffline(t *testing.T) {
	mock := &mockNotifier{available: true}
	mgr := NewManager(DefaultConfig(), mock)

	err := mgr.ServiceOffline("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.sent))
	}
	if mock.sent[0].Event != EventServiceOffline {
		t.Errorf("expected event %s, got %s", EventServiceOffline, mock.sent[0].Event)
	}
}

func TestServiceRenamed(t *testing.T) {
	mock := &mockNotifier{available: true}
	mgr := NewManager(DefaultConfig(), mock)

	err := mgr.ServiceRenamed("old-app", "new-app")
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.sent))
	}
	n := mock.sent[0]
	if n.Event != EventServiceRenamed {
		t.Errorf("expected event %s, got %s", EventServiceRenamed, n.Event)
	}
	if n.URL != "http://new-app.localhost" {
		t.Errorf("unexpected URL: %s", n.URL)
	}
}

func TestServiceDiscoveredFiltered(t *testing.T) {
	mock := &mockNotifier{available: true}
	cfg := DefaultConfig()
	cfg.EventFilter[EventServiceDiscovered] = false
	mgr := NewManager(cfg, mock)

	err := mgr.ServiceDiscovered("myapp", "3000")
	if err != nil {
		t.Fatal(err)
	}
	if len(mock.sent) != 0 {
		t.Error("should not send filtered-out convenience notification")
	}
}
