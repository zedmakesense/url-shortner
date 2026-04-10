# URL Shortener Architecture Explained

## Quick Summary

This is a Go backend with **layered architecture**:

```
HTTP Request
     ↓
Router → Handler → Service → Repository → Database
```

Each layer has exactly one job:
- **Router**: Maps URLs to handlers
- **Handler**: Converts HTTP requests to function calls
- **Service**: Contains business logic
- **Repository**: Talks to the database

---

## The Layers Explained

### 1. Router (`app/router.go`)

The entry point. It maps URL paths to handler functions.

```
POST /api/v1/auth/register → h.Register
GET  /api/v1/urls          → auth(h.GetURLs)
```

That's it. It doesn't know WHAT the handler does, just WHERE to send the request.

**Why it exists**: Without it, we'd have one giant function checking every URL manually.

---

### 2. Handler (`handler/handler.go`)

Converts HTTP requests into function calls. It:
- Reads the request body
- Validates input
- Calls the Service layer
- Formats and returns the response

```go
type Handler struct {
    service service.Service  // Uses interface, not concrete type
    log     *slog.Logger
    mail    *resend.Client
}
```

**Why `service.Service` is an interface**:
- Handler doesn't need to know about UserService, SessionService, etc.
- It just needs to call methods like `Register()`, `Login()`, etc.
- The interface defines exactly what methods Handler can call

---

### 3. Service Layer (`service/`)

Contains **business logic** - the rules of your application.

```
service/
├── user_service.go      (Register, Login, DeleteUser, etc.)
├── session_service.go   (StoreTokens, RevokeToken, etc.)
├── email_service.go     (SendEmail, VerifyEmail, etc.)
├── url_service.go       (InsertURL, GetLongURL, etc.)
├── password_service.go  (ChangePasswordAndRevoke)
└── password.go          (hashPassword, comparePassword)
```

**Example - Registration Flow**:
```
Register(email, password) {
    1. Hash the password
    2. Insert user into database (via userRepo)
    3. Send verification email (via emailSvc)
    4. Return user ID
}
```

**Why separate services exist**:
- **Single Responsibility**: Each service handles ONE domain (users, sessions, emails)
- **Reusability**: EmailService can be used by Register AND ForgotPassword
- **Testability**: Easy to test each service in isolation

---

### 4. Repository Layer (`repository/`)

Talks directly to the database. Each repository handles ONE database table.

```
repository/
├── user_repo.go     (users table)
├── session_repo.go  (sessions table)
├── email_repo.go    (email_table)
├── url_repo.go      (urls table)
├── password_repo.go (for transactions across tables)
└── common.go        (shared slow query logger)
```

**Example - UserRepository**:
```go
InsertUser(email, name, hashedPassword) → SQL INSERT
GetUserByEmail(email)                   → SQL SELECT
DeleteUser(userID)                       → SQL DELETE
```

**Why `common.go` exists**:
```go
// Before: Every repo had this duplicate code
func (r *UserRepo) logSlowQueries(...) { ... }
func (r *SessionRepo) logSlowQueries(...) { ... }

// After: One shared logger
type slowQueryLogger struct { log *slog.Logger }
```

---

## Dependency Injection (DI)

Dependency Injection means: **"Don't create dependencies inside functions, receive them from outside."**

### Why DI?

**Without DI** (bad):
```go
func Register(ctx, email, password) {
    userRepo := NewUserRepository()  // Where does db come from?
    emailSvc := NewEmailService()   // Where does mail client come from?
    // Now I need to create THOSE dependencies too...
}
```

**With DI** (good):
```go
func Register(ctx, email, password, userRepo, emailSvc) {
    userRepo.InsertUser(...)  // Dependencies already exist
    emailSvc.SendEmail(...)
}
```

### How It Works in This App

Look at `app.go`:

```go
// 1. Create database connection ONCE
dbpool, err := pgxpool.NewWithConfig(...)

// 2. Create repositories with the SAME db connection
userRepo := repository.NewUserRepository(dbpool, log)
sessionRepo := repository.NewSessionRepository(dbpool, log)

// 3. Create services, injecting repositories
sessionSvc := service.NewSessionService(sessionRepo, log)
emailSvc := service.NewEmailService(emailRepo, userRepo, log, mail)

// 4. UserService needs other services too
userSvc := service.NewUserService(userRepo, sessionSvc, emailSvc, passwordSvc, log)

// 5. Finally, create the adapter and router
svc := service.NewService(userSvc, sessionSvc, emailSvc, urlSvc, passwordSvc)
router := NewRouter(svc, log, mail)
```

### The Dependency Graph

```
Database (dbpool) ──────────────────────────────┐
    │                                           │
    ├─→ UserRepository ──→ UserService ──────┐ │
    ├─→ SessionRepository ──→ SessionService  │ │
    ├─→ EmailRepository ──→ EmailService ───┐  │ │
    ├─→ URLRepository ──→ URLService ──────┐ │  │ │
    └─→ PasswordRepository ──→ PasswordSvc ┘ │  │ │
                                            │  │ │
                                    ┌───────┘  │ │
                                    │ UserSvc ◄─┘ │
                                    │            │
                                    └─→ ServiceAdapter ──→ Router ──→ HTTP Server
```

---

## The Service Interface (Why It Exists)

The `Service` interface in `user_service.go` is a **facade** - it combines all services into ONE interface.

```go
type Service interface {
    Register(...)            // from UserService
    Login(...)               // from UserService
    StoreTokens(...)         // from SessionService
    RevokeToken(...)         // from SessionService
    SendEmail(...)           // from EmailService
    GetLongURL(...)          // from URLService
    ChangePasswordAndRevoke(...) // from PasswordService
    // ...all 20+ methods
}
```

**Why this interface exists**:

```
Handler needs to call methods from ALL services.
But Handler struct can only hold ONE thing.

Solution: One interface that includes ALL methods.
```

```go
type Handler struct {
    service service.Service  // ONE interface = access to everything
}
```

---

## Complete Request Flow: Registration

Let's trace a `POST /api/v1/auth/register` request:

```
1. HTTP Request
   POST /api/v1/auth/register
   Body: {"email": "test@example.com", "name": "Test", "password": "secret"}

2. Router (router.go)
   Sees "POST /api/v1/auth/register" → calls h.Register

3. Handler (handler.go:Register)
   - Reads body, validates email format
   - Calls: h.service.Register(ctx, email, name, password)

4. Service Adapter (user_service.go:serviceAdapter.Register)
   - Delegates to: s.userService.Register(ctx, email, name, password)

5. UserService (user_service.go:UserService.Register)
   - Hashes password using hashPassword()
   - Calls: s.userRepo.InsertUser(ctx, email, name, hashedPassword)
   - Calls: s.emailSvc.SendEmail(ctx, email, userID, 1)
   - Returns userID

6. UserRepository (user_repo.go:InsertUser)
   - Executes: INSERT INTO users ...
   - Logs slow queries via slowQueryLogger
   - Returns userID or error

7. Response flows back up the chain:
   Repository → Service → Handler → HTTP Response
```

---

## Directory Structure Overview

```
backend/internal/
├── app/
│   ├── app.go        # Creates everything (DI composition root)
│   ├── router.go     # URL routing
│   └── middleware.go # Auth, logging middleware
│
├── handler/
│   └── handler.go    # HTTP request/response handling
│
├── service/          # Business logic
│   ├── user_service.go      # User registration, login, deletion
│   ├── session_service.go   # Token generation, validation, refresh
│   ├── email_service.go     # Email sending, verification
│   ├── url_service.go       # URL creation, retrieval, click tracking
│   ├── password_service.go  # Password changes
│   └── password.go         # Password hashing utilities
│
├── repository/        # Database operations
│   ├── common.go     # Shared slow query logger, DB interfaces
│   ├── user_repo.go  # User CRUD operations
│   ├── session_repo.go # Session/token CRUD operations
│   ├── email_repo.go # Email token operations
│   ├── url_repo.go   # URL CRUD operations
│   └── password_repo.go # Password change with session revocation
│
└── domain/
    ├── user.go       # User, Token structs
    ├── url.go        # URL struct
    └── err.go        # Error definitions
```

---

## Key Principles Applied

| Principle | How It's Used |
|-----------|---------------|
| **Single Responsibility** | Each file does one thing (user_repo = user table, url_service = url logic) |
| **Dependency Injection** | Dependencies passed via constructors, not created inside |
| **Interface Segregation** | Small interfaces (DBQuerier, DBRows) instead of one giant interface |
| **DRY (Don't Repeat Yourself)** | `slowQueryLogger` in common.go, not duplicated in every repo |
| **Facade Pattern** | `Service` interface combines all services for Handler |

---

## Common Questions

**Q: Why does UserService need EmailService?**
A: When a user registers, we need to send a verification email. Instead of importing email logic into UserService, we inject EmailService so UserService can "use" email functionality.

**Q: Why is there an adapter (`serviceAdapter`)?**
A: It's a bridge. Handler expects ONE `Service` interface with ALL methods. But we have FIVE separate services. The adapter combines them into one interface.

**Q: Can Handler call repositories directly?**
A: Technically yes, but DON'T. Handler should only know about business logic (Service), not database details. This keeps layers clean and testable.

**Q: Why is `slowQueryLogger` in `common.go`?**
A: Because ALL repositories need to log slow queries. Duplicating the code in 5 files violates DRY. One shared implementation is easier to maintain and modify (e.g., changing the 100ms threshold).
