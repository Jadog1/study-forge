package parser

import (
	"fmt"
	"sort"
	"strings"
)

// Format renders a QuizFile as normalized .sfq text.
func Format(qf *QuizFile) string {
	if qf == nil {
		return ""
	}

	var b strings.Builder
	if qf.Title != "" {
		fmt.Fprintf(&b, "# %s\n", qf.Title)
	}
	if qf.Author != "" {
		fmt.Fprintf(&b, "author: %s\n", qf.Author)
	}
	if qf.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", qf.Description)
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}

	for _, q := range qf.Questions {
		NormalizeQuestion(&q)
		b.WriteString("---\n")
		if q.ID != "" {
			fmt.Fprintf(&b, "id: %s\n", q.ID)
		}
		if q.Type != "" {
			fmt.Fprintf(&b, "type: %s\n", q.Type)
		}
		if q.Title != "" {
			fmt.Fprintf(&b, "title: %q\n", q.Title)
		}
		if q.Hint != "" {
			fmt.Fprintf(&b, "hint: %q\n", q.Hint)
		}
		if len(q.Tags) > 0 {
			tags := append([]string(nil), q.Tags...)
			sort.Strings(tags)
			fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
		}
		b.WriteString("\n")

		writePrompt(&b, q.Prompt)
		b.WriteString("\n")

		switch q.Type {
		case TypeOrdering:
			items := append([]Choice(nil), q.Choices...)
			sort.SliceStable(items, func(i, j int) bool {
				if items[i].OrderIndex == items[j].OrderIndex {
					return i < j
				}
				return items[i].OrderIndex < items[j].OrderIndex
			})
			for idx, c := range items {
				fmt.Fprintf(&b, "%d. %s\n", idx+1, c.Text)
			}

		case TypeMultiTrueFalse:
			for _, c := range q.Choices {
				marker := "F"
				if c.TFValue != nil && *c.TFValue {
					marker = "T"
				}
				fmt.Fprintf(&b, "- [%s] %s\n", marker, c.Text)
			}

		default:
			for _, c := range q.Choices {
				marker := " "
				if c.Correct {
					marker = "x"
				}
				fmt.Fprintf(&b, "- [%s] %s\n", marker, c.Text)
			}
		}

		if q.Answer != "" {
			fmt.Fprintf(&b, "answer: %q\n", q.Answer)
		}
		if q.Explanation != "" {
			writeExplanation(&b, q.Explanation)
		}
	}

	if len(qf.Questions) > 0 {
		b.WriteString("---\n")
	}
	return b.String()
}

func writePrompt(b *strings.Builder, prompt string) {
	lines := strings.Split(prompt, "\n")
	if len(lines) == 0 {
		b.WriteString("?\n")
		return
	}
	fmt.Fprintf(b, "? %s\n", lines[0])
	for _, ln := range lines[1:] {
		if ln == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(ln)
		b.WriteString("\n")
	}
}

func writeExplanation(b *strings.Builder, explanation string) {
	lines := strings.Split(explanation, "\n")
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "explanation: %s\n", lines[0])
	for _, ln := range lines[1:] {
		b.WriteString(ln)
		b.WriteString("\n")
	}
}
