// Package main contains all sfq CLI command definitions and the binary entrypoint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Jadog1/study-forge/internal/generator"
	"github.com/Jadog1/study-forge/internal/parser"
	"github.com/Jadog1/study-forge/internal/schema"
	"github.com/Jadog1/study-forge/internal/state"
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
		generateCmd(),
		openCmd(),
		exportCmd(),
		validateCmd(),
		infoCmd(),
		schemaCmd(),
	)
	return root
}

// ── generate ─────────────────────────────────────────────────────────────────

func generateCmd() *cobra.Command {
	var outputPath string
	var noOpen bool

	cmd := &cobra.Command{
		Use:   "generate <file.sfq>",
		Short: "Parse a .sfq file and generate an interactive HTML quiz page",
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
			return openBrowser(s.LastOutput)
		},
	}
}

// ── export ────────────────────────────────────────────────────────────────────

func exportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export <file.sfq>",
		Short: "Generate HTML from a .sfq file without opening the browser (pipeline-friendly)",
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
		return openBrowser(htmlPath)
	}
	return nil
}

// openBrowser opens the given file:// path in the OS default browser.
func openBrowser(htmlPath string) error {
	url := "file://" + filepath.ToSlash(htmlPath)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	return nil
}
