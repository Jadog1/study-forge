package parser_test

import (
	"testing"

	"github.com/Jadog1/study-forge/internal/parser"
)

const sampleMultipleChoice = `
---
id: q1
type: multiple-choice
title: "Closest planet to the Sun"
hint: "It's the smallest planet."

? Which planet is closest to the Sun?
- [x] Mercury
- [ ] Venus
- [ ] Earth
- [ ] Mars

explanation: Mercury is the closest planet to the Sun, orbiting at an average distance of about 57.9 million km.
---
`

func TestMultipleChoice(t *testing.T) {
	qf, err := parser.ParseString(sampleMultipleChoice)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(qf.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qf.Questions))
	}
	q := qf.Questions[0]
	if q.ID != "q1" {
		t.Errorf("ID: got %q, want %q", q.ID, "q1")
	}
	if q.Type != parser.TypeMultipleChoice {
		t.Errorf("Type: got %q, want %q", q.Type, parser.TypeMultipleChoice)
	}
	if q.Title != "Closest planet to the Sun" {
		t.Errorf("Title: got %q", q.Title)
	}
	if q.Hint != "It's the smallest planet." {
		t.Errorf("Hint: got %q", q.Hint)
	}
	if len(q.Choices) != 4 {
		t.Fatalf("expected 4 choices, got %d", len(q.Choices))
	}
	if !q.Choices[0].Correct {
		t.Error("first choice (Mercury) should be correct")
	}
	for _, c := range q.Choices[1:] {
		if c.Correct {
			t.Errorf("choice %q should not be correct", c.Text)
		}
	}
	if q.Explanation == "" {
		t.Error("explanation should be non-empty")
	}
}

const sampleMultiSelect = `
---
type: multi-select
title: "Primary colors"

? Which of the following are primary colors (RYB model)?
- [x] Red
- [x] Yellow
- [x] Blue
- [ ] Green
- [ ] Orange
---
`

func TestMultiSelect(t *testing.T) {
	qf, err := parser.ParseString(sampleMultiSelect)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	q := qf.Questions[0]
	if q.Type != parser.TypeMultiSelect {
		t.Errorf("Type: got %q, want %q", q.Type, parser.TypeMultiSelect)
	}
	correctCount := 0
	for _, c := range q.Choices {
		if c.Correct {
			correctCount++
		}
	}
	if correctCount != 3 {
		t.Errorf("expected 3 correct choices, got %d", correctCount)
	}
}

const sampleTrueFalse = `
---
type: true-false
title: "Earth is flat"

? The Earth is flat.
- [ ] True
- [x] False
---
`

func TestTrueFalse(t *testing.T) {
	qf, err := parser.ParseString(sampleTrueFalse)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	q := qf.Questions[0]
	if q.Type != parser.TypeTrueFalse {
		t.Errorf("Type: got %q, want %q", q.Type, parser.TypeTrueFalse)
	}
	if len(q.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(q.Choices))
	}
	if q.Choices[0].Correct {
		t.Error("True should not be correct")
	}
	if !q.Choices[1].Correct {
		t.Error("False should be correct")
	}
}

const sampleMultiTrueFalse = `
---
type: multi-true-false
title: "Science facts"

? Classify each statement as True or False:
- [T] Water boils at 100°C at sea level.
- [F] The Sun revolves around the Earth.
- [T] Humans have 46 chromosomes.
---
`

func TestMultiTrueFalse(t *testing.T) {
	qf, err := parser.ParseString(sampleMultiTrueFalse)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	q := qf.Questions[0]
	if q.Type != parser.TypeMultiTrueFalse {
		t.Errorf("Type: got %q, want %q", q.Type, parser.TypeMultiTrueFalse)
	}
	if len(q.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(q.Choices))
	}
	trueVal := true
	falseVal := false
	expected := []*bool{&trueVal, &falseVal, &trueVal}
	for i, c := range q.Choices {
		if c.TFValue == nil {
			t.Errorf("choice %d TFValue is nil", i)
			continue
		}
		if *c.TFValue != *expected[i] {
			t.Errorf("choice %d TFValue: got %v, want %v", i, *c.TFValue, *expected[i])
		}
	}
}

const sampleShortAnswer = `
---
type: short-answer
title: "Capital of France"

? What is the capital of France?
answer: "Paris"
---
`

func TestShortAnswer(t *testing.T) {
	qf, err := parser.ParseString(sampleShortAnswer)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	q := qf.Questions[0]
	if q.Type != parser.TypeShortAnswer {
		t.Errorf("Type: got %q, want %q", q.Type, parser.TypeShortAnswer)
	}
	if q.Answer != "Paris" {
		t.Errorf("Answer: got %q, want %q", q.Answer, "Paris")
	}
}

const sampleOrdering = `
---
type: ordering
title: "Water cycle steps"

? Order the steps of the water cycle:
1. Evaporation
2. Condensation
3. Precipitation
4. Collection
---
`

func TestOrdering(t *testing.T) {
	qf, err := parser.ParseString(sampleOrdering)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	q := qf.Questions[0]
	if q.Type != parser.TypeOrdering {
		t.Errorf("Type: got %q, want %q", q.Type, parser.TypeOrdering)
	}
	if len(q.Choices) != 4 {
		t.Fatalf("expected 4 choices, got %d", len(q.Choices))
	}
	if q.Choices[0].Text != "Evaporation" {
		t.Errorf("first item: got %q, want %q", q.Choices[0].Text, "Evaporation")
	}
}

const sampleFileHeader = `# OOP Fundamentals Quiz
author: Alan Turing
description: A quiz about object-oriented programming principles.

---
id: q1

? What does OOP stand for?
- [x] Object-Oriented Programming
- [ ] Open Object Protocol
- [ ] Operational Output Protocol
---
`

func TestFileHeader(t *testing.T) {
	qf, err := parser.ParseString(sampleFileHeader)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if qf.Title != "OOP Fundamentals Quiz" {
		t.Errorf("Title: got %q", qf.Title)
	}
	if qf.Author != "Alan Turing" {
		t.Errorf("Author: got %q", qf.Author)
	}
	if len(qf.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qf.Questions))
	}
}

func TestAutoIDGeneration(t *testing.T) {
	src := "---\n? First question.\n- [x] Yes\n- [ ] No\n---\n? Second question.\n- [x] Yes\n- [ ] No\n---\n"
	qf, err := parser.ParseString(src)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(qf.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(qf.Questions))
	}
	if qf.Questions[0].ID != "q1" {
		t.Errorf("auto ID q1: got %q", qf.Questions[0].ID)
	}
	if qf.Questions[1].ID != "q2" {
		t.Errorf("auto ID q2: got %q", qf.Questions[1].ID)
	}
}

func TestMissingPromptError(t *testing.T) {
	src := "---\nid: bad\ntype: multiple-choice\n---\n"
	_, err := parser.ParseString(src)
	if err == nil {
		t.Error("expected parse error for missing prompt, got nil")
	}
}

func TestTagParsing(t *testing.T) {
	src := "---\ntags: [oop, fundamentals, design]\n\n? What is encapsulation?\n- [x] Hiding internal state\n- [ ] Being very large\n---\n"
	qf, err := parser.ParseString(src)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	q := qf.Questions[0]
	if len(q.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(q.Tags), q.Tags)
	}
	if q.Tags[0] != "oop" || q.Tags[1] != "fundamentals" || q.Tags[2] != "design" {
		t.Errorf("unexpected tags: %v", q.Tags)
	}
}
