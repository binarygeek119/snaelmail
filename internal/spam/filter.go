package spam

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"snailmail/internal/store"
)

// Score represents a spam score result
type Score struct {
	Total    float64
	DNSBL    float64
	SPF      float64
	Content  float64
	Reasons  []string
}

// Filter handles spam filtering
type Filter struct {
	store *store.Store
}

// NewFilter creates a new spam filter
func NewFilter(store *store.Store) *Filter {
	return &Filter{store: store}
}

// CheckDNSBL checks if an IP is in DNS blacklists
func (f *Filter) CheckDNSBL(ip string) (bool, float64, []string) {
	var reasons []string
	score := 0.0

	dnsbls := []string{
		"zen.spamhaus.org",
		"bl.spamcop.net",
		"b.barracudacentral.org",
	}

	reversedIP := reverseIP(ip)

	for _, dnsbl := range dnsbls {
		query := reversedIP + "." + dnsbl
		_, err := net.LookupHost(query)
		if err == nil {
			// IP is listed
			score += 5.0
			reasons = append(reasons, "Listed in "+dnsbl)
		}
	}

	return len(reasons) > 0, score, reasons
}

// CheckContent performs basic content analysis
func (f *Filter) CheckContent(subject, body string) (float64, []string) {
	score := 0.0
	var reasons []string

	text := strings.ToLower(subject + " " + body)

	// Spam keywords
	spamKeywords := []string{
		"viagra", "cialis", "lottery", "winner", "prince",
		"inheritance", "million dollars", "wire transfer",
		"click here", "unsubscribe", "free money",
		"act now", "limited time", "congratulations",
	}

	for _, keyword := range spamKeywords {
		if strings.Contains(text, keyword) {
			score += 2.0
			reasons = append(reasons, "Spam keyword: "+keyword)
		}
	}

	// Check for excessive caps
	capsCount := 0
	for _, c := range subject {
		if c >= 'A' && c <= 'Z' {
			capsCount++
		}
	}
	if len(subject) > 0 && float64(capsCount)/float64(len(subject)) > 0.5 {
		score += 1.5
		reasons = append(reasons, "Excessive capitalization in subject")
	}

	// Check for too many links
	linkCount := strings.Count(body, "http://") + strings.Count(body, "https://")
	if linkCount > 5 {
		score += 2.0
		reasons = append(reasons, "Too many links in body")
	}

	return score, reasons
}

// ScoreEmail calculates a spam score for an email
func (f *Filter) ScoreEmail(ctx context.Context, fromIP, fromAddr, subject, body string) Score {
	score := Score{Reasons: []string{}}

	// DNSBL check
	if fromIP != "" {
		listed, dnsblScore, reasons := f.CheckDNSBL(fromIP)
		if listed {
			score.DNSBL = dnsblScore
			score.Reasons = append(score.Reasons, reasons...)
		}
	}

	// Content analysis
	contentScore, reasons := f.CheckContent(subject, body)
	score.Content = contentScore
	score.Reasons = append(score.Reasons, reasons...)

	score.Total = score.DNSBL + score.SPF + score.Content

	return score
}

// IsSpam determines if an email is spam based on score
func (f *Filter) IsSpam(score Score) bool {
	return score.Total >= 5.0
}

// reverseIP reverses an IP address for DNSBL lookup
func reverseIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	return parts[3] + "." + parts[2] + "." + parts[1] + "." + parts[0]
}

// LogScore logs the spam score for debugging
func (f *Filter) LogScore(from, subject string, score Score) {
	if score.Total > 0 {
		log.Printf("Spam score for %s (%s): %.1f - %s", from, subject, score.Total, strings.Join(score.Reasons, ", "))
	}
}

var _ = fmt.Sprintf // keep import
var _ = context.Background
