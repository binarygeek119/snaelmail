package imap

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"snailmail/internal/auth"
	"snailmail/internal/store"

	"github.com/google/uuid"
)

// Server represents an IMAP server
type Server struct {
	store     *store.Store
	listener  net.Listener
	tlsConfig *tls.Config
	clients   map[net.Conn]*Client
	mu        sync.RWMutex
	done      chan struct{}
}

// Client represents an IMAP client connection
type Client struct {
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	userID   uuid.UUID
	state    string // "not_authenticated", "authenticated", "selected"
	mailbox  string // Current selected mailbox
	seqNum   uint32 // Current sequence number
	messages []*Message
}

// Message represents an email message
type Message struct {
	UID        uint32
	Subject    string
	From       string
	To         string
	Date       time.Time
	Flags      []string
	Size       int
	Body       string
	EnvelopeID uuid.UUID
}

// Mailbox represents a mailbox (folder)
type Mailbox struct {
	Name        string
	Flags       []string
	Exists      uint32
	Recent      uint32
	FirstUnseen uint32
	UIDNext     uint32
	UIDValidity uint32
}

// NewServer creates a new IMAP server
func NewServer(store *store.Store, port string, certFile, keyFile string) (*Server, error) {
	// Load TLS certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	return &Server{
		store:     store,
		listener:  listener,
		tlsConfig: tlsConfig,
		clients:   make(map[net.Conn]*Client),
		done:      make(chan struct{}),
	}, nil
}

// Start starts the IMAP server
func (s *Server) Start() error {
	log.Printf("IMAP server starting on %s", s.listener.Addr())

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

// Stop stops the IMAP server
func (s *Server) Stop() error {
	close(s.done)
	return s.listener.Close()
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	client := &Client{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		state:  "not_authenticated",
	}

	s.mu.Lock()
	s.clients[conn] = client
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
	}()

	// Send greeting
	client.sendResponse("* OK [CAPABILITY IMAP4rev1 AUTH=PLAIN STARTTLS] Snaelmail IMAP server ready")

	// Handle commands
	for {
		line, err := client.reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from client: %v", err)
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !client.handleCommand(s.store, line) {
			return
		}
	}
}

// handleCommand handles a single IMAP command
func (c *Client) handleCommand(store *store.Store, line string) bool {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		c.sendResponse("* BAD Invalid command")
		return true
	}

	tag := parts[0]
	cmd := strings.ToUpper(parts[1])
	args := ""
	if len(parts) > 2 {
		args = parts[2]
	}

	switch cmd {
	case "CAPABILITY":
		c.handleCapability(tag)
	case "NOOP":
		c.sendResponse(tag + " OK NOOP completed")
	case "LOGOUT":
		c.sendResponse("* BYE IMAP4rev1 Server logging out")
		c.sendResponse(tag + " OK LOGOUT completed")
		return false
	case "LOGIN":
		c.handleLogin(store, tag, args)
	case "LIST":
		c.handleList(store, tag, args)
	case "SELECT":
		c.handleSelect(store, tag, args)
	case "EXAMINE":
		c.handleSelect(store, tag, args) // Same as SELECT but read-only
	case "FETCH":
		c.handleFetch(store, tag, args)
	case "STORE":
		c.handleStore(store, tag, args)
	case "SEARCH":
		c.handleSearch(store, tag, args)
	case "STARTTLS":
		c.sendResponse(tag + " OK Begin TLS negotiation now")
		// Note: In a real implementation, you'd upgrade the connection here
	default:
		c.sendResponse(tag + " BAD Command not implemented")
	}

	return true
}

// handleCapability handles the CAPABILITY command
func (c *Client) handleCapability(tag string) {
	c.sendResponse("* CAPABILITY IMAP4rev1 AUTH=PLAIN STARTTLS")
	c.sendResponse(tag + " OK CAPABILITY completed")
}

// handleLogin handles the LOGIN command
func (c *Client) handleLogin(store *store.Store, tag string, args string) {
	if c.state != "not_authenticated" {
		c.sendResponse(tag + " BAD Already authenticated")
		return
	}

	// Parse username and password
	parts := strings.SplitN(args, " ", 2)
	if len(parts) != 2 {
		c.sendResponse(tag + " BAD Invalid LOGIN arguments")
		return
	}

	username := strings.Trim(parts[0], "\"")
	password := strings.Trim(parts[1], "\"")

	// Authenticate user
	ctx := context.Background()
	user, err := store.GetUserByHandle(ctx, username)
	if err != nil || user == nil {
		c.sendResponse(tag + " NO [AUTHENTICATIONFAILED] Invalid credentials")
		return
	}

	// Verify password
	valid, err := auth.VerifyPassword(password, user.PassHash)
	if err != nil || !valid {
		c.sendResponse(tag + " NO [AUTHENTICATIONFAILED] Invalid credentials")
		return
	}

	c.userID = user.ID
	c.state = "authenticated"
	c.sendResponse(tag + " OK [CAPABILITY IMAP4rev1] LOGIN completed")
}

// handleList handles the LIST command
func (c *Client) handleList(store *store.Store, tag string, args string) {
	if c.state == "not_authenticated" {
		c.sendResponse(tag + " NO Not authenticated")
		return
	}

	// List available mailboxes
	mailboxes := []string{"INBOX", "Sent", "Drafts", "Trash"}
	
	for _, mailbox := range mailboxes {
		c.sendResponse(fmt.Sprintf(`* LIST (\HasNoChildren) "/" "%s"`, mailbox))
	}
	
	c.sendResponse(tag + " OK LIST completed")
}

// handleSelect handles the SELECT command
func (c *Client) handleSelect(store *store.Store, tag string, args string) {
	if c.state == "not_authenticated" {
		c.sendResponse(tag + " NO Not authenticated")
		return
	}

	mailboxName := strings.Trim(args, "\"")
	
	// Get messages for this mailbox
	ctx := context.Background()
	var messages []*Message
	var err error

	switch mailboxName {
	case "INBOX":
		messages, err = c.getInboxMessages(ctx, store)
	case "Sent":
		messages, err = c.getSentMessages(ctx, store)
	default:
		c.sendResponse(tag + " NO Mailbox does not exist")
		return
	}

	if err != nil {
		c.sendResponse(tag + " NO Error selecting mailbox")
		return
	}

	c.mailbox = mailboxName
	c.messages = messages
	c.state = "selected"
	c.seqNum = uint32(len(messages))

	// Send mailbox information
	c.sendResponse(fmt.Sprintf("* %d EXISTS", len(messages)))
	c.sendResponse(fmt.Sprintf("* 0 RECENT"))
	c.sendResponse("* OK [UIDVALIDITY 1] UIDs valid")
	c.sendResponse(fmt.Sprintf("* OK [UIDNEXT %d] Predicted next UID", len(messages)+1))
	c.sendResponse("* FLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)")
	c.sendResponse(fmt.Sprintf("* OK [PERMANENTFLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)] Limited"))
	
	c.sendResponse(tag + " OK [READ-WRITE] SELECT completed")
}

// handleFetch handles the FETCH command
func (c *Client) handleFetch(store *store.Store, tag string, args string) {
	if c.state != "selected" {
		c.sendResponse(tag + " NO No mailbox selected")
		return
	}

	// Parse fetch command
	parts := strings.SplitN(args, " ", 2)
	if len(parts) != 2 {
		c.sendResponse(tag + " BAD Invalid FETCH arguments")
		return
	}

	seqSet := parts[0]
	items := parts[1]

	// Parse sequence set (simplified - just handle "1:*" or specific numbers)
	var startSeq, endSeq uint32
	if seqSet == "1:*" {
		startSeq = 1
		endSeq = c.seqNum
	} else {
		fmt.Sscanf(seqSet, "%d", &startSeq)
		endSeq = startSeq
	}

	// Fetch messages
	for i := startSeq; i <= endSeq && i <= uint32(len(c.messages)); i++ {
		msg := c.messages[i-1]
		
		if strings.Contains(items, "BODY[]") || strings.Contains(items, "RFC822") {
			// Return full message
			c.sendResponse(fmt.Sprintf("* %d FETCH (BODY[] {%d}", i, len(msg.Body)))
			c.sendResponse(msg.Body)
			c.sendResponse(")")
		} else if strings.Contains(items, "FLAGS") {
			flags := strings.Join(msg.Flags, " ")
			c.sendResponse(fmt.Sprintf("* %d FETCH (FLAGS (%s))", i, flags))
		} else if strings.Contains(items, "ENVELOPE") {
			c.sendResponse(fmt.Sprintf(`* %d FETCH (ENVELOPE ("%s" NIL NIL NIL NIL NIL NIL NIL "<%s@%s>"))`, 
				i, msg.Subject, msg.EnvelopeID, "snaelmail.org"))
		}
	}

	c.sendResponse(tag + " OK FETCH completed")
}

// handleStore handles the STORE command (for setting flags)
func (c *Client) handleStore(store *store.Store, tag string, args string) {
	if c.state != "selected" {
		c.sendResponse(tag + " NO No mailbox selected")
		return
	}

	// Parse store command
	parts := strings.SplitN(args, " ", 3)
	if len(parts) != 3 {
		c.sendResponse(tag + " BAD Invalid STORE arguments")
		return
	}

	seqSet := parts[0]
	action := strings.ToUpper(parts[1])
	flags := strings.Trim(parts[2], "()")

	// Parse sequence set
	var seqNum uint32
	fmt.Sscanf(seqSet, "%d", &seqNum)

	if seqNum < 1 || seqNum > uint32(len(c.messages)) {
		c.sendResponse(tag + " NO Invalid sequence number")
		return
	}

	// Update flags
	msg := c.messages[seqNum-1]
	switch action {
	case "+FLAGS":
		msg.Flags = append(msg.Flags, strings.Split(flags, " ")...)
	case "-FLAGS":
		// Remove flags
		flagList := strings.Split(flags, " ")
		newFlags := []string{}
		for _, f := range msg.Flags {
			found := false
			for _, remove := range flagList {
				if f == remove {
					found = true
					break
				}
			}
			if !found {
				newFlags = append(newFlags, f)
			}
		}
		msg.Flags = newFlags
	case "FLAGS":
		msg.Flags = strings.Split(flags, " ")
	}

	c.sendResponse(fmt.Sprintf("* %d FETCH (FLAGS (%s))", seqNum, strings.Join(msg.Flags, " ")))
	c.sendResponse(tag + " OK STORE completed")
}

// handleSearch handles the SEARCH command
func (c *Client) handleSearch(store *store.Store, tag string, args string) {
	if c.state != "selected" {
		c.sendResponse(tag + " NO No mailbox selected")
		return
	}

	// Return all message sequence numbers
	seqNums := []string{}
	for i := uint32(1); i <= c.seqNum; i++ {
		seqNums = append(seqNums, fmt.Sprintf("%d", i))
	}

	c.sendResponse("* SEARCH " + strings.Join(seqNums, " "))
	c.sendResponse(tag + " OK SEARCH completed")
}

// sendResponse sends a response to the client
func (c *Client) sendResponse(response string) {
	c.writer.WriteString(response + "\r\n")
	c.writer.Flush()
}

// getInboxMessages retrieves inbox messages for the user
func (c *Client) getInboxMessages(ctx context.Context, store *store.Store) ([]*Message, error) {
	envelopes, err := store.ListInbox(ctx, c.userID, 100)
	if err != nil {
		return nil, err
	}

	messages := make([]*Message, len(envelopes))
	for i, env := range envelopes {
		messages[i] = &Message{
			UID:        uint32(i + 1),
			Subject:    env.Subject,
			From:       "sender@snaelmail.org", // TODO: Get actual sender
			To:         "user@snaelmail.org",   // TODO: Get actual recipient
			Date:       env.QueuedAt,
			Flags:      []string{"\\Seen"},
			Size:       env.SizeBytes,
			Body:       string(env.Ciphertext), // TODO: Decrypt
			EnvelopeID: env.ID,
		}
	}

	return messages, nil
}

// getSentMessages retrieves sent messages for the user
func (c *Client) getSentMessages(ctx context.Context, store *store.Store) ([]*Message, error) {
	envelopes, err := store.ListOutbox(ctx, c.userID, 100)
	if err != nil {
		return nil, err
	}

	messages := make([]*Message, len(envelopes))
	for i, env := range envelopes {
		messages[i] = &Message{
			UID:        uint32(i + 1),
			Subject:    env.Subject,
			From:       "user@snaelmail.org",   // TODO: Get actual sender
			To:         "recipient@snaelmail.org", // TODO: Get actual recipient
			Date:       env.QueuedAt,
			Flags:      []string{"\\Seen"},
			Size:       env.SizeBytes,
			Body:       string(env.Ciphertext), // TODO: Decrypt
			EnvelopeID: env.ID,
		}
	}

	return messages, nil
}
