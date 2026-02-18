package naming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBuiltinRules(t *testing.T) {
	rules := LoadBuiltinRules()
	if len(rules) == 0 {
		t.Fatal("expected builtin rules to be non-empty")
	}

	// Verify sorted by priority
	for i := 1; i < len(rules); i++ {
		if rules[i].Priority < rules[i-1].Priority {
			t.Errorf("rules not sorted by priority: %s (%d) before %s (%d)",
				rules[i-1].ID, rules[i-1].Priority, rules[i].ID, rules[i].Priority)
		}
	}
}

func TestAppBundleRule(t *testing.T) {
	engine := NewRuleEngine()

	tests := []struct {
		exePath string
		want    string
	}{
		{"/Applications/Ollama.app/Contents/MacOS/Ollama", "Ollama"},
		{"/Applications/Visual Studio Code.app/Contents/MacOS/Electron", "Visual Studio Code"},
		{"/System/Library/CoreServices/ControlCenter.app/Contents/MacOS/ControlCenter", "ControlCenter"},
		{"/usr/bin/curl", ""},
	}

	for _, tt := range tests {
		t.Run(tt.exePath, func(t *testing.T) {
			got := engine.Match(tt.exePath, "/tmp", nil, 0)
			// For non-app-bundle paths, Match may return something from other rules
			if tt.want != "" && got != tt.want {
				t.Errorf("Match(%q) = %q, want %q", tt.exePath, got, tt.want)
			}
		})
	}
}

func TestNodeScriptRule(t *testing.T) {
	engine := NewRuleEngine()

	got := engine.Match("/usr/local/bin/node", "/home/user/projects/myapp",
		[]string{"node", "/home/user/projects/myapp/server.js"}, 3000)
	if got != "myapp" {
		t.Errorf("node script match = %q, want %q", got, "myapp")
	}
}

func TestPythonScriptRule(t *testing.T) {
	engine := NewRuleEngine()

	got := engine.Match("/usr/bin/python3", "/home/user/projects/django-app",
		[]string{"python3", "/home/user/projects/django-app/manage.py"}, 8000)
	if got != "django-app" {
		t.Errorf("python script match = %q, want %q", got, "django-app")
	}
}

func TestPythonHttpServerRule(t *testing.T) {
	engine := NewRuleEngine()

	got := engine.Match("/usr/bin/python3", "/home/user/projects/docs",
		[]string{"python3", "-m", "http.server"}, 8000)
	if got != "docs" {
		t.Errorf("python http.server match = %q, want %q", got, "docs")
	}
}

func TestCwdToolsRule(t *testing.T) {
	engine := NewRuleEngine()

	tools := []struct {
		exe  string
		args []string
	}{
		{"/usr/local/bin/serve", []string{"serve"}},
		{"/usr/local/bin/http-server", []string{"http-server"}},
		{"/usr/local/bin/npx", []string{"npx"}},
		{"/usr/local/bin/live-server", []string{"live-server"}},
		{"/usr/local/bin/browser-sync", []string{"browser-sync"}},
	}

	for _, tt := range tools {
		t.Run(filepath.Base(tt.exe), func(t *testing.T) {
			got := engine.Match(tt.exe, "/home/user/projects/website", tt.args, 3000)
			if got != "website" {
				t.Errorf("cwd tool %s match = %q, want %q", filepath.Base(tt.exe), got, "website")
			}
		})
	}
}

func TestSystemBinaryRule(t *testing.T) {
	engine := NewRuleEngine()

	got := engine.Match("/usr/bin/caddy", "/home/user", []string{"caddy"}, 80)
	if got != "caddy" {
		t.Errorf("system binary match = %q, want %q", got, "caddy")
	}
}

func TestParentDirFallback(t *testing.T) {
	engine := NewRuleEngine()

	got := engine.Match("/opt/myapp/server", "/home/user", []string{"server"}, 8080)
	if got != "myapp" {
		t.Errorf("parent dir fallback = %q, want %q", got, "myapp")
	}
}

func TestStaticNameRule(t *testing.T) {
	rules := []NamingRule{
		{
			ID:         "test-static",
			Priority:   1,
			ExePattern: "(^|/)myserver$",
			NameSource: "static",
			StaticName: "my-custom-name",
		},
	}
	engine := NewRuleEngineFromRules(rules)

	got := engine.Match("/usr/local/bin/myserver", "/tmp", []string{"myserver"}, 8080)
	if got != "my-custom-name" {
		t.Errorf("static name match = %q, want %q", got, "my-custom-name")
	}
}

func TestPriorityOrdering(t *testing.T) {
	rules := []NamingRule{
		{
			ID:         "low-priority",
			Priority:   100,
			NameSource: "static",
			StaticName: "low",
		},
		{
			ID:         "high-priority",
			Priority:   1,
			NameSource: "static",
			StaticName: "high",
		},
	}
	engine := NewRuleEngineFromRules(rules)

	got := engine.Match("/usr/bin/test", "/tmp", nil, 0)
	if got != "high" {
		t.Errorf("priority ordering: got %q, want %q", got, "high")
	}
}

func TestMergeRulesOverride(t *testing.T) {
	builtin := []NamingRule{
		{ID: "rule-a", Priority: 10, Description: "builtin A", NameSource: "exe"},
		{ID: "rule-b", Priority: 20, Description: "builtin B", NameSource: "exe"},
	}
	user := []NamingRule{
		{ID: "rule-a", Priority: 5, Description: "user override A", NameSource: "cwd"},
		{ID: "rule-c", Priority: 15, Description: "user new C", NameSource: "static", StaticName: "custom"},
	}

	merged := MergeRules(builtin, user)

	if len(merged) != 3 {
		t.Fatalf("expected 3 merged rules, got %d", len(merged))
	}

	// Check that rule-a was overridden
	found := false
	for _, r := range merged {
		if r.ID == "rule-a" {
			found = true
			if r.Priority != 5 {
				t.Errorf("rule-a priority = %d, want 5 (user override)", r.Priority)
			}
			if r.NameSource != "cwd" {
				t.Errorf("rule-a NameSource = %q, want %q (user override)", r.NameSource, "cwd")
			}
		}
	}
	if !found {
		t.Error("rule-a not found in merged rules")
	}

	// Check rule-c was added
	found = false
	for _, r := range merged {
		if r.ID == "rule-c" {
			found = true
		}
	}
	if !found {
		t.Error("rule-c not found in merged rules")
	}

	// Verify sorted by priority
	for i := 1; i < len(merged); i++ {
		if merged[i].Priority < merged[i-1].Priority {
			t.Errorf("merged rules not sorted: %s (%d) after %s (%d)",
				merged[i].ID, merged[i].Priority, merged[i-1].ID, merged[i-1].Priority)
		}
	}
}

func TestLoadUserRulesFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	rulesFile := filepath.Join(tmpDir, "rules.json")

	rules := []NamingRule{
		{
			ID:          "custom-rule",
			Description: "My custom rule",
			Priority:    1,
			ExePattern:  "(^|/)myapp$",
			NameSource:  "static",
			StaticName:  "my-app",
		},
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal rules: %v", err)
	}

	if err := os.WriteFile(rulesFile, data, 0644); err != nil {
		t.Fatalf("failed to write rules file: %v", err)
	}

	loaded, err := LoadUserRules(rulesFile)
	if err != nil {
		t.Fatalf("LoadUserRules failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(loaded))
	}

	if loaded[0].ID != "custom-rule" {
		t.Errorf("loaded rule ID = %q, want %q", loaded[0].ID, "custom-rule")
	}
}

func TestLoadUserRulesFileNotFound(t *testing.T) {
	_, err := LoadUserRules("/nonexistent/path/rules.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPortPatternRule(t *testing.T) {
	rules := []NamingRule{
		{
			ID:          "port-match",
			Priority:    1,
			PortPattern: "^3000$",
			NameSource:  "static",
			StaticName:  "dev-server",
		},
	}
	engine := NewRuleEngineFromRules(rules)

	got := engine.Match("/usr/bin/node", "/tmp", nil, 3000)
	if got != "dev-server" {
		t.Errorf("port pattern match = %q, want %q", got, "dev-server")
	}

	got = engine.Match("/usr/bin/node", "/tmp", nil, 8080)
	if got != "" {
		t.Errorf("port pattern non-match = %q, want %q", got, "")
	}
}

func TestCwdPatternRule(t *testing.T) {
	rules := []NamingRule{
		{
			ID:         "cwd-match",
			Priority:   1,
			CwdPattern: "/projects/",
			NameSource: "cwd",
		},
	}
	engine := NewRuleEngineFromRules(rules)

	got := engine.Match("/usr/bin/node", "/home/user/projects/myapp", nil, 0)
	if got != "myapp" {
		t.Errorf("cwd pattern match = %q, want %q", got, "myapp")
	}

	got = engine.Match("/usr/bin/node", "/tmp", nil, 0)
	if got != "" {
		t.Errorf("cwd pattern non-match = %q, want %q", got, "")
	}
}

func TestExportRulesJSON(t *testing.T) {
	engine := NewRuleEngine()
	data, err := engine.ExportRulesJSON()
	if err != nil {
		t.Fatalf("ExportRulesJSON failed: %v", err)
	}

	var rules []NamingRule
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("exported JSON is not valid: %v", err)
	}

	if len(rules) == 0 {
		t.Error("exported rules should not be empty")
	}
}

func TestEmptyCwdReturnsEmpty(t *testing.T) {
	rules := []NamingRule{
		{
			ID:         "cwd-rule",
			Priority:   1,
			NameSource: "cwd",
		},
	}
	engine := NewRuleEngineFromRules(rules)

	got := engine.Match("/usr/bin/node", "", nil, 0)
	if got != "" {
		t.Errorf("empty cwd should return empty, got %q", got)
	}
}
