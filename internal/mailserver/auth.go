package mailserver

import (
	"crypto/hmac"
	"strings"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

var errSMTPAuthRequired = &smtp.SMTPError{
	Code:         530,
	EnhancedCode: smtp.EnhancedCode{5, 7, 0},
	Message:      "Authentication required",
}

var _ smtp.AuthSession = (*Session)(nil)
var _ sasl.Server = (*loginServer)(nil)

// AuthMechanisms advertises the mechanisms supported in both modes. In NO
// AUTH mode credentials are accepted without validation so applications that
// insist on configuring SMTP credentials can still be tested without server
// setup.
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain, sasl.Login}
}

// Auth implements smtp.AuthSession for PLAIN and LOGIN authentication.
func (s *Session) Auth(mechanism string) (sasl.Server, error) {
	switch strings.ToUpper(mechanism) {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			if s.mailServer.authRequired() && identity != "" && !secureStringEqual(identity, username) {
				return smtp.ErrAuthFailed
			}
			return s.authenticate(username, password)
		}), nil
	case sasl.Login:
		return newLoginServer(s.authenticate), nil
	default:
		return nil, smtp.ErrAuthUnknownMechanism
	}
}

func (s *Session) authenticate(username, password string) error {
	if s.mailServer.authRequired() && !credentialsEqual(
		username,
		password,
		s.mailServer.authConfig.Username,
		s.mailServer.authConfig.Password,
	) {
		return smtp.ErrAuthFailed
	}
	s.authenticated = true
	return nil
}

func (s *Session) requireAuthentication() error {
	if s.mailServer.authRequired() && !s.authenticated {
		return errSMTPAuthRequired
	}
	return nil
}

func (ms *MailServer) authRequired() bool {
	return ms.authConfig != nil && ms.authConfig.Enabled
}

func credentialsEqual(username, password, expectedUsername, expectedPassword string) bool {
	usernameMatches := secureStringEqual(username, expectedUsername)
	passwordMatches := secureStringEqual(password, expectedPassword)
	return usernameMatches && passwordMatches
}

func secureStringEqual(value, expected string) bool {
	return hmac.Equal([]byte(value), []byte(expected))
}

type loginServer struct {
	state        int
	username     string
	authenticate func(username, password string) error
}

func newLoginServer(authenticate func(username, password string) error) sasl.Server {
	return &loginServer{authenticate: authenticate}
}

func (s *loginServer) Next(response []byte) ([]byte, bool, error) {
	switch s.state {
	case 0:
		if response == nil {
			s.state = 1
			return []byte("Username:"), false, nil
		}
		s.username = string(response)
		s.state = 2
		return []byte("Password:"), false, nil
	case 1:
		if response == nil {
			return nil, false, sasl.ErrUnexpectedClientResponse
		}
		s.username = string(response)
		s.state = 2
		return []byte("Password:"), false, nil
	case 2:
		if response == nil {
			return nil, false, sasl.ErrUnexpectedClientResponse
		}
		s.state = 3
		return nil, true, s.authenticate(s.username, string(response))
	default:
		return nil, false, sasl.ErrUnexpectedClientResponse
	}
}
