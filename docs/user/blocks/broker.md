---
title: "Broker Block"
sidebar: true
order: 9
---

# Broker Block

The broker block creates a blind bidding system where participants can pay points to unlock information. Players enter how many points they're willing to spend, and they receive the best information tier their bid can afford. This creates strategic decision-making around information value.

Unlike other blocks, the broker block has no completion bonus - players pay exactly what they bid, making it a pure economic exchange.

## How It Works

1. **Blind Bidding**: Players enter their point offer without knowing the tier thresholds
2. **Tiered Information**: Admins configure multiple information tiers with different point requirements
3. **Best Value**: Players get the highest tier their bid can afford
4. **Default Fallback**: 0-point bids always receive the admin-defined default information
5. **Full Payment**: Players pay exactly what they bid, regardless of tier unlocked

## Best Practices

- **Progressive Information**: Structure tiers from basic to detailed information
- **Fair Pricing**: Set tier thresholds that match information value
- **Meaningful Default**: Make the 0-point response useful but limited
- **Clear Prompts**: Help players understand they're making a blind bid

## Example

**Broker Block Configuration:**
- **Prompt**: `The merchant eyes you suspiciously. "I might have information about the missing artifact... depends on how much it's worth to you."`
- **Default Information (0 points)**: `"I don't know anything about any artifact." *The merchant looks away dismissively.*`
- **Tier 1 (10 points)**: `"Well... I did see some strangers asking about old relics yesterday. They headed toward the docks."`
- **Tier 2 (25 points)**: `"Those strangers had a map with strange symbols. One looked like a **spiral with three dots**. They mentioned something about 'the vault beneath the lighthouse.'"`
- **Tier 3 (50 points)**: `"Listen carefully - the artifact is hidden in the lighthouse basement. The combination is the **birth year of the lighthouse keeper's daughter** - check the cemetery records. But beware, others are searching too."`

**Player Experience:**
1. Player sees the merchant's suspicious greeting
2. Player enters their point bid (e.g., 15 points)
3. System determines they can afford Tier 1 (10 points required)
4. Player pays 15 points and receives Tier 1 information
5. Block completes with the information displayed

## Bidding Scenarios

**Scenario 1: Generous Bid**
- Player bids 30 points
- Can afford Tier 2 (25 points)
- Pays 30 points, receives Tier 2 information

**Scenario 2: Insufficient Bid**
- Player bids 5 points
- Cannot afford any tier (minimum is 10)
- Pays 5 points, receives default information

**Scenario 3: Conservative Bid**
- Player bids 0 points
- Pays nothing, receives default information

## Notes

- **No Completion Bonus**: Players pay exactly their bid amount
- **Information Persistence**: Once purchased, information remains visible
- **One Purchase Only**: Players cannot rebid after completing the block
- **Blind Bidding**: Players don't see tier thresholds, creating strategic tension
- **Admin Flexibility**: Create any number of tiers with custom point requirements
- **Default Safety Net**: 0-point bids ensure all players get some information