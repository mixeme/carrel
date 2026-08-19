// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	defaultShutdownGrace = 15 * time.Second
	healthPollInterval   = 200 * time.Millisecond
	healthStartTimeout   = 60 * time.Second
)

var ErrSidecarMissing = errors.New("desktop: sidecar binary not found")

// Supervisor starts and stops the carrel sidecar for Local mode.
type Supervisor struct {
	SidecarPath string
	DataDir     string
	Bind        string
	Grace       time.Duration
	startCmd    sidecarStartFunc
}

// RunningSidecar is a started sidecar process.
type RunningSidecar struct {
	Port int
	cmd  *exec.Cmd
}

type sidecarStartFunc func(ctx context.Context, path string, env []string, stdout, stderr io.Writer) (*exec.Cmd, error)

func defaultSidecarStart(ctx context.Context, path string, env []string, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, path)
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// Start picks a loopback port, launches the sidecar, and waits for /healthz.
func (s *Supervisor) Start(ctx context.Context) (*RunningSidecar, error) {
	if s.SidecarPath == "" {
		return nil, errors.New("desktop: sidecar path empty")
	}
	if _, err := os.Stat(s.SidecarPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSidecarMissing, s.SidecarPath)
		}
		return nil, fmt.Errorf("desktop: sidecar stat: %w", err)
	}
	bind := s.Bind
	if bind == "" {
		bind = loopbackBind
	}
	if err := os.MkdirAll(s.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("desktop: data dir: %w", err)
	}

	port, err := PickFreePort(bind)
	if err != nil {
		return nil, err
	}

	start := s.startCmd
	if start == nil {
		start = defaultSidecarStart
	}

	env := append(os.Environ(),
		"CARREL_DATA_DIR="+s.DataDir,
		"CARREL_PORT="+strconv.Itoa(port),
		"CARREL_BIND="+bind,
	)
	cmd, err := start(ctx, s.SidecarPath, env, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("desktop: start sidecar: %w", err)
	}

	healthCtx, cancel := context.WithTimeout(ctx, healthStartTimeout)
	defer cancel()
	baseURL := LocalBaseURL(port)
	if err := waitHealthy(healthCtx, baseURL); err != nil {
		_ = stopProcess(cmd, s.graceDuration())
		return nil, err
	}
	return &RunningSidecar{Port: port, cmd: cmd}, nil
}

// Stop signals the sidecar and waits up to Grace before killing it.
func (s *Supervisor) Stop(r *RunningSidecar) error {
	if r == nil || r.cmd == nil {
		return nil
	}
	stopProcess(r.cmd, s.graceDuration())
	return nil
}

func (s *Supervisor) graceDuration() time.Duration {
	if s.Grace > 0 {
		return s.Grace
	}
	return defaultShutdownGrace
}

func waitHealthy(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := stringsTrimRightSlash(baseURL) + "/healthz"
	ticker := time.NewTicker(healthPollInterval)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return fmt.Errorf("desktop: sidecar health: %w", ctx.Err())
			}
			return fmt.Errorf("desktop: sidecar health: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
