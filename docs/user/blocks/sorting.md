---
title: "Sorting Block"
sidebar: true
order: 9
---

# Sorting Block

The sorting block allows you to create a list of items that participants need to arrange in the correct order. This interactive block challenges players to think about sequence, priority, or chronology, and can award points based on different scoring schemes.

## Features

- **Drag-and-drop interface** makes it easy for participants to reorder items
- **Multiple scoring options** for different gameplay needs
- **Deterministic shuffling** ensures each player sees a consistent shuffle
- **Up/down buttons** provide alternative reordering method for accessibility

## Scoring Schemes

The sorting block offers four different scoring schemes:

1. **All or Nothing**: Players get one attempt. Full points are awarded only if all items are in the correct order.
2. **Correct Item, Correct Place**: Points are awarded for each item in the correct position.
3. **Retry Until Correct**: Players can try multiple times until they get the order completely correct.

## Example

<iframe class="w-full aspect-video" src="/static/images/docs/user/blocks/block-sorting-preview.mp4" frameborder="0" allowfullscreen></iframe>

## Best Practices

- **Keep instructions clear**: Explain exactly what order you want items to be sorted in (chronological, priority, etc.)
- **Use reasonable list lengths**: 4-8 items work well; too many items can be overwhelming
- **Consider your scoring scheme**: 
  - Use "All or Nothing" for critical sequence knowledge
  - Use "Correct Item, Correct Place" for partial credit
  - Use "Retry Until Correct" for practice exercises

## Technical Details

- Items are shuffled deterministically based on the block ID and player ID
- This ensures that each player always sees the same shuffle every time they return to this block
- The block saves the player's most recent attempt, allowing them to continue where they left off
