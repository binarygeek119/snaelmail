package smtp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"snailmail/internal/store"
)

// Server represents an SMTP server
type Server struct {
	store    *store.Store
	listener net.Listener
	done     chan struct{}
}

// NewServer creates a new SMTP server
func NewServer(store *store.Store, port string) (*Server, error) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	return &Server{
		store:    store,
		listener: listener,
		done:     make(chan struct{}),
	}, nil
}

// Start starts the SMTP server
func (s *Server) Start() error {
	log.Printf("SMTP server starting on %s", s.listener.Addr())

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				log.Printf("Error accepting connection: %v", err)
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// Stop stops the SMTP server
func (s *Server) Stop() error {
	close(s.done)
	return s.listener.Close()
}

// handleConnection handles a single SMTP client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Send greeting
	writer.WriteString("220 snaelmail.org ESMTP Snaelmail Mail Server\r\n")
	writer.Flush()

	var from string
	var to []string
	var data strings.Builder
	inData := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from client: %v", err)
			}
			return
		}

		line = strings.TrimSpace(line)

		if inData {
			if line == "." {
				// End of data
				writer.WriteString("250 OK: Message accepted\r\n")
				writer.Flush()

				// Process the message
				if err := s.processMessage(from, to, data.String()); err != nil {
					log.Printf("Error processing message: %v", err)
				}

				// Reset for next message
				from = ""
				to = []string{}
				data.Reset()
				inData = false
			} else {
				data.WriteString(line + "\r\n")
			}
			continue
		}

		// Parse SMTP commands
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "HELO", "EHLO":
			writer.WriteString("250 Hello\r\n")
			writer.Flush()

		case "MAIL":
			// MAIL FROM:<sender@example.com>
			if len(parts) < 2 {
				writer.WriteString("500 Syntax error\r\n")
				writer.Flush()
				continue
			}

			// Extract email address
			from = extractEmail(parts[1])
			if from == "" {
				writer.WriteString("500 Invalid sender address\r\n")
				writer.Flush()
				continue
			}

			writer.WriteString("250 OK\r\n")
			writer.Flush()

		case "RCPT":
			// RCPT TO:<recipient@example.com>
			if len(parts) < 2 {
				writer.WriteString("500 Syntax error\r\n")
				writer.Flush()
				continue
			}

			recipient := extractEmail(parts[1])
			if recipient == "" {
				writer.WriteString("500 Invalid recipient address\r\n")
				writer.Flush()
				continue
			}

			// Validate recipient exists
			ctx := context.Background()
			user, err := s.store.GetUserByAddress(ctx, recipient)
			if err != nil || user == nil {
				writer.WriteString("550 User unknown\r\n")
				writer.Flush()
				continue
			}

			to = append(to, recipient)
			writer.WriteString("250 OK\r\n")
			writer.Flush()

		case "DATA":
			if from == "" || len(to) == 0 {
				writer.WriteString("503 Need MAIL and RCPT first\r\n")
				writer.Flush()
				continue
			}

			writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
			writer.Flush()
			inData = true

		case "QUIT":
			writer.WriteString("221 Bye\r\n")
			writer.Flush()
			return

		case "RSET":
			from = ""
			to = []string{}
			data.Reset()
			inData = false
			writer.WriteString("250 OK\r\n")
			writer.Flush()

		case "NOOP":
			writer.WriteString("250 OK\r\n")
			writer.Flush()

		default:
			writer.WriteString("500 Command not recognized\r\n")
			writer.Flush()
		}
	}
}

// extractEmail extracts the email address from SMTP command arguments
func extractEmail(arg string) string {
	// Handle MAIL FROM:<email> or RCPT TO:<email>
	arg = strings.TrimPrefix(arg, "FROM:")
	arg = strings.TrimPrefix(arg, "TO:")
	arg = strings.TrimPrefix(arg, "from:")
	arg = strings.TrimPrefix(arg, "to:")

	// Remove angle brackets
	arg = strings.Trim(arg, "<>")
	
	// Validate email format
	if !strings.Contains(arg, "@") {
		return ""
	}

	return strings.TrimSpace(arg)
}

// processMessage processes an incoming email message and stores it
func (s *Server) processMessage(from string, to []string, data string) error {
	subject := extractSubject(data)
	body := extractBody(data)

	for _, recipient := range to {
		ctx := context.Background()

		user, err := s.store.GetUserByAddress(ctx, recipient)
		if err != nil || user == nil {
			log.Printf("Recipient not found: %s", recipient)
			continue
		}

		// Store the received email
		_, err = s.store.Pool().Exec(ctx, `
			INSERT INTO received_emails (user_id, from_address, to_address, subject, body_text, received_at)
			VALUES ($1, $2, $3, $4, $5, now())
		`, user.ID, from, recipient, subject, body)
		if err != nil {
			log.Printf("Failed to store email for %s: %v", recipient, err)
			continue
		}

		log.Printf("Stored email for %s from %s: %s", recipient, from, subject)
	}

	return nil
}

// extractSubject extracts the subject from email data
func extractSubject(data string) string {
	lines := strings.Split(data, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "subject:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		}
	}
	return "(No Subject)"
}

// extractBody extracts the body from email data
func extractBody(data string) string {
	// Simple body extraction - find the empty line that separates headers from body
	parts := strings.Split(data, "\r\n\r\n")
	if len(parts) > 1 {
		return parts[1]
	}
	return data
}
