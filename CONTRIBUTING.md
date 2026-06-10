# Contributing to Go SNMP OLT ZTE C320

Thank you for your interest in contributing to this project! We welcome contributions from the community.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

### Expected Behavior

- Use welcoming and inclusive language
- Be respectful of differing viewpoints and experiences
- Gracefully accept constructive criticism
- Focus on what is best for the community
- Show empathy towards other community members

## Getting Started

### Prerequisites

- Go 1.26 or higher
- Docker & Docker Compose
- Task (task runner)
- Git
- A ZTE C320 OLT device or SNMP simulator for testing (optional)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
```bash
git clone https://github.com/YOUR_USERNAME/snmp-olt-zte.git
cd snmp-olt-zte
```

3. Add the upstream repository:
```bash
git remote add upstream https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte.git
```

### Environment Setup

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Update `.env` with your local configuration:
```bash
# SNMP Configuration (for testing)
SNMP_HOST=192.168.1.1  # Your OLT IP
SNMP_PORT=161
SNMP_COMMUNITY=public

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
```

3. Install dependencies:
```bash
task init
```

4. Start development environment:
```bash
task dev
```

## Development Workflow

### Branching Strategy

We use a simplified Git Flow:

- `main` - Production-ready code
- `develop` - Integration branch for features
- `feature/*` - Feature branches
- `bugfix/*` - Bug fix branches
- `hotfix/*` - Urgent production fixes

### Creating a Feature Branch

```bash
# Update your local develop branch
git checkout develop
git pull upstream develop

# Create a feature branch
git checkout -b feature/your-feature-name
```

### Branch Naming Convention

- Features: `feature/add-onu-filtering`
- Bug fixes: `bugfix/fix-redis-timeout`
- Hotfixes: `hotfix/critical-snmp-error`
- Documentation: `docs/update-api-readme`
- Tests: `test/add-handler-tests`

## Coding Standards

### Go Style Guide

Follow the official [Effective Go](https://go.dev/doc/effective_go) guidelines and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Code Formatting

- Use `gofmt` for formatting (automatically applied in our workflow)
- Run `go vet` to catch common mistakes
- Use `golangci-lint` for comprehensive linting

```bash
# Format code
go fmt ./...

# Vet code
go vet ./...

# Lint (if installed)
golangci-lint run
```

### Project Structure

```
.
├── api/              # OpenAPI 3.1 specification
├── app/              # Application setup and routing
├── cmd/              # Application entrypoints
│   └── api/         # API server entrypoint
├── config/          # Configuration (env-based, OID generation)
├── internal/        # Private application code
│   ├── errors/     # Typed error system (validation, SNMP, Redis, config)
│   ├── handler/    # HTTP handlers with request ID correlation
│   ├── middleware/ # Auth, CORS, rate limiting, security, validation
│   ├── model/      # Data models
│   ├── repository/ # SNMP connection pool, Redis cache operations
│   ├── trap/       # SNMP Trap listener, handler, webhook
│   ├── usecase/    # Business logic, singleflight, background refresh
│   └── utils/      # OID extractors, power converters, response helpers
├── scripts/         # Testing scripts (trap testing)
└── pkg/            # Public libraries
    ├── graceful/   # Graceful shutdown
    ├── pagination/ # Pagination calculation
    ├── redis/      # Redis client factory
    └── snmp/       # SNMP connection setup
```

### Naming Conventions

- **Files**: Use lowercase with underscores (`onu_handler.go`, `snmp_test.go`)
- **Packages**: Use lowercase, single word (`handler`, `middleware`, `usecase`)
- **Interfaces**: Suffix with "Interface" (`OnuUsecaseInterface`)
- **Constants**: Use PascalCase (`MaxPageSize`, `DefaultTimeout`)
- **Variables**: Use camelCase (`onuID`, `boardNumber`)

### Comments and Documentation

- Add godoc comments for all exported functions, types, and constants
- Use complete sentences starting with the function/type name
- Explain **why**, not **what** (code should be self-explanatory)

```go
// GetByBoardIDAndPonID retrieves all ONUs for a specific board and PON combination.
// It first checks the Redis cache and falls back to SNMP if cache miss occurs.
// Returns an error if both board_id and pon_id are invalid.
func (h *OnuHandler) GetByBoardIDAndPonID(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### Error Handling

- Use custom error types from `internal/errors` package
- Log errors with appropriate levels (ERROR, WARN, INFO, DEBUG)
- Never expose internal errors to API responses

```go
// Good
if err != nil {
    log.Error().Err(err).Msg("Failed to fetch ONU data from SNMP")
    return apperrors.NewSNMPError("Get", err)
}

// Bad - exposes internal error
if err != nil {
    return err
}
```

### Logging Standards

Use structured logging with zerolog:

```go
// Good - structured logging
log.Info().
    Int("board_id", boardID).
    Int("pon_id", ponID).
    Msg("Fetching ONU information")

// Bad - string concatenation
log.Info().Msg("Fetching ONU for board " + strconv.Itoa(boardID))
```

**Log Levels:**
- **ERROR**: Real problems (SNMP failures, Redis connection issues, system errors)
- **WARN**: Client errors (validation failures, rate limits, cache misses)
- **INFO**: Successful operations (cache hits, successful API calls)
- **DEBUG**: Detailed troubleshooting (query details, response sizes)

## Testing Requirements

### Unit Tests

All new code must include unit tests with minimum **96% code coverage** for:
- Handlers
- Usecases
- Repositories
- Utility functions

### Writing Tests

```go
func TestOnuHandler_GetByBoardIDAndPonID_Success(t *testing.T) {
    // Arrange - setup mocks and test data
    usecase := &mockOnuUsecase{
        GetByBoardIDAndPonIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
            return []model.ONUInfoPerBoard{{Board: 1, PON: 1, ID: 1}}, nil
        },
    }

    // Act - execute the test
    handler := NewOnuHandler(usecase)
    req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1", nil)
    rr := httptest.NewRecorder()
    handler.GetByBoardIDAndPonID(rr, req)

    // Assert - verify results
    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
}
```

### Running Tests

```bash
# Run all tests
task test

# Run with verbose output
task test-verbose

# Generate coverage report
task test-coverage

# View HTML coverage report
task test-html
```

### Test Coverage Requirements

- **New features**: ≥96% coverage required
- **Bug fixes**: Add tests reproducing the bug
- **Refactoring**: Maintain or improve existing coverage

## Pull Request Process

### Before Submitting

1. **Update from upstream**:
```bash
git checkout develop
git pull upstream develop
git checkout your-feature-branch
git rebase develop
```

2. **Run tests**:
```bash
task test
```

3. **Check coverage**:
```bash
task test-coverage
# Ensure overall coverage is ≥96%
# Current project coverage is 99%
```

4. **Format and lint**:
```bash
go fmt ./...
go vet ./...
```

5. **Commit your changes**:
```bash
git add .
git commit -m "feat: add ONU filtering by status"
```

### Commit Message Convention

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Build process or auxiliary tool changes
- `perf`: Performance improvements

**Examples:**
```
feat(handler): add filtering by ONU status

Add query parameter 'status' to filter ONUs by online/offline status.
Includes unit tests and API documentation updates.

Closes #123
```

```
fix(usecase): prevent race condition in cache updates

Use mutex to synchronize cache writes when multiple requests
update the same board/PON combination simultaneously.

Fixes #456
```

### Creating a Pull Request

1. Push to your fork:
```bash
git push origin your-feature-branch
```

2. Go to GitHub and create a Pull Request from your branch to `develop`

3. Fill in the PR template with:
   - **Description**: What does this PR do?
   - **Motivation**: Why is this change needed?
   - **Testing**: How was this tested?
   - **Screenshots**: If UI changes (not applicable for API)
   - **Checklist**: Complete all items

### PR Review Process

- At least one maintainer approval required
- All CI checks must pass (tests, linting, build)
- Code coverage must not decrease
- Address all review comments
- Squash commits before merging (if requested)

## Reporting Issues

### Bug Reports

Use the GitHub issue tracker with the following information:

**Title**: Clear, concise description

**Description**:
- **Expected behavior**: What should happen?
- **Actual behavior**: What actually happens?
- **Steps to reproduce**:
  1. Step 1
  2. Step 2
  3. Step 3
- **Environment**:
  - Go version: `go version`
  - OS: Linux/macOS/Windows
  - Docker version (if applicable)
- **Logs**: Include relevant log output
- **Additional context**: Screenshots, stack traces

### Feature Requests

**Title**: Feature request: <feature name>

**Description**:
- **Problem statement**: What problem does this solve?
- **Proposed solution**: How would you implement it?
- **Alternatives considered**: Other approaches?
- **Use case**: Real-world scenario where this is needed

## Development Tips

### Hot Reload

Use Air for hot reloading during development:

```bash
task dev  # Uses Air automatically
```

### Debugging

Enable debug logging:

```bash
export APP_ENV=development
export LOG_LEVEL=debug
```

### Testing with Real OLT

If you have access to a ZTE C320 OLT:

```bash
# Update .env
SNMP_HOST=<your-olt-ip>
SNMP_COMMUNITY=<your-community-string>

# Start service
task dev
```

### Load Testing

Test performance with k6:

```bash
task load-test
```

## Questions?

- Open a GitHub Discussion for questions
- Join our community chat (if available)
- Email maintainers (check README.md)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing! 🎉
