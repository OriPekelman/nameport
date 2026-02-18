package naming

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed rules_builtin.json
var builtinRulesJSON []byte

// NamingRule defines a data-driven naming heuristic
type NamingRule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Priority    int    `json:"priority"` // lower = higher priority

	// Match conditions (all optional, AND-ed when present)
	ExePattern  string `json:"exe_pattern,omitempty"`  // regex on exe path
	ArgPattern  string `json:"arg_pattern,omitempty"`  // regex on joined args
	CwdPattern  string `json:"cwd_pattern,omitempty"`  // regex on cwd
	PortPattern string `json:"port_pattern,omitempty"` // regex on port string

	// Name extraction
	NameSource string `json:"name_source"`            // "exe", "cwd", "arg", "parent_dir", "app_bundle", "static"
	NameRegex  string `json:"name_regex,omitempty"`    // capture group 1 = name
	StaticName string `json:"static_name,omitempty"`   // when name_source = "static"
}

// RuleEngine applies naming rules in priority order
type RuleEngine struct {
	rules []NamingRule
}

// NewRuleEngine creates a RuleEngine loaded with built-in and user rules
func NewRuleEngine() *RuleEngine {
	builtin := LoadBuiltinRules()
	userRules, _ := LoadUserRules(defaultUserRulesPath())
	merged := MergeRules(builtin, userRules)
	return &RuleEngine{rules: merged}
}

// NewRuleEngineFromRules creates a RuleEngine from the given rules (for testing)
func NewRuleEngineFromRules(rules []NamingRule) *RuleEngine {
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return &RuleEngine{rules: rules}
}

// Rules returns the current rules (sorted by priority)
func (re *RuleEngine) Rules() []NamingRule {
	result := make([]NamingRule, len(re.rules))
	copy(result, re.rules)
	return result
}

// LoadBuiltinRules parses the embedded rules JSON
func LoadBuiltinRules() []NamingRule {
	var rules []NamingRule
	if err := json.Unmarshal(builtinRulesJSON, &rules); err != nil {
		// Should never happen with embedded data
		panic(fmt.Sprintf("failed to parse builtin rules: %v", err))
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return rules
}

// LoadUserRules loads rules from a user config file
func LoadUserRules(path string) ([]NamingRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rules []NamingRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse user rules from %s: %w", path, err)
	}

	return rules, nil
}

// MergeRules merges user rules on top of builtin rules.
// User rules with the same ID override builtin rules; new IDs are added.
func MergeRules(builtin, user []NamingRule) []NamingRule {
	ruleMap := make(map[string]NamingRule, len(builtin)+len(user))
	for _, r := range builtin {
		ruleMap[r.ID] = r
	}
	for _, r := range user {
		ruleMap[r.ID] = r
	}

	merged := make([]NamingRule, 0, len(ruleMap))
	for _, r := range ruleMap {
		merged = append(merged, r)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Priority == merged[j].Priority {
			return merged[i].ID < merged[j].ID
		}
		return merged[i].Priority < merged[j].Priority
	})

	return merged
}

// Match tries rules in priority order and returns the first matching name, or ""
func (re *RuleEngine) Match(exePath, cwd string, args []string, port int) string {
	joinedArgs := strings.Join(args, " ")
	portStr := strconv.Itoa(port)

	for _, rule := range re.rules {
		if !ruleMatches(rule, exePath, joinedArgs, cwd, portStr) {
			continue
		}

		name := extractName(rule, exePath, cwd, args)
		if name != "" {
			return name
		}
	}

	return ""
}

// ruleMatches checks if all specified patterns in a rule match the inputs
func ruleMatches(rule NamingRule, exePath, joinedArgs, cwd, portStr string) bool {
	if rule.ExePattern != "" {
		matched, err := regexp.MatchString(rule.ExePattern, exePath)
		if err != nil || !matched {
			return false
		}
	}

	if rule.ArgPattern != "" {
		matched, err := regexp.MatchString(rule.ArgPattern, joinedArgs)
		if err != nil || !matched {
			return false
		}
	}

	if rule.CwdPattern != "" {
		matched, err := regexp.MatchString(rule.CwdPattern, cwd)
		if err != nil || !matched {
			return false
		}
	}

	if rule.PortPattern != "" {
		matched, err := regexp.MatchString(rule.PortPattern, portStr)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// extractName extracts the name based on the rule's NameSource
func extractName(rule NamingRule, exePath, cwd string, args []string) string {
	switch rule.NameSource {
	case "exe":
		return filepath.Base(exePath)

	case "cwd":
		if cwd == "" {
			return ""
		}
		return filepath.Base(cwd)

	case "arg":
		if len(args) < 2 {
			return ""
		}
		// Apply NameRegex to each arg (skip argv[0]) to find a match
		if rule.NameRegex != "" {
			re, err := regexp.Compile(rule.NameRegex)
			if err != nil {
				return ""
			}
			for _, arg := range args[1:] {
				matches := re.FindStringSubmatch(arg)
				if len(matches) >= 2 {
					return matches[1]
				}
			}
		}
		return ""

	case "parent_dir":
		dir := filepath.Dir(exePath)
		base := filepath.Base(dir)
		if isGenericDir(base) {
			return filepath.Base(exePath)
		}
		return base

	case "app_bundle":
		if rule.NameRegex != "" {
			re, err := regexp.Compile(rule.NameRegex)
			if err != nil {
				return ""
			}
			matches := re.FindStringSubmatch(exePath)
			if len(matches) >= 2 {
				return matches[1]
			}
		}
		// Fallback: scan path parts for .app suffix
		parts := strings.Split(exePath, string(filepath.Separator))
		for _, part := range parts {
			if strings.HasSuffix(part, ".app") {
				return strings.TrimSuffix(part, ".app")
			}
		}
		return ""

	case "static":
		return rule.StaticName

	default:
		return ""
	}
}

// defaultUserRulesPath returns the path for user-defined naming rules
func defaultUserRulesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "localhost-magic", "naming-rules.json")
}

// UserRulesPath returns the path to the user rules file (exported for CLI)
func UserRulesPath() string {
	return defaultUserRulesPath()
}

// ExportRulesJSON exports the current rules as formatted JSON
func (re *RuleEngine) ExportRulesJSON() ([]byte, error) {
	return json.MarshalIndent(re.rules, "", "  ")
}
