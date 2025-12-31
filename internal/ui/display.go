package ui

import (
	"fmt"
	"sort"
	"strings"

	"catops/internal/metrics"

	"github.com/charmbracelet/lipgloss"
)

// Legacy color constants - kept for backward compatibility
// These will be phased out in favor of lipgloss styles
const (
	ORANGE  = "\033[38;5;75m"
	SUCCESS = "\033[38;5;46m"
	WARNING = "\033[38;5;226m"
	ERROR   = "\033[38;5;196m"
	INFO    = "\033[38;5;75m"
	WHITE   = "\033[38;5;15m"
	GRAY    = "\033[38;5;250m"
	DARK    = "\033[38;5;240m"
	BOLD    = "\033[1m"
	NC      = "\033[0m"
)

// PrintHeader prints the application header using lipgloss
func PrintHeader() {
	fmt.Println(RenderBanner())
	fmt.Println(RenderSubtitle())
}

// PrintSection prints a section header using lipgloss
func PrintSection(title string) {
	fmt.Println(RenderSectionStart(title))
}

// PrintSectionEnd prints a section footer using lipgloss
func PrintSectionEnd() {
	fmt.Println(RenderSectionEnd())
}

// PrintTableSectionEnd prints a section footer for tables
func PrintTableSectionEnd() {
	fmt.Println(RenderTableSectionEnd())
}

// PrintStatus prints a status message using lipgloss
func PrintStatus(status, message string) {
	// Handle debug status separately (not displayed)
	if status == "debug" {
		return
	}
	fmt.Println(RenderStatus(status, message))
}

// PrintErrorWithSupport prints an error message with support contact
func PrintErrorWithSupport(message string) {
	fmt.Println(RenderStatus("error", message))
	supportMsg := GrayStyle.Render("Need help? Telegram: @mfhonley")
	fmt.Println("  " + InfoStyle.Render("💬") + " " + supportMsg)
}

// CreateTable creates a formatted table using lipgloss
func CreateTable(data map[string]string) string {
	return createStyledList(data, true)
}

// CreateFixedTable creates a fixed-width table using lipgloss
func CreateFixedTable(data map[string]string) string {
	return createStyledList(data, true)
}

// CreatePerfectTable creates a perfectly aligned table using lipgloss
func CreatePerfectTable(data map[string]string) string {
	return createStyledList(data, true)
}

// CreateBeautifulList creates a beautiful bulleted list using lipgloss
func CreateBeautifulList(data map[string]string) string {
	return createStyledList(data, false)
}

// createStyledList is the internal function for creating styled lists
func createStyledList(data map[string]string, align bool) string {
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

	// Find max key length for alignment
	maxKeyLen := 0
	if align {
		for _, item := range items {
			if len(item.key) > maxKeyLen {
				maxKeyLen = len(item.key)
			}
		}
		if maxKeyLen > 25 {
			maxKeyLen = 25
		}
	}

	// Build list
	for _, item := range items {
		displayKey := item.key
		displayValue := item.value

		// Truncate if too long
		if len(displayKey) > 25 {
			displayKey = displayKey[:22] + "..."
		}
		if len(displayValue) > 40 {
			displayValue = displayValue[:37] + "..."
		}

		// Pad key if aligning
		if align && maxKeyLen > 0 {
			displayKey = fmt.Sprintf("%-*s", maxKeyLen, displayKey)
		}

		result.WriteString(RenderKeyValue(displayKey, displayValue))
		result.WriteString("\n")
	}

	return result.String()
}

// GetAlertEmoji returns an emoji based on usage percentage
func GetAlertEmoji(usage float64) string {
	switch {
	case usage >= 90:
		return "🚨"
	case usage >= 70:
		return "⚠️"
	case usage >= 50:
		return "📊"
	default:
		return "✅"
	}
}

// CreateProcessTable creates a formatted table for processes
func CreateProcessTable(processes []metrics.ProcessInfo) string {
	var result strings.Builder

	if len(processes) == 0 {
		result.WriteString("  " + GrayStyle.Render("No processes found") + "\n")
		return result.String()
	}

	// Calculate total CPU usage
	var totalCPU float64
	for _, proc := range processes {
		totalCPU += proc.CPUUsage
	}

	// Header with summary
	summaryStyle := lipgloss.NewStyle().Foreground(SubtextColor)
	result.WriteString("  " + summaryStyle.Render(fmt.Sprintf("Top %d processes using %.1f%% of total system CPU", len(processes), totalCPU)) + "\n")

	// Separator
	result.WriteString("  " + BorderStyle.Render(repeatChar(BoxHorizontal, TableWidth)) + "\n")

	// Column headers
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(TextColor)
	result.WriteString("  " + headerStyle.Render(fmt.Sprintf("%6s %15s %8s %8s %12s %8s %8s %s",
		"PID", "USER", "CPU%", "MEM%", "MEMORY", "STATUS", "TTY", "COMMAND")) + "\n")

	// Separator
	result.WriteString("  " + BorderStyle.Render(repeatChar(BoxHorizontal, TableWidth)) + "\n")

	// Process rows
	for _, proc := range processes {
		var statusStyle lipgloss.Style
		switch proc.Status {
		case "R":
			statusStyle = SuccessStyle // Running
		case "S":
			statusStyle = WarningStyle // Sleeping
		case "Z":
			statusStyle = ErrorStyle // Zombie
		case "D":
			statusStyle = InfoStyle // Disk sleep
		default:
			statusStyle = MutedStyle
		}

		row := fmt.Sprintf("%6d %15s %8.1f %8.1f %12s ",
			proc.PID,
			truncateString(proc.User, 15),
			proc.CPUUsage,
			proc.MemoryUsage,
			formatKB(proc.MemoryKB))

		result.WriteString("  " + row)
		result.WriteString(statusStyle.Render(fmt.Sprintf("%8s", proc.Status)))
		result.WriteString(fmt.Sprintf(" %8s %s\n",
			truncateString(proc.TTY, 8),
			truncateString(proc.Command, 25)))
	}

	return result.String()
}

// CreateProcessTableByMemory creates a formatted table for processes sorted by memory
func CreateProcessTableByMemory(processes []metrics.ProcessInfo) string {
	var result strings.Builder

	if len(processes) == 0 {
		result.WriteString("  " + GrayStyle.Render("No processes found") + "\n")
		return result.String()
	}

	// Calculate total memory usage
	var totalMemory float64
	for _, proc := range processes {
		totalMemory += proc.MemoryUsage
	}

	// Header with summary
	summaryStyle := lipgloss.NewStyle().Foreground(SubtextColor)
	result.WriteString("  " + summaryStyle.Render(fmt.Sprintf("Top %d processes using %.1f%% of total system memory", len(processes), totalMemory)) + "\n")

	// Separator
	result.WriteString("  " + BorderStyle.Render(repeatChar(BoxHorizontal, TableWidth)) + "\n")

	// Column headers
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(TextColor)
	result.WriteString("  " + headerStyle.Render(fmt.Sprintf("%6s %15s %8s %8s %12s %8s %8s %s",
		"PID", "USER", "CPU%", "MEM%", "MEMORY", "STATUS", "TTY", "COMMAND")) + "\n")

	// Separator
	result.WriteString("  " + BorderStyle.Render(repeatChar(BoxHorizontal, TableWidth)) + "\n")

	// Process rows
	for _, proc := range processes {
		var statusStyle lipgloss.Style
		switch proc.Status {
		case "R":
			statusStyle = SuccessStyle
		case "S":
			statusStyle = WarningStyle
		case "Z":
			statusStyle = ErrorStyle
		case "D":
			statusStyle = InfoStyle
		default:
			statusStyle = MutedStyle
		}

		row := fmt.Sprintf("%6d %15s %8.1f %8.1f %12s ",
			proc.PID,
			truncateString(proc.User, 15),
			proc.CPUUsage,
			proc.MemoryUsage,
			formatKB(proc.MemoryKB))

		result.WriteString("  " + row)
		result.WriteString(statusStyle.Render(fmt.Sprintf("%8s", proc.Status)))
		result.WriteString(fmt.Sprintf(" %8s %s\n",
			truncateString(proc.TTY, 8),
			truncateString(proc.Command, 25)))
	}

	return result.String()
}

// CreateDetailedResourceTable creates a detailed resource usage table
func CreateDetailedResourceTable(title string, usage metrics.ResourceUsage, formatFunc func(float64, int64, int64) string) string {
	var result strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(TextColor)
	result.WriteString("  " + titleStyle.Render(title) + "\n")

	// Main usage line with progress bar
	usageText := formatFunc(usage.Usage, usage.Used, usage.Total)
	result.WriteString(RenderKeyValue("Usage", usageText) + "\n")

	// Progress bar
	result.WriteString("  " + BulletStyle.Render(IconBullet) + " ")
	result.WriteString(RenderProgressBar(usage.Usage, 30))
	result.WriteString(fmt.Sprintf(" %.1f%%\n", usage.Usage))

	// Detailed breakdown
	if usage.Total > 0 {
		result.WriteString(RenderKeyValue("Total", formatKB(usage.Total)) + "\n")
		result.WriteString(RenderKeyValue("Used", formatKB(usage.Used)) + "\n")
		result.WriteString(RenderKeyValue("Free", formatKB(usage.Free)) + "\n")

		if usage.Available != usage.Free {
			result.WriteString(RenderKeyValue("Available", formatKB(usage.Available)) + "\n")
		}
	}

	return result.String()
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatKB(kb int64) string {
	switch {
	case kb < 1024:
		return fmt.Sprintf("%d KB", kb)
	case kb < 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(kb)/1024)
	case kb < 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(kb)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f TB", float64(kb)/(1024*1024*1024))
	}
}
