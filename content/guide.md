---
title: Features & Formatting Guide
category: Documentation
published on: 2026-01-01
updated on: 2026-01-02
description: A complete guide to the shortcodes, formatting, and features available in this documentation portal.
---

# Introduction
This application supports **Github Flavored Markdown (GFM)** along with several custom extensions for technical documentation. This guide demonstrates how to use them.

---

## 1. Admonitions (Callouts)
We support GitHub-style alerts to highlight important information. These are rendered as colored boxes.

**How to write them:**
> [!NOTE]
> This is a standard note.

> [!TIP]
> This is a helpful tip or trick.

> [!IMPORTANT]
> This info is crucial for the user.

> [!WARNING]
> Be careful when performing this action.

> [!CAUTION]
> This action has negative consequences.


---

## 2. Internal Linking (Wiki Links)
Instead of writing full standard markdown links `[Link Text](/folder/file.html)`, you can use the faster "Wiki Link" syntax.

### Basic Link
Link to a file by its filename (slug).
* **Syntax:** `[[guide]]`
* **Result:** [[guide]]

### Custom Text Link
Link to a file but show different text.
* **Syntax:** `[[guide|Read the Guide]]`
* **Result:** [[guide|Read the Guide]]

> [!TIP]
> Wiki links automatically check for the generated HTML hash link. You don't need to worry about the file extension.

---

## 3. Content Transclusion (Refs)
This is a powerful feature that allows you to **embed a section from one page into another**. This is useful for repeating installation steps or API configurations without copy-pasting.

**Syntax:** `{{ref:page-slug#header-id}}`

### Example
Below this line, we are fetching the **"Admonitions"** section from *this very page* (referencing itself) to demonstrate.

**The Result:**
{{ref:formatting-the-disk#step-2-edit-etcfstab}}

> [!NOTE]
> The content above was dynamically fetched! You can reference any page in your `content` folder.

---

## 4. Code Blocks & Copying
We use the **Dracula** theme for syntax highlighting. A **Copy** button is automatically added to all code blocks.

**Go Example:**
```go
func main() {
    fmt.Println("Hello, World!")
}