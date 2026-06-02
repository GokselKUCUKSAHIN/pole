package pole

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type testConfig struct {
	Name    string `json:"name"`
	Value   int    `json:"value"`
	Enabled bool   `json:"enabled"`
}

type testConfigWithCallback struct {
	Value int `json:"value" onChange:"OnValueChanged"`

	changedOld atomic.Int64
	changedNew atomic.Int64
	callCount  atomic.Int32
}

func (c *testConfigWithCallback) OnValueChanged(old, new int) {
	c.changedOld.Store(int64(old))
	c.changedNew.Store(int64(new))
	c.callCount.Add(1)
}

func createTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}

func shortInterval[T any]() func(*FileReader[T]) {
	return WithCheckInterval[T](10 * time.Millisecond)
}

func TestRead_ReturnsFileReader(t *testing.T) {
	path := createTempJSON(t, `{"name":"test","value":42,"enabled":true}`)

	reader, err := Read[testConfig](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reader == nil {
		t.Fatal("expected non-nil FileReader")
	}
}

func TestRead_NonExistentFile(t *testing.T) {
	_, err := Read[testConfig]("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestRead_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.cfg")
	_ = os.WriteFile(path, []byte("key=value"), 0644)

	_, err := Read[testConfig](path)
	if err == nil {
		t.Fatal("expected error for unsupported file extension")
	}
}

func TestCurrent_ReturnsInitialValues(t *testing.T) {
	path := createTempJSON(t, `{"name":"hello","value":99,"enabled":true}`)

	reader, err := Read[testConfig](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := reader.Current()
	if cfg == nil {
		t.Fatal("Current() returned nil")
	}
	if cfg.Name != "hello" {
		t.Errorf("Name: got %q, want %q", cfg.Name, "hello")
	}
	if cfg.Value != 99 {
		t.Errorf("Value: got %d, want %d", cfg.Value, 99)
	}
	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
}

// Core regression test: Current() must reflect file changes, not return stale first-read values.
func TestCurrent_UpdatesAfterFileChange(t *testing.T) {
	path := createTempJSON(t, `{"name":"initial","value":1,"enabled":false}`)

	reader, err := Read[testConfig](path, shortInterval[testConfig]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.activeWatcher.Stop()

	if reader.Current().Name != "initial" {
		t.Fatalf("pre-condition failed: expected 'initial', got %q", reader.Current().Name)
	}

	time.Sleep(20 * time.Millisecond)

	if err := os.WriteFile(path, []byte(`{"name":"updated","value":2,"enabled":true}`), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if reader.Current().Name == "updated" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cfg := reader.Current()
	if cfg.Name != "updated" {
		t.Errorf("Name: got %q, want %q — Current() returned stale value", cfg.Name, "updated")
	}
	if cfg.Value != 2 {
		t.Errorf("Value: got %d, want 2", cfg.Value)
	}
	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
}

func TestCurrent_MultipleChangesAllReflected(t *testing.T) {
	path := createTempJSON(t, `{"name":"v1","value":1,"enabled":false}`)

	reader, err := Read[testConfig](path, shortInterval[testConfig]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.activeWatcher.Stop()

	waitFor := func(want string) {
		t.Helper()
		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			if reader.Current().Name == want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Errorf("timed out waiting for Name=%q, got %q", want, reader.Current().Name)
	}

	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(path, []byte(`{"name":"v2","value":2,"enabled":true}`), 0644)
	waitFor("v2")

	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(path, []byte(`{"name":"v3","value":3,"enabled":false}`), 0644)
	waitFor("v3")

	cfg := reader.Current()
	if cfg.Value != 3 {
		t.Errorf("Value: got %d, want 3", cfg.Value)
	}
}

func TestCurrent_OnChangeCallbackFired(t *testing.T) {
	path := createTempJSON(t, `{"value":10}`)

	reader, err := Read[testConfigWithCallback](path, shortInterval[testConfigWithCallback]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.activeWatcher.Stop()

	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(path, []byte(`{"value":20}`), 0644)

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if reader.Current().callCount.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cfg := reader.Current()
	if cfg.callCount.Load() == 0 {
		t.Error("onChange callback was not called")
	}
	if cfg.changedOld.Load() != 10 {
		t.Errorf("callback old value: got %d, want 10", cfg.changedOld.Load())
	}
	if cfg.changedNew.Load() != 20 {
		t.Errorf("callback new value: got %d, want 20", cfg.changedNew.Load())
	}
}

func TestCurrent_NoCallbackWhenValueUnchanged(t *testing.T) {
	path := createTempJSON(t, `{"value":10}`)

	reader, err := Read[testConfigWithCallback](path, shortInterval[testConfigWithCallback]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.activeWatcher.Stop()

	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(path, []byte(`{"value":10}`), 0644)
	time.Sleep(100 * time.Millisecond)

	if reader.Current().callCount.Load() != 0 {
		t.Error("onChange should not fire when value is unchanged")
	}
}

func TestCurrent_ConcurrentReads(t *testing.T) {
	path := createTempJSON(t, `{"name":"start","value":0,"enabled":false}`)

	reader, err := Read[testConfig](path, shortInterval[testConfig]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.activeWatcher.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_ = os.WriteFile(path, []byte(`{"name":"updated","value":1,"enabled":true}`), 0644)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// concurrent reads must not race
	for i := 0; i < 100; i++ {
		_ = reader.Current()
		time.Sleep(2 * time.Millisecond)
	}

	<-done
}

func TestRead_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("name: yaml\nvalue: 7\nenabled: true\n"), 0644)

	reader, err := Read[testConfig](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := reader.Current()
	if cfg.Name != "yaml" || cfg.Value != 7 {
		t.Errorf("unexpected values: %+v", cfg)
	}
}

func TestRead_TOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(path, []byte("name = \"toml\"\nvalue = 5\nenabled = false\n"), 0644)

	reader, err := Read[testConfig](path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := reader.Current()
	if cfg.Name != "toml" || cfg.Value != 5 {
		t.Errorf("unexpected values: %+v", cfg)
	}
}

func TestWithCheckInterval_UsedByWatcher(t *testing.T) {
	path := createTempJSON(t, `{"name":"before","value":1,"enabled":false}`)

	// 500ms interval — change should NOT be picked up within 100ms
	reader, err := Read[testConfig](path, WithCheckInterval[testConfig](500*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.activeWatcher.Stop()

	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(path, []byte(`{"name":"after","value":2,"enabled":true}`), 0644)
	time.Sleep(100 * time.Millisecond)

	if reader.Current().Name == "after" {
		t.Error("config updated too soon — check interval not respected")
	}
}
