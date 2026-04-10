# URL Shortener - Refactored Architecture

> **Branch created by:** big-pickle (OpenCode AI)

---

## Architecture Overview

```
HTTP Request
     ↓
Router → Handler → Service → Repository → Database
```

### Flow Diagram

```mermaid
graph TD
    A[HTTP Request] --> B[Router]
    B --> C[Handler]
    C --> D[Service Layer]
    D --> E[Repository Layer]
    E --> F[(PostgreSQL)]
    
    G[Email Service] --> H[(Resend API)]
    D --> G
```

---

## Directory Structure

```
backend/internal/
├── app/
│   ├── app.go          # Composition root, DI setup
│   ├── router.go       # URL routing, CORS setup
│   └── middleware.go   # Auth, logging middleware
│
├── handler/
│   └── handler.go      # HTTP request/response handling
│
├── service/           # Business logic (domain-focused)
│   ├── user_service.go     # Register, Login, GetUser, DeleteUser
│   ├── session_service.go  # Token generation, validation, refresh
│   ├── email_service.go    # Email verification, forgot password
│   ├── url_service.go     # URL creation, retrieval, click tracking
│   ├── password_service.go # Password change with session revocation
│   └── password.go        # hashPassword, comparePassword, hashToken
│
├── repository/        # Database operations (table-focused)
│   ├── slowQueryLogger.go # Shared slowQueryLogger
│   ├── user_repo.go      # users table CRUD
│   ├── session_repo.go    # sessions table CRUD
│   ├── email_repo.go      # email_table CRUD
│   ├── url_repo.go        # urls table CRUD
│   └── password_repo.go   # Password change transaction
│
└── domain/
    ├── user.go       # User, Token, EmailToken structs
    ├── url.go        # URL structs
    └── err.go        # Error definitions
```

---

## Key Components

### Router (`app/router.go`)

Maps HTTP endpoints to handler functions.

```go
func NewRouter(services *service.Services, log *slog.Logger) http.Handler
```

**Endpoints:**
```
POST /api/v1/auth/register     → h.Register
POST /api/v1/auth/login        → h.Login
POST /api/v1/auth/logout       → h.Logout
POST /api/v1/auth/refresh      → h.Refresh
POST /api/v1/auth/verify-email → h.VerifyEmail
POST /api/v1/auth/forgot-password → h.ForgotPassword
POST /api/v1/auth/reset-password  → h.ResetPassword
GET  /{slug}                   → h.Redirect
GET  /api/v1/auth/me           → h.Me
DELETE /api/v1/auth/me         → h.Me
POST /api/v1/urls             → h.InsertURL
GET  /api/v1/urls             → h.GetURLs
GET  /api/v1/urls/{slug}      → h.GetURL
DELETE /api/v1/urls/{slug}     → h.DeleteURL
```

---

### Handler (`handler/handler.go`)

Converts HTTP requests to service calls. Uses `*service.Services` for DI.

```go
type Handler struct {
    services *service.Services
    log      *slog.Logger
}

func NewHandler(services *service.Services, log *slog.Logger) *Handler
```

---

### Service Layer (`service/`)

Business logic, separated by domain.

#### UserService
```go
type UserService struct {
    userRepo    *repository.UserRepository
    sessionSvc  *SessionService
    emailSvc    *EmailService
    passwordSvc *PasswordService
    log         *slog.Logger
}

func (s *UserService) Register(ctx context.Context, email, name, password string) (int, error)
func (s *UserService) Login(ctx context.Context, email, password string) (int, error)
func (s *UserService) GetUserByUserID(ctx context.Context, userID int) (domain.User, error)
func (s *UserService) CheckPassword(ctx context.Context, userID int, password string) error
func (s *UserService) DeleteUser(ctx context.Context, userID int) error
```

#### SessionService
```go
type SessionService struct {
    sessionRepo *repository.SessionRepository
    log         *slog.Logger
}

func (s *SessionService) StoreTokens(ctx context.Context, userID int, accessToken, refreshToken string, accessExpiresAt, refreshExpiresAt time.Time) error
func (s *SessionService) RevokeToken(ctx context.Context, refreshToken string) error
func (s *SessionService) RevokeTokens(ctx context.Context, userID, sessionID int) error
func (s *SessionService) ReplaceTokens(ctx context.Context, accessToken, refreshToken string, userID int, accessExpiresAt, refreshExpiresAt time.Time) error
func (s *SessionService) GetByAccessToken(ctx context.Context, accessToken string) (int, int, error)
func (s *SessionService) GetByRefreshToken(ctx context.Context, refreshToken string) (int, int, error)
func (s *SessionService) ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error)
func (s *SessionService) GenerateToken() (string, error)
```

#### EmailService
```go
type EmailService struct {
    emailRepo *repository.EmailRepository
    userRepo  *repository.UserRepository
    log       *slog.Logger
    mail      *resend.Client
}

func (s *EmailService) SendEmail(ctx context.Context, email string, userID int, expiresAt int) error
func (s *EmailService) CheckEmail(ctx context.Context, userID int) error
func (s *EmailService) VerifyEmail(ctx context.Context, token string) error
func (s *EmailService) VerifyEmailToken(ctx context.Context, token string) (int, error)
func (s *EmailService) SendForgotPasswordMail(ctx context.Context, email string) error
```

#### URLService
```go
type URLService struct {
    urlRepo *repository.URLRepository
    log     *slog.Logger
}

func (s *URLService) GetLongURL(ctx context.Context, shortCode string) (string, error)
func (s *URLService) URLClicked(ctx context.Context, shortCode string) error
func (s *URLService) InsertURL(ctx context.Context, longURL string, userID int) (string, error)
func (s *URLService) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error)
func (s *URLService) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error)
func (s *URLService) DeleteURLByShortCode(ctx context.Context, shortCode string) error
```

#### PasswordService
```go
type PasswordService struct {
    passwordRepo *repository.PasswordRepository
    log         *slog.Logger
}

func (s *PasswordService) ChangePasswordAndRevoke(ctx context.Context, userID int, password string) error
```

---

### Repository Layer (`repository/`)

Database operations, separated by table.

#### UserRepository
```go
func (r *UserRepository) InsertUser(ctx context.Context, email, name, hashedPassword string) (int, error)
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error)
func (r *UserRepository) GetUserByUserID(ctx context.Context, userID int) (domain.User, error)
func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error
func (r *UserRepository) MarkUserVerified(ctx context.Context, userID int) error
```

#### SessionRepository
```go
func (r *SessionRepository) InsertSession(ctx context.Context, userID int, accessTokenHash, refreshTokenHash []byte, accessExpiresAt, refreshExpiresAt time.Time) error
func (r *SessionRepository) GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error)
func (r *SessionRepository) GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error)
func (r *SessionRepository) RevokeToken(ctx context.Context, sessionID int) error
func (r *SessionRepository) RevokeTokens(ctx context.Context, userID, sessionID int) error
func (r *SessionRepository) ReplaceTokens(ctx context.Context, accessTokenHash, refreshTokenHash []byte, sessionID int, accessExpiresAt, refreshExpiresAt time.Time) error
```

#### EmailRepository
```go
func (r *EmailRepository) GetEmailTableByID(ctx context.Context, userID int) (domain.EmailToken, error)
func (r *EmailRepository) GetEmailTableByToken(ctx context.Context, hashedToken []byte) (domain.EmailToken, error)
func (r *EmailRepository) RevokeEmailTokens(ctx context.Context, userID int) error
func (r *EmailRepository) InsertEmailToken(ctx context.Context, userID int, hashedToken []byte, expiresAt time.Time) error
```

#### URLRepository
```go
func (r *URLRepository) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error)
func (r *URLRepository) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error)
func (r *URLRepository) InsertURL(ctx context.Context, shortCode, longURL string, userID int, createdAt time.Time) error
func (r *URLRepository) URLClicked(ctx context.Context, shortCode string) error
func (r *URLRepository) DeleteURLByShortCode(ctx context.Context, shortCode string) error
```

---

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/jackc/pgx/v5` | latest | PostgreSQL driver |
| `github.com/redis/go-redis/v9` | latest | Redis client |
| `github.com/resend/resend-go/v3` | latest | Email service |
| `golang.org/x/crypto/bcrypt` | latest | Password hashing |
| `github.com/rs/cors` | latest | CORS handling |

---

## Configuration

Environment variables or `.env` file:

```env
DATABASE_URL=postgres://user:pass@localhost:5432/dbname
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
RESEND_API_KEY=your_api_key
PORT=3000
LOG_LEVEL=info
```

---

## Build & Run

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- Redis 7+

### Commands

```bash
# Install dependencies
cd backend && go mod tidy

# Build
go build ./...

# Run
go run ./cmd/server/main.go

# Format
make format-backend

# Lint
make lint-backend
```

### Docker

```bash
docker-compose -f dev-compose.yml up -d
```

---

## Design Principles

| Principle | Implementation |
|-----------|---------------|
| **Single Responsibility** | Each service handles one domain, each repo handles one table |
| **Dependency Injection** | Dependencies passed via constructors |
| **DRY** | Shared `slowQueryLogger` in `slowQueryLogger.go` |
| **Layered Architecture** | Handler → Service → Repository → Database |

---

## Token Flow

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant SessionService
    participant SessionRepo
    
    Client->>Handler: POST /register
    Handler->>SessionService: GenerateToken()
    SessionService-->>Handler: accessToken, refreshToken
    Handler->>SessionService: StoreTokens()
    SessionService->>SessionRepo: InsertSession()
    SessionRepo-->>SessionService: OK
    Handler-->>Client: Set-Cookie: access_token, refresh_token
```

---

## Error Handling

Domain errors defined in `domain/err.go`:
- `ErrEmailAlreadyExists`
- `ErrUserDoesNotExist`
- `ErrTokenNotFound`
- `ErrAccessTokenExpired`
- `ErrRefreshTokenExpired`
- `ErrEmailAlreadyVerified`
- `ErrEmailVerificationFailed`
- `ErrURLAlreadyExist`
- `ErrURLDoesNotExist`

---

## Performance

- Slow query logging: queries > 100ms logged as warnings
- Connection pooling: configurable max/min connections
- Token hashing: SHA-256 for token storage
- Password hashing: bcrypt with default cost
