---
title: "Writing with Markdown"
sidebar: true
order: 7
---

# Writing with Markdown

*Markdown* is a lightweight markup language that turns plain text into formatted text.

The two main places where you'll use Markdown are in **Location Clues** and **Content Blocks**. This lets you format your text in a way that's easy to read and write.

## Headers

To create a heading, add one to six `#` symbols before your heading text. The number of `#` you use will determine the hierarchy level and typeface size of the heading.

```
 # A first level header
 ## A second level header
 ### A third level header
 #### And so on...
```

## Styling text

You can indicate emphasis with bold, italic, or strikethrough text.

| Style       | Syntax         | Output         |
|:------------|:---------------|:---------------|
| Italic      | `*italic*`     | *italic*       |
| Bold        | `**bold**`     | **bold**       |
| Strikethrough | `~~strikethrough~~` | ~~strikethrough~~ |
| Bold and italic | `***bold and italic***` | ***bold and italic*** |
| Bold and nested italic | `**bold and *italic* nested**` | **bold and *italic* nested** |

## Paragraphs

To create a new paragraph, leave a blank line between lines of text.

## Quoting text

You can quote text with a `>`.

```
> This is a quote.
```

> This is a quote.

## Links

You can create an inline link by wrapping link text in brackets `[ ]`, followed by the URL in parentheses `( )`.

```
You can find more information on [Markdown](https://www.markdownguide.org).
```

You can find more information on [Markdown](https://www.markdownguide.org).

## Images

Images are similar to links, but they have an exclamation mark in front:

```
![Screenshots of Rapua on mobile](/static/images/s2.webp)
```

![Screenshots of Rapua on mobile](/static/images/s2.webp)

An easier way to include images is to use the [image block](/docs/user/blocks/image).

## Lists

You can create ordered and unordered lists by typing the list items on separate lines and starting the line with a `*`, `-`, or `1.`.

Creating a bullet list is simple. Just use `*` or `-`:

```
* fruits
    * pears
    * peaches
* vegetables
    * broccoli
```

-   fruits
    -   pears
    -   peaches
-   vegetables
    -   broccoli

To create a numbered list, use `1.`:

```
1. Item 1
2. Item 2
3. Item 3
1. Item 4
```

1.  Item 1
2.  Item 2
3.  Item 3
4.  Item 4

For interactive checklists, use the [checklist block](/docs/user/blocks/checklist).

## Horizontal Rules

Horizontal rules are easily created by putting three or more `***` or `---` on a line:

```
---
***
```

* * * * *

## Hiding text with comments

You can hide text by enclosing it in an HTML comment. This is useful for notes or reminders that you want to keep in the admin panel but not display to players.

```
<!-- This text will be hidden in the rendered output -->
```

