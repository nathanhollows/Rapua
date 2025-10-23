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

### Phase 2: Stripe Integration Implementation

### Week 8: Payment Processing & Security

#### Webhook Processing
- [x] Implement idempotent webhook processing (checks purchase status before processing)
- [x] Add database transaction handling for credit updates
- [x] Log all transactions in `credit_adjustments` table
- [x] Handle failed payments (checkout.session.async_payment_failed)
- [x] Add comprehensive error logging via slog
- [x] Write integration tests for webhook processing

#### Security & Reliability
- [ ] Implement rate limiting on purchase creation
- [x] Add input sanitization and validation (1-1000 credit range)
- [x] Test webhook signature validation (Stripe webhook.ConstructEvent)
- [x] Handle duplicate webhook events gracefully (returns 200 for already processed)
- [ ] Add monitoring for failed payments
- [ ] Test atomic credit updates under load

### Week 9: Testing & Quality Assurance

#### End-to-End Testing
- [ ] Test complete purchase flow from modal to credit update
- [ ] Test webhook processing with Stripe test events
- [ ] Verify atomic credit updates and rollback scenarios
- [ ] Test edge cases (network failures, cancelled payments)
- [ ] Load test payment processing with multiple concurrent users
- [ ] Test mobile checkout experience

#### Integration Testing
- [ ] Test Stripe webhook with various event types
- [ ] Verify credit balance updates across all UI components
- [ ] Test purchase history and transaction logging
- [ ] Validate error handling and user feedback
- [ ] Test admin visibility of purchase records

### Week 10: Deployment & Monitoring

#### Production Deployment
- [ ] Deploy Stripe webhook endpoint to production
- [ ] Configure production Stripe API keys and webhook URLs
- [ ] Run database migrations on production
- [ ] Test production webhook connectivity
- [ ] Deploy frontend credit purchase features
- [ ] Verify end-to-end flow in production

#### Monitoring & Analytics
- [ ] Set up monitoring for payment processing
- [ ] Add analytics tracking for purchase funnel
- [ ] Create alerts for failed payments
- [ ] Monitor credit purchase conversion rates
- [ ] Set up reporting for financial reconciliation
- [ ] Document troubleshooting procedures

## Success Criteria (Phase 2)

- [ ] Users can successfully purchase credits via Stripe Checkout
- [x] Credit balance updates immediately after successful payment (via webhook)
- [x] Webhook processing is reliable and idempotent
- [x] Purchase history is accurate and complete (stored in credit_purchases table)
- [x] Payment processing handles errors gracefully (error states handled)
- [ ] Mobile checkout experience works seamlessly
- [ ] Financial reconciliation between Stripe and app is accurate
- [ ] Security measures prevent fraud and abuse
- [ ] Performance impact of payment processing is minimal
- [x] Admin can view and manage user purchases (visible in credit_adjustments)

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

### Phase 2 Features to Add

- Stripe payment processing for credit purchases
- Credit top-up modal with amount selection and price calculation
- Stripe Checkout integration with preset quantity and pricing
- Payment webhooks and receipt generation
- Automated credit addition from successful payments
- Credit purchase UI replacing admin credit management

### Credit Purchase User Flow

#### Frontend Flow
1. **Credit Top-Up Modal**:
   - User clicks "Top Up Credits" button (when balance low or from settings)
   - Modal displays current credit balance
   - Input field for desired credit amount (with validation: min 1, max 1000)
   - Real-time price calculation: `amount * $0.35 = total price`
   - "Purchase Credits" button to proceed

2. **Stripe Checkout Integration**:
   - Frontend creates Stripe Checkout session via API call
   - Redirect to Stripe-hosted payment page with:
     - Line item: "{quantity} Credits" at $0.35 each
     - Total amount pre-calculated
     - Success/cancel URLs configured
   - User completes payment on Stripe's secure page

3. **Post-Payment Handling**:
   - Success: Redirect to success page showing new credit balance
   - Cancel: Return to app with no changes
   - Error: Show error message with retry option

#### Backend Implementation
1. **API Endpoints**:
   ```
   POST /api/credits/purchase/create-session
   - Creates Stripe Checkout session
   - Validates credit amount (1-1000)
   - Stores pending purchase in database
   - Returns session URL for redirect

   POST /api/webhooks/stripe (webhook endpoint)  
   - Handles payment completion events
   - Validates webhook signature
   - Updates credit balance atomically
   - Logs transaction in credit_purchases table
   ```

2. **Database Updates**:
   ```sql
   -- Add session tracking for pending purchases
   ALTER TABLE credit_purchases ADD COLUMN stripe_session_id VARCHAR(255);
   ALTER TABLE credit_purchases ADD COLUMN status VARCHAR(20) DEFAULT 'pending';
   -- status: pending, completed, failed, cancelled
   ```

3. **Payment Processing Flow**:
   - Create Checkout session → Store pending purchase record
   - Webhook receives payment completion → Validate session
   - Atomic transaction:
     - Add credits to user account (paid_credits column)
     - Update purchase record status to 'completed'
     - Log credit adjustment with reason "stripe_purchase"

#### Security & Reliability
- Stripe webhook signature validation for security
- Idempotent webhook processing (handle duplicate events)
- Database transactions for atomic credit updates
- Error handling and logging for failed payments
- Rate limiting on purchase creation (prevent abuse)

### Integration Approach
- Keep all existing credit tracking logic unchanged
- Replace admin credit addition with Stripe purchase flow  
- Add `credit_purchases` table to track payment transactions
- The existing `credit_adjustments` table continues to log all credit changes
- Maintain flat $0.35/credit pricing (no bulk discounts for simplicity)
