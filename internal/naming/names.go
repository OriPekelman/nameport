// Package naming generates stable, human-readable names from process information
package naming

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ExtractBaseName extracts the best possible name from an executable path and CWD
// Uses heuristics for different types of applications
func ExtractBaseName(exePath string, cwd string, args []string) string {
	exeName := filepath.Base(exePath)

	// 1. Check for macOS .app bundles
	if appName := extractAppBundleName(exePath); appName != "" {
		return appName
	}

	// 2. Check for common package managers/languages with project indicators
	if projectName := extractProjectNameFromArgs(args); projectName != "" {
		return projectName
	}

	// 3. For tools that run from CWD (serve, python -m http.server, node without full path, etc.)
	// use the CWD directory name instead
	if shouldUseCwd(exeName, args) && cwd != "" {
		return filepath.Base(cwd)
	}

	// 4. For system binaries (/usr/bin, /usr/sbin, /bin, /sbin), use executable name
	if isSystemBinary(exePath) {
		return exeName
	}

	// 5. Use the parent directory name (original behavior)
	dir := filepath.Dir(exePath)
	base := filepath.Base(dir)

	// 6. If parent is generic (bin, sbin, lib, libexec), use the executable name instead
	if isGenericDir(base) {
		return exeName
	}

	return base
}

// isSystemBinary checks if the executable is a system binary
func isSystemBinary(exePath string) bool {
	systemPaths := []string{"/usr/bin/", "/usr/sbin/", "/bin/", "/sbin/"}
	for _, prefix := range systemPaths {
		if strings.HasPrefix(exePath, prefix) {
			return true
		}
	}
	return false
}

// shouldUseCwd checks if the tool should use CWD for naming
// These are tools that derive their context from where they're run
func shouldUseCwd(exeName string, args []string) bool {
	// Standalone tools that serve current directory
	standaloneTools := []string{"serve", "http-server", "hs", "npx", "live-server", "browser-sync"}
	for _, tool := range standaloneTools {
		if exeName == tool {
			return true
		}
	}

	// Python module servers
	if exeName == "python" || exeName == "python3" {
		// Check for http.server module
		for _, arg := range args {
			if arg == "http.server" || arg == "-mhttp.server" {
				return true
			}
		}
	}

	// Node tools that serve current directory
	if exeName == "node" || exeName == "nodejs" {
		// If no script path provided (just running with flags)
		if len(args) < 2 {
			return true
		}
		// Check if first arg is a flag (not a script path)
		if len(args) > 1 && strings.HasPrefix(args[1], "-") {
			return true
		}
		// If script path is relative (no directory separators), use CWD
		if len(args) > 1 && !strings.Contains(args[1], string(filepath.Separator)) && !strings.HasPrefix(args[1], ".") {
			return true
		}
	}

	return false
}

// extractAppBundleName extracts the app name from a macOS bundle path
// e.g., "/Applications/Ollama.app/Contents/MacOS/Ollama" -> "Ollama"
// e.g., "/System/Library/CoreServices/ControlCenter.app/..." -> "ControlCenter"
func extractAppBundleName(exePath string) string {
	// Look for .app in the path
	parts := strings.Split(exePath, string(filepath.Separator))

	for _, part := range parts {
		if strings.HasSuffix(part, ".app") {
			// Remove .app suffix
			name := strings.TrimSuffix(part, ".app")
			return name
		}
	}

	return ""
}

// extractProjectNameFromArgs tries to find the project name from command line arguments
// e.g., node server.js -> looks for package.json or parent dir
// e.g., python manage.py -> looks for parent dir
func extractProjectNameFromArgs(args []string) string {
	if len(args) < 2 {
		return ""
	}

	// For interpreted languages, look at the script path
	scriptPath := args[1]

	// Only use this if it's an absolute or relative path (not a module name)
	if strings.Contains(scriptPath, string(filepath.Separator)) || strings.HasPrefix(scriptPath, ".") {
		dir := filepath.Dir(scriptPath)
		if dir != "" && dir != "." {
			return filepath.Base(dir)
		}
	}

	return ""
}

// isGenericDir checks if a directory name is too generic to be useful
func isGenericDir(name string) bool {
	generic := []string{"bin", "sbin", "lib", "libexec", "usr", "local", "opt", "var", "etc"}
	for _, g := range generic {
		if strings.EqualFold(name, g) {
			return true
		}
	}
	return false
}

// Generator creates stable names from process information
type Generator struct {
	usedNames  map[string]bool // Tracks which names are in use
	ruleEngine *RuleEngine     // Data-driven naming rules
}

// NewGenerator creates a new name generator with a RuleEngine
func NewGenerator() *Generator {
	return &Generator{
		usedNames:  make(map[string]bool),
		ruleEngine: NewRuleEngine(),
	}
}

// NewGeneratorWithEngine creates a new name generator with a specific RuleEngine
func NewGeneratorWithEngine(engine *RuleEngine) *Generator {
	return &Generator{
		usedNames:  make(map[string]bool),
		ruleEngine: engine,
	}
}

// RuleEngine returns the generator's rule engine
func (g *Generator) RuleEngine() *RuleEngine {
	return g.ruleEngine
}

// GenerateName creates a .localhost name from an executable path
// Handles collisions by appending -1, -2, etc.
func (g *Generator) GenerateName(exePath string, cwd string, args []string) string {
	// Try data-driven rules first
	baseName := ""
	if g.ruleEngine != nil {
		baseName = g.ruleEngine.Match(exePath, cwd, args, 0)
	}

	// Fall back to hardcoded heuristics for edge cases
	if baseName == "" {
		baseName = ExtractBaseName(exePath, cwd, args)
	}

	cleaned := SanitizeName(baseName)

	// Try the base name first
	if !g.usedNames[cleaned] {
		g.usedNames[cleaned] = true
		return cleaned + ".localhost"
	}

	// Find next available number
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", cleaned, i)
		if !g.usedNames[candidate] {
			g.usedNames[candidate] = true
			return candidate + ".localhost"
		}
	}

	// Fallback: use hash
	hash := computeHash(exePath)
	shortHash := hash[:8]
	return fmt.Sprintf("%s-%s.localhost", cleaned, shortHash)
}

// ReleaseName marks a name as no longer in use
func (g *Generator) ReleaseName(name string) {
	// Remove .localhost suffix if present
	name = strings.TrimSuffix(name, ".localhost")
	delete(g.usedNames, name)
}

// SanitizeName converts to lowercase and keeps only alphanumeric characters
func SanitizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace non-alphanumeric with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	name = re.ReplaceAllString(name, "-")

	// Trim hyphens from ends
	name = strings.Trim(name, "-")

	// Ensure it's not empty
	if name == "" {
		name = "app"
	}

	// Limit length
	if len(name) > 50 {
		name = name[:50]
	}

	return name
}

// computeHash creates a stable hash of the executable path
func computeHash(exePath string) string {
	h := sha256.New()
	h.Write([]byte(exePath))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ComputeIdentityHash creates a unique identifier for a process
// Based on executable path and arguments
func ComputeIdentityHash(exePath string, args []string) string {
	h := sha256.New()
	h.Write([]byte(exePath))
	h.Write([]byte("\x00"))
	for _, arg := range args {
		h.Write([]byte(arg))
		h.Write([]byte("\x00"))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
