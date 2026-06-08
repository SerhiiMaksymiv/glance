package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "gl",
}

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start monitoring",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Graceful shutdown
		sigChan := make(chan os.Signal, 1)

		signal.Notify(
			sigChan,
			syscall.SIGINT,
			syscall.SIGTERM,
		)

		sta, err := Load("monitors.json")
		if err != nil {
			fmt.Println(err)
			return
		}

		startWithContext(ctx, sta)
		<-sigChan
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type Monitor struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	URL       string        `json:"url"`
	Method    string        `json:"method"`
	Interval  time.Duration `json:"interval"`
	Enabled   bool          `json:"enabled"`
	Condition Condition     `json:"condition"`
}

type Condition struct {
	Value string `json:"value"`
}

func fetchURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func evaluate(body []byte, expected string) bool {
	return strings.Contains(string(body), expected)
}

func ExecuteEvaluationWithContext(ctx context.Context, m Monitor) {
	ticker := time.NewTicker(m.Interval * time.Second)
	defer ticker.Stop()

	for {

		body, err := fetchURL(ctx, m.URL)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if !evaluate(body, m.Condition.Value) {
			fmt.Println(m.URL)
			fmt.Println(string(body))
			fmt.Println("============")
		}

		<-ticker.C
	}
}

func Load(path string) ([]Monitor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var monitors []Monitor
	err = json.Unmarshal(data, &monitors)
	return monitors, err
}

func Save(path string, monitors []Monitor) error {
	data, err := json.MarshalIndent(monitors, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func startWithContext(ctx context.Context, monitors []Monitor) {
	for _, m := range monitors {
		go ExecuteEvaluationWithContext(ctx, m)
	}
}
