---
title: "Game Status"
sidebar: true
order: 18
tag: System
---

# Game Status

The game status alert block automatically displays the current game status to teams on the Start page. It shows different messages based on whether the game is scheduled, active, or closed, with an optional countdown timer.

**Important:** This block can only be placed on the Start page. It is automatically created for all new Start pages and cannot be deleted, though it can be moved and edited to customise the messages.

## Options

When configuring the game status alert block, you can customise:

- **Closed Message:** Text displayed when the game is not yet scheduled or has ended (e.g., "The game hasn't started yet", "This experience is currently closed").
- **Scheduled Message:** Text displayed when the game is scheduled but not yet active (e.g., "The game will start soon!", "Get ready to begin!").
- **Show Countdown:** Tick box to display a countdown timer when the game is scheduled. When enabled, teams see how much time remains until the game starts.

## Behaviour

The block automatically adapts based on the game instance's status:

- **Closed:** Shows the closed message in a neutral alert style
- **Scheduled:** Shows the scheduled message in an info alert style with optional countdown
- **Active:** The alert disappears completely once the game begins

The countdown timer, when enabled, displays the time remaining in a human-readable format (e.g., "2 hours 15 minutes", "30 seconds") and updates automatically.

**Note:** To schedule when your game starts and ends, see [Scheduling Games](/docs/user/scheduling-games).

## Example

<iframe width="100%" class="rounded-2xl aspect-square" style="aspect-ratio: 1/1;" src="/static/images/docs/user/blocks/block-game-status-preview.mp4" frameborder="0" allowfullscreen></iframe>
