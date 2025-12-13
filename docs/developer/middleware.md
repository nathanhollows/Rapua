---
title: "Middleware"
sidebar: true
order: 6
---

# Middleware in the Application

Middleware in this application provides a way to intercept and process HTTP requests before they reach the final handler. Each middleware serves a specific purpose, such as authentication, session management, or request modification.

---

## Types of Middleware

### 1. Preview Middleware

**File:** `/internal/middlewares/preview_middleware.go`

**Purpose:**
The Preview Middleware enables temporary preview functionality for administrators and developers, allowing them to render game content without affecting the live game.

**Key Features:**
- Detects HTMX preview requests from admin or template pages
- Creates a temporary team instance with a preview context
- Sets a short-lived game instance (1 hour duration), overriding the active instance

**Usage Example:**
```go
middleware := PreviewMiddleware(teamService, nextHandler)
```

### 2. Team Middleware

**File:** `/internal/middlewares/team_middleware.go`

**Purpose:**
The Team Middleware extracts team information from the session and loads the associated game instance.

**Key Features:**
- Retrieves team code from the session
- Loads team and instance relationships
- Adds team context to the request

**Usage Example:**
```go
middleware := TeamMiddleware(teamService, nextHandler)
```

### 3. Start Middleware

**File:** `/internal/middlewares/start_middleware.go`

**Purpose:**
The Start Middleware manages team access based on the game instance status, redirecting users to the Start page when necessary.

**Key Features:**
- Checks game instance status
- Redirects to Start page for inactive game instances
- Adds team context to the request

**Usage Example:**
```go
middleware := StartMiddleware(teamService, nextHandler)
```

### 4. Admin Authentication Middleware

**File:** `admin.go`

**Purpose:**
Manages authentication and authorization for administrative routes.

**Key Features:**
- Verifies user authentication
- Checks email verification status
- Ensures users have selected an instance for admin actions

**Usage Example:**
```go
middleware := AdminAuthMiddleware(authService, nextHandler)
middleware := AdminCheckInstanceMiddleware(nextHandler)
```

### 5. Text HTML Middleware

**File:** `/internal/middlewares/middleware.go`

**Purpose:**
A simple middleware to set the content type for responses.

**Key Features:**
- Sets `Content-Type` header to `text/html`

**Usage Example:**
```go
middleware := TextHTMLMiddleware(nextHandler)
```

---

## Best Practices

1. **Chaining Middleware**: Use multiple middlewares in a chain to modularise request processing.
    - Preview Middleware *must* be the first middleware in the chain when used.
2. **Context Management**: Utilise request context to pass additional information between middlewares.
    - Use the internal `contextkeys` package for context keys.
3. **Performance**: Keep middleware logic lightweight and efficient.

---

## Common Patterns

### Adding Team to Context
```go
ctx := context.WithValue(r.Context(), contextkeys.TeamKey, team)
next.ServeHTTP(w, r.WithContext(ctx))
```

### Passing Through Non-Matching Requests
```go
if !matchingCondition {
    next.ServeHTTP(w, r)
    return
}
```

---

## Extending Middleware

To create a new middleware:
1. Define a function that takes a `next http.Handler`
2. Return a new `http.Handler`
3. Implement request interception logic
4. Call `next.ServeHTTP()` to continue the request chain

---

## Testing

Middleware can be tested by:
- Mocking services
- Creating test requests
- Verifying context modifications
- Checking response behaviours

Refer to `internal/middlewares/preview_middleware_test.go` for example tests.
