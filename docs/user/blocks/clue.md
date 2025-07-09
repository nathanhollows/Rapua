---
title: "Clue Block"
sidebar: true
order: 4
---

# Clue Block

The clue block allows you to provide hints to participants that they can reveal in exchange for points. This block requires double confirmation before revealing the clue, helping prevent accidental reveals. Once revealed, the clue remains visible and the block is marked as complete.

The cost will always be shown if points are enabled. If points are not enabled, the block will still function but without point deductions.

## Best Practices

- Start with subtle hints, save direct answers for later clues
- Tailor clues to the specific challenge participants face
- Price clues appropriately - more valuable hints should cost more points

## Example

<iframe class="w-full aspect-[4/3]" src="/static/images/docs/user/blocks/block-clue-preview.mp4" frameborder="0" allowfullscreen></iframe>

**Clue Block Configuration:**
- **Points**: `-15`
- **Description Text**: `Stuck on the puzzle? This clue will reveal the pattern you need to find. **Costs 15 points.**`
- **Clue Text**: `Look for the **Fibonacci sequence** in the arrangement of objects. Start counting from the red item.`
- **Button Label**: `Get Pattern Hint`

**Player Experience:**
1. Player sees: "Stuck on the puzzle? This clue will reveal the pattern you need to find. **Costs 15 points.**"
2. Player clicks "Get Pattern Hint" 
3. Button changes to "Confirm Reveal Clue?" for 3 seconds
4. Player clicks again to confirm
5. Clue is revealed: "Look for the **Fibonacci sequence** in the arrangement of objects. Start counting from the red item."
6. Player loses 15 points and clue remains visible

## Notes

- The double-confirmation prevents accidental point spending
- Points are only deducted when the clue is successfully revealed
- Once revealed, participants cannot "un-reveal" a clue
- Multiple clue blocks can be used for progressive hint systems
- Works with or without points enabled in your experience settings
