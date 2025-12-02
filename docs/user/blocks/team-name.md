---
title: "Team Name"
sidebar: true
order: 15
tag: new
---

# Team Name Block

The team name block allows teams to set or change their team name on the lobby page. This block lets admins choose whether teams can update their name after initially setting it. The old approach was built in and always allowed changing.

**Important:** This block can only be placed on the lobby page, which is where teams wait before the experience begins.

## Options

When creating a team name block, you can configure:

- **Points:** (Optional) Number of points awarded to the team for setting their name.
- **Prompt Text:** Custom text shown above the input field (e.g., "Choose your team name!", "What shall we call your group?"). If left empty, defaults to "Set your team name!"
- **Allow changing team name after it's set:** Checkbox that controls whether teams can edit their name after initially setting it. When enabled, teams can update their name at any time. When disabled, the name becomes permanent after the first save.

## Behaviour

The team name block adapts based on the team's state:

- **Before name is set:** Shows an input field with a "Save" button
- **After name is set (allow changing enabled):** Shows the current name in an editable input field with an "Update" button
- **After name is set (allow changing disabled):** Displays the team name as read-only text

When a team saves their name, the block briefly shows a green checkmark for visual confirmation before returning to its normal state.

## Example

<iframe width="100%" class="rounded-2xl aspect-video" style="aspect-ratio: 4/3;" src="/static/images/docs/user/blocks/block-team-name-preview.mp4" frameborder="0" allowfullscreen></iframe>
