---
title: "Choice"
sidebar: true
order: 5
tag: new
---

# Choice Block

The choice block presents participants with a set of labelled options. Each option maps to a variable name you define — selecting it sets that variable to `true`, which other blocks and locations can respond to via `when` conditions.

In single-select mode participants pick exactly one option; enable **Allow multiple selections** to let them pick any number. Once confirmed the choice is final and cannot be changed.

If points are enabled, they are awarded when the choice is confirmed.

## Notes

- Variable names must be lowercase letters and underscores only, e.g. `forest_path`
- Only the chosen option's variable is written — unchosen options remain unset
- Re-submitting after completion has no effect
- The prompt field supports [Markdown](/docs/user/markdown-guide)
