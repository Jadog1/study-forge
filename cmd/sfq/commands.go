// Package main contains all sfq CLI command definitions and the binary entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Jadog1/study-forge/internal/editor"
	"github.com/Jadog1/study-forge/internal/generator"
	"github.com/Jadog1/study-forge/internal/parser"
	"github.com/Jadog1/study-forge/internal/schema"
	"github.com/Jadog1/study-forge/internal/server"
	"github.com/Jadog1/study-forge/internal/session"
	"github.com/Jadog1/study-forge/internal/state"
	"github.com/Jadog1/study-forge/internal/tui"
	"github.com/spf13/cobra"
)

var Version = "1.0.0"

// Root returns the root cobra command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "sfq",
		Short: "StudyForge — AI-friendly CLI for generating interactive HTML quiz pages from .sfq files",
		Long: `sfq parses .sfq (StudyForge Questions) files and produces self-contained
interactive HTML quiz pages. Run 'sfq schema' to get a machine-readable
description of all commands and the .sfq syntax.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		trackCmd(),
		editCmd(),
		generateCmd(),
		openCmd(),
		exportCmd(),
		validateCmd(),
		infoCmd(),
		historyCmd(),
		resultsCmd(),
		retakeCmd(),
		schemaCmd(),
	)
	return root
}

// ── track ─────────────────────────────────────────────────────────────────────

func trackCmd() *cobra.Command {
	var noOpen bool

	cmd := &cobra.Command{
		Use:   "track <file.sfq>",
		Short: "Start a tracked quiz session (local HTTP server, saves answers and score)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrack(args[0], !noOpen)
		},
	}
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open the browser automatically")
	return cmd
}

// ── edit ─────────────────────────────────────────────────────────────────────

func editCmd() *cobra.Command {
	var (
		opsPath string
		output  string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "edit <file.sfq>",
		Short: "Apply a machine-readable JSON edit plan to a .sfq quiz",
		Long: `Apply machine-friendly JSON operations to a quiz file.
Use --ops <path.json> or --ops - to read the plan from stdin.

Supported operations:
  - set-header
  - add-question
  - replace-question
  - delete-question
  - move-question`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opsPath) == "" {
				return fmt.Errorf("--ops is required")
			}

			qf, err := parser.ParseFile(args[0])
			if err != nil {
				return fmt.Errorf("parse error: %w", err)
			}

			planBytes, err := readOpsInput(opsPath)
			if err != nil {
				return err
			}

			var plan editor.Plan
			if err := json.Unmarshal(planBytes, &plan); err != nil {
				return fmt.Errorf("invalid edit plan JSON: %w", err)
			}
			if err := editor.Apply(qf, plan); err != nil {
				return fmt.Errorf("applying edit plan: %w", err)
			}

			rendered := parser.Format(qf)
			if _, err := parser.ParseString(rendered); err != nil {
				return fmt.Errorf("edited quiz failed validation: %w", err)
			}

			if dryRun {
				fmt.Print(rendered)
				return nil
			}

			target := args[0]
			if strings.TrimSpace(output) != "" {
				target = output
			}

			if err := os.WriteFile(target, []byte(rendered), 0o644); err != nil {
				return fmt.Errorf("writing edited quiz: %w", err)
			}

			absTarget, _ := filepath.Abs(target)
			fmt.Printf("Edited quiz written: %s\n", absTarget)
			fmt.Printf("Questions: %d\n", len(qf.Questions))
			return nil
		},
	}

	cmd.Flags().StringVar(&opsPath, "ops", "", "Path to edit plan JSON file, or '-' to read from stdin")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default: overwrite input file)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print edited .sfq to stdout instead of writing a file")
	return cmd
}

// ── generate ─────────────────────────────────────────────────────────────────

func generateCmd() *cobra.Command {
	var outputPath string
	var noOpen bool

	cmd := &cobra.Command{
		Use:   "generate <file.sfq>",
		Short: "Generate a static HTML quiz page (does not save answers or score)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(args[0], outputPath, !noOpen)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output HTML file path (default: <input_basename>.html in same directory)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open the browser after generation")
	return cmd
}

// ── open ──────────────────────────────────────────────────────────────────────

func openCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open the last generated HTML file in the default browser",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := state.Load()
			if err != nil {
				return fmt.Errorf("loading state: %w", err)
			}
			if s.LastOutput == "" {
				return fmt.Errorf("no HTML file has been generated yet — run 'sfq generate <file.sfq>' first")
			}
			if _, err := os.Stat(s.LastOutput); os.IsNotExist(err) {
				return fmt.Errorf("last generated file no longer exists: %s", s.LastOutput)
			}
			fmt.Printf("Opening: %s\n", s.LastOutput)
			return server.OpenFile(s.LastOutput)
		},
	}
}

// ── export ────────────────────────────────────────────────────────────────────

func exportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export <file.sfq>",
		Short: "Generate static HTML without opening the browser (pipeline-friendly, no result tracking)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(args[0], outputPath, false)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output HTML file path")
	return cmd
}

// ── validate ──────────────────────────────────────────────────────────────────

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file.sfq>",
		Short: "Validate an .sfq file for syntax errors",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qf, err := parser.ParseFile(args[0])
			if err != nil {
				return fmt.Errorf("parse error: %w", err)
			}
			fmt.Printf("✓ Valid: %d question(s) found\n", len(qf.Questions))
			for i, q := range qf.Questions {
				fmt.Printf("  [%d] %s (%s)\n", i+1, q.Title, q.Type)
			}
			return nil
		},
	}
}

// ── info ──────────────────────────────────────────────────────────────────────

func infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Print the current state (last generated file, timestamp)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := state.Load()
			if err != nil {
				return fmt.Errorf("loading state: %w", err)
			}
			if s.LastFile == "" {
				fmt.Println("No files generated yet.")
				return nil
			}
			fmt.Printf("Last source file : %s\n", s.LastFile)
			fmt.Printf("Last HTML output : %s\n", s.LastOutput)
			fmt.Printf("Last generated   : %s\n", s.LastGeneratedAt.In(time.Local).Format(time.RFC1123))
			return nil
		},
	}
}

// ── history ───────────────────────────────────────────────────────────────────

func historyCmd() *cobra.Command {
	var (
		tags   []string
		since  string
		until  string
		limit  int
		offset int
		asJSON bool
		plain  bool
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Browse past tracked quiz sessions interactively (or use --plain/--json for pipelines)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := session.ListSessionsFilter{
				Tags:   tags,
				Limit:  limit,
				Offset: offset,
			}
			if since != "" {
				t, err := time.Parse("2006-01-02", since)
				if err != nil {
					return fmt.Errorf("invalid --since date (use YYYY-MM-DD): %w", err)
				}
				opts.Since = t
			}
			if until != "" {
				t, err := time.Parse("2006-01-02", until)
				if err != nil {
					return fmt.Errorf("invalid --until date (use YYYY-MM-DD): %w", err)
				}
				opts.Until = t.Add(24*time.Hour - time.Second)
			}

			sessions, err := session.ListSessions(opts)
			if err != nil {
				return fmt.Errorf("listing sessions: %w", err)
			}

			// ── JSON mode (agent-friendly) ──
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sessions)
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			// ── Plain mode (pipe/CI-friendly) ──
			if plain {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "SESSION ID\tQUIZ TITLE\tSTARTED\tSCORE\tTAGS")
				fmt.Fprintln(w, "──────────\t──────────\t───────\t─────\t────")
				for _, s := range sessions {
					scoreStr := "in progress"
					if s.Score != nil {
						scoreStr = fmt.Sprintf("%d%%  (%d/%d)", s.Score.Pct, s.Score.Correct, s.TotalQuestions)
					}
					tagsStr := strings.Join(s.Tags, ", ")
					if tagsStr == "" {
						tagsStr = "—"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						s.SessionID,
						s.QuizTitle,
						s.StartedAt.Local().Format("2006-01-02 15:04"),
						scoreStr,
						tagsStr,
					)
				}
				return w.Flush()
			}

			// ── TUI mode (default) ──
			retakeID, err := tui.RunHistory(sessions)
			if err != nil {
				return err
			}
			if retakeID == "" {
				return nil
			}
			// User pressed `r` — launch a fresh tracked session on the same quiz.
			dir, err := session.SessionDirByID(retakeID)
			if err != nil {
				return fmt.Errorf("resolving session: %w", err)
			}
			meta, err := session.LoadMeta(dir)
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}
			if _, statErr := os.Stat(meta.QuizFile); os.IsNotExist(statErr) {
				return fmt.Errorf("original quiz file no longer exists: %s", meta.QuizFile)
			}
			qf, err := parser.ParseFile(meta.QuizFile)
			if err != nil {
				return fmt.Errorf("parse error: %w", err)
			}
			fmt.Printf("Retaking tracked quiz: %s (%d questions)\n", qf.Title, len(qf.Questions))
			return server.Run(qf, meta.QuizFile, true)
		},
	}
	cmd.Flags().StringArrayVarP(&tags, "tag", "t", nil, "Filter by tag (repeatable: -t go -t concurrency)")
	cmd.Flags().StringVar(&since, "since", "", "Only show sessions on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Only show sessions on or before this date (YYYY-MM-DD)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of sessions to show (0 = all)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip this many sessions (for pagination)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON (for agent use)")
	cmd.Flags().BoolVar(&plain, "plain", false, "Plain text table output (no TUI, for pipes/CI)")
	return cmd
}

// ── results ───────────────────────────────────────────────────────────────────

func resultsCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "results <session-id>",
		Short: "Show detailed results for a past tracked quiz session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sid := args[0]
			dir, err := session.SessionDirByID(sid)
			if err != nil {
				return fmt.Errorf("resolving session: %w", err)
			}

			meta, err := session.LoadMeta(dir)
			if err != nil {
				return fmt.Errorf("loading session %q: %w", sid, err)
			}
			answers, err := session.LoadAnswers(dir)
			if err != nil {
				return fmt.Errorf("loading answers for %q: %w", sid, err)
			}

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"session": meta,
					"answers": answers,
				})
			}

			// Human-readable output
			fmt.Printf("\n📚 %s\n", meta.QuizTitle)
			fmt.Printf("   Session  : %s\n", meta.SessionID)
			fmt.Printf("   Started  : %s\n", meta.StartedAt.Local().Format("2006-01-02 15:04:05"))
			if meta.EndedAt != nil {
				fmt.Printf("   Ended    : %s\n", meta.EndedAt.Local().Format("2006-01-02 15:04:05"))
			}
			if meta.Score != nil {
				fmt.Printf("   Score    : %d%% — %d correct, %d partial, %d incorrect, %d skipped\n",
					meta.Score.Pct, meta.Score.Correct, meta.Score.Partial, meta.Score.Incorrect, meta.Score.Skipped)
			}
			if len(meta.Tags) > 0 {
				fmt.Printf("   Tags     : %s\n", strings.Join(meta.Tags, ", "))
			}

			if len(answers) == 0 {
				fmt.Println("\n   No answers recorded yet.")
				return nil
			}

			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  #\tRESULT\tQ-ID\tTITLE\tTYPE\tTIME\tTAGS")
			fmt.Fprintln(w, "  ─\t──────\t────\t─────\t────\t────\t────")
			for i, a := range answers {
				result := "✅"
				if !a.Correct {
					if a.PartialCredit > 0 {
						result = fmt.Sprintf("⚡%.0f%%", a.PartialCredit*100)
					} else {
						result = "❌"
					}
				}
				tagsStr := strings.Join(a.Tags, ", ")
				if tagsStr == "" {
					tagsStr = "—"
				}
				fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\t%ds\t%s\n",
					i+1, result, a.QuestionID, truncate(a.QuestionTitle, 30), a.Type, a.TimeSpentSecs, tagsStr)
			}
			w.Flush()
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON (for agent use)")
	return cmd
}

// ── retake ────────────────────────────────────────────────────────────────────

func retakeCmd() *cobra.Command {
	var noOpen bool

	cmd := &cobra.Command{
		Use:   "retake <session-id>",
		Short: "Re-run the quiz from a previous tracked session (starts a fresh tracked session)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sid := args[0]
			dir, err := session.SessionDirByID(sid)
			if err != nil {
				return fmt.Errorf("resolving session: %w", err)
			}
			meta, err := session.LoadMeta(dir)
			if err != nil {
				return fmt.Errorf("loading session %q: %w", sid, err)
			}
			if _, err := os.Stat(meta.QuizFile); os.IsNotExist(err) {
				return fmt.Errorf("original quiz file no longer exists: %s\nRun 'sfq track <path>' with the new location instead.", meta.QuizFile)
			}
			qf, err := parser.ParseFile(meta.QuizFile)
			if err != nil {
				return fmt.Errorf("parse error: %w", err)
			}
			fmt.Printf("Retaking tracked quiz: %s (%d questions)\n", qf.Title, len(qf.Questions))
			return server.Run(qf, meta.QuizFile, !noOpen)
		},
	}
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open the browser automatically")
	return cmd
}

// ── schema ────────────────────────────────────────────────────────────────────

func schemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print a machine-readable JSON schema of all commands and the .sfq syntax",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s := schema.Build(Version)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(s)
		},
	}
}

func readOpsInput(opsPath string) ([]byte, error) {
	if opsPath == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading edit plan from stdin: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(opsPath)
	if err != nil {
		return nil, fmt.Errorf("reading edit plan file: %w", err)
	}
	return data, nil
}

// ── shared ────────────────────────────────────────────────────────────────────

func runGenerate(sourcePath, outputPath string, openAfter bool) error {
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolving source path: %w", err)
	}

	fmt.Printf("Parsing: %s\n", absSource)
	qf, err := parser.ParseFile(absSource)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	fmt.Printf("  Found %d question(s)\n", len(qf.Questions))

	htmlPath, err := generator.Generate(qf, absSource, generator.Options{OutputPath: outputPath})
	if err != nil {
		return fmt.Errorf("generation error: %w", err)
	}
	fmt.Printf("Generated: %s\n", htmlPath)

	if err := state.RecordGeneration(absSource, htmlPath); err != nil {
		// Non-fatal — warn but don't fail.
		fmt.Fprintf(os.Stderr, "warning: could not save state: %v\n", err)
	}

	if openAfter {
		fmt.Println("Opening in browser...")
		return server.OpenFile(htmlPath)
	}
	return nil
}

func runTrack(sourcePath string, openBrowser bool) error {
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}
	qf, err := parser.ParseFile(absPath)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	fmt.Printf("Starting tracked quiz: %s (%d questions)\n", qf.Title, len(qf.Questions))
	return server.Run(qf, absPath, openBrowser)
}

// truncate shortens s to at most n runes, appending "…" if trimmed.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
