package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	constants "catops/config"
	"catops/internal/config"
	"catops/internal/metrics"
	"catops/internal/ui"
)

// NewAskCmd creates the AI assistant command
func NewAskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask [question]",
		Short: "Ask AI assistant about your server",
		Long: `Ask AI assistant questions about your server metrics, alerts, and operations.

The AI assistant analyzes your current server state and provides intelligent
answers based on real-time metrics and historical data.

Examples:
  catops ask "Why is my CPU usage high?"
  catops ask "What's causing memory spikes?"
  catops ask "Should I be worried about disk usage?"
  catops ask "Explain the recent alerts"

Features:
  • FREE - No subscription required
  • Context-aware - Analyzes current metrics
  • Fast responses - Optimized for CLI
  • Privacy-first - Only sends metrics, not logs`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			question := strings.Join(args, " ")
			runAsk(question)
		},
	}
}

type AskRequest struct {
	Question  string                 `json:"question"`
	Context   map[string]interface{} `json:"context"`
	ServerID  string                 `json:"server_id,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

type AskResponse struct {
	Success bool   `json:"success"`
	Answer  string `json:"answer"`
	Error   string `json:"error,omitempty"`
}

func runAsk(question string) {
	ui.PrintSection("AI Assistant")
	fmt.Println()
	fmt.Printf("  Q: %s\n", question)
	fmt.Println()

	// Gather context (current metrics)
	currentMetrics, err := metrics.GetMetricsWithCache()
	if err != nil {
		ui.PrintStatus("warning", "Could not gather metrics, answering without context")
		currentMetrics = nil
	}

	// Build context
	context := make(map[string]interface{})
	if currentMetrics != nil {
		context["cpu_usage"] = currentMetrics.CPUUsage
		context["memory_usage"] = currentMetrics.MemoryUsage
		context["disk_usage"] = currentMetrics.DiskUsage
		context["uptime"] = currentMetrics.Uptime
		context["os"] = currentMetrics.OSName

		// Add top processes if available
		if len(currentMetrics.TopProcesses) > 0 {
			topProcs := make([]map[string]interface{}, 0, 5)
			for i, proc := range currentMetrics.TopProcesses {
				if i >= 5 {
					break
				}
				topProcs = append(topProcs, map[string]interface{}{
					"name":       proc.Name,
					"cpu_usage":  proc.CPUUsage,
					"memory_kb":  proc.MemoryKB,
				})
			}
			context["top_processes"] = topProcs
		}
	}

	// Load config for server_id
	cfg, _ := config.LoadConfig()
	serverID := ""
	if cfg != nil && cfg.ServerID != "" {
		serverID = cfg.ServerID
	}

	// Build request
	payload := AskRequest{
		Question:  question,
		Context:   context,
		ServerID:  serverID,
		Timestamp: time.Now().Unix(),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		ui.PrintStatus("error", "Failed to prepare request")
		showOfflineHelp(question)
		return
	}

	// Send to backend
	url := constants.CATOPS_WEBSITE + "/api/ai/ask-cli"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		showOfflineHelp(question)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", constants.HEADER_USER_AGENT)

	// Show loading indicator
	fmt.Print("  ")
	ui.PrintStatus("info", "Thinking...")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println()
		showOfflineHelp(question)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println()
		ui.PrintStatus("error", "Failed to read response")
		showOfflineHelp(question)
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Println()
		ui.PrintStatus("error", "AI service unavailable")
		showOfflineHelp(question)
		return
	}

	// Parse response
	var aiResp AskResponse
	if err := json.Unmarshal(body, &aiResp); err != nil {
		fmt.Println()
		ui.PrintStatus("error", "Invalid response from AI")
		showOfflineHelp(question)
		return
	}

	if !aiResp.Success {
		fmt.Println()
		ui.PrintStatus("error", aiResp.Error)
		showOfflineHelp(question)
		return
	}

	// Display answer
	fmt.Println()
	ui.PrintStatus("success", "Answer:")
	fmt.Println()

	// Format answer with word wrapping
	displayWrappedText(aiResp.Answer, 70)

	fmt.Println()
	ui.PrintSectionEnd()
}

func showOfflineHelp(question string) {
	fmt.Println()
	ui.PrintStatus("info", "AI assistant is currently unavailable")
	fmt.Println()
	fmt.Println("  Try these commands to diagnose issues:")
	fmt.Println()

	// Suggest relevant commands based on question keywords
	questionLower := strings.ToLower(question)

	if strings.Contains(questionLower, "cpu") || strings.Contains(questionLower, "high") {
		fmt.Println("  • catops status        - Check current CPU usage")
		fmt.Println("  • catops processes     - See top CPU-consuming processes")
	} else if strings.Contains(questionLower, "memory") || strings.Contains(questionLower, "ram") {
		fmt.Println("  • catops status        - Check current memory usage")
		fmt.Println("  • catops processes -m  - See top memory-consuming processes")
	} else if strings.Contains(questionLower, "disk") || strings.Contains(questionLower, "storage") {
		fmt.Println("  • catops status        - Check disk usage")
		fmt.Println("  • df -h                - Show disk space details")
	} else if strings.Contains(questionLower, "alert") {
		fmt.Println("  • catops status        - Check daemon status")
		fmt.Println("  • cat /tmp/catops.log  - View daemon logs")
	} else {
		fmt.Println("  • catops status        - Check system overview")
		fmt.Println("  • catops processes     - View running processes")
		fmt.Println("  • cat /tmp/catops.log  - Check daemon logs")
	}

	fmt.Println()
	fmt.Println("  For help: https://docs.catops.io")
	fmt.Println()
}

func displayWrappedText(text string, maxWidth int) {
	// Split by paragraphs
	paragraphs := strings.Split(text, "\n\n")

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		// Check if it's a bullet point or numbered list
		if strings.HasPrefix(paragraph, "•") || strings.HasPrefix(paragraph, "-") || strings.HasPrefix(paragraph, "*") {
			fmt.Printf("  %s\n", paragraph)
			continue
		}

		// Check for numbered items (1., 2., etc.)
		if len(paragraph) > 2 && paragraph[1] == '.' && paragraph[0] >= '0' && paragraph[0] <= '9' {
			fmt.Printf("  %s\n", paragraph)
			continue
		}

		// Word wrap for regular paragraphs
		words := strings.Fields(paragraph)
		line := "  "

		for _, word := range words {
			if len(line)+len(word)+1 > maxWidth+2 {
				fmt.Println(line)
				line = "  " + word
			} else {
				if line == "  " {
					line += word
				} else {
					line += " " + word
				}
			}
		}

		if line != "  " {
			fmt.Println(line)
		}

		fmt.Println() // Paragraph spacing
	}
}
