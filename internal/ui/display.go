package ui

import (
	"fmt"
	"catops/internal/metrics"
	"sort"
	"strings"
)

// Color constants - Simple Orange Theme
const (
	// Main orange color
	ORANGE = "\033[38;5;214m" // Orange
	
	// Status colors
	SUCCESS = "\033[38;5;46m"  // Green
	WARNING = "\033[38;5;226m" // Yellow
	ERROR   = "\033[38;5;196m" // Red
	INFO    = "\033[38;5;75m"  // Blue
	
	// Text colors
	WHITE = "\033[38;5;15m"  // White
	GRAY  = "\033[38;5;250m" // Light gray
	DARK  = "\033[38;5;240m" // Dark gray
	
	// Special effects
	BOLD = "\033[1m"
	
	// Reset
	NC = "\033[0m" // No Color
)

// PrintHeader prints the application header
func PrintHeader() {
	fmt.Printf("%s%s ██████╗ █████╗ ████████╗ ██████╗ ██████╗ ███████╗%s\n", BOLD, ORANGE, NC)
	fmt.Printf("%s%s██╔════╝██╔══██╗╚══██╔══╝██╔═══██╗██╔══██╗██╔════╝%s\n", BOLD, ORANGE, NC)
	fmt.Printf("%s%s██║     ███████║   ██║   ██║   ██║██████╔╝███████╗%s\n", BOLD, ORANGE, NC)
	fmt.Printf("%s%s██║     ██╔══██║   ██║   ██║   ██║██╔═══╝ ╚════██║%s\n", BOLD, ORANGE, NC)
	fmt.Printf("%s%s╚██████╗██║  ██║   ██║   ╚██████╔╝██║     ███████║%s\n", BOLD, ORANGE, NC)
	fmt.Printf("%s%s ╚═════╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝     ╚══════╝%s\n", BOLD, ORANGE, NC)
	fmt.Printf("%s%s                    Server Monitor%s\n", BOLD, WHITE, NC)
}

// PrintSection prints a section header
func PrintSection(title string) {
	titleWidth := len(title) + 4 // 4 for "┌─ " and " ─"
	totalWidth := 60             // Fixed total width
	dashCount := totalWidth - titleWidth
	if dashCount < 0 {
		dashCount = 0
	}
	fmt.Printf("%s%s┌─ %s%s%s ─%s%s┐%s\n",
		ORANGE,
		BOLD,
		WHITE, title, ORANGE,
		strings.Repeat("─", dashCount),
		BOLD,
		NC)
}

// PrintSectionEnd prints a section footer
func PrintSectionEnd() {
	totalWidth := 60 // Same fixed total width as PrintSection
	fmt.Printf("%s%s└%s%s┘%s\n", ORANGE, BOLD, strings.Repeat("─", totalWidth), BOLD, NC)
}

// PrintStatus prints a status message
func PrintStatus(status, message string) {
	switch status {
	case "success":
		fmt.Printf("  %s%s✓%s %s%s\n", SUCCESS, BOLD, NC, WHITE, message)
	case "warning":
		fmt.Printf("  %s%s⚠%s %s%s\n", WARNING, BOLD, NC, WHITE, message)
	case "error":
		fmt.Printf("  %s%s✗%s %s%s\n", ERROR, BOLD, NC, WHITE, message)
	case "info":
		fmt.Printf("  %s%sℹ%s %s%s\n", INFO, BOLD, NC, WHITE, message)
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
		result.WriteString(fmt.Sprintf("%s%s%s", ORANGE, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, displayKey, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", DARK, ":", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", GRAY, displayValue, NC))
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
		result.WriteString(fmt.Sprintf("%s%s%s", ORANGE, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, paddedKey, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", DARK, ":", NC))
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
		result.WriteString(fmt.Sprintf("%s%s%s", ORANGE, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, paddedKey, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", DARK, ":", NC))
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
		result.WriteString(fmt.Sprintf("%s%s%s", ORANGE, "•", NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", WHITE, item.key, NC))
		result.WriteString(" ")
		result.WriteString(fmt.Sprintf("%s%s%s", DARK, ":", NC))
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
		PRIMARY, strings.Repeat("─", 80), NC))

	// Column headers
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s%-6s %-15s %-8s %-8s %-12s %-8s %-8s %s%s\n",
		BOLD, WHITE, "PID", "USER", "CPU%", "MEM%", "MEMORY", "STATUS", "TTY", "COMMAND", NC))

	// Separator
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s%s\n",
		PRIMARY, strings.Repeat("─", 100), NC))

	// Process rows
	for _, proc := range processes {
		// Color code for status
		statusColor := DARK
		switch proc.Status {
		case "R":
			statusColor = SUCCESS // Running
		case "S":
			statusColor = WARNING // Sleeping
		case "Z":
			statusColor = ERROR // Zombie
		case "D":
			statusColor = INFO // Disk sleep
		}

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%-6d %-15s %-8.1f %-8.1f %-12s %s%-8s%s %-8s %s\n",
			proc.PID,
			truncateString(proc.User, 15),
			proc.CPUUsage,
			proc.MemoryUsage,
			formatKB(proc.MemoryKB),
			statusColor, proc.Status, NC,
			truncateString(proc.TTY, 8),
			truncateString(proc.Command, 25)))
	}

	return result.String()
}

// CreateDetailedResourceTable creates a detailed resource usage table
func CreateDetailedResourceTable(title string, usage metrics.ResourceUsage, formatFunc func(float64, int64, int64) string) string {
	var result strings.Builder

	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s%s%s\n", BOLD, WHITE, title, NC))

	// Main usage line
	result.WriteString("  ")
	result.WriteString(fmt.Sprintf("%s%s•%s %sUsage: %s%s\n",
		ORANGE, BOLD, NC, WHITE, GRAY, formatFunc(usage.Usage, usage.Used, usage.Total)))

	// Detailed breakdown
	if usage.Total > 0 {
		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s•%s %sTotal: %s%s\n",
			ORANGE, BOLD, NC, WHITE, GRAY, formatKB(usage.Total)))

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s•%s %sUsed: %s%s\n",
			ORANGE, BOLD, NC, WHITE, GRAY, formatKB(usage.Used)))

		result.WriteString("  ")
		result.WriteString(fmt.Sprintf("%s%s•%s %sFree: %s%s\n",
			ORANGE, BOLD, NC, WHITE, GRAY, formatKB(usage.Free)))

		if usage.Available != usage.Free {
			result.WriteString("  ")
			result.WriteString(fmt.Sprintf("%s%s•%s %sAvailable: %s%s\n",
				ORANGE, BOLD, NC, WHITE, GRAY, formatKB(usage.Available)))
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
