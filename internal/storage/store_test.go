package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStorePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "services.json")
}

func TestNewStoreEmpty(t *testing.T) {
	store, err := NewStore(tempStorePath(t))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if len(store.List()) != 0 {
		t.Errorf("expected 0 records, got %d", len(store.List()))
	}
}

func TestNewStoreExistingData(t *testing.T) {
	path := tempStorePath(t)

	records := []*ServiceRecord{
		{ID: "id1", Name: "app1.localhost", Port: 3000, ExePath: "/bin/app1"},
		{ID: "id2", Name: "app2.localhost", Port: 4000, ExePath: "/bin/app2"},
	}
	data, _ := json.MarshalIndent(records, "", "  ")
	os.WriteFile(path, data, 0666)

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if len(store.List()) != 2 {
		t.Errorf("expected 2 records, got %d", len(store.List()))
	}
}

func TestNewStoreInvalidJSON(t *testing.T) {
	path := tempStorePath(t)
	os.WriteFile(path, []byte("{invalid json"), 0666)

	_, err := NewStore(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewStoreCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "services.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestGetFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	record := &ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000}
	store.Save(record)

	got, ok := store.Get("id1")
	if !ok {
		t.Fatal("expected record to be found")
	}
	if got.Name != "app.localhost" {
		t.Errorf("expected name app.localhost, got %s", got.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected record not to be found")
	}
}

func TestGetByNameFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	record := &ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000}
	store.Save(record)

	got, ok := store.GetByName("app.localhost")
	if !ok {
		t.Fatal("expected record to be found by name")
	}
	if got.ID != "id1" {
		t.Errorf("expected ID id1, got %s", got.ID)
	}
}

func TestGetByNameNotFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	_, ok := store.GetByName("nonexistent.localhost")
	if ok {
		t.Error("expected record not to be found")
	}
}

func TestSaveNew(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	record := &ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000}

	err := store.Save(record)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if len(store.List()) != 1 {
		t.Errorf("expected 1 record, got %d", len(store.List()))
	}
}

func TestSaveUpdate(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	record := &ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000}
	store.Save(record)

	record.Port = 4000
	err := store.Save(record)
	if err != nil {
		t.Fatalf("Save update failed: %v", err)
	}

	got, _ := store.Get("id1")
	if got.Port != 4000 {
		t.Errorf("expected port 4000, got %d", got.Port)
	}
	if len(store.List()) != 1 {
		t.Errorf("expected 1 record after update, got %d", len(store.List()))
	}
}

func TestUpdateNameCleansOldName(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	record := &ServiceRecord{ID: "id1", Name: "old.localhost", Port: 3000}
	store.Save(record)

	store.UpdateName("id1", "new.localhost")

	_, ok := store.GetByName("old.localhost")
	if ok {
		t.Error("old name should no longer resolve")
	}
	_, ok = store.GetByName("new.localhost")
	if !ok {
		t.Error("new name should resolve")
	}
}

func TestUpdateNameSuccess(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	record := &ServiceRecord{ID: "id1", Name: "old.localhost", Port: 3000}
	store.Save(record)

	err := store.UpdateName("id1", "new.localhost")
	if err != nil {
		t.Fatalf("UpdateName failed: %v", err)
	}

	got, _ := store.Get("id1")
	if got.Name != "new.localhost" {
		t.Errorf("expected name new.localhost, got %s", got.Name)
	}
	if !got.UserDefined {
		t.Error("expected UserDefined to be true after rename")
	}
}

func TestUpdateNameNotFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	err := store.UpdateName("nonexistent", "new.localhost")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestUpdateNameConflict(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "app1.localhost", Port: 3000})
	store.Save(&ServiceRecord{ID: "id2", Name: "app2.localhost", Port: 4000})

	err := store.UpdateName("id1", "app2.localhost")
	if err == nil {
		t.Error("expected error for name conflict")
	}
}

func TestList(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "app1.localhost", Port: 3000})
	store.Save(&ServiceRecord{ID: "id2", Name: "app2.localhost", Port: 4000})

	records := store.List()
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

func TestIsNameAvailable(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "taken.localhost", Port: 3000})

	if store.IsNameAvailable("taken.localhost") {
		t.Error("expected name to be unavailable")
	}
	if !store.IsNameAvailable("free.localhost") {
		t.Error("expected name to be available")
	}
}

func TestUpdateKeep(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000, Keep: false})

	err := store.UpdateKeep("id1", true)
	if err != nil {
		t.Fatalf("UpdateKeep failed: %v", err)
	}

	got, _ := store.Get("id1")
	if !got.Keep {
		t.Error("expected Keep to be true")
	}
}

func TestUpdateKeepNotFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	err := store.UpdateKeep("nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestStoreRemove(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000})

	err := store.Remove("id1")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if len(store.List()) != 0 {
		t.Errorf("expected 0 records after remove, got %d", len(store.List()))
	}
	_, ok := store.GetByName("app.localhost")
	if ok {
		t.Error("name should be freed after remove")
	}
}

func TestStoreRemoveNotFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	err := store.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestRemoveByName(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "app.localhost", Port: 3000})

	err := store.RemoveByName("app.localhost")
	if err != nil {
		t.Fatalf("RemoveByName failed: %v", err)
	}
	if len(store.List()) != 0 {
		t.Error("expected 0 records after remove by name")
	}
}

func TestRemoveByNameNotFound(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	err := store.RemoveByName("nonexistent.localhost")
	if err == nil {
		t.Error("expected error for nonexistent name")
	}
}

func TestAddManualService(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	record, err := store.AddManualService("api.localhost", 8080, "192.168.1.1")
	if err != nil {
		t.Fatalf("AddManualService failed: %v", err)
	}
	if record.Name != "api.localhost" {
		t.Errorf("expected name api.localhost, got %s", record.Name)
	}
	if record.Port != 8080 {
		t.Errorf("expected port 8080, got %d", record.Port)
	}
	if record.TargetHost != "192.168.1.1" {
		t.Errorf("expected target host 192.168.1.1, got %s", record.TargetHost)
	}
	if !record.UserDefined {
		t.Error("expected UserDefined to be true")
	}
	if !record.Keep {
		t.Error("expected Keep to be true for manual service")
	}
}

func TestAddManualServiceDefaultHost(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))

	record, err := store.AddManualService("api.localhost", 8080, "")
	if err != nil {
		t.Fatalf("AddManualService failed: %v", err)
	}
	if record.TargetHost != "127.0.0.1" {
		t.Errorf("expected default target host 127.0.0.1, got %s", record.TargetHost)
	}
}

func TestAddManualServiceConflict(t *testing.T) {
	store, _ := NewStore(tempStorePath(t))
	store.Save(&ServiceRecord{ID: "id1", Name: "taken.localhost", Port: 3000})

	_, err := store.AddManualService("taken.localhost", 8080, "")
	if err == nil {
		t.Error("expected error for name conflict")
	}
}

func TestEffectiveTargetHost(t *testing.T) {
	r := &ServiceRecord{TargetHost: ""}
	if r.EffectiveTargetHost() != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", r.EffectiveTargetHost())
	}

	r.TargetHost = "10.0.0.1"
	if r.EffectiveTargetHost() != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", r.EffectiveTargetHost())
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	path := tempStorePath(t)

	store1, _ := NewStore(path)
	store1.Save(&ServiceRecord{
		ID:       "id1",
		Name:     "app.localhost",
		Port:     3000,
		PID:      1234,
		ExePath:  "/bin/app",
		Args:     []string{"/bin/app", "--port", "3000"},
		Keep:     true,
		IsActive: true,
		LastSeen: time.Now(),
	})

	store2, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore reload failed: %v", err)
	}

	records := store2.List()
	if len(records) != 1 {
		t.Fatalf("expected 1 record after reload, got %d", len(records))
	}

	r := records[0]
	if r.ID != "id1" || r.Name != "app.localhost" || r.Port != 3000 {
		t.Errorf("round-trip data mismatch: %+v", r)
	}
	if !r.Keep {
		t.Error("expected Keep to survive round-trip")
	}
	if len(r.Args) != 3 {
		t.Errorf("expected 3 args, got %d", len(r.Args))
	}
}
