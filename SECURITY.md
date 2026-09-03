# Security Policy

## Supported Versions

We currently provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.8.x | ✅ Yes |
| 0.7.x | ✅ Yes |
| 0.6.x and older | ❌ No |

## Reporting a Vulnerability

We take the security of OwlMail seriously. If you discover a security vulnerability, please **do not** report it in a public issue.

### How to Report

Please report security vulnerabilities by:

1. **Email**: Send to [security@owlmail.dev](mailto:security@owlmail.dev)
   - Please use a descriptive subject line
   - Include a detailed description of the vulnerability
   - Provide steps to reproduce (if possible)
   - Explain the potential impact

2. **Wait for Response**: We will acknowledge receipt within 48 hours

3. **Process**:
   - We will assess the severity of the vulnerability
   - If confirmed as a security issue, we will:
     - Develop a fix
     - Prepare a security advisory
     - Release a patched version
   - We will keep you updated on the progress

### What to Include

To help us better understand and fix the vulnerability, please include in your report:

- **Vulnerability Type**: e.g., SQL injection, XSS, privilege escalation, etc.
- **Affected Component**: Which feature or component is affected
- **Steps to Reproduce**: Detailed steps on how to reproduce the vulnerability
- **Potential Impact**: What consequences the vulnerability might have
- **Suggested Fix** (if any)

### Bug Bounty

While we don't currently have a formal bug bounty program, we take security contributions seriously and will acknowledge them appropriately (with your permission).

## Security Best Practices

### For Users

- **Keep Updated**: Keep OwlMail updated to the latest version
- **Network Security**: Use HTTPS/TLS in production environments
- **Access Control**: Configure appropriate authentication and authorization
- **Environment Isolation**: Don't expose unprotected instances on public networks
- **Sensitive Information**: Don't hardcode passwords or keys in code or configuration

### HTML Email Preview Isolation

OwlMail treats captured HTML email as untrusted content. The server removes
active markup and unsafe URL schemes, constrains stylesheet links and inline
styles, and hardens external anchors with safe targets and relationship tokens.
The web inbox then renders the result inside a sandboxed `srcdoc` iframe without
script, form, popup, same-origin, or top-navigation permissions. The frame also
uses `referrerpolicy="no-referrer"` and a restrictive Content Security Policy.

Remote images, fonts, stylesheets, and media can disclose that a message was
viewed, the viewer's IP address, and request-specific tracking identifiers.
OwlMail blocks those requests by default. The **Load remote content** control is
an explicit, per-message, non-persistent exception intended for visual template
inspection. Loading remote content contacts infrastructure selected by the
message author. CID images are mapped to OwlMail's local attachment endpoint
and do not require enabling remote content.

The isolation is defense in depth, not a reason to expose OwlMail publicly.
Keep authentication enabled and restrict network access whenever captured mail
may contain sensitive data.

### For Developers

- **Dependency Updates**: Regularly update dependencies to get security patches
- **Code Review**: Carefully review all code changes
- **Security Testing**: Perform security testing during development
- **Least Privilege**: Follow the principle of least privilege
- **Input Validation**: Always validate and sanitize user input

## Known Security Issues

We will disclose known security issues after they have been fixed. Check [Security Advisories](https://github.com/soulteary/owlmail/security/advisories) for details.

## Security Updates

Security updates will be released through:

- GitHub Releases
- Security Advisories
- Project documentation updates

## Contact

- **Security Issues**: [security@owlmail.dev](mailto:security@owlmail.dev)
- **General Issues**: Submit in [GitHub Issues](https://github.com/soulteary/owlmail/issues)

## Acknowledgments

We appreciate all researchers and users who responsibly report security issues. Your contributions help us keep OwlMail secure.
