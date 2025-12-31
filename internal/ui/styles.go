package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme colors - Modern blue theme inspired by Claude Code
var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("#5B9BD5") // Blue
	SecondaryColor = lipgloss.Color("#7B68EE") // Medium slate blue
	AccentColor    = lipgloss.Color("#00D4AA") // Teal accent

	// Status colors
	SuccessColor = lipgloss.Color("#2ECC71") // Green
	WarningColor = lipgloss.Color("#F1C40F") // Yellow
	ErrorColor   = lipgloss.Color("#E74C3C") // Red
	InfoColor    = lipgloss.Color("#5B9BD5") // Blue

	// Text colors
	TextColor     = lipgloss.Color("#FFFFFF") // White
	SubtextColor  = lipgloss.Color("#B0B0B0") // Light gray
	MutedColor    = lipgloss.Color("#6C6C6C") // Dark gray
	DimColor      = lipgloss.Color("#4A4A4A") // Dimmed

	// Background colors
	BgColor       = lipgloss.Color("#1A1A2E") // Dark background
	BgAltColor    = lipgloss.Color("#16213E") // Alternative dark
	BorderColor   = lipgloss.Color("#5B9BD5") // Border blue
)

// Base styles
var (
	// Bold text style
	BoldStyle = lipgloss.NewStyle().Bold(true)

	// Primary text style
	PrimaryStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	// Success style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	// Warning style
	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	// Info style
	InfoStyle = lipgloss.NewStyle().
			Foreground(InfoColor).
			Bold(true)

	// White text
	WhiteStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	// Gray text for values
	GrayStyle = lipgloss.NewStyle().
			Foreground(SubtextColor)

	// Muted/dark text
	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	// Dim text
	DimStyle = lipgloss.NewStyle().
			Foreground(DimColor)
)

// Component styles
var (
	// Header banner style
	BannerStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	// Section header style
	SectionHeaderStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	// Section title style
	SectionTitleStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Bold(true)

	// Border style
	BorderStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	// List bullet style
	BulletStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor)

	// Key style (for key-value pairs)
	KeyStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	// Value style (for key-value pairs)
	ValueStyle = lipgloss.NewStyle().
			Foreground(SubtextColor)

	// Separator style
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(MutedColor)
)

// Status icons
const (
	IconSuccess = "✓"
	IconWarning = "⚠"
	IconError   = "✗"
	IconInfo    = "ℹ"
	IconBullet  = "•"
	IconArrow   = "→"
	IconDot     = "●"
)

// Box drawing characters
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
)

// Progress bar characters
const (
	ProgressFull  = "█"
	ProgressEmpty = "░"
	ProgressHalf  = "▒"
)

// DefaultWidth is the default terminal width for formatting
const DefaultWidth = 60

// TableWidth is the width for table displays
const TableWidth = 100

// RenderBanner returns the styled ASCII banner
func RenderBanner() string {
	banner := ` ██████╗ █████╗ ████████╗ ██████╗ ██████╗ ███████╗
██╔════╝██╔══██╗╚══██╔══╝██╔═══██╗██╔══██╗██╔════╝
██║     ███████║   ██║   ██║   ██║██████╔╝███████╗
██║     ██╔══██║   ██║   ██║   ██║██╔═══╝ ╚════██║
╚██████╗██║  ██║   ██║   ╚██████╔╝██║     ███████║
 ╚═════╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝     ╚══════╝`
	return BannerStyle.Render(banner)
}

// RenderSubtitle returns the styled subtitle
func RenderSubtitle() string {
	return BoldStyle.Foreground(TextColor).Render("                    Server Monitor")
}

// RenderSectionStart returns a styled section header
func RenderSectionStart(title string) string {
	titlePart := SectionTitleStyle.Render(title)

	// Calculate padding
	titleLen := len(title) + 4 // "┌─ " + title + " ─"
	dashCount := DefaultWidth - titleLen
	if dashCount < 0 {
		dashCount = 0
	}

	prefix := BorderStyle.Render(BoxTopLeft + BoxHorizontal + " ")
	suffix := BorderStyle.Render(" " + BoxHorizontal + repeatChar(BoxHorizontal, dashCount) + BoxTopRight)

	return prefix + titlePart + suffix
}

// RenderSectionEnd returns a styled section footer
func RenderSectionEnd() string {
	return BorderStyle.Render(BoxBottomLeft + repeatChar(BoxHorizontal, DefaultWidth) + BoxBottomRight)
}

// RenderTableSectionEnd returns a styled section footer for tables
func RenderTableSectionEnd() string {
	return BorderStyle.Render(BoxBottomLeft + repeatChar(BoxHorizontal, TableWidth) + BoxBottomRight)
}

// RenderStatus returns a styled status message
func RenderStatus(status, message string) string {
	var icon string
	var style lipgloss.Style

	switch status {
	case "success":
		icon = IconSuccess
		style = SuccessStyle
	case "warning":
		icon = IconWarning
		style = WarningStyle
	case "error":
		icon = IconError
		style = ErrorStyle
	case "info":
		icon = IconInfo
		style = InfoStyle
	default:
		icon = IconInfo
		style = InfoStyle
	}

	return "  " + style.Render(icon) + " " + WhiteStyle.Render(message)
}

// RenderKeyValue returns a styled key-value pair
func RenderKeyValue(key, value string) string {
	return "  " + BulletStyle.Render(IconBullet) + " " +
		KeyStyle.Render(key) + " " +
		SeparatorStyle.Render(":") + " " +
		ValueStyle.Render(value)
}

// RenderProgressBar returns a styled progress bar
func RenderProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	empty := width - filled

	// Color based on percentage
	var barStyle lipgloss.Style
	switch {
	case percent >= 90:
		barStyle = ErrorStyle
	case percent >= 70:
		barStyle = WarningStyle
	default:
		barStyle = SuccessStyle
	}

	bar := barStyle.Render(repeatChar(ProgressFull, filled)) +
		DimStyle.Render(repeatChar(ProgressEmpty, empty))

	return bar
}

// Helper function to repeat a character
func repeatChar(char string, count int) string {
	if count <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}
