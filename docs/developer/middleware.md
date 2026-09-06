---
title: "Middleware"
sidebar: true
order: 7
---

# Middleware in the Application

Middleware in this application provides a way to intercept and process HTTP requests before they reach the final handler. Each middleware serves a specific purpose, such as authentication, session management, or request modification.

---

## Types of Middleware

### 1. Preview Middleware

**File:** `/internal/middlewares/preview_middleware.go`

**Purpose:**
Lets an author see their own quest as a player would, without a real run
existing. It builds a run named "Preview" in memory and never writes it.

**Key Features:**
- Detects HTMX preview requests from admin or template pages
- Builds an unsaved `models.Run` with the code `preview`, on the quest named
  in the request
- Gives that run a quest window open from now until an hour ahead, so status
  checks downstream see an active game
- Templates are public; any other quest is refused unless the requester owns it
- Marks the request as a preview, which the middlewares below pass straight
  through

**Usage Example:**
```go
middleware := PreviewMiddleware(logger, runService, questService, identityService, nextHandler)
```

### 2. Run Middleware

**File:** `/internal/middlewares/run_middleware.go`

**Purpose:**
Extracts the run code from the session and finds the matching quest.

**Key Features:**
- Retrieves the run code from the session
- Loads the run and its quest
- Adds the run to the request context
- Passes preview requests straight through

**Usage Example:**
```go
middleware := RunMiddleware(logger, runService, nextHandler)
```

### 3. Start Middleware

**File:** `/internal/middlewares/start_middleware.go`

**Purpose:**
Holds players on the start page until the game is open and they have pressed
Start. It reads the run that Run Middleware put in the context; it does not add
one of its own.

**Key Features:**
- Sends a request with no run in context to `/play`
- Sends a player to `/start` while the quest is not active, or while their run
  has not started
- Lets the blocks the start page itself needs through regardless: the team name
  block, the game status alert, and the start button
- Passes preview requests straight through

**Usage Example:**
```go
middleware := StartMiddleware(runService, nextHandler)
```

### 4. Admin Authentication Middleware

**File:** `/internal/middlewares/admin_auth_middleware.go`

**Purpose:**
Manages authentication and authorisation for administrative routes.

**Key Features:**
- Verifies user authentication
- Checks email verification status
- Ensures users have selected a quest for admin actions

**Usage Example:**
```go
middleware := AdminAuthMiddleware(logger, authService, questLoader, nextHandler)
middleware := AdminCheckInstanceMiddleware(nextHandler)
```

### 5. Auth Status Middleware

**File:** `/internal/middlewares/auth_status_middleware.go`

**Purpose:**
Records whether an admin is logged in, for pages that are public but render
differently when they are.

**Key Features:**
- Adds a `UserStatus` to the request context

**Usage Example:**
```go
middleware := AuthStatusMiddleware(authService, nextHandler)
```

### 6. Quest Editable Middleware

**File:** `/internal/middlewares/quest_editable_middleware.go`

**Purpose:**
Refuses edits to a running game. A change made mid-run reaches players
immediately and cannot be taken back, so the editor is locked while a quest is
active and the author has to stop it, or duplicate it and work on the copy.

**Key Features:**
- Read-only methods pass through untouched
- Any other method is refused while the quest is active
- An author who has confirmed they mean it sends an unlock header, which passes
- What a refusal renders is the caller's to decide, via `onRefused`

**Usage Example:**
```go
middleware := QuestEditableMiddleware(logger, onRefused, nextHandler)
```

### 7. Text HTML and HTMX-Only Middleware

**File:** `/internal/middlewares/middleware.go`

**Purpose:**
Two small guards: one sets the response content type, the other keeps
fragment-only routes from being opened directly.

**Key Features:**
- `TextHTMLMiddleware` sets `Content-Type` to `text/html`
- `HtmxOnlyMiddleware` redirects a request that did not come from HTMX, since a
  fragment rendered on its own is not a page

**Usage Example:**
```go
middleware := TextHTMLMiddleware(nextHandler)
middleware := HtmxOnlyMiddleware(logger, "/admin/quest", nextHandler)
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

### Adding the Run to Context
```go
ctx := context.WithValue(r.Context(), contextkeys.RunKey, run)
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
