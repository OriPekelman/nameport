package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"localhost-magic/internal/naming"
	"localhost-magic/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	storePath := storage.DefaultStorePath()
	blacklistPath := storage.DefaultBlacklistPath()

	// Check for custom store path
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			storePath = os.Args[i+1]
			// Remove these args
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
	}

	store, err := storage.NewStore(storePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}

	blacklistStore, err := storage.NewBlacklistStore(blacklistPath)
	if err != nil {
		log.Fatalf("Failed to open blacklist store: %v", err)
	}

	command := os.Args[1]

	switch command {
	case "list", "ls":
		cmdList(store)
	case "rename", "mv":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: localhost-magic rename <old-name> <new-name>\n")
			os.Exit(1)
		}
		cmdRename(store, os.Args[2], os.Args[3])
	case "keep":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: localhost-magic keep <name> [true|false]\n")
			os.Exit(1)
		}
		keepVal := true
		if len(os.Args) > 3 {
			keepVal = strings.ToLower(os.Args[3]) == "true" || os.Args[3] == "1"
		}
		cmdKeep(store, os.Args[2], keepVal)
	case "blacklist":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: localhost-magic blacklist <subcommand>\n")
			fmt.Fprintf(os.Stderr, "  blacklist <type> <value>     Add to blacklist (type: pid|path|pattern)\n")
			fmt.Fprintf(os.Stderr, "  blacklist list               List all blacklist entries\n")
			fmt.Fprintf(os.Stderr, "  blacklist remove <id>        Remove a blacklist entry\n")
			os.Exit(1)
		}
		subCmd := os.Args[2]
		switch subCmd {
		case "list":
			cmdBlacklistList(blacklistStore)
		case "remove":
			if len(os.Args) < 4 {
				fmt.Fprintf(os.Stderr, "Usage: localhost-magic blacklist remove <id>\n")
				os.Exit(1)
			}
			cmdBlacklistRemove(blacklistStore, os.Args[3])
		default:
			// Treat as blacklist add: blacklist <type> <value>
			if len(os.Args) < 4 {
				fmt.Fprintf(os.Stderr, "Usage: localhost-magic blacklist <type> <value>\n")
				fmt.Fprintf(os.Stderr, "  type: pid|path|pattern\n")
				os.Exit(1)
			}
			cmdBlacklistAdd(blacklistStore, os.Args[2], os.Args[3])
		}
	case "rules":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: localhost-magic rules <list|export|import> [file]\n")
			os.Exit(1)
		}
		cmdRules(os.Args[2:])
	case "add":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: localhost-magic add <name> [host:]<port>\n")
			os.Exit(1)
		}
		target := os.Args[3]
		var targetHost string
		var port int
		if idx := strings.LastIndex(target, ":"); idx != -1 {
			// host:port format
			targetHost = target[:idx]
			port, err = strconv.Atoi(target[idx+1:])
			if err != nil {
				log.Fatalf("Invalid port number in %s", target)
			}
		} else {
			// port only, default to 127.0.0.1
			port, err = strconv.Atoi(target)
			if err != nil {
				log.Fatalf("Invalid port number: %s", target)
			}
		}
		cmdAdd(store, os.Args[2], port, targetHost)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("localhost-magic - Manage local service DNS names")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  localhost-magic list                          List all registered services")
	fmt.Println("  localhost-magic rename <old> <new>            Rename a service")
	fmt.Println("  localhost-magic keep <name> [true|false]      Toggle keep status (default: true)")
	fmt.Println("  localhost-magic blacklist <type> <value>      Add to blacklist")
	fmt.Println("  localhost-magic blacklist list                List all blacklist entries")
	fmt.Println("  localhost-magic blacklist remove <id>         Remove a blacklist entry")
	fmt.Println("  localhost-magic rules list                    List naming rules")
	fmt.Println("  localhost-magic rules export                  Export rules as JSON")
	fmt.Println("  localhost-magic rules import <file>           Import user rules from file")
	fmt.Println("  localhost-magic add <name> [host:]<port>       Add manual service entry")
	fmt.Println("  localhost-magic --config <path>               Use custom config path")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  <type> for blacklist: pid, path, or pattern")
	fmt.Println("  <value>: PID number, executable path, or regex pattern")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  localhost-magic list")
	fmt.Println("  localhost-magic rename myapp.localhost api.localhost")
	fmt.Println("  localhost-magic keep myapp.localhost")
	fmt.Println("  localhost-magic keep myapp.localhost false")
	fmt.Println("  localhost-magic blacklist pid 12345")
	fmt.Println("  localhost-magic blacklist path /usr/sbin/cupsd")
	fmt.Println("  localhost-magic blacklist pattern '^localhost-magic'")
	fmt.Println("  localhost-magic add myapp.localhost 3000")
	fmt.Println("  localhost-magic add myapp.localhost 192.168.0.1:3000")
}

func cmdList(store *storage.Store) {
	records := store.List()

	if len(records) == 0 {
		fmt.Println("No services registered.")
		fmt.Println("Start the daemon and run some local HTTP services.")
		return
	}

	fmt.Printf("%-30s %-22s %-8s %-6s %s\n", "NAME", "TARGET", "PID", "KEEP", "COMMAND")
	fmt.Println(strings.Repeat("-", 110))

	for _, r := range records {
		cmd := r.ExePath
		if len(r.Args) > 1 {
			cmd = fmt.Sprintf("%s %s", r.ExePath, strings.Join(r.Args[1:], " "))
		}
		if len(cmd) > 50 {
			cmd = cmd[:47] + "..."
		}

		markers := ""
		if r.UserDefined {
			markers += "*"
		}
		if r.Keep {
			markers += "K"
		}

		keepStr := ""
		if r.Keep {
			keepStr = "YES"
		}

		target := fmt.Sprintf("%s:%d", r.EffectiveTargetHost(), r.Port)
		fmt.Printf("%-30s %-22s %-8d %-6s %s%s\n", r.Name, target, r.PID, keepStr, markers, cmd)
	}

	fmt.Println()
	fmt.Println("* = user-defined name, K = kept, YES = keep enabled")
}

func cmdRename(store *storage.Store, oldName, newName string) {
	// Ensure .localhost suffix
	if !strings.HasSuffix(oldName, ".localhost") {
		oldName = oldName + ".localhost"
	}
	if !strings.HasSuffix(newName, ".localhost") {
		newName = newName + ".localhost"
	}

	// Find the service
	record, ok := store.GetByName(oldName)
	if !ok {
		log.Fatalf("Service not found: %s", oldName)
	}

	// Check if new name is available
	if _, exists := store.GetByName(newName); exists {
		log.Fatalf("Name already in use: %s", newName)
	}

	// Perform rename
	if err := store.UpdateName(record.ID, newName); err != nil {
		log.Fatalf("Failed to rename: %v", err)
	}

	fmt.Printf("Renamed %s -> %s\n", oldName, newName)
	fmt.Println("Note: You may need to restart the daemon for changes to take effect.")
}

func cmdKeep(store *storage.Store, name string, keep bool) {
	// Ensure .localhost suffix
	if !strings.HasSuffix(name, ".localhost") {
		name = name + ".localhost"
	}

	// Find the service
	record, ok := store.GetByName(name)
	if !ok {
		log.Fatalf("Service not found: %s", name)
	}

	// Update keep status
	if err := store.UpdateKeep(record.ID, keep); err != nil {
		log.Fatalf("Failed to update keep status: %v", err)
	}

	status := "enabled"
	if !keep {
		status = "disabled"
	}
	fmt.Printf("Keep %s for %s\n", status, name)
	fmt.Println("Note: You may need to restart the daemon for changes to take effect.")
}

func cmdBlacklistAdd(blacklistStore *storage.BlacklistStore, blacklistType, value string) {
	entry, err := blacklistStore.Add(blacklistType, value)
	if err != nil {
		log.Fatalf("Failed to add blacklist entry: %v", err)
	}

	fmt.Printf("Added blacklist entry: [%s] %s = %s\n", entry.ID, entry.Type, entry.Value)
	fmt.Println("Note: The daemon will pick up this change on its next scan cycle.")
}

func cmdBlacklistList(blacklistStore *storage.BlacklistStore) {
	entries := blacklistStore.List()

	if len(entries) == 0 {
		fmt.Println("No user-defined blacklist entries.")
		fmt.Println("(Built-in system blacklist rules are always active.)")
		return
	}

	fmt.Printf("%-18s %-10s %-40s %s\n", "ID", "TYPE", "VALUE", "CREATED")
	fmt.Println(strings.Repeat("-", 90))

	for _, e := range entries {
		fmt.Printf("%-18s %-10s %-40s %s\n", e.ID, e.Type, e.Value, e.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func cmdBlacklistRemove(blacklistStore *storage.BlacklistStore, id string) {
	if err := blacklistStore.Remove(id); err != nil {
		log.Fatalf("Failed to remove blacklist entry: %v", err)
	}

	fmt.Printf("Removed blacklist entry: %s\n", id)
}

func cmdAdd(store *storage.Store, name string, port int, targetHost string) {
	// Ensure .localhost suffix
	if !strings.HasSuffix(name, ".localhost") {
		name = name + ".localhost"
	}

	// Add the manual service
	record, err := store.AddManualService(name, port, targetHost)
	if err != nil {
		log.Fatalf("Failed to add service: %v", err)
	}

	fmt.Printf("Added manual service: %s -> %s:%d\n", record.Name, record.EffectiveTargetHost(), record.Port)
	fmt.Println("Note: This service will be kept even when not running.")
	fmt.Println("      Restart the daemon to activate the proxy.")
}

func cmdRules(args []string) {
	subCmd := args[0]
	engine := naming.NewRuleEngine()

	switch subCmd {
	case "list":
		rules := engine.Rules()
		fmt.Printf("%-25s %-8s %s\n", "ID", "PRIORITY", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 80))
		for _, r := range rules {
			fmt.Printf("%-25s %-8d %s\n", r.ID, r.Priority, r.Description)
		}
		fmt.Printf("\n%d rules loaded (user overrides: %s)\n", len(rules), naming.UserRulesPath())

	case "export":
		data, err := engine.ExportRulesJSON()
		if err != nil {
			log.Fatalf("Failed to export rules: %v", err)
		}
		fmt.Println(string(data))

	case "import":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: localhost-magic rules import <file>\n")
			os.Exit(1)
		}
		srcFile := args[1]

		// Validate the source file is valid JSON rules
		_, err := naming.LoadUserRules(srcFile)
		if err != nil {
			log.Fatalf("Invalid rules file: %v", err)
		}

		// Read source
		data, err := os.ReadFile(srcFile)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}

		// Ensure destination directory exists
		destPath := naming.UserRulesPath()
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			log.Fatalf("Failed to create config directory: %v", err)
		}

		// Write to user config
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			log.Fatalf("Failed to write user rules: %v", err)
		}

		fmt.Printf("Imported rules to %s\n", destPath)
		fmt.Println("Note: Rules will take effect on next daemon restart.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown rules command: %s\n", subCmd)
		fmt.Fprintf(os.Stderr, "Usage: localhost-magic rules <list|export|import> [file]\n")
		os.Exit(1)
	}
}
