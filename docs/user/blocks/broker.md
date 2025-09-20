---
title: "Broker Block"
sidebar: true
order: 3
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

Structure your tiers from basic to detailed information, ensuring that each tier provides progressively more valuable insights. This encourages players to think strategically about how much they value the information.

Set tier thresholds that match the perceived value of the information. The first tier should be accessible, while higher tiers should require more points, reflecting their increased value.

Make the 0-point response useful but limited, even if it doesn't provide actionable information. This ensures players still receive some context without incentivising 0-point bids.

Help players understand they're making a blind bid by providing clear prompts. You could lean into the [Anchoring Effect](https://en.wikipedia.org/wiki/Anchoring_effect) by giving a vague hint about the value of the information, but avoid revealing specific tier thresholds.

## Example

<iframe class="w-full aspect-square" src="/static/images/docs/user/blocks/block-broker-preview.mp4" frameborder="0" allowfullscreen></iframe>

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

- Players pay exactly their bid amount
- Once purchased, information remains visible
- Players cannot rebid after completing the block
- Players don't see tier thresholds, creating strategic tension
- Create any number of tiers with custom point requirements
- 0-point bids are always met with the defined default response
