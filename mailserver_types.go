package main

import (
	"sync"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-smtp"
)

const (
	defaultPort    = 1025
	defaultHost    = "localhost"
	defaultMailDir = "owlmail"
)

// Email represents a parsed email message
type Email struct {
	ID            string                 `json:"id"`
	Time          time.Time              `json:"time"`
	Read          bool                   `json:"read"`
	Subject       string                 `json:"subject"`
	From          []*mail.Address        `json:"from"`
	To            []*mail.Address        `json:"to"`
	CC            []*mail.Address        `json:"cc"`
	BCC           []*mail.Address        `json:"bcc"`
	CalculatedBCC []*mail.Address        `json:"calculatedBcc"`
	Text          string                 `json:"text"`
	HTML          string                 `json:"html"`
	Attachments   []*Attachment          `json:"attachments"`
	Envelope      *Envelope              `json:"envelope"`
	Source        string                 `json:"source"`
	Size          int64                  `json:"size"`
	SizeHuman     string                 `json:"sizeHuman"`
	Headers       map[string]interface{} `json:"headers"`
}

// Attachment represents an email attachment
type Attachment struct {
	ContentType       string `json:"contentType"`
	FileName          string `json:"fileName"`
	GeneratedFileName string `json:"generatedFileName"`
	ContentID         string `json:"contentId"`
	Size              int64  `json:"size"`
	transformed       bool
}

// Envelope represents SMTP envelope information
type Envelope struct {
	From          string   `json:"from"`
	To            []string `json:"to"`
	Host          string   `json:"host"`
	RemoteAddress string   `json:"remoteAddress"`
}

// SMTPAuthConfig represents SMTP authentication configuration
type SMTPAuthConfig struct {
	Username string
	Password string
	Enabled  bool
}

// TLSConfig represents TLS configuration for SMTP server
type TLSConfig struct {
	CertFile string
	KeyFile  string
	Enabled  bool
}

// MailServer represents the SMTP mail server
type MailServer struct {
	store          []*Email
	storeMutex     sync.RWMutex
	mailDir        string
	port           int
	host           string
	smtpServer     *smtp.Server
	smtpsServer    *smtp.Server // SMTPS server (direct TLS on 465)
	eventChan      chan Event
	listeners      map[string][]func(*Email)
	listenersMutex sync.RWMutex
	outgoing       *OutgoingMail
	authConfig     *SMTPAuthConfig
	tlsConfig      *TLSConfig
}

// Event represents a server event
type Event struct {
	Type  string
	Email *Email
	ID    string
}
