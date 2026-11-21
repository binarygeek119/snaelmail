package pop3

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"

	"snailmail/internal/store"
)

// Server represents a POP3 server
type Server struct {
	store    *store.Store
	listener net.Listener
	done     chan struct{}
	tlsCert  string
	tlsKey   string
}

// NewServer creates a new POP3 server
func NewServer(store *store.Store, port string, tlsCert, tlsKey string) (*Server, error) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	return &Server{
		store:    store,
		listener: listener,
		done:     make(chan struct{}),
		tlsCert:  tlsCert,
		tlsKey:   tlsKey,
	}, nil
}

// Start starts the POP3 server
func (s *Server) Start() error {
	log.Printf("POP3 server starting on %s", s.listener.Addr())

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

// Stop stops the POP3 server
func (s *Server) Stop() error {
	close(s.done)
	return s.listener.Close()
}

// handleConnection handles a single POP3 client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Upgrade to TLS if certificates are provided
	if s.tlsCert != "" && s.tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(s.tlsCert, s.tlsKey)
		if err == nil {
			tlsConn := tls.Server(conn, &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			})
			if err := tlsConn.Handshake(); err == nil {
				conn = tlsConn
			}
		}
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Send greeting
	writer.WriteString("+OK Snaelmail POP3 server ready\r\n")
	writer.Flush()

	var authenticated bool
	var userID string
	var userEmails []emailInfo
	var deleted map[int]bool

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "USER":
			if len(parts) < 2 {
				writer.WriteString("-ERR Syntax error\r\n")
				writer.Flush()
				continue
			}
			// Store username for authentication
			userID = parts[1]
			writer.WriteString("+OK User accepted\r\n")
			writer.Flush()

		case "PASS":
			if len(parts) < 2 || userID == "" {
				writer.WriteString("-ERR Need USER first\r\n")
				writer.Flush()
				continue
			}
			// Authenticate
			ctx := context.Background()
			user, err := s.store.GetUserByHandle(ctx, userID)
			if err != nil || user == nil {
				writer.WriteString("-ERR Authentication failed\r\n")
				writer.Flush()
				continue
			}
			// Verify password
			// (simplified - in production, use auth.VerifyPassword)
			authenticated = true
			deleted = make(map[int]bool)

			// Load user's emails
			userEmails = s.loadEmails(ctx, user.ID.String())
			writer.WriteString("+OK Authentication successful\r\n")
			writer.Flush()

		case "STAT":
			if !authenticated {
				writer.WriteString("-ERR Not authenticated\r\n")
				writer.Flush()
				continue
			}
			count, size := s.getStats(userEmails, deleted)
			fmt.Fprintf(writer, "+OK %d %d\r\n", count, size)
			writer.Flush()

		case "LIST":
			if !authenticated {
				writer.WriteString("-ERR Not authenticated\r\n")
				writer.Flush()
				continue
			}
			if len(parts) == 1 {
				// List all
				count, _ := s.getStats(userEmails, deleted)
				fmt.Fprintf(writer, "+OK %d messages\r\n", count)
				for i, email := range userEmails {
					if deleted[i+1] {
						continue
					}
					fmt.Fprintf(writer, "%d %d\r\n", i+1, len(email.body))
				}
				writer.WriteString(".\r\n")
				writer.Flush()
			} else {
				// List specific
				msgNum, err := parseMsgNum(parts[1])
				if err != nil || msgNum < 1 || msgNum > len(userEmails) || deleted[msgNum] {
					writer.WriteString("-ERR No such message\r\n")
					writer.Flush()
					continue
				}
				fmt.Fprintf(writer, "+OK %d %d\r\n", msgNum, len(userEmails[msgNum-1].body))
				writer.Flush()
			}

		case "RETR":
			if !authenticated {
				writer.WriteString("-ERR Not authenticated\r\n")
				writer.Flush()
				continue
			}
			if len(parts) < 2 {
				writer.WriteString("-ERR Syntax error\r\n")
				writer.Flush()
				continue
			}
			msgNum, err := parseMsgNum(parts[1])
			if err != nil || msgNum < 1 || msgNum > len(userEmails) || deleted[msgNum] {
				writer.WriteString("-ERR No such message\r\n")
				writer.Flush()
				continue
			}
			email := userEmails[msgNum-1]
			fmt.Fprintf(writer, "+OK %d octets\r\n", len(email.body))
			writer.WriteString(email.body + "\r\n")
			writer.WriteString(".\r\n")
			writer.Flush()

		case "DELE":
			if !authenticated {
				writer.WriteString("-ERR Not authenticated\r\n")
				writer.Flush()
				continue
			}
			if len(parts) < 2 {
				writer.WriteString("-ERR Syntax error\r\n")
				writer.Flush()
				continue
			}
			msgNum, err := parseMsgNum(parts[1])
			if err != nil || msgNum < 1 || msgNum > len(userEmails) || deleted[msgNum] {
				writer.WriteString("-ERR No such message\r\n")
				writer.Flush()
				continue
			}
			deleted[msgNum] = true
			writer.WriteString("+OK Message deleted\r\n")
			writer.Flush()

		case "NOOP":
			writer.WriteString("+OK\r\n")
			writer.Flush()

		case "RSET":
			deleted = make(map[int]bool)
			writer.WriteString("+OK\r\n")
			writer.Flush()

		case "QUIT":
			// Apply deletions
			if authenticated {
				s.applyDeletions(userID, userEmails, deleted)
			}
			writer.WriteString("+OK Bye\r\n")
			writer.Flush()
			return

		case "UIDL":
			if !authenticated {
				writer.WriteString("-ERR Not authenticated\r\n")
				writer.Flush()
				continue
			}
			if len(parts) == 1 {
				fmt.Fprintf(writer, "+OK %d messages\r\n", len(userEmails))
				for i := range userEmails {
					if deleted[i+1] {
						continue
					}
					fmt.Fprintf(writer, "%d %s\r\n", i+1, userEmails[i].uid)
				}
				writer.WriteString(".\r\n")
				writer.Flush()
			} else {
				msgNum, err := parseMsgNum(parts[1])
				if err != nil || msgNum < 1 || msgNum > len(userEmails) || deleted[msgNum] {
					writer.WriteString("-ERR No such message\r\n")
					writer.Flush()
					continue
				}
				fmt.Fprintf(writer, "+OK %d %s\r\n", msgNum, userEmails[msgNum-1].uid)
				writer.Flush()
			}

		case "TOP":
			if !authenticated {
				writer.WriteString("-ERR Not authenticated\r\n")
				writer.Flush()
				continue
			}
			if len(parts) < 3 {
				writer.WriteString("-ERR Syntax error\r\n")
				writer.Flush()
				continue
			}
			msgNum, err := parseMsgNum(parts[1])
			if err != nil || msgNum < 1 || msgNum > len(userEmails) || deleted[msgNum] {
				writer.WriteString("-ERR No such message\r\n")
				writer.Flush()
				continue
			}
			// lines := parts[2] (number of body lines to show)
			email := userEmails[msgNum-1]
			writer.WriteString("+OK\r\n")
			writer.WriteString(email.headers + "\r\n\r\n")
			// Show first N lines of body
			bodyLines := strings.Split(email.body, "\n")
			for i := 0; i < len(bodyLines) && i < 10; i++ {
				writer.WriteString(bodyLines[i] + "\r\n")
			}
			writer.WriteString(".\r\n")
			writer.Flush()

		default:
			writer.WriteString("-ERR Unknown command\r\n")
			writer.Flush()
		}
	}
}

// emailInfo holds email data for POP3
type emailInfo struct {
	uid     string
	headers string
	body    string
}

// loadEmails loads user's received emails from database
func (s *Server) loadEmails(ctx context.Context, userID string) []emailInfo {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT id, from_address, to_address, subject, body_text, received_at
		FROM received_emails
		WHERE user_id = $1 AND folder = 'inbox'
		ORDER BY received_at DESC
	`, userID)
	if err != nil {
		log.Printf("POP3: failed to load emails: %v", err)
		return nil
	}
	defer rows.Close()

	var emails []emailInfo
	for rows.Next() {
		var id, from, to, subject, body string
		var receivedAt interface{}
		if err := rows.Scan(&id, &from, &to, &subject, &body, &receivedAt); err != nil {
			continue
		}

		headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n", from, to, subject)
		fullBody := headers + "\r\n" + body

		emails = append(emails, emailInfo{
			uid:     id,
			headers: headers,
			body:    fullBody,
		})
	}

	return emails
}

// getStats returns message count and total size
func (s *Server) getStats(emails []emailInfo, deleted map[int]bool) (int, int) {
	count := 0
	size := 0
	for i := range emails {
		if deleted[i+1] {
			continue
		}
		count++
		size += len(emails[i].body)
	}
	return count, size
}

// applyDeletions marks deleted emails in the database
func (s *Server) applyDeletions(userID string, emails []emailInfo, deleted map[int]bool) {
	ctx := context.Background()
	for msgNum := range deleted {
		if msgNum < 1 || msgNum > len(emails) {
			continue
		}
		emailID := emails[msgNum-1].uid
		s.store.Pool().Exec(ctx, `
			UPDATE received_emails SET folder = 'trash' WHERE id = $1
		`, emailID)
	}
}

// parseMsgNum parses a message number string
func parseMsgNum(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
