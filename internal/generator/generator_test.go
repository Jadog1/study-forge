package generator

import (
	"testing"

	"github.com/Jadog1/study-forge/internal/parser"
)

func TestBuildTemplateDataMultiTrueFalseFallsBackToCorrectFlags(t *testing.T) {
	qf := &parser.QuizFile{
		Questions: []parser.Question{{
			ID:     "q1",
			Type:   parser.TypeMultiTrueFalse,
			Prompt: "Classify each statement.",
			Choices: []parser.Choice{
				{Text: "Statement A", Correct: true},
				{Text: "Statement B", Correct: false},
			},
		}},
	}

	data, err := buildTemplateData(qf, false)
	if err != nil {
		t.Fatalf("buildTemplateData: %v", err)
	}
	if len(data.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(data.Questions))
	}
	q := data.Questions[0]
	if len(q.Choices) != 2 {
		t.Fatalf("expected 2 choices, got %d", len(q.Choices))
	}
	if q.Choices[0].TFCorrect != "true" {
		t.Fatalf("first choice TFCorrect: got %q, want %q", q.Choices[0].TFCorrect, "true")
	}
	if q.Choices[1].TFCorrect != "false" {
		t.Fatalf("second choice TFCorrect: got %q, want %q", q.Choices[1].TFCorrect, "false")
	}
}
