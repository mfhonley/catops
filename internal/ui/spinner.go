package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerModel is the model for an interactive spinner
type SpinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
	done     bool
	result   string
	err      error
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(PrimaryColor)
	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case spinnerDoneMsg:
		m.done = true
		m.result = msg.result
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m SpinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return RenderStatus("error", m.err.Error()) + "\n"
		}
		return RenderStatus("success", m.result) + "\n"
	}
	return "  " + m.spinner.View() + " " + WhiteStyle.Render(m.message) + "\n"
}

type spinnerDoneMsg struct {
	result string
	err    error
}

// SimpleSpinner is a non-interactive spinner for simple loading states
type SimpleSpinner struct {
	frames  []string
	current int
	message string
	done    chan bool
}

// NewSimpleSpinner creates a simple non-blocking spinner
func NewSimpleSpinner(message string) *SimpleSpinner {
	return &SimpleSpinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
		message: message,
		done:    make(chan bool),
	}
}

// Start starts the spinner animation
func (s *SimpleSpinner) Start() {
	go func() {
		style := lipgloss.NewStyle().Foreground(PrimaryColor)
		for {
			select {
			case <-s.done:
				return
			default:
				fmt.Printf("\r  %s %s", style.Render(s.frames[s.current]), WhiteStyle.Render(s.message))
				s.current = (s.current + 1) % len(s.frames)
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner and clears the line
func (s *SimpleSpinner) Stop() {
	s.done <- true
	fmt.Print("\r\033[K") // Clear line
}

// StopWithSuccess stops the spinner and shows a success message
func (s *SimpleSpinner) StopWithSuccess(message string) {
	s.done <- true
	fmt.Print("\r\033[K") // Clear line
	fmt.Println(RenderStatus("success", message))
}

// StopWithError stops the spinner and shows an error message
func (s *SimpleSpinner) StopWithError(message string) {
	s.done <- true
	fmt.Print("\r\033[K") // Clear line
	fmt.Println(RenderStatus("error", message))
}

// StopWithWarning stops the spinner and shows a warning message
func (s *SimpleSpinner) StopWithWarning(message string) {
	s.done <- true
	fmt.Print("\r\033[K") // Clear line
	fmt.Println(RenderStatus("warning", message))
}

// WithSpinner executes a function while showing a spinner
func WithSpinner(message string, fn func() error) error {
	spinner := NewSimpleSpinner(message)
	spinner.Start()
	err := fn()
	if err != nil {
		spinner.StopWithError(err.Error())
		return err
	}
	spinner.StopWithSuccess(message + " - Done!")
	return nil
}

// WithSpinnerResult executes a function while showing a spinner and returns result
func WithSpinnerResult(message string, fn func() (string, error)) (string, error) {
	spinner := NewSimpleSpinner(message)
	spinner.Start()
	result, err := fn()
	if err != nil {
		spinner.StopWithError(err.Error())
		return "", err
	}
	spinner.StopWithSuccess(result)
	return result, nil
}
