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

**Admin Configuration:**

![Admin view of the sorting block configuration - PLACEHOLDER FOR IMAGE]()

**Player Perspective:**

![Player view of the sorting block - PLACEHOLDER FOR VIDEO]()

## Creating a Sorting Block

1. Navigate to the location where you want to add the sorting block
2. Click the "Add Block" button and select "Sorting" from the list
3. Enter instructions for your participants in the "Content" field
4. Add your items to be sorted in the correct order
5. Select your preferred scoring scheme
6. If using "Percentage of Correct Items," set the scoring percentage
7. Set the points value if points are enabled
8. Save your changes

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
