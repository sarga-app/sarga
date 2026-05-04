package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

var (
	healthcheckAddr    string
	healthcheckTimeout time.Duration
)

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Probe the local /healthz endpoint and exit non-zero on failure",
	Long: `Probe the running sarga server's /healthz endpoint.

Designed for container HEALTHCHECK directives — no external dependencies (curl, wget) needed.
Exits 0 on HTTP 200, non-zero otherwise.

Probes plaintext loopback by convention. When TLS is enabled (Phase 7+), sarga will continue
to expose /healthz on the loopback interface in plaintext for probe purposes; public traffic
runs on the TLS bind.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return probe(cmd.Context(), healthcheckAddr, healthcheckTimeout)
	},
}

func init() {
	healthcheckCmd.Flags().StringVar(&healthcheckAddr, "addr", "127.0.0.1:8080",
		"host:port of the loopback /healthz endpoint")
	healthcheckCmd.Flags().DurationVar(&healthcheckTimeout, "timeout", 5*time.Second,
		"total probe timeout (covers DNS, dial, TLS, request, response)")
	rootCmd.AddCommand(healthcheckCmd)
}

func probe(parent context.Context, addr string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	u := url.URL{Scheme: "http", Host: addr, Path: "/healthz"}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "sarga-healthcheck/1")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
