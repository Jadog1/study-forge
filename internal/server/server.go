// Package server implements the local HTTP server for sfq's interactive quiz mode.
// It serves the quiz HTML, records answers via POST /answer, and detects session
// completion either via explicit POST /finish or via heartbeat timeout (window close).
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Jadog1/study-forge/internal/generator"
	"github.com/Jadog1/study-forge/internal/parser"
	"github.com/Jadog1/study-forge/internal/session"
)

// heartbeatTimeout is how long the server waits without a heartbeat before
// treating the browser window as closed and finishing the session.
const heartbeatTimeout = 15 * time.Second

// Run starts a local HTTP quiz server, optionally opens the browser, and blocks
// until the quiz session ends (either by explicit finish or heartbeat timeout).
// It prints the server URL to stdout before opening the browser.
func Run(qf *parser.QuizFile, sfqPath string, openBrowser bool) error {
	// ── Create session record ──
	sess, err := session.Create(qf, sfqPath)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	sessDir, err := session.SessionDirByID(sess.SessionID)
	if err != nil {
		return fmt.Errorf("resolving session dir: %w", err)
	}
	fmt.Printf("Session: %s\n", sess.SessionID)

	// ── Generate in-memory HTML (server mode) ──
	htmlBytes, err := generator.GenerateBytes(qf, generator.Options{ServerMode: true})
	if err != nil {
		return fmt.Errorf("generating HTML: %w", err)
	}

	// ── Pick a free port ──
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("finding free port: %w", err)
	}
	addr := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
	fmt.Printf("sfq: quiz server running at %s\n", addr)

	// ── Shared state ──
	var (
		mu          sync.Mutex
		lastSeen    = time.Now()
		answerCount int
		finished    bool
	)

	// ── HTTP server ──
	srv := &http.Server{}
	mux := http.NewServeMux()
	srv.Handler = mux

	// Graceful shutdown helper — idempotent.
	shutdown := func() {
		mu.Lock()
		if finished {
			mu.Unlock()
			return
		}
		finished = true
		mu.Unlock()

		if err := session.Finish(sessDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not finalise session: %v\n", err)
		}
		fmt.Printf("\nSession saved: %s\n", sess.SessionID)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}

	// GET / — serve quiz HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "quiz.html", time.Now(), bytes.NewReader(htmlBytes))
	})

	// POST /answer — record one submitted answer
	mux.HandleFunc("/answer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			QuestionID    string   `json:"question_id"`
			QuestionTitle string   `json:"question_title"`
			Tags          []string `json:"tags"`
			Type          string   `json:"type"`
			Correct       bool     `json:"correct"`
			PartialCredit float64  `json:"partial_credit"`
			TimeSpentSecs int      `json:"time_spent_s"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		a := session.Answer{
			QuestionID:    payload.QuestionID,
			QuestionTitle: payload.QuestionTitle,
			Tags:          payload.Tags,
			Type:          payload.Type,
			Correct:       payload.Correct,
			PartialCredit: payload.PartialCredit,
			SubmittedAt:   time.Now().UTC(),
			TimeSpentSecs: payload.TimeSpentSecs,
		}
		if err := session.AppendAnswer(sessDir, a); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not record answer: %v\n", err)
		}

		mu.Lock()
		answerCount++
		mu.Unlock()

		w.WriteHeader(http.StatusNoContent)
	})

	// POST /finish — explicit session completion from the browser
	mux.HandleFunc("/finish", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go shutdown()
	})

	// POST /heartbeat — browser pings this every 5 s while the tab is open
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lastSeen = time.Now()
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})

	// ── Heartbeat watchdog goroutine ──
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			isFinished := finished
			elapsed := time.Since(lastSeen)
			mu.Unlock()

			if isFinished {
				return
			}
			if elapsed > heartbeatTimeout {
				log.Printf("sfq: no heartbeat for %v — treating as window close\n", elapsed.Round(time.Second))
				shutdown()
				return
			}
		}
	}()

	// ── Start listening and optionally open browser ──
	if openBrowser {
		go func() {
			time.Sleep(150 * time.Millisecond) // tiny delay so server is ready
			_ = openURL(addr)
		}()
	}

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// openURL opens the given URL in the OS default browser.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// openFile opens a local file path in the OS default browser (kept for generate/export).
func OpenFile(htmlPath string) error {
	url := "file://" + filepath.ToSlash(htmlPath)
	return openURL(url)
}
