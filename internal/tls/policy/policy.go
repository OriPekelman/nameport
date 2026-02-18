// Package policy enforces domain restrictions for the local TLS CA,
// preventing certificate issuance for real (publicly-routable) domains.
package policy

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed tlds.txt
var tldData string

// Policy holds the allow/block lists used to validate certificate requests.
type Policy struct {
	allowedTLDs map[string]bool
	blockedTLDs map[string]bool
}

// NewPolicy returns a Policy initialised with the hardcoded allowed TLDs and
// the IANA-sourced blocked TLDs embedded in the binary.
func NewPolicy() *Policy {
	p := &Policy{
		allowedTLDs: map[string]bool{
			".localhost":  true,
			".test":       true,
			".localdev":   true,
			".internal":   true,
			".home.arpa":  true,
		},
		blockedTLDs: make(map[string]bool),
	}

	for _, line := range strings.Split(tldData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Store as lowercase with leading dot, e.g. ".com"
		p.blockedTLDs["."+strings.ToLower(line)] = true
	}

	return p
}

// IsAllowedTLD reports whether the given TLD (with leading dot, e.g. ".localhost")
// is in the set of allowed local TLDs.
func (p *Policy) IsAllowedTLD(tld string) bool {
	return p.allowedTLDs[strings.ToLower(tld)]
}

// ValidateDomain checks that domain is safe for the local CA to issue a
// certificate for. It must end with an allowed TLD and must not end with a
// blocked (public) TLD.
func (p *Policy) ValidateDomain(domain string) error {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if domain == "" {
		return fmt.Errorf("empty domain")
	}

	// Check against allowed TLDs (longest match first for .home.arpa).
	if p.matchesAllowed(domain) {
		return nil
	}

	// Check if it ends with a blocked public TLD.
	if p.matchesBlocked(domain) {
		return fmt.Errorf("domain %q ends with a public TLD; local CA must not issue certificates for real domains", domain)
	}

	return fmt.Errorf("domain %q does not end with an allowed TLD (.localhost, .test, .localdev, .internal, .home.arpa)", domain)
}

// ValidateWildcard checks that a wildcard pattern is safe for the local CA.
// Rules:
//   - The wildcard (*) must appear only as the left-most label.
//   - The pattern must have depth >= 2 below the TLD
//     (e.g. *.myapp.localhost is OK, *.localhost is NOT).
func (p *Policy) ValidateWildcard(pattern string) error {
	pattern = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(pattern), "."))
	if pattern == "" {
		return fmt.Errorf("empty wildcard pattern")
	}

	// Must start with "*."
	if !strings.HasPrefix(pattern, "*.") {
		return fmt.Errorf("wildcard %q: wildcard must be the left-most label (e.g. *.myapp.localhost)", pattern)
	}

	// No additional wildcards allowed.
	rest := pattern[2:]
	if strings.Contains(rest, "*") {
		return fmt.Errorf("wildcard %q: only a single left-most wildcard is allowed", pattern)
	}

	// The base (everything after *.) must itself be a valid domain.
	if err := p.ValidateDomain(rest); err != nil {
		return fmt.Errorf("wildcard %q: %w", pattern, err)
	}

	// Depth check: rest must have at least 2 labels (e.g. "myapp.localhost").
	// For .home.arpa the TLD is two labels, so we need at least 3 labels in rest.
	labels := strings.Split(rest, ".")
	if p.endsWithMultiLabelTLD(rest) {
		// e.g. rest = "myapp.home.arpa" → labels = [myapp, home, arpa] → need >= 3
		if len(labels) < 3 {
			return fmt.Errorf("wildcard %q: wildcard requires at least one label before the TLD (e.g. *.myapp.home.arpa)", pattern)
		}
	} else {
		// e.g. rest = "myapp.localhost" → labels = [myapp, localhost] → need >= 2
		if len(labels) < 2 {
			return fmt.Errorf("wildcard %q: wildcard requires at least one label before the TLD (e.g. *.myapp.localhost)", pattern)
		}
	}

	return nil
}

// matchesAllowed reports whether domain ends with one of the allowed TLDs.
func (p *Policy) matchesAllowed(domain string) bool {
	for tld := range p.allowedTLDs {
		if domain == tld[1:] || strings.HasSuffix(domain, tld) {
			return true
		}
	}
	return false
}

// matchesBlocked reports whether domain ends with one of the blocked TLDs.
func (p *Policy) matchesBlocked(domain string) bool {
	for tld := range p.blockedTLDs {
		if domain == tld[1:] || strings.HasSuffix(domain, tld) {
			return true
		}
	}
	return false
}

// endsWithMultiLabelTLD checks if the domain ends with a multi-label allowed
// TLD such as .home.arpa.
func (p *Policy) endsWithMultiLabelTLD(domain string) bool {
	// Currently only .home.arpa is multi-label.
	return strings.HasSuffix(domain, ".home.arpa") || domain == "home.arpa"
}
