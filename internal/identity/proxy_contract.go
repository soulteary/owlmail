package identity

// IdentityHeaders reports the configured assertion names for documentation.
// It exposes no peer allowlist, namespace, credentials or authentication state.
func (p *Proxy) IdentityHeaders() (subject, email, verified string) {
	return p.subjectHeader, p.emailHeader, p.verifiedHeader
}
