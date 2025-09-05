---
title: "Billing PRD"
sidebar: true
order: 99
---

# Credit Tracking System PRD

## Overview

Implement a credit tracking system for Rapua where users have a credit balance that gets deducted when teams begin playing. This establishes the foundation for future Stripe payment integration.

**Phase 1**: Build and test the credit tracking system with manual admin credit management.
**Phase 2**: Add Stripe integration to allow users to purchase credits directly.

## Core Concept

**Team Starts**: When the first player enters a team code, the team becomes "started" and one credit is deducted from the user's account. This is tracked via the `HasStarted` field in `/models/teams.go`.

**Credit Management**: Users receive automatic monthly credit allowances on the last day of each month (10 credits for regular users, 50 for educators). Admins can manually add additional credits to user accounts. Users can view their credit balance and usage history.

## Implementation Checklist

### Week 1: Database & Models

#### Database Setup
- [x] Create database migration file
- [x] Add `free_credits` column to users table
- [x] Add `paid_credits` column to users table
- [x] Create `team_start_log` table
- [x] Create `credit_adjustments` table
- [x] Add database indexes:
  - [x] `team_start_log.user_id`
  - [x] `team_start_log.created_at`
  - [x] `credit_adjustments.user_id`
  - [x] `credit_adjustments.created_at`
- [x] Run migration on development database
- [x] Test migration rollback
- [x] Add 500 paid credits to existing users as a one-time bonus

#### Go Models
- [x] Update `User` model in `models/users.go`:
  - [x] Add `Credits int` field
  - [x] Add `IsEducator bool` field  
  - [x] Add relationship to `TeamStartLog` and `CreditAdjustments`
- [x] Create `TeamStartLog` model in `models/billing_models.go`
- [x] Create `CreditAdjustment` model in `models/billing_models.go`

### Week 2: Core Services

#### Credit Service
- [x] Create `services/credit_service.go`
- [x] Implement `GetCreditBalance(userID string) int`
- [x] Implement `DeductCredit(userID, teamID, instanceID string) error` 
  - [x] Ensure this handles credit validation and deduction within existing transaction
  - [ ] Return clear errors for insufficient credits (handled by team service)
  - [ ] Support future overage logic within this function
- [x] Implement `AddCredits(userID string, credits int, reason string) error`
- [x] Add validation for insufficient credits
- [x] Ensure atomic credit operations with database transactions
- [ ] Write unit tests for credit service

#### Monthly Credit Refresh Service
- [x] Create `services/monthly_refresh_service.go`
- [x] Implement `GrantMonthlyCredits() error` function
- [x] Add logic to determine credit amount (10 for regular, 50 for educators)
- [x] Ensure idempotency (don't grant multiple times per month)
- [x] Log all monthly grants in `credit_adjustments` table with reason "monthly_allowance"
- [x] Write unit tests for monthly refresh service

#### Team Start Logging Service  
- [x] Create `services/team_start_service.go`
- [x] Implement `LogTeamStart(userID, teamID, instanceID string) error`
- [x] Implement `GetTeamStartHistory(userID string) ([]TeamStartLog, error)`
- [x] Hook into team start event in existing team handler
- [ ] Add error handling for team start logging failures
- [ ] Write unit tests for team start service

#### Team Start Integration
- [x] Find where `Team.HasStarted` is set to `true` in team service
- [x] Update team service to orchestrate the complete team start flow:
  - [x] Start database transaction
  - [x] Call `credit_service.DeductCredit(ownerID, teamID, instanceID)` 
  - [x] On success: log team start via `team_start_service.LogTeamStart()`
  - [x] On success: update `Team.HasStarted = true`
  - [x] Commit transaction
  - [x] On any failure: rollback transaction and return error
- [ ] Add error handling for insufficient credits with clear user messages
- [ ] Test with multiple concurrent team starts
- [ ] Test transaction rollback on credit deduction failures
- [x] Ensure atomic operations (all-or-nothing for credit + log + team update)

### Week 3: Scheduled Tasks & API Endpoints

#### Monthly Credit Refresh Automation
- [x] Research Go cron/scheduler options (consider `github.com/robfig/cron/v3`)
- [x] Create scheduled task to run monthly refresh on first day of each month
- [x] Add job scheduling to application startup
- [x] Set up monitoring and logging for scheduled tasks
- [x] Test scheduler with mock dates and time manipulation
- [ ] Add error handling and retry logic for failed monthly grants

### Week 4: Frontend Integration

#### Teams Page Integration
- [x] Add credit balance display to Teams page
- [ ] Show "Insufficient Credits" warning when balance is low
- [ ] Display error message when trying to start team without credits
- [ ] Test UI with different credit balance scenarios

#### Team Start History Page
- [x] Create team start history page `/credits/team-starts`
- [x] Display paginated list of team starts
- [x] Show current credit balance prominently
- [x] Add date filtering and search
- [ ] Test with various team start histories

#### Admin Credit Management
- [x] Create admin page for credit management
- [ ] Allow admins to add credits
- [x] Show user's current balance and team start history
- [x] Add reason field for credit adjustments
- [ ] Test admin functionality with various scenarios

### Week 5: Testing & Deployment

#### Comprehensive Testing
- [ ] Write end-to-end tests for credit deduction
- [ ] Test concurrent team starts with credit deduction
- [ ] Test edge cases (exactly 0 credits, negative balances)
- [ ] Load test with 100 concurrent users
- [ ] Test admin credit addition functionality

#### Security Review
- [ ] Validate all user inputs and sanitization
- [ ] Review database access patterns for security
- [ ] Scan for common web vulnerabilities
- [ ] Test concurrent access patterns

#### Documentation & Deployment
- [ ] Document admin credit management process
- [ ] Create troubleshooting guide for common issues
- [ ] Test deployment process on staging environment
- [ ] Deploy to production
- [ ] Set up monitoring for credit operations
- [ ] Create runbook for credit management

## Success Criteria (Phase 1)

- [ ] Credit deduction works reliably when teams start
- [x] Teams page shows accurate credit balance
- [ ] Users cannot start teams without sufficient credits
- [ ] Team start history is complete and accessible
- [ ] Admins can successfully add credits to accounts
- [x] Team start performance impact <50ms
- [ ] System handles 100+ concurrent users
- [ ] Database operations are atomic and consistent

## Future Phase 2: Stripe Integration

Once Phase 1 is complete and tested, add Stripe payment processing:

### Additional Database Schema for Phase 2
```sql
CREATE TABLE credit_purchases (
  id VARCHAR(36) PRIMARY KEY,
  user_id VARCHAR(36) NOT NULL REFERENCES users(id),
  credits INT NOT NULL,
  amount_paid INT NOT NULL, -- cents
  stripe_payment_id VARCHAR(255),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add to existing users table
ALTER TABLE users ADD COLUMN stripe_customer_id VARCHAR(255);
```

### Phase 2 Features to Add
- Stripe payment processing for credit purchases
- Tiered pricing (1-19 credits: $0.35 each, 20-39: $0.32, etc.)
- Payment webhooks and receipt generation
- Credit purchase UI replacing admin credit management
- Automated credit addition from successful payments

### Integration Approach
- Keep all existing credit tracking logic unchanged
- Replace admin credit addition with Stripe purchase flow  
- Add `credit_purchases` table to track payment transactions
- The existing `credit_adjustments` table continues to log all credit changes
