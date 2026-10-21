---
title: "Choice"
sidebar: true
order: 5
tag: new
---

# Choice Block

The choice block presents participants with a set of labelled options. Each option carries a variable, which is set when a participant picks it. Objectives and content blocks can be locked on that variable, so a choice is how a team takes one path rather than another (see [keys and locks](/docs/user/sections#keys-and-locks)).

In single-select mode participants pick exactly one option; enable **Allow multiple selections** to let them pick any number. Once confirmed the choice is final and cannot be changed.

If points are enabled, they are awarded when the choice is confirmed.

## Notes

- Variable names are lowercase letters, digits and underscores, not starting with a digit, e.g. `forest_path`
- Only the chosen option's variable is set; the others stay unset
- In single-select mode a team only ever sets one option, so two things locked on different options will never both open
- Re-submitting after completion has no effect
- The prompt field supports [Markdown](/docs/user/markdown-guide)
