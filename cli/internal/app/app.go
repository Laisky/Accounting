// Package app implements the Accounting command line client.
package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
)

const version = "0.1.0"

// Run dispatches command line arguments to the requested CLI command and writes output to out.
func Run(ctx context.Context, args []string, out io.Writer) error {
	if out == nil {
		return errors.WithStack(errors.New("output writer is nil"))
	}

	command := "help"
	if len(args) > 0 {
		command = strings.TrimSpace(args[0])
	}

	switch command {
	case "", "help", "--help", "-h":
		_, err := fmt.Fprint(out, usage())
		if err != nil {
			return errors.Wrap(err, "write usage")
		}
		return nil
	case "version", "--version", "-v":
		_, err := fmt.Fprintf(out, "accounting %s\n", version)
		if err != nil {
			return errors.Wrap(err, "write version")
		}
		return nil
	case "health":
		return runHealth(ctx, args[1:], out)
	default:
		return errors.Errorf("unknown command %q", command)
	}
}

// runHealth checks the backend health endpoint and writes a short status line to out.
func runHealth(ctx context.Context, args []string, out io.Writer) error {
	baseURL := "http://localhost:8080"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(args[0]), "/")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/api/health", nil)
	if err != nil {
		return errors.Wrap(err, "create health request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "call health endpoint")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("health endpoint returned %s", resp.Status)
	}

	_, err = fmt.Fprintln(out, "ok")
	if err != nil {
		return errors.Wrap(err, "write health result")
	}

	return nil
}

// usage returns the command line help text.
func usage() string {
	return `Accounting CLI

Usage:
  accounting help
  accounting version
  accounting health [base-url]
`
}
