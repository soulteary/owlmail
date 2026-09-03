# Contributing Guide

Thank you for your interest in the OwlMail project! We welcome contributions of all kinds.

## How to Contribute

### Reporting Issues

If you find a bug or have a feature suggestion, please:

1. Check [Issues](https://github.com/soulteary/owlmail/issues) to see if a similar issue already exists
2. If not, create a new Issue using the appropriate template
3. Provide as much detail as possible, including:
   - Problem description
   - Steps to reproduce
   - Expected behavior
   - Actual behavior
   - Environment information (OS, Go version, etc.)

### Submitting Code

1. **Fork the Repository**
   ```bash
   git clone https://github.com/soulteary/owlmail.git
   cd owlmail
   ```

2. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

3. **Make Changes**
   - Write clear code
   - Follow the project's code style
   - Add necessary tests
   - Update relevant documentation

4. **Run Tests**
   ```bash
   # Run all tests
   go test ./...
   
   # Run tests with coverage
   go test -cover ./...
   
   # Run tests for specific packages
   go test ./internal/api/...

   # Match the local CI checks before opening a pull request
   go test -race ./...
   go vet ./...
   bun build ./web/*.js --target=browser --outdir=./.bun-check
   bun test ./tests/web ./tests/docs
   ```

5. **Commit Changes**
   ```bash
   git add .
   git commit -m "feat: add new feature description"
   # or
   git commit -m "fix: fix issue description"
   ```

   Commit messages should follow [Conventional Commits](https://www.conventionalcommits.org/) specification:
   - `feat:` New feature
   - `fix:` Bug fix
   - `docs:` Documentation changes
   - `style:` Code style (formatting, no code change)
   - `refactor:` Code refactoring
   - `test:` Adding or updating tests
   - `chore:` Build process or auxiliary tool changes

6. **Push and Create Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```
   
   Then create a Pull Request on GitHub and fill in the PR template information.

## Code Standards

### Go Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format code
- Run `golangci-lint run --timeout=5m` to match the lint workflow
- Keep functions concise with single responsibility

### Testing Requirements

- New features must include tests
- Bug fixes should include regression tests
- Test coverage should not decrease
- Use table-driven tests for multiple test cases

### Documentation Requirements

- Public APIs must have documentation comments
- Complex logic should have explanatory comments
- Update relevant README or documentation

## Development Environment Setup

### Prerequisites

- Go 1.27.0 or higher
- Bun version pinned in `.bun-version`
- Git

Install Bun using the [official instructions](https://bun.sh/docs/installation), then run `bun --version` and confirm it matches `.bun-version` before executing the browser and documentation checks.

### Setup Steps

1. Fork and clone the repository
   ```bash
   git clone https://github.com/YOUR_USERNAME/owlmail.git
   cd owlmail
   ```

2. Add upstream repository
   ```bash
   git remote add upstream https://github.com/soulteary/owlmail.git
   ```

3. Install dependencies
   ```bash
   go mod download
   ```

4. Run tests to ensure everything works
   ```bash
   go test ./...
   ```

## Pull Request Process

1. Ensure your branch is based on the latest `main` branch
   ```bash
   git checkout main
   git pull upstream main
   git checkout your-branch
   git rebase main
   ```

2. Ensure all tests pass
   ```bash
   go test ./...
   ```

3. Ensure code is formatted
   ```bash
   go fmt ./...
   ```

4. Create Pull Request
   - Use clear title and description
   - Link related Issues (if any)
   - Describe your changes and reasons
   - Add test screenshots or examples (if applicable)

5. Wait for Code Review
   - Maintainers will review your PR
   - Some modifications may be needed
   - Please respond to review comments promptly

## Project Structure

```
OwlMail/
├── cmd/
│   └── owlmail/          # Main program entry
├── internal/
│   ├── api/              # Web API implementation
│   ├── attachmentstore/  # Local and S3 attachment storage
│   ├── common/           # Common utilities (logging, error handling)
│   ├── config/           # Layered configuration and validation
│   ├── maildev/          # MailDev compatibility layer
│   ├── mailserver/       # SMTP server implementation
│   ├── mcpserver/        # Read-only MCP transports and workflows
│   ├── outgoing/         # Email relay implementation
│   ├── sendmail/         # Sendmail-compatible client
│   ├── types/            # Type definitions
│   └── webhook/          # Webhook filtering and delivery
├── docs/                 # User-facing and historical documentation
├── examples/             # Runnable integration examples
├── tests/                # Browser and documentation tests
├── web/                  # Web frontend files
└── .github/              # GitHub configuration files
```

## Types of Contributions

We welcome the following types of contributions:

- 🐛 **Bug Fixes**: Fix issues with existing features
- ✨ **New Features**: Add new features or improve existing ones
- 📝 **Documentation**: Improve documentation, add examples or tutorials
- 🎨 **UI/UX**: Improve web interface
- ⚡ **Performance**: Performance optimizations
- 🧪 **Tests**: Add or improve tests
- 🔧 **Tools**: Improve development tools or build processes

## Questions

If you encounter any issues during contribution, please:

1. Check existing [Issues](https://github.com/soulteary/owlmail/issues)
2. Search the [documentation](../docs/README.md)
3. Create a new Issue describing your question or problem

## Code of Conduct

Please follow our [Code of Conduct](CODE_OF_CONDUCT.md) to keep the community friendly and respectful.

## License

By contributing, you agree that your contributions will be licensed under the same [MIT License](../LICENSE) as the project.

---

Thank you again for contributing to OwlMail! 🦉
