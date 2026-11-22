package outbound

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// DeliveryResult represents the result of an email delivery attempt
type DeliveryResult struct {
	Success bool
	Error   error
	Host    string
	Code    int
	Message string
}

// DeliverEmail sends an email directly to the recipient's MX server
func DeliverEmail(ctx context.Context, from, to, subject, body string, dkimSignature string) DeliveryResult {
	// Extract domain from recipient address
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return DeliveryResult{Success: false, Error: fmt.Errorf("invalid recipient address")}
	}
	domain := parts[1]

	// Look up MX records
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("MX lookup failed: %w", err)}
	}

	if len(mxRecords) == 0 {
		return DeliveryResult{Success: false, Error: fmt.Errorf("no MX records found for %s", domain)}
	}

	// Sort by priority (lower = higher priority)
	// net.LookupMX already returns sorted by priority

	// Try each MX server in order
	for _, mx := range mxRecords {
		host := strings.TrimSuffix(mx.Host, ".")
		result := tryDeliver(ctx, host, from, to, subject, body, dkimSignature)
		if result.Success {
			return result
		}
		log.Printf("Delivery to %s failed: %v, trying next MX", host, result.Error)
	}

	return DeliveryResult{Success: false, Error: fmt.Errorf("all MX servers failed")}
}

// tryDeliver attempts delivery to a single MX server
func tryDeliver(ctx context.Context, host, from, to, subject, body, dkimSignature string) DeliveryResult {
	// Resolve MX hostname to IPv4 address
	ips, err := net.LookupIP(host)
	if err != nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("DNS lookup failed: %w", err), Host: host}
	}

	var ipv4 net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4 = ip.To4()
			break
		}
	}

	if ipv4 == nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("no IPv4 address for %s", host), Host: host}
	}

	// Connect to MX server on port 25 (IPv4 only)
	conn, err := net.DialTimeout("tcp4", ipv4.String()+":25", 30*time.Second)
	if err != nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("connection failed: %w", err), Host: host}
	}
	defer conn.Close()

	// Set deadline
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, err := reader.ReadString('\n')
	if err != nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("read greeting: %w", err), Host: host}
	}
	if !strings.HasPrefix(greeting, "220") {
		return DeliveryResult{Success: false, Error: fmt.Errorf("bad greeting: %s", greeting), Host: host}
	}

	// EHLO
	hostname := "mail.snaelmail.org"
	fmt.Fprintf(writer, "EHLO %s\r\n", hostname)
	writer.Flush()

	// Read EHLO response
	ehloResp, err := readSMTPResponse(reader)
	if err != nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("EHLO failed: %w", err), Host: host}
	}

	// Check if STARTTLS is supported
	supportsTLS := strings.Contains(strings.ToLower(ehloResp), "starttls")

	// Try STARTTLS if supported
	if supportsTLS {
		fmt.Fprintf(writer, "STARTTLS\r\n")
		writer.Flush()

		resp, err := reader.ReadString('\n')
		if err != nil || !strings.HasPrefix(resp, "220") {
			// TLS not available, continue without
			log.Printf("STARTTLS not available on %s, continuing plaintext", host)
		} else {
			// Upgrade to TLS
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName: host,
				MinVersion: tls.VersionTLS12,
			})
			if err := tlsConn.Handshake(); err != nil {
				log.Printf("TLS handshake failed on %s: %v, continuing plaintext", host, err)
			} else {
				conn = tlsConn
				reader = bufio.NewReader(conn)
				writer = bufio.NewWriter(conn)
				// Re-EHLO after TLS
				fmt.Fprintf(writer, "EHLO %s\r\n", hostname)
				writer.Flush()
				ehloResp, _ = readSMTPResponse(reader)
			}
		}
	}

	// MAIL FROM
	fmt.Fprintf(writer, "MAIL FROM:<%s>\r\n", from)
	writer.Flush()
	resp, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(resp, "250") {
		return DeliveryResult{Success: false, Error: fmt.Errorf("MAIL FROM rejected: %s", resp), Host: host}
	}

	// RCPT TO
	fmt.Fprintf(writer, "RCPT TO:<%s>\r\n", to)
	writer.Flush()
	resp, err = reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(resp, "250") {
		return DeliveryResult{Success: false, Error: fmt.Errorf("RCPT TO rejected: %s", resp), Host: host, Code: parseSMTPCode(resp)}
	}

	// DATA
	fmt.Fprintf(writer, "DATA\r\n")
	writer.Flush()
	resp, err = reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(resp, "354") {
		return DeliveryResult{Success: false, Error: fmt.Errorf("DATA rejected: %s", resp), Host: host}
	}

	// Build message
	message := buildMessage(from, to, subject, body, dkimSignature)

	// Send message data
	fmt.Fprint(writer, message)
	fmt.Fprintf(writer, "\r\n.\r\n")
	writer.Flush()

	resp, err = reader.ReadString('\n')
	if err != nil {
		return DeliveryResult{Success: false, Error: fmt.Errorf("read response: %w", err), Host: host}
	}

	if strings.HasPrefix(resp, "250") {
		// QUIT
		fmt.Fprintf(writer, "QUIT\r\n")
		writer.Flush()
		return DeliveryResult{Success: true, Host: host, Message: resp}
	}

	return DeliveryResult{Success: false, Error: fmt.Errorf("message rejected: %s", resp), Host: host, Code: parseSMTPCode(resp)}
}

// readSMTPResponse reads a multi-line SMTP response
func readSMTPResponse(reader *bufio.Reader) (string, error) {
	var response strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return response.String(), err
		}
		response.WriteString(line)
		// Check if this is the last line (format: "250 " not "250-")
		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}
	return response.String(), nil
}

// buildMessage constructs the full email message with headers
func buildMessage(from, to, subject, body, dkimSignature string) string {
	var msg strings.Builder

	// DKIM signature (if available)
	if dkimSignature != "" {
		msg.WriteString(dkimSignature + "\r\n")
	}

	contentType := "text/plain; charset=UTF-8"
	lowerBody := strings.ToLower(body)
	if strings.Contains(lowerBody, "<!doctype html") || strings.Contains(lowerBody, "<html") {
		contentType = "text/html; charset=UTF-8"
	}

	// Standard headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}

// parseSMTPCode extracts the SMTP response code
func parseSMTPCode(resp string) int {
	if len(resp) >= 3 {
		code := 0
		for i := 0; i < 3; i++ {
			if resp[i] >= '0' && resp[i] <= '9' {
				code = code*10 + int(resp[i]-'0')
			}
		}
		return code
	}
	return 0
}
