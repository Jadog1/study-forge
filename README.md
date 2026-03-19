# sfq — StudyForge CLI

> AI-friendly CLI tool for generating interactive HTML quiz pages from `.sfq` files.

## Installation

```bash
go build -o sfq ./cmd/sfq
```

Or install directly:

```bash
go install github.com/Jadog1/study-forge/cmd/sfq
```

## Quick Start

```bash
# Generate an HTML quiz and open it in the browser
sfq generate examples/sample.sfq

# Generate without opening
sfq export examples/sample.sfq --output my-quiz.html

# Validate a .sfq file
sfq validate examples/sample.sfq

# Open the last generated quiz
sfq open

# Show current state
sfq info

# Print the full machine-readable schema (for AI agents)
sfq schema
```

## The `.sfq` File Format

An `.sfq` file consists of an optional header and one or more question blocks separated by `---`.

### Header (Optional)

```
# My Quiz Title
author: Your Name
description: A short quiz about X.
```

### Question Block

```
---
id: q1                           # optional — auto-generated as q1, q2, ...
type: multiple-choice            # see types below; inferred if omitted
title: "Short sidebar title"     # shows in navigation sidebar
hint: "A helpful hint."          # revealed on demand
tags: [tag1, tag2]               # optional categorisation

? Your question prompt goes here.
  It can span multiple lines and supports **markdown**.

- [x] Correct option
- [ ] Wrong option A
- [ ] Wrong option B

explanation: This explanation is shown after the user answers.
  It also supports **markdown**.
---
```

### Question Types

| Type | `type:` key | Choice syntax |
|---|---|---|
| Multiple Choice | `multiple-choice` | `- [x]` correct, `- [ ]` wrong |
| Multi-Select | `multi-select` | `- [x]` correct (multiple), `- [ ]` wrong |
| True / False | `true-false` | `- [x] True` or `- [x] False` |
| True/False (Multi) | `multi-true-false` | `- [T] Statement` / `- [F] Statement` |
| Short Answer | `short-answer` | `answer: "Expected text"` |
| Ordering | `ordering` | `1. First`, `2. Second`, ... |

The type is **inferred automatically** if omitted:
- Two choices labelled "True"/"False" → `true-false`
- Multiple `[x]` choices → `multi-select`
- Any `[T]`/`[F]` markers → `multi-true-false`
- Numbered items → `ordering`
- No choices → `short-answer`
- Anything else → `multiple-choice`

## HTML Features

- **Sidebar** with all question titles for instant navigation
- **Progress bar** tracking answered questions
- **Keyboard navigation** — `←` / `→` arrows to move between questions
- **Hints** — revealed on demand
- **Per-question feedback** — correct/wrong highlighting with color-coded answers
- **Explanations** — shown after submitting
- **Score summary** at the bottom with percentage and breakdown
- **Fully self-contained** — no external dependencies, works offline
- **Dark mode** design

## AI Agent Usage

Run `sfq schema` to get a full JSON description of every command, flag, and the `.sfq` syntax:

```bash
sfq schema | jq '.commands[].name'
```

The schema output is designed to be parsed by AI agents for tool-use contexts.

## Project Structure

```
study-forge/
├── cmd/sfq/
│   ├── main.go          # Entry point
│   └── commands.go      # All CLI commands (generate, export, open, validate, info, schema)
├── internal/
│   ├── parser/          # .sfq file parser
│   ├── generator/       # HTML generator + embedded template
│   ├── state/           # Persistent state (~/.sfq/state.json)
│   └── schema/          # MCP-like introspection schema
├── examples/
│   └── sample.sfq       # Demo file covering all question types
└── go.mod
```
