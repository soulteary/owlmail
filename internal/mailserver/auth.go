package mailserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
			if s.mailServer.authRequired() && identity != "" {
				if s.mailServer.authVerifier == nil || !s.mailServer.authVerifier.stringsEqual(identity, username) {
					return smtp.ErrAuthFailed
				}
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
	if s.mailServer.authRequired() {
		if s.mailServer.authVerifier == nil || !s.mailServer.authVerifier.credentialsEqual(username, password) {
			return smtp.ErrAuthFailed
		}
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

// credentialVerifier normalizes credentials into fixed-size, keyed tags. The
// expected tags are computed once at startup so request timing does not depend
// on the configured username or password length.
type credentialVerifier struct {
	key                 [sha256.Size]byte
	expectedUsernameTag [sha256.Size]byte
	expectedPasswordTag [sha256.Size]byte
}

func newCredentialVerifier(username, password string) (*credentialVerifier, error) {
	verifier := &credentialVerifier{}
	if _, err := rand.Read(verifier.key[:]); err != nil {
		return nil, err
	}
	verifier.expectedUsernameTag = verifier.tag(username)
	verifier.expectedPasswordTag = verifier.tag(password)
	return verifier, nil
}

func (v *credentialVerifier) credentialsEqual(username, password string) bool {
	usernameTag := v.tag(username)
	passwordTag := v.tag(password)
	usernameMatches := subtle.ConstantTimeCompare(usernameTag[:], v.expectedUsernameTag[:])
	passwordMatches := subtle.ConstantTimeCompare(passwordTag[:], v.expectedPasswordTag[:])
	return usernameMatches&passwordMatches == 1
}

func (v *credentialVerifier) stringsEqual(value, expected string) bool {
	valueTag := v.tag(value)
	expectedTag := v.tag(expected)
	return subtle.ConstantTimeCompare(valueTag[:], expectedTag[:]) == 1
}

func (v *credentialVerifier) tag(value string) [sha256.Size]byte {
	mac := hmac.New(sha256.New, v.key[:])
	_, _ = mac.Write([]byte(value))
	var tag [sha256.Size]byte
	mac.Sum(tag[:0])
	return tag
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
