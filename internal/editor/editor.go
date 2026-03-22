// Package editor applies machine-friendly JSON edit plans to parsed .sfq quizzes.
package editor

import (
	"fmt"
	"strings"

	"github.com/Jadog1/study-forge/internal/parser"
)

// Plan is a sequence of operations to apply to a quiz file.
type Plan struct {
	Operations []Operation `json:"operations"`
}

// Operation describes one mutation against a quiz file.
type Operation struct {
	Op       string        `json:"op"`
	Question string        `json:"question,omitempty"`
	Header   *HeaderPatch  `json:"header,omitempty"`
	Data     *QuestionData `json:"data,omitempty"`
	Index    *int          `json:"index,omitempty"`
}

// HeaderPatch changes quiz-level metadata.
type HeaderPatch struct {
	Title       *string `json:"title,omitempty"`
	Author      *string `json:"author,omitempty"`
	Description *string `json:"description,omitempty"`
}

// QuestionData is the machine-editable question payload.
type QuestionData struct {
	ID          string       `json:"id,omitempty"`
	Type        string       `json:"type,omitempty"`
	Title       string       `json:"title,omitempty"`
	Prompt      string       `json:"prompt,omitempty"`
	Hint        string       `json:"hint,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Choices     []ChoiceData `json:"choices,omitempty"`
	Answer      string       `json:"answer,omitempty"`
	Explanation string       `json:"explanation,omitempty"`
}

// ChoiceData maps to parser.Choice in JSON plans.
type ChoiceData struct {
	Text    string `json:"text"`
	Correct bool   `json:"correct,omitempty"`
	TF      string `json:"tf,omitempty"` // "T" or "F" for multi-true-false
	Order   int    `json:"order,omitempty"`
}

// Apply executes all operations in order.
func Apply(qf *parser.QuizFile, plan Plan) error {
	if qf == nil {
		return fmt.Errorf("quiz file is nil")
	}
	if len(plan.Operations) == 0 {
		return fmt.Errorf("edit plan has no operations")
	}

	for i, op := range plan.Operations {
		if err := applyOp(qf, op); err != nil {
			return fmt.Errorf("operation %d (%s): %w", i+1, op.Op, err)
		}
	}

	if err := validateUniqueIDs(qf); err != nil {
		return err
	}
	return nil
}

func applyOp(qf *parser.QuizFile, op Operation) error {
	switch op.Op {
	case "set-header":
		if op.Header == nil {
			return fmt.Errorf("set-header requires header")
		}
		if op.Header.Title != nil {
			qf.Title = *op.Header.Title
		}
		if op.Header.Author != nil {
			qf.Author = *op.Header.Author
		}
		if op.Header.Description != nil {
			qf.Description = *op.Header.Description
		}
		return nil

	case "add-question":
		if op.Data == nil {
			return fmt.Errorf("add-question requires data")
		}
		q, err := toQuestion(*op.Data)
		if err != nil {
			return err
		}
		if q.ID == "" {
			q.ID = fmt.Sprintf("q%d", len(qf.Questions)+1)
		}
		if indexByID(qf.Questions, q.ID) >= 0 {
			return fmt.Errorf("question id %q already exists", q.ID)
		}
		if op.Index != nil {
			if *op.Index < 0 || *op.Index > len(qf.Questions) {
				return fmt.Errorf("index out of range: %d", *op.Index)
			}
			qf.Questions = insertQuestion(qf.Questions, *op.Index, q)
			return nil
		}
		qf.Questions = append(qf.Questions, q)
		return nil

	case "replace-question":
		if op.Question == "" {
			return fmt.Errorf("replace-question requires question id in question")
		}
		if op.Data == nil {
			return fmt.Errorf("replace-question requires data")
		}
		idx := indexByID(qf.Questions, op.Question)
		if idx < 0 {
			return fmt.Errorf("question id %q not found", op.Question)
		}
		q, err := toQuestion(*op.Data)
		if err != nil {
			return err
		}
		if q.ID == "" {
			q.ID = op.Question
		}
		if q.ID != op.Question {
			other := indexByID(qf.Questions, q.ID)
			if other >= 0 && other != idx {
				return fmt.Errorf("question id %q already exists", q.ID)
			}
		}
		qf.Questions[idx] = q
		return nil

	case "delete-question":
		if op.Question == "" {
			return fmt.Errorf("delete-question requires question id in question")
		}
		idx := indexByID(qf.Questions, op.Question)
		if idx < 0 {
			return fmt.Errorf("question id %q not found", op.Question)
		}
		qf.Questions = append(qf.Questions[:idx], qf.Questions[idx+1:]...)
		return nil

	case "move-question":
		if op.Question == "" {
			return fmt.Errorf("move-question requires question id in question")
		}
		if op.Index == nil {
			return fmt.Errorf("move-question requires index")
		}
		idx := indexByID(qf.Questions, op.Question)
		if idx < 0 {
			return fmt.Errorf("question id %q not found", op.Question)
		}
		to := *op.Index
		if to < 0 || to >= len(qf.Questions) {
			return fmt.Errorf("index out of range: %d", to)
		}
		if idx == to {
			return nil
		}
		item := qf.Questions[idx]
		qf.Questions = append(qf.Questions[:idx], qf.Questions[idx+1:]...)
		qf.Questions = insertQuestion(qf.Questions, to, item)
		return nil
	}

	return fmt.Errorf("unknown op %q", op.Op)
}

func toQuestion(d QuestionData) (parser.Question, error) {
	q := parser.Question{
		ID:          strings.TrimSpace(d.ID),
		Type:        parser.NormalizeQuestionType(parser.QuestionType(strings.TrimSpace(d.Type))),
		Title:       d.Title,
		Prompt:      d.Prompt,
		Hint:        d.Hint,
		Tags:        d.Tags,
		Answer:      d.Answer,
		Explanation: d.Explanation,
	}
	if strings.TrimSpace(q.Prompt) == "" {
		return q, fmt.Errorf("question prompt is required")
	}
	for _, c := range d.Choices {
		pc := parser.Choice{Text: c.Text, Correct: c.Correct, OrderIndex: c.Order}
		switch strings.ToUpper(strings.TrimSpace(c.TF)) {
		case "":
		case "T":
			v := true
			pc.TFValue = &v
		case "F":
			v := false
			pc.TFValue = &v
		default:
			return q, fmt.Errorf("invalid choice tf value %q (expected T/F)", c.TF)
		}
		q.Choices = append(q.Choices, pc)
	}
	if q.Type == "" {
		q.Type = inferTypeFromQuestion(q)
	}
	parser.NormalizeQuestion(&q)
	return q, nil
}

func inferTypeFromQuestion(q parser.Question) parser.QuestionType {
	if len(q.Choices) == 0 {
		return parser.TypeShortAnswer
	}

	hasTF := false
	allOrdered := len(q.Choices) > 1
	correctCount := 0
	for _, c := range q.Choices {
		if c.TFValue != nil {
			hasTF = true
		}
		if c.OrderIndex == 0 {
			allOrdered = false
		}
		if c.Correct {
			correctCount++
		}
	}
	if hasTF {
		return parser.TypeMultiTrueFalse
	}
	if allOrdered {
		return parser.TypeOrdering
	}
	if len(q.Choices) == 2 {
		a := strings.ToLower(strings.TrimSpace(q.Choices[0].Text))
		b := strings.ToLower(strings.TrimSpace(q.Choices[1].Text))
		if (a == "true" || a == "false") && (b == "true" || b == "false") {
			return parser.TypeTrueFalse
		}
	}
	if correctCount > 1 {
		return parser.TypeMultiSelect
	}
	return parser.TypeMultipleChoice
}

func validateUniqueIDs(qf *parser.QuizFile) error {
	seen := map[string]struct{}{}
	for i := range qf.Questions {
		id := strings.TrimSpace(qf.Questions[i].ID)
		if id == "" {
			qf.Questions[i].ID = fmt.Sprintf("q%d", i+1)
			id = qf.Questions[i].ID
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate question id %q", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func indexByID(questions []parser.Question, id string) int {
	for i, q := range questions {
		if q.ID == id {
			return i
		}
	}
	return -1
}

func insertQuestion(qs []parser.Question, idx int, q parser.Question) []parser.Question {
	qs = append(qs, parser.Question{})
	copy(qs[idx+1:], qs[idx:])
	qs[idx] = q
	return qs
}
