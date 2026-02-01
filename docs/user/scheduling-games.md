---
title: "Scheduling Games"
sidebar: true
order: 12
---

# Scheduling Games

Control when your game starts and ends. You can run games manually or schedule them to start and end automatically.

![](/static/images/docs/user/activity-schedule-buttons.webp)

## Manual Start/Stop

Use the **Start** and **Stop** buttons on the Activity Tracker to control the game manually.

**Start** — Opens the game immediately. Teams can begin playing.

**Stop** — Closes the game immediately. Teams can no longer check in.

Manual buttons override any scheduled times.

---

## Scheduling

Click the **Schedule** button (calendar icon) on the Activity Tracker to set automatic start and end times.

**Scheduled Start**
- Tick the checkbox and set a date and time
- Game automatically becomes active at that time
- Teams see a countdown (configured in the [Game Status block](/docs/user/blocks/game-status-alert))

**Scheduled End**
- Tick the checkbox and set a date and time
- Game automatically closes at that time
- Teams in progress can finish their current location

You can schedule just a start time, just an end time, or both. Start time must be before end time.

---

## Game States

**Closed** — No schedule set or game has ended. Teams see a closed message and cannot start.

**Scheduled** — Start time is set but hasn't been reached. Teams see a countdown and the start button is disabled.

**Active** — Game is running. Teams can start playing.

Configure what teams see for each state in the [Game Status Alert block](/docs/user/blocks/game-status-alert).

---

**Note:** All times use your local timezone and are automatically converted.
