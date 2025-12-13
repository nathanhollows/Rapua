---
title: "Random Clue"
sidebar: true
order: 13
---

# Random Clue Block

The random clue block displays a randomly selected clue from a list to each user. Each team consistently receives the same clue, thogh different teams will likely get different clues. This block was designed for giving players clues in a scavenger hunt or puzzle hunt, while preventing teams from sharing clues.

## How It Works

The random clue block uses a deterministic algorithm to select a clue based on the team code and block ID. This means:

- Each team will always see the same clue for a specific block
- Different teams will likely see different clues
- The selection remains consistent across multiple visits
- No points are awarded or deducted

## Preview

![](/static/images/docs/user/blocks/block-random-clue.webp)

## Best Practices

- **Difficulty**: Keep clues at similar difficulty levels since teams can't choose
- **Clarity**: Write clear, self-contained clues that work independently
- **Relevance**: Ensure all clues are helpful for the current challenge

## Notes

- Points cannot be awarded for this block
- If no clues are configured, displays "No clues available"
- If only one clue is set, all teams see the same clue
