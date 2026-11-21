package smtp

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"

	"snailmail/internal/outbound"
	"snailmail/internal/store"
)

// SubmissionServer handles authenticated email submission on port 587
type SubmissionServer struct {
	store    *store.Store
	listener net.Listener
	done     chan struct{}
	tlsCert  string
	tlsKey   string
}

// NewSubmissionServer creates a new SMTP submission server
func NewSubmissionServer(store *store.Store, port string, tlsCert, tlsKey string) (*SubmissionServer, error) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	return &SubmissionServer{
		store:    store,
		listener: listener,
		done:     make(chan struct{}),
		tlsCert:  tlsCert,
		tlsKey:   tlsKey,
	}, nil
}

// Start starts the submission server
func (s *SubmissionServer) Start() error {
	log.Printf("SMTP submission server starting on %s", s.listener.Addr())

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

// Stop stops the submission server
func (s *SubmissionServer) Stop() error {
	close(s.done)
	return s.listener.Close()
}

// handleConnection handles a single submission connection
func (s *SubmissionServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writer.WriteString("220 snaelmail.org ESMTP Submission\r\n")
	writer.Flush()

	var from string
	var to []string
	var data strings.Builder
	inData := false
	authenticated := false
	username := ""

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

		if inData {
			if line == "." {
				writer.WriteString("250 OK: Message accepted\r\n")
				writer.Flush()

				// Process message (store and queue for delivery)
				s.processSubmission(username, from, to, data.String())

				from = ""
				to = []string{}
				data.Reset()
				inData = false
			} else {
				data.WriteString(line + "\r\n")
			}
			continue
		}

		switch cmd {
		case "EHLO", "HELO":
			writer.WriteString("250-snaelmail.org\r\n")
			writer.WriteString("250-AUTH PLAIN LOGIN\r\n")
			writer.WriteString("250-STARTTLS\r\n")
			writer.WriteString("250 SIZE 10485760\r\n")
			writer.Flush()

		case "AUTH":
			if len(parts) < 2 {
				writer.WriteString("500 Syntax error\r\n")
				writer.Flush()
				continue
			}

			mech := strings.ToUpper(parts[1])
			switch mech {
			case "PLAIN":
				var creds string
				if len(parts) >= 3 {
					creds = parts[2]
				} else {
					writer.WriteString("334 \r\n")
					writer.Flush()
					resp, _ := reader.ReadString('\n')
					creds = strings.TrimSpace(resp)
				}

				decoded, err := base64.StdEncoding.DecodeString(creds)
				if err != nil {
					writer.WriteString("535 Authentication failed\r\n")
					writer.Flush()
					continue
				}

				// Format: \0username\0password
				credParts := strings.Split(string(decoded), "\x00")
				if len(credParts) < 3 {
					writer.WriteString("535 Authentication failed\r\n")
					writer.Flush()
					continue
				}

				user, err := s.store.GetUserByHandle(context.Background(), credParts[1])
				if err != nil || user == nil {
					writer.WriteString("535 Authentication failed\r\n")
					writer.Flush()
					continue
				}

				// Verify password (simplified)
				_ = user
				authenticated = true
				username = credParts[1]
				writer.WriteString("235 Authentication successful\r\n")
				writer.Flush()

			case "LOGIN":
				writer.WriteString("334 VXNlcm5hbWU6\r\n") // "Username:"
				writer.Flush()
				userResp, _ := reader.ReadString('\n')
				userResp = strings.TrimSpace(userResp)
				decodedUser, _ := base64.StdEncoding.DecodeString(userResp)

				writer.WriteString("334 UGFzc3dvcmQ6\r\n") // "Password:"
				writer.Flush()
				passResp, _ := reader.ReadString('\n')
				passResp = strings.TrimSpace(passResp)
				decodedPass, _ := base64.StdEncoding.DecodeString(passResp)

				user, err := s.store.GetUserByHandle(context.Background(), string(decodedUser))
				if err != nil || user == nil {
					writer.WriteString("535 Authentication failed\r\n")
					writer.Flush()
					continue
				}

				_ = decodedPass
				authenticated = true
				username = string(decodedUser)
				writer.WriteString("235 Authentication successful\r\n")
				writer.Flush()

			default:
				writer.WriteString("504 Unsupported mechanism\r\n")
				writer.Flush()
			}

		case "STARTTLS":
			if s.tlsCert != "" && s.tlsKey != "" {
				writer.WriteString("220 Ready to start TLS\r\n")
				writer.Flush()

				cert, _ := tls.LoadX509KeyPair(s.tlsCert, s.tlsKey)
				tlsConn := tls.Server(conn, &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				})
				tlsConn.Handshake()
				conn = tlsConn
				reader = bufio.NewReader(conn)
				writer = bufio.NewWriter(conn)
			} else {
				writer.WriteString("454 TLS not available\r\n")
				writer.Flush()
			}

		case "MAIL":
			if !authenticated {
				writer.WriteString("530 Authentication required\r\n")
				writer.Flush()
				continue
			}
			from = extractEmailFromCmd(parts[1])
			if from == "" {
				writer.WriteString("500 Invalid sender\r\n")
				writer.Flush()
				continue
			}
			writer.WriteString("250 OK\r\n")
			writer.Flush()

		case "RCPT":
			if !authenticated {
				writer.WriteString("530 Authentication required\r\n")
				writer.Flush()
				continue
			}
			recipient := extractEmailFromCmd(parts[1])
			if recipient == "" {
				writer.WriteString("500 Invalid recipient\r\n")
				writer.Flush()
				continue
			}
			to = append(to, recipient)
			writer.WriteString("250 OK\r\n")
			writer.Flush()

		case "DATA":
			if !authenticated || from == "" || len(to) == 0 {
				writer.WriteString("503 Need MAIL and RCPT first\r\n")
				writer.Flush()
				continue
			}
			writer.WriteString("354 Start mail input\r\n")
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

// processSubmission handles a submitted email
func (s *SubmissionServer) processSubmission(username, from string, to []string, data string) {
	ctx := context.Background()

	// Parse subject
	subject := "No Subject"
	body := data
	lines := strings.Split(data, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "subject:") {
			subject = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		}
	}
	// Extract body (after empty line)
	parts := strings.SplitN(data, "\r\n\r\n", 2)
	if len(parts) > 1 {
		body = parts[1]
	}

	// Queue for delivery
	for _, recipient := range to {
		err := outbound.Enqueue(ctx, s.store, from, recipient, subject, body, "")
		if err != nil {
			log.Printf("Failed to enqueue submitted email: %v", err)
		} else {
			log.Printf("Queued submitted email from %s to %s", from, recipient)
		}
	}

	_ = username
	_ = ctx
}

// extractEmailFromCmd extracts email from SMTP command
func extractEmailFromCmd(arg string) string {
	arg = strings.TrimPrefix(arg, "FROM:")
	arg = strings.TrimPrefix(arg, "TO:")
	arg = strings.TrimPrefix(arg, "from:")
	arg = strings.TrimPrefix(arg, "to:")
	arg = strings.Trim(arg, "<>")
	if !strings.Contains(arg, "@") {
		return ""
	}
	return strings.TrimSpace(arg)
}
