package types

import (
	"time"

	"github.com/emersion/go-message/mail"
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
	Transformed       bool   `json:"-"`
}

// Envelope represents SMTP envelope information
type Envelope struct {
	From          string   `json:"from"`
	To            []string `json:"to"`
	CC            []string `json:"cc"`
	BCC           []string `json:"bcc"`
	CalculatedBCC []string `json:"calculatedBcc"`
	Host          string   `json:"host"`
	RemoteAddress string   `json:"remoteAddress"`
}
