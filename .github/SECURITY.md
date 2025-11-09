# Security Policy

## Supported Versions

We currently provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| Latest  | ✅ Yes             |
| Previous major version | ✅ Yes |
| Older versions | ❌ No |

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
