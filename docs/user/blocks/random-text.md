---
title: "Random Text"
sidebar: true
order: 17
---

# Random Text Block

The random text block displays a randomly selected text from a list to each team. Each team consistently receives the same text, though different teams will likely get different texts. It exists so two teams doing the same objective get different wording and cannot trail each other.

## How It Works

The random text block uses a deterministic algorithm to select a text based on the team code and block ID. This means:

- Each team will always see the same text for a specific block
- Different teams will likely see different texts
- The selection remains consistent across multiple visits
- No points are awarded or deducted

## Preview

![](/static/images/docs/user/blocks/block-random-text.webp)

## Best Practices

- **Difficulty**: Keep texts at a similar difficulty level, since teams can't choose which one they get
- **Clarity**: Write clear, self-contained texts that work independently
- **Relevance**: Ensure every text is helpful for the current challenge

## Notes

- Points cannot be awarded for this block
- If no texts are configured, displays "No text available"
- If only one text is set, all teams see the same text
