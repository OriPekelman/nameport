package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nameport/internal/naming"
	"nameport/internal/notify"
	"nameport/internal/storage"
	"nameport/internal/tls/ca"
	"nameport/internal/tls/issuer"
	"nameport/internal/tls/policy"
	"nameport/internal/tls/trust"
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
			fmt.Fprintf(os.Stderr, "Usage: nameport rename <old-name> <new-name>\n")
			os.Exit(1)
		}
		cmdRename(store, os.Args[2], os.Args[3])
	case "keep":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport keep <name> [true|false]\n")
			os.Exit(1)
		}
		keepVal := true
		if len(os.Args) > 3 {
			keepVal = strings.ToLower(os.Args[3]) == "true" || os.Args[3] == "1"
		}
		cmdKeep(store, os.Args[2], keepVal)
	case "blacklist":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport blacklist <subcommand>\n")
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
				fmt.Fprintf(os.Stderr, "Usage: nameport blacklist remove <id>\n")
				os.Exit(1)
			}
			cmdBlacklistRemove(blacklistStore, os.Args[3])
		default:
			// Treat as blacklist add: blacklist <type> <value>
			if len(os.Args) < 4 {
				fmt.Fprintf(os.Stderr, "Usage: nameport blacklist <type> <value>\n")
				fmt.Fprintf(os.Stderr, "  type: pid|path|pattern\n")
				os.Exit(1)
			}
			cmdBlacklistAdd(blacklistStore, os.Args[2], os.Args[3])
		}
	case "rules":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport rules <list|export|import> [file]\n")
			os.Exit(1)
		}
		cmdRules(os.Args[2:])
	case "notify":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport notify <status|enable|disable|events>\n")
			os.Exit(1)
		}
		cmdNotify(os.Args[2:])
	case "tls":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport tls <init|status|ensure|list|revoke|rotate|export|untrust>\n")
			os.Exit(1)
		}
		cmdTLS(os.Args[2:])
	case "cleanup":
		cmdCleanup()
	case "remove", "rm":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport remove <name>\n")
			os.Exit(1)
		}
		cmdRemove(store, os.Args[2])
	case "add":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: nameport add <name> [host:]<port>\n")
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
	fmt.Println("nameport - Manage local service DNS names")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  nameport list                          List all registered services")
	fmt.Println("  nameport rename <old> <new>            Rename a service")
	fmt.Println("  nameport keep <name> [true|false]      Toggle keep status (default: true)")
	fmt.Println("  nameport blacklist <type> <value>      Add to blacklist")
	fmt.Println("  nameport blacklist list                List all blacklist entries")
	fmt.Println("  nameport blacklist remove <id>         Remove a blacklist entry")
	fmt.Println("  nameport rules list                    List naming rules")
	fmt.Println("  nameport rules export                  Export rules as JSON")
	fmt.Println("  nameport rules import <file>           Import user rules from file")
	fmt.Println("  nameport remove <name>                 Remove a service entry")
	fmt.Println("  nameport add <name> [host:]<port>      Add manual service entry")
	fmt.Println("  nameport notify status                 Show notification config")
	fmt.Println("  nameport notify enable                 Enable notifications")
	fmt.Println("  nameport notify disable                Disable notifications")
	fmt.Println("  nameport notify events <type> on|off   Toggle event type")
	fmt.Println()
	fmt.Println("TLS Commands:")
	fmt.Println("  nameport tls init                      Bootstrap CA and install into trust store")
	fmt.Println("  nameport tls status                    Show CA and trust status")
	fmt.Println("  nameport tls ensure <domain>           Issue/return cert for domain")
	fmt.Println("  nameport tls list                      List issued certificates")
	fmt.Println("  nameport tls rotate                    Rotate intermediate CA")
	fmt.Println("  nameport tls export <format> <domain>  Export cert config (nginx|caddy|traefik)")
	fmt.Println("  nameport tls untrust                   Remove CA from OS trust store")
	fmt.Println()
	fmt.Println("System Commands:")
	fmt.Println("  nameport cleanup                       Remove all nameport data and trust entries")
	fmt.Println()
	fmt.Println("  nameport --config <path>               Use custom config path")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  nameport list")
	fmt.Println("  nameport rename myapp.localhost api.localhost")
	fmt.Println("  nameport tls init")
	fmt.Println("  nameport tls ensure myapp.localhost")
	fmt.Println("  nameport tls export nginx myapp.localhost")
	fmt.Println("  nameport cleanup")
}

func cmdList(store *storage.Store) {
	records := store.List()

	if len(records) == 0 {
		fmt.Println("No services registered.")
		fmt.Println("Start the daemon and run some local HTTP services.")
		return
	}

	// Backfill group for records that don't have one
	for _, r := range records {
		if r.Group == "" {
			r.Group = naming.ExtractGroup(r.Name)
		}
	}

	// Sort by group, then by name
	sort.Slice(records, func(i, j int) bool {
		if records[i].Group != records[j].Group {
			return records[i].Group < records[j].Group
		}
		return records[i].Name < records[j].Name
	})

	// Build group counts
	groupCounts := make(map[string]int)
	for _, r := range records {
		groupCounts[r.Group]++
	}

	fmt.Printf("%-30s %-22s %-8s %-6s %s\n", "NAME", "TARGET", "PID", "KEEP", "COMMAND")
	fmt.Println(strings.Repeat("-", 110))

	lastGroup := ""
	for _, r := range records {
		// Show group header for groups with 2+ members
		if r.Group != lastGroup && groupCounts[r.Group] > 1 {
			fmt.Printf("\n  [%s] (%d services)\n", r.Group, groupCounts[r.Group])
		}
		lastGroup = r.Group

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

		// Indent grouped services
		nameStr := r.Name
		if groupCounts[r.Group] > 1 {
			nameStr = "  " + r.Name
		}

		fmt.Printf("%-30s %-22s %-8d %-6s %s%s\n", nameStr, target, r.PID, keepStr, markers, cmd)
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

func cmdRemove(store *storage.Store, name string) {
	if !strings.HasSuffix(name, ".localhost") {
		name = name + ".localhost"
	}

	if _, ok := store.GetByName(name); !ok {
		log.Fatalf("Service not found: %s", name)
	}

	if err := store.RemoveByName(name); err != nil {
		log.Fatalf("Failed to remove service: %v", err)
	}

	fmt.Printf("Removed %s\n", name)
	fmt.Println("Note: You may need to restart the daemon for changes to take effect.")
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
			fmt.Fprintf(os.Stderr, "Usage: nameport rules import <file>\n")
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
		fmt.Fprintf(os.Stderr, "Usage: nameport rules <list|export|import> [file]\n")
		os.Exit(1)
	}
}

func cmdNotify(args []string) {
	configPath := notify.DefaultConfigPath()
	cfg, err := notify.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load notification config: %v", err)
	}

	subCmd := args[0]
	switch subCmd {
	case "status":
		status := "disabled"
		if cfg.Enabled {
			status = "enabled"
		}
		fmt.Printf("Notifications: %s\n", status)
		fmt.Printf("Config: %s\n", configPath)
		fmt.Println()
		fmt.Printf("%-25s %s\n", "EVENT", "STATUS")
		fmt.Println(strings.Repeat("-", 40))
		for _, e := range notify.AllEvents() {
			eventStatus := "on"
			if allowed, exists := cfg.EventFilter[e]; exists && !allowed {
				eventStatus = "off"
			}
			fmt.Printf("%-25s %s\n", e, eventStatus)
		}

	case "enable":
		cfg.Enabled = true
		if err := notify.SaveConfig(configPath, cfg); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Println("Notifications enabled.")
		fmt.Println("Note: Restart the daemon for changes to take effect.")

	case "disable":
		cfg.Enabled = false
		if err := notify.SaveConfig(configPath, cfg); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Println("Notifications disabled.")
		fmt.Println("Note: Restart the daemon for changes to take effect.")

	case "events":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport notify events <type> on|off\n")
			fmt.Fprintf(os.Stderr, "\nEvent types:\n")
			for _, e := range notify.AllEvents() {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
			os.Exit(1)
		}
		eventType := notify.EventType(args[1])
		toggle := args[2]

		// Validate event type
		valid := false
		for _, e := range notify.AllEvents() {
			if e == eventType {
				valid = true
				break
			}
		}
		if !valid {
			log.Fatalf("Unknown event type: %s", eventType)
		}

		switch toggle {
		case "on":
			cfg.EventFilter[eventType] = true
		case "off":
			cfg.EventFilter[eventType] = false
		default:
			log.Fatalf("Expected 'on' or 'off', got: %s", toggle)
		}

		if err := notify.SaveConfig(configPath, cfg); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Printf("Event %s set to %s.\n", eventType, toggle)
		fmt.Println("Note: Restart the daemon for changes to take effect.")

	default:
		fmt.Fprintf(os.Stderr, "Unknown notify command: %s\n", subCmd)
		fmt.Fprintf(os.Stderr, "Usage: nameport notify <status|enable|disable|events>\n")
		os.Exit(1)
	}
}

// caStorePath returns the expanded CA store directory.
func caStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", ".localtls")
	}
	return filepath.Join(home, ".localtls")
}

func cmdTLS(args []string) {
	subCmd := args[0]

	switch subCmd {
	case "init":
		cmdTLSInit()
	case "status":
		cmdTLSStatus()
	case "ensure":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: nameport tls ensure <domain>\n")
			os.Exit(1)
		}
		cmdTLSEnsure(args[1])
	case "list":
		cmdTLSList()
	case "rotate":
		cmdTLSRotate()
	case "export":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: nameport tls export <nginx|caddy|traefik> <domain>\n")
			os.Exit(1)
		}
		cmdTLSExport(args[1], args[2])
	case "untrust":
		cmdTLSUntrust()
	default:
		fmt.Fprintf(os.Stderr, "Unknown tls command: %s\n", subCmd)
		fmt.Fprintf(os.Stderr, "Usage: nameport tls <init|status|ensure|list|rotate|export|untrust>\n")
		os.Exit(1)
	}
}

func cmdTLSInit() {
	storePath := caStorePath()
	tlsCA, err := ca.NewCA(storePath)
	if err != nil {
		log.Fatalf("Failed to access CA store: %v", err)
	}

	if !tlsCA.IsInitialized() {
		fmt.Println("Bootstrapping new certificate authority...")
		if err := tlsCA.Init(); err != nil {
			log.Fatalf("Failed to initialize CA: %v", err)
		}
		fmt.Printf("CA created at %s\n", storePath)
	} else {
		fmt.Println("CA already initialized.")
	}

	// Install into trust store
	trustor := trust.NewPlatformTrustor()
	if trustor.IsInstalled(tlsCA.RootCertPEM()) {
		fmt.Println("Root CA is already trusted by the OS.")
		return
	}

	if trustor.NeedsElevation() {
		fmt.Println("Installing root CA into system trust store (may require sudo)...")
	} else {
		fmt.Println("Installing root CA into system trust store...")
	}

	if err := trustor.Install(tlsCA.RootCertPEM()); err != nil {
		log.Fatalf("Failed to install CA: %v\nYou may need to run this command with sudo.", err)
	}

	fmt.Println("Root CA installed and trusted.")
	fmt.Println("HTTPS is now available for all .localhost domains.")
}

func cmdTLSStatus() {
	storePath := caStorePath()
	tlsCA, err := ca.NewCA(storePath)
	if err != nil {
		log.Fatalf("Failed to access CA store: %v", err)
	}

	fmt.Printf("CA Store: %s\n", storePath)

	if !tlsCA.IsInitialized() {
		fmt.Println("Status: NOT INITIALIZED")
		fmt.Println("  Run 'nameport tls init' to bootstrap the CA.")
		return
	}

	fmt.Println("Status: INITIALIZED")
	fmt.Printf("  Root CA:         %s\n", tlsCA.RootCert.Subject.CommonName)
	fmt.Printf("  Root expires:    %s\n", tlsCA.RootCert.NotAfter.Format("2006-01-02"))
	fmt.Printf("  Intermediate:    %s\n", tlsCA.InterCert.Subject.CommonName)
	fmt.Printf("  Inter expires:   %s\n", tlsCA.InterCert.NotAfter.Format("2006-01-02"))

	// Check if intermediate needs rotation
	if time.Until(tlsCA.InterCert.NotAfter) < 30*24*time.Hour {
		fmt.Println("  WARNING: Intermediate CA expires within 30 days. Run 'nameport tls rotate'.")
	}

	// Check trust status
	trustor := trust.NewPlatformTrustor()
	if trustor.IsInstalled(tlsCA.RootCertPEM()) {
		fmt.Println("  OS Trust:        INSTALLED")
	} else {
		fmt.Println("  OS Trust:        NOT INSTALLED")
		fmt.Println("    Run 'sudo nameport tls init' to install into system trust store.")
	}

	// List issued certs
	certsDir := filepath.Join(storePath, "certs")
	entries, err := os.ReadDir(certsDir)
	if err == nil {
		certCount := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".pem") {
				certCount++
			}
		}
		fmt.Printf("  Issued certs:    %d\n", certCount)
	}
}

func cmdTLSEnsure(domain string) {
	// Ensure .localhost suffix for bare names
	if !strings.Contains(domain, ".") {
		domain = domain + ".localhost"
	}

	storePath := caStorePath()
	tlsCA, err := ca.NewCA(storePath)
	if err != nil {
		log.Fatalf("Failed to access CA store: %v", err)
	}

	if !tlsCA.IsInitialized() {
		log.Fatalf("CA not initialized. Run 'nameport tls init' first.")
	}

	pol := policy.NewPolicy()
	iss := issuer.NewIssuer(tlsCA, pol)

	// Build DNS names: for wildcards, also include the base domain
	dnsNames := []string{domain}
	if strings.HasPrefix(domain, "*.") {
		base := domain[2:]
		dnsNames = append(dnsNames, base)
	}

	cached, err := iss.Issue(issuer.IssueRequest{
		DNSNames: dnsNames,
	})
	if err != nil {
		log.Fatalf("Failed to issue certificate: %v", err)
	}

	// Save cert and key to disk
	certsDir := filepath.Join(storePath, "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		log.Fatalf("Failed to create certs directory: %v", err)
	}

	// Use sanitized filename
	safeName := strings.ReplaceAll(strings.ReplaceAll(domain, "*", "_wildcard"), "/", "_")
	certPath := filepath.Join(certsDir, safeName+".pem")
	keyPath := filepath.Join(certsDir, safeName+".key")

	if err := os.WriteFile(certPath, cached.CertPEM, 0644); err != nil {
		log.Fatalf("Failed to write certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, cached.KeyPEM, 0600); err != nil {
		log.Fatalf("Failed to write key: %v", err)
	}

	fmt.Printf("Certificate issued for: %s\n", strings.Join(dnsNames, ", "))
	fmt.Printf("  Cert: %s\n", certPath)
	fmt.Printf("  Key:  %s\n", keyPath)
	fmt.Printf("  Expires: %s\n", cached.Expiry.Format("2006-01-02 15:04:05"))
}

func cmdTLSList() {
	storePath := caStorePath()
	certsDir := filepath.Join(storePath, "certs")

	entries, err := os.ReadDir(certsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No certificates issued yet.")
			return
		}
		log.Fatalf("Failed to read certs directory: %v", err)
	}

	certFiles := []string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pem") {
			certFiles = append(certFiles, e.Name())
		}
	}

	if len(certFiles) == 0 {
		fmt.Println("No certificates issued yet.")
		return
	}

	fmt.Printf("%-40s %s\n", "DOMAIN", "CERT FILE")
	fmt.Println(strings.Repeat("-", 70))

	for _, f := range certFiles {
		domain := strings.TrimSuffix(f, ".pem")
		domain = strings.ReplaceAll(domain, "_wildcard", "*")
		fmt.Printf("%-40s %s\n", domain, filepath.Join(certsDir, f))
	}
}

func cmdTLSRotate() {
	storePath := caStorePath()
	tlsCA, err := ca.NewCA(storePath)
	if err != nil {
		log.Fatalf("Failed to access CA store: %v", err)
	}

	if !tlsCA.IsInitialized() {
		log.Fatalf("CA not initialized. Run 'nameport tls init' first.")
	}

	fmt.Println("Rotating intermediate CA...")
	if err := tlsCA.RotateIntermediate(); err != nil {
		log.Fatalf("Failed to rotate intermediate: %v", err)
	}

	fmt.Println("Intermediate CA rotated successfully.")
	fmt.Printf("  New expiry: %s\n", tlsCA.InterCert.NotAfter.Format("2006-01-02"))
	fmt.Println("Note: Existing leaf certificates remain valid until they expire.")
}

func cmdTLSExport(format, domain string) {
	// Ensure .localhost suffix for bare names
	if !strings.Contains(domain, ".") {
		domain = domain + ".localhost"
	}

	storePath := caStorePath()
	certsDir := filepath.Join(storePath, "certs")
	safeName := strings.ReplaceAll(strings.ReplaceAll(domain, "*", "_wildcard"), "/", "_")
	certPath := filepath.Join(certsDir, safeName+".pem")
	keyPath := filepath.Join(certsDir, safeName+".key")

	// Check if cert exists, issue if not
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		fmt.Printf("No certificate found for %s. Issuing one...\n", domain)
		cmdTLSEnsure(domain)
	}

	switch strings.ToLower(format) {
	case "nginx":
		fmt.Printf("# nginx SSL configuration for %s\n", domain)
		fmt.Printf("server {\n")
		fmt.Printf("    listen 443 ssl;\n")
		fmt.Printf("    server_name %s;\n\n", domain)
		fmt.Printf("    ssl_certificate     %s;\n", certPath)
		fmt.Printf("    ssl_certificate_key %s;\n", keyPath)
		fmt.Printf("    ssl_protocols       TLSv1.2 TLSv1.3;\n")
		fmt.Printf("}\n")

	case "caddy":
		fmt.Printf("# Caddy configuration for %s\n", domain)
		fmt.Printf("%s {\n", domain)
		fmt.Printf("    tls %s %s\n", certPath, keyPath)
		fmt.Printf("    reverse_proxy localhost:PORT\n")
		fmt.Printf("}\n")

	case "traefik":
		fmt.Printf("# Traefik dynamic configuration for %s\n", domain)
		fmt.Printf("tls:\n")
		fmt.Printf("  certificates:\n")
		fmt.Printf("    - certFile: %s\n", certPath)
		fmt.Printf("      keyFile: %s\n", keyPath)

	default:
		fmt.Fprintf(os.Stderr, "Unknown export format: %s\n", format)
		fmt.Fprintf(os.Stderr, "Supported formats: nginx, caddy, traefik\n")
		os.Exit(1)
	}
}

func cmdTLSUntrust() {
	storePath := caStorePath()
	tlsCA, err := ca.NewCA(storePath)
	if err != nil {
		log.Fatalf("Failed to access CA store: %v", err)
	}

	if !tlsCA.IsInitialized() {
		fmt.Println("CA not initialized. Nothing to untrust.")
		return
	}

	trustor := trust.NewPlatformTrustor()
	if !trustor.IsInstalled(tlsCA.RootCertPEM()) {
		fmt.Println("Root CA is not in the system trust store.")
		return
	}

	fmt.Println("Removing root CA from system trust store...")
	if err := trustor.Uninstall(); err != nil {
		log.Fatalf("Failed to remove CA: %v\nYou may need to run this command with sudo.", err)
	}

	fmt.Println("Root CA removed from system trust store.")
}

func cmdCleanup() {
	fmt.Println("nameport cleanup")
	fmt.Println("This will remove:")
	fmt.Println("  - Root CA from system trust store")
	fmt.Println("  - All CA material and issued certificates")
	fmt.Println("  - Service records and configuration")
	fmt.Println()

	// Remove CA from trust store
	storePath := caStorePath()
	tlsCA, err := ca.NewCA(storePath)
	if err == nil && tlsCA.IsInitialized() {
		trustor := trust.NewPlatformTrustor()
		if trustor.IsInstalled(tlsCA.RootCertPEM()) {
			fmt.Println("Removing root CA from system trust store...")
			if err := trustor.Uninstall(); err != nil {
				fmt.Printf("Warning: failed to remove CA from trust store: %v\n", err)
				fmt.Println("  You may need to run 'sudo nameport tls untrust' separately.")
			} else {
				fmt.Println("  Root CA removed from trust store.")
			}
		}
	}

	// Remove CA store
	if _, err := os.Stat(storePath); err == nil {
		fmt.Printf("Removing CA store: %s\n", storePath)
		if err := os.RemoveAll(storePath); err != nil {
			fmt.Printf("Warning: failed to remove CA store: %v\n", err)
		} else {
			fmt.Println("  CA store removed.")
		}
	}

	// Remove service config
	configDir := filepath.Dir(storage.DefaultStorePath())
	if _, err := os.Stat(configDir); err == nil {
		fmt.Printf("Removing configuration: %s\n", configDir)
		if err := os.RemoveAll(configDir); err != nil {
			fmt.Printf("Warning: failed to remove config: %v\n", err)
		} else {
			fmt.Println("  Configuration removed.")
		}
	}

	fmt.Println()
	fmt.Println("Cleanup complete. nameport data has been removed.")
	fmt.Println("Note: If the daemon is installed as a system service, run:")
	fmt.Println("  sudo nameport uninstall")
}
