---
title: "Roadmap"
sidebar: true
order: 10
---

# Roadmap and wishlist

The following is a list of features that I would like to add to Rapua. Some of these features are already in progress, while others are just ideas. They are not in any particular order.

If you want to [request a feature](https://github.com/nathanhollows/Rapua/issues/new?assignees=&labels=&projects=&template=feature_request.md) or want to check the progress of a feature, please check out the project on [GitHub](https://github.com/nathanhollows/Rapua/issues).

## Stage content and navigation embed

Stages (groups) become block owners with a new `stage` context. Admins click into a stage name to edit content blocks that appear around the navigation view on the player side. A new `navigation` block type acts as a positional marker. It has no fields, it just says "render the map/list here." If omitted, navigation is auto-appended at the end (current behaviour unchanged). This enables pre-navigation and post-navigation content.

Admin UI: group name becomes clickable, links to a stage block editor like a system or location page. The navigation block renders as a dashed placeholder showing the navigation mode.

## Conditional visibility

Blocks gain a `when` condition (the key is already reserved in the v7 spec). Conditions evaluate against player and game state. This enables content that adapts to the player's journey.

## Per-game theming

Each game can override DaisyUI CSS custom properties (`--p`, `--s`, `--a`, `--b1`, etc.) via a `theme` map in settings. Injected as a `<style>` block scoped to the game. Safe, validated, can't break layout. Pre-built theme presets as a starting point, with a visual picker and live preview in the admin UI.

## Content blocks

- **Map:** Mapbox integration with arbitrary markers, zooming, and coordinates.
- **Audio waveform:** A block for admins to upload audio files that users can listen to, with a waveform visualisation.

## Admin tools to help users

Admins should be able to help players who are stuck. This could include marking a block as complete or fast-tracking a team by awarding points ([#32](https://github.com/nathanhollows/Rapua/issues/32)).

Perhaps an "imitate" feature.

## Public game repository

A public repository of games that users can browse, preview, and import. The v7 JSON format and import/export system make this close to viable. Initially suggested in [this issue](https://github.com/nathanhollows/Rapua/issues/11), closer now that [Templates](/docs/user/templates) are in place ([#50](https://github.com/nathanhollows/Rapua/issues/50)).

## Team accounts

Rapua currently only supports anonymous users. Adding team accounts would let multiple users share progress and would support the public game repository.

## Reports and data export

Admin reporting: player progress summaries, activity graphs, completion rates. Data export: CSV/JSON dumps of player data for external analysis.
