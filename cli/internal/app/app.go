// Package app implements the Accounting command line client.
package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
)

const (
	version               = "0.1.0"
	defaultBaseURL        = "http://localhost:8080"
	defaultSessionCookie  = "ACCOUNTING_SESSION_COOKIE"
	maxPreviewUploadBytes = 5 * 1024 * 1024
)

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
	case "wacai-preview":
		return runWacaiPreview(ctx, args[1:], out)
	default:
		return errors.Errorf("unknown command %q", command)
	}
}

// runHealth checks the backend health endpoint and writes a short status line to out.
func runHealth(ctx context.Context, args []string, out io.Writer) error {
	baseURL := defaultBaseURL
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		baseURL = strings.TrimRight(strings.TrimSpace(args[0]), "/")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/api/v1/health", nil)
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

// runWacaiPreview receives CLI args, uploads a bounded Wacai CSV preview request, and writes JSON to out.
func runWacaiPreview(ctx context.Context, args []string, out io.Writer) error {
	options, err := parseWacaiPreviewOptions(args)
	if err != nil {
		return err
	}

	body, contentType, err := buildPreviewMultipart(options.file)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, options.baseURL+"/api/v1/imports/wacai/preview", body)
	if err != nil {
		return errors.Wrap(err, "create wacai preview request")
	}
	req.Header.Set("Content-Type", contentType)
	if options.sessionCookie != "" {
		req.Header.Set("Cookie", options.sessionCookie)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "call wacai preview endpoint")
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return errors.Wrap(readErr, "read failed wacai preview response")
		}
		return errors.Errorf("wacai preview endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return errors.Wrap(err, "write wacai preview response")
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return errors.Wrap(err, "write wacai preview newline")
	}

	return nil
}

type wacaiPreviewOptions struct {
	baseURL       string
	file          string
	sessionCookie string
	timeout       time.Duration
}

// parseWacaiPreviewOptions receives command args and returns validated Wacai preview options.
func parseWacaiPreviewOptions(args []string) (wacaiPreviewOptions, error) {
	options := wacaiPreviewOptions{
		baseURL:       defaultBaseURL,
		sessionCookie: strings.TrimSpace(os.Getenv(defaultSessionCookie)),
		timeout:       30 * time.Second,
	}

	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--base-url":
			index++
			if index >= len(args) {
				return wacaiPreviewOptions{}, errors.WithStack(errors.New("--base-url requires a value"))
			}
			options.baseURL = strings.TrimRight(strings.TrimSpace(args[index]), "/")
		case "--file":
			index++
			if index >= len(args) {
				return wacaiPreviewOptions{}, errors.WithStack(errors.New("--file requires a value"))
			}
			options.file = strings.TrimSpace(args[index])
		case "--session-cookie-env":
			index++
			if index >= len(args) {
				return wacaiPreviewOptions{}, errors.WithStack(errors.New("--session-cookie-env requires a value"))
			}
			envName := strings.TrimSpace(args[index])
			if envName == "" {
				return wacaiPreviewOptions{}, errors.WithStack(errors.New("--session-cookie-env cannot be empty"))
			}
			options.sessionCookie = strings.TrimSpace(os.Getenv(envName))
		case "--timeout":
			index++
			if index >= len(args) {
				return wacaiPreviewOptions{}, errors.WithStack(errors.New("--timeout requires a value"))
			}
			timeout, err := time.ParseDuration(strings.TrimSpace(args[index]))
			if err != nil {
				return wacaiPreviewOptions{}, errors.Wrap(err, "parse --timeout")
			}
			options.timeout = timeout
		default:
			return wacaiPreviewOptions{}, errors.WithStack(errors.Errorf("unknown wacai-preview option %q", args[index]))
		}
	}

	if options.baseURL == "" {
		return wacaiPreviewOptions{}, errors.WithStack(errors.New("--base-url cannot be empty"))
	}
	if options.file == "" {
		return wacaiPreviewOptions{}, errors.WithStack(errors.New("--file is required"))
	}
	if options.timeout <= 0 {
		return wacaiPreviewOptions{}, errors.WithStack(errors.New("--timeout must be positive"))
	}

	return options, nil
}

// buildPreviewMultipart receives a CSV path and returns a bounded multipart request body.
func buildPreviewMultipart(filename string) (*bytes.Reader, string, error) {
	cleaned := filepath.Clean(filename)
	stat, err := os.Stat(cleaned)
	if err != nil {
		return nil, "", errors.Wrap(err, "stat preview file")
	}
	if stat.IsDir() {
		return nil, "", errors.WithStack(errors.New("preview file cannot be a directory"))
	}
	if stat.Size() <= 0 {
		return nil, "", errors.WithStack(errors.New("preview file cannot be empty"))
	}
	if stat.Size() > maxPreviewUploadBytes {
		return nil, "", errors.WithStack(errors.Errorf("preview file exceeds %d bytes", maxPreviewUploadBytes))
	}

	data, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, "", errors.Wrap(err, "read preview file")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(cleaned))
	if err != nil {
		return nil, "", errors.Wrap(err, "create preview file part")
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", errors.Wrap(err, "write preview file part")
	}
	if err := writer.Close(); err != nil {
		return nil, "", errors.Wrap(err, "close preview multipart writer")
	}

	return bytes.NewReader(body.Bytes()), writer.FormDataContentType(), nil
}

// usage returns the command line help text.
func usage() string {
	return `Accounting CLI

Usage:
  accounting help
  accounting version
  accounting health [base-url]
  accounting wacai-preview --file export.csv [--base-url http://localhost:8080] [--session-cookie-env ACCOUNTING_SESSION_COOKIE] [--timeout 30s]
`
}
