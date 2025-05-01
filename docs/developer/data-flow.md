---
title: "Data Flow"
sidebar: true
order: 5 
---

# Data Flow in Rapua

This document explains how data flows through the Rapua application, from initialization to request handling.

## System Overview

Rapua follows a layered architecture pattern with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                         HTTP Layer                          │
│  (Chi Router, Middlewares, Handlers/Controllers)            │
├─────────────────────────────────────────────────────────────┤
│                       Service Layer                         │
│  (Business Logic, Validation, Orchestration)                │
├─────────────────────────────────────────────────────────────┤
│                     Repository Layer                        │
│  (Data Access, DB Operations)                               │
├─────────────────────────────────────────────────────────────┤
│                      Database Layer                         │
│  (SQLite via Bun ORM)                                       │
└─────────────────────────────────────────────────────────────┘
```

## Application Startup

1. **Initialization** (`cmd/rapua/main.go`):
   - Environment variables are loaded from `.env`
   - Database connection is established using `db.MustOpen()`
   - Migrations are configured but only run if explicitly requested
   - Repositories are instantiated with database connections
   - Services are created and wired together
   - The HTTP server is started

2. **Service Initialization**:
   - Services receive their dependencies via constructor injection
   - Services may depend on repositories and other services
   - Transactor is used for transactions spanning multiple repositories

## Request Lifecycle

When a request arrives at the application:

1. **Router** (`internal/server/routes.go`):
   - The Chi router determines which handler should process the request
   - Global middleware is applied: compression, path cleaning, etc.
   - Route-specific middleware may be applied (authentication, etc.)

2. **Middleware** (`internal/middlewares/`):
   - Authentication status is checked
   - For admin routes, admin permissions are verified
   - For player routes, team association is verified
   - Context values may be set for handlers to use

3. **Handlers** (`internal/handlers/`):
   - Organized by user type: admin, players, public
   - Handle HTTP-specific concerns (parsing params, rendering responses)
   - Call appropriate services to execute business logic
   - Render templates using the Templ templating system

4. **Services** (`internal/services/`):
   - Implement core business logic
   - Coordinate calls to repositories
   - Handle validation and business rules
   - May use transactions for operations affecting multiple entities

5. **Repositories** (`repositories/`):
   - Provide data access methods
   - Execute database queries using Bun ORM
   - Return domain models to services
   - Handle database-specific concerns

6. **Response** (back through the layers):
   - Services return results to handlers
   - Handlers format data and render templates
   - Response passes back through middleware
   - HTTP response is sent to the client

## Key Data Flows

### Authentication Flow

```
┌──────────┐    ┌──────────┐    ┌─────────────┐    ┌────────────┐
│ Router   │───▶│ Auth     │───▶│ Auth        │───▶│ User       │
│          │    │Middleware│    │ Service     │    │ Repository │
└──────────┘    └──────────┘    └─────────────┘    └────────────┘
```

### Player Game Flow

```
┌──────────┐    ┌──────────┐    ┌─────────────┐    ┌────────────┐
│ Player   │───▶│ Team     │───▶│ Gameplay    │───▶│ Multiple   │
│ Handlers │    │Middleware│    │ Service     │    │Repositories│
└──────────┘    └──────────┘    └─────────────┘    └────────────┘
                                      │
                                      ▼
                                ┌─────────────┐
                                │ Block       │
                                │ Service     │
                                └─────────────┘
```

### Admin Dashboard Flow

```
┌──────────┐    ┌──────────┐    ┌─────────────┐    ┌────────────┐
│ Admin    │───▶│ Admin    │───▶│ Game        │───▶│  Multiple  │
│ Handlers │    │Middleware│    │ Manager     │    │Repositories│
└──────────┘    └──────────┘    └─────────────┘    └────────────┘
                                      │
                                      ▼
                               ┌─────────────┐
                               │ Location    │
                               │ Service     │
                               └─────────────┘
```

## Database Transactions

For operations that need to modify multiple entities atomically, Rapua uses a transactor pattern:

```
┌──────────┐    ┌─────────────┐    ┌────────────┐    ┌────────────┐
│ Handler  │───▶│ Service     │───▶│ Transactor │───▶│ Repository │
│          │    │             │    │            │    │            │
└──────────┘    └─────────────┘    └────────────┘    └────────────┘
                                         │                  ▲
                                         │                  │
                                         ▼                  │
                                   ┌────────────┐    ┌────────────┐
                                   │ Repository │───▶│  Database  │
                                   │            │    │            │
                                   └────────────┘    └────────────┘
```

This ensures operations like creating a new game instance, which affects multiple tables, either succeed completely or fail without partial updates.

## Block Content Flow

Game content in Rapua is organized around "blocks" - reusable content elements like text, images, pincodes, etc.

```
┌──────────┐    ┌─────────────┐    ┌────────────┐    ┌────────────┐
│ Block    │───▶│ Block       │───▶│ Block      │───▶│ Block      │
│ Handler  │    │ Service     │    │ Repository │    │ State      │
└──────────┘    └─────────────┘    └────────────┘    │ Repository │
                                                     └────────────┘
```

Player interactions with blocks update the block state, which is tracked separately from the block definition.
