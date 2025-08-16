package ui

import (
	"fmt"
	"moniq/internal/metrics"
	"sort"
	"strings"
)

// Color constants
const (
	RED    = "\033[0;31m"
	GREEN  = "\033[0;32m"
	YELLOW = "\033[1;33m"
	BLUE   = "\033[0;34m"
	CYAN   = "\033[0;36m"
	WHITE  = "\033[1;37m"
	GRAY   = "\033[0;37m"
	NC     = "\033[0m" // No Color
)

// PrintHeader prints the application header
func PrintHeader() {
	fmt.Printf("%s███╗   ███╗ ██████╗ ███╗   ██╗██╗ ██████╗    ███████╗██╗  ██╗%s\n", CYAN, NC)
	fmt.Printf("%s████╗ ████║██╔═══██╗████╗  ██║██║██╔═══██╗   ██╔════╝██║  ██║%s\n", CYAN, NC)
	fmt.Printf("%s██╔████╔██║██║   ██║██╔██╗ ██║██║██║   ██║   ███████╗███████║%s\n", CYAN, NC)
	fmt.Printf("%s██║╚██╔╝██║██║   ██║██║╚██╗██║██║██║▄▄ ██║   ╚════██║██╔══██║%s\n", CYAN, NC)
	fmt.Printf("%s██║ ╚═╝ ██║╚██████╔╝██║ ╚████║██║╚██████╔╝██╗███████║██║  ██║%s\n", CYAN, NC)
	fmt.Printf("%s╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝ ╚══▀▀═╝ ╚═╝╚══════╝╚═╝  ╚═╝%s\n", CYAN, NC)
	fmt.Printf("%s                    Server Monitor%s\n", WHITE, NC)
}

// PrintSection prints a section header
func PrintSection(title string) {
	titleWidth := len(title) + 4 // 4 for "┌─ " and " ─"
	totalWidth := 60             // Fixed total width
	dashCount := totalWidth - titleWidth
	if dashCount < 0 {
		dashCount = 0
	}
	fmt.Printf("%s┌─ %s%s%s ─%s┐%s\n",
		CYAN,
		WHITE, title, CYAN,
		strings.Repeat("─", dashCount),
		NC)
}

// PrintSectionEnd prints a section footer
func PrintSectionEnd() {
	totalWidth := 60 // Same fixed total width as PrintSection
	fmt.Printf("%s└%s┘%s\n", CYAN, strings.Repeat("─", totalWidth), NC)
}

// PrintStatus prints a status message
func PrintStatus(status, message string) {
	switch status {
	case "success":
		fmt.Printf("  %s✓%s %s\n", GREEN, NC, message)
	case "warning":
		fmt.Printf("  %s⚠%s %s\n", YELLOW, NC, message)
	case "error":
		fmt.Printf("  %s✗%s %s\n", RED, NC, message)
	case "info":
		fmt.Printf("  %sℹ%s %s\n", BLUE, NC, message)
	}
}

// CreateTable creates a formatted table
func CreateTable(data map[string]string) string {
	var result strings.Builder
	var items []struct {
		key   string
		value string
	}

	// Convert map to slice for sorting
	for key, value := range data {
		items = append(items, struct {
			key   string
			value string
		}{key, value})
	}

	// Sort by key
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})

	// Find max key length
	maxKeyLen := 0
	for _, item := range items {
		if len(item.key) > maxKeyLen {
			maxKeyLen = len(item.key)
		}
	}

	// Find max value length
	maxValueLen := 0
	for _, item := range items {
		if len(item.value) > maxValueLen {
			maxValueLen = len(item.value)
		}
	}

	// Limit lengths to prevent overflow
	if maxKeyLen > 30 {
		maxKeyLen = 30
	}
	if maxValueLen > 40 {
		maxValueLen = 40
	}

	// Build table
	for _, item := range items {
		displayKey := item.key
		displayValue := item.value

		// Truncate if too long
		if len(displayKey) > maxKeyLen {
			displayKey = displayKey[:maxKeyLen-5] + "..."
		}
		if len(displayValue) > maxValueLen {
			displayValue = displayValue[:maxValueLen-5] + "..."
		}

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s%s", CYAN, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%-*s", maxKeyLen, displayKey))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, ":", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%-*s", maxValueLen, displayValue))
		result.WriteString("\n")
	}

	return result.String()
}

// CreateFixedTable creates a fixed-width table
func CreateFixedTable(data map[string]string) string {
	var result strings.Builder
	var items []struct {
		key   string
		value string
	}

	// Convert map to slice for sorting
	for key, value := range data {
		items = append(items, struct {
			key   string
			value string
		}{key, value})
	}

	// Sort by key
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})

	// Fixed widths
	keyWidth := 20
	valueWidth := 35

	// Build table
	for _, item := range items {
		displayKey := item.key
		displayValue := item.value

		// Truncate if too long
		if len(displayKey) > keyWidth-4 {
			displayKey = displayKey[:keyWidth-4] + "..."
		}
		if len(displayValue) > valueWidth-4 {
			displayValue = displayValue[:valueWidth-4] + "..."
		}

		paddedKey := fmt.Sprintf("%-*s", keyWidth, displayKey)
		paddedValue := fmt.Sprintf("%-*s", valueWidth, displayValue)

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s%s", CYAN, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, paddedKey, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, ":", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, paddedValue, NC))
		result.WriteString("\n")
	}

	return result.String()
}

// CreatePerfectTable creates a perfectly aligned table
func CreatePerfectTable(data map[string]string) string {
	var result strings.Builder
	var items []struct {
		key   string
		value string
	}

	// Convert map to slice for sorting
	for key, value := range data {
		items = append(items, struct {
			key   string
			value string
		}{key, value})
	}

	// Sort by key
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})

	// Find max key length
	maxKeyLen := 0
	for _, item := range items {
		if len(item.key) > maxKeyLen {
			maxKeyLen = len(item.key)
		}
	}

	// Find max value length
	maxValueLen := 0
	for _, item := range items {
		if len(item.value) > maxValueLen {
			maxValueLen = len(item.value)
		}
	}

	// Limit lengths
	if maxKeyLen > 25 {
		maxKeyLen = 25
	}
	if maxValueLen > 35 {
		maxValueLen = 35
	}

	// Build table
	for _, item := range items {
		displayKey := item.key
		displayValue := item.value

		// Truncate if too long
		if len(displayKey) > maxKeyLen {
			displayKey = displayKey[:maxKeyLen-3] + "..."
		}
		if len(displayValue) > maxValueLen {
			displayValue = displayValue[:maxValueLen-3] + "..."
		}

		paddedKey := fmt.Sprintf("%-*s", maxKeyLen, displayKey)
		paddedValue := fmt.Sprintf("%-*s", maxValueLen, displayValue)

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s%s", CYAN, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, paddedKey, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, ":", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, paddedValue, NC))
		result.WriteString("\n")
	}

	return result.String()
}

// CreateBeautifulList creates a beautiful bulleted list
func CreateBeautifulList(data map[string]string) string {
	var result strings.Builder
	var items []struct {
		key   string
		value string
	}

	// Convert map to slice for sorting
	for key, value := range data {
		items = append(items, struct {
			key   string
			value string
		}{key, value})
	}

	// Sort by key
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})

	// Build list
	for _, item := range items {
		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s%s", CYAN, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, item.key, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, ":", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, item.value, NC))
		result.WriteString("\n")
	}

	return result.String()
}

// GetAlertEmoji returns an emoji based on usage percentage
func GetAlertEmoji(usage float64) string {
	if usage >= 90 {
		return "🚨"
	} else if usage >= 70 {
		return "⚠️"
	} else if usage >= 50 {
		return "📊"
	} else {
		return "✅"
	}
}

// CreateProcessTable creates a formatted table for processes
func CreateProcessTable(processes []metrics.ProcessInfo) string {
	var result strings.Builder

	if len(processes) == 0 {
		result.WriteString("  No processes found\n")
		return result.String()
	}

	// Calculate total CPU usage of shown processes
	var totalCPU float64
	for _, proc := range processes {
		totalCPU += proc.CPUUsage
	}

	// Header with summary
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%sTop %d processes using %.1f%% of total system CPU%s\n",
		GRAY, len(processes), totalCPU, NC))
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s%s\n",
		CYAN, strings.Repeat("─", 80), NC))

	// Column headers
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%-6s %-15s %-8s %-8s %-12s %s%s\n",
		WHITE, "PID", "USER", "CPU%", "MEM%", "MEMORY", "COMMAND", NC))

	// Separator
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s%s\n",
		CYAN, strings.Repeat("─", 80), NC))

	// Process rows
	for _, proc := range processes {
		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%-6d %-15s %-8.1f %-8.1f %-12s %s\n",
			proc.PID,
			truncateString(proc.User, 15),
			proc.CPUUsage,
			proc.MemoryUsage,
			formatKB(proc.MemoryKB),
			truncateString(proc.Command, 30)))
	}

	return result.String()
}

// CreateDetailedResourceTable creates a detailed resource usage table
func CreateDetailedResourceTable(title string, usage metrics.ResourceUsage, formatFunc func(float64, int64, int64) string) string {
	var result strings.Builder

	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s%s\n", WHITE, title, NC))

	// Main usage line
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s•%s Usage: %s\n",
		CYAN, NC, formatFunc(usage.Usage, usage.Used, usage.Total)))

	// Detailed breakdown
	if usage.Total > 0 {
		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s•%s Total: %s\n",
			CYAN, NC, formatKB(usage.Total)))

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s•%s Used: %s\n",
			CYAN, NC, formatKB(usage.Used)))

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s•%s Free: %s\n",
			CYAN, NC, formatKB(usage.Free)))

		if usage.Available != usage.Free {
			result.WriteString("  ")
			result.WriteString(fmt.Sprintf("%s•%s Available: %s\n",
				CYAN, NC, formatKB(usage.Available)))
		}
	}

	return result.String()
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatKB(kb int64) string {
	if kb < 1024 {
		return fmt.Sprintf("%d KB", kb)
	} else if kb < 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(kb)/1024)
	} else {
		return fmt.Sprintf("%.1f GB", float64(kb)/(1024*1024))
	}
}
