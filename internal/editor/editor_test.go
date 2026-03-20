package editor_test

import (
	"testing"

	"github.com/Jadog1/study-forge/internal/editor"
	"github.com/Jadog1/study-forge/internal/parser"
)

func TestApplySetHeaderAndReplaceQuestion(t *testing.T) {
	src := `# Quiz
author: Me

---
id: q1
? 2+2?
- [x] 4
- [ ] 5
---
`
	qf, err := parser.ParseString(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	title := "New Title"
	plan := editor.Plan{
		Operations: []editor.Operation{
			{Op: "set-header", Header: &editor.HeaderPatch{Title: &title}},
			{Op: "replace-question", Question: "q1", Data: &editor.QuestionData{
				ID:     "q1",
				Type:   "multiple-choice",
				Prompt: "Which number is even?",
				Choices: []editor.ChoiceData{
					{Text: "3", Correct: false},
					{Text: "4", Correct: true},
				},
			}},
		},
	}

	if err := editor.Apply(qf, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if qf.Title != "New Title" {
		t.Fatalf("title not updated: %q", qf.Title)
	}
	if qf.Questions[0].Prompt != "Which number is even?" {
		t.Fatalf("prompt not replaced: %q", qf.Questions[0].Prompt)
	}

	out := parser.Format(qf)
	if _, err := parser.ParseString(out); err != nil {
		t.Fatalf("formatted output should parse: %v", err)
	}
}

func TestApplyAddDeleteMoveQuestion(t *testing.T) {
	src := `---
id: q1
? First?
- [x] Yes
- [ ] No
---
---
id: q2
? Second?
- [x] Yes
- [ ] No
---
`
	qf, err := parser.ParseString(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	idx := 0
	plan := editor.Plan{
		Operations: []editor.Operation{
			{Op: "add-question", Data: &editor.QuestionData{ID: "q3", Prompt: "Third?", Choices: []editor.ChoiceData{{Text: "A", Correct: true}, {Text: "B", Correct: false}}}},
			{Op: "move-question", Question: "q3", Index: &idx},
			{Op: "delete-question", Question: "q1"},
		},
	}

	if err := editor.Apply(qf, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(qf.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(qf.Questions))
	}
	if qf.Questions[0].ID != "q3" {
		t.Fatalf("expected q3 first, got %s", qf.Questions[0].ID)
	}
	if qf.Questions[1].ID != "q2" {
		t.Fatalf("expected q2 second, got %s", qf.Questions[1].ID)
	}
}
