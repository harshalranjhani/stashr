package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Level represents the log level
type Level int

const (
	// DEBUG level for detailed debugging information
	DEBUG Level = iota
	// INFO level for informational messages
	INFO
	// WARN level for warning messages
	WARN
	// ERROR level for error messages
	ERROR
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a structured logger
type Logger struct {
	level      Level
	output     io.Writer
	fileLogger *log.Logger
	verbose    bool
	colorized  bool
}

var (
	// defaultLogger is the default logger instance
	defaultLogger *Logger

	// Color functions
	successColor = color.New(color.FgGreen).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	warnColor    = color.New(color.FgYellow).SprintFunc()
	infoColor    = color.New(color.FgCyan).SprintFunc()
	debugColor   = color.New(color.FgMagenta).SprintFunc()
)

func init() {
	defaultLogger = New(INFO, os.Stdout, true, false)
}

// New creates a new logger instance
func New(level Level, output io.Writer, colorized bool, verbose bool) *Logger {
	return &Logger{
		level:     level,
		output:    output,
		verbose:   verbose,
		colorized: colorized,
	}
}

// SetLevel sets the log level
func SetLevel(level Level) {
	defaultLogger.level = level
}

// SetVerbose enables or disables verbose mode
func SetVerbose(verbose bool) {
	defaultLogger.verbose = verbose
	if verbose {
		defaultLogger.level = DEBUG
	}
}

// SetOutput sets the output writer
func SetOutput(output io.Writer) {
	defaultLogger.output = output
}

// SetFileOutput sets a file logger in addition to stdout
func SetFileOutput(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	defaultLogger.fileLogger = log.New(file, "", log.LstdFlags)
	return nil
}

// log is the internal logging function
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	// Log to file if file logger is set
	if l.fileLogger != nil {
		l.fileLogger.Printf("[%s] %s", level.String(), message)
	}

	// Format for console output
	var levelStr string
	if l.colorized {
		switch level {
		case DEBUG:
			levelStr = debugColor("[DEBUG]")
		case INFO:
			levelStr = infoColor("[INFO]")
		case WARN:
			levelStr = warnColor("[WARN]")
		case ERROR:
			levelStr = errorColor("[ERROR]")
		}
	} else {
		levelStr = fmt.Sprintf("[%s]", level.String())
	}

	output := fmt.Sprintf("%s %s %s\n", timestamp, levelStr, message)
	fmt.Fprint(l.output, output)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	defaultLogger.log(DEBUG, format, args...)
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	defaultLogger.log(INFO, format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	defaultLogger.log(WARN, format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	defaultLogger.log(ERROR, format, args...)
}

// Success prints a success message with a checkmark
func Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if defaultLogger.colorized {
		fmt.Fprintf(defaultLogger.output, "%s %s\n", successColor("✓"), message)
	} else {
		fmt.Fprintf(defaultLogger.output, "✓ %s\n", message)
	}
}

// Failure prints a failure message with an X
func Failure(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if defaultLogger.colorized {
		fmt.Fprintf(defaultLogger.output, "%s %s\n", errorColor("✗"), message)
	} else {
		fmt.Fprintf(defaultLogger.output, "✗ %s\n", message)
	}
}

// Warning prints a warning message with a warning symbol
func Warning(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if defaultLogger.colorized {
		fmt.Fprintf(defaultLogger.output, "%s %s\n", warnColor("⚠"), message)
	} else {
		fmt.Fprintf(defaultLogger.output, "⚠ %s\n", message)
	}
}

// Progress prints a progress message
func Progress(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if defaultLogger.colorized {
		fmt.Fprintf(defaultLogger.output, "%s %s\n", infoColor("→"), message)
	} else {
		fmt.Fprintf(defaultLogger.output, "→ %s\n", message)
	}
}

// Header prints a formatted header
func Header(title string) {
	line := strings.Repeat("━", len(title))
	if defaultLogger.colorized {
		fmt.Fprintf(defaultLogger.output, "\n%s\n%s\n\n", color.New(color.Bold).Sprint(title), line)
	} else {
		fmt.Fprintf(defaultLogger.output, "\n%s\n%s\n\n", title, line)
	}
}

// Separator prints a separator line
func Separator() {
	fmt.Fprintln(defaultLogger.output, "")
}

// PrintError prints a formatted error message
func PrintError(err error) {
	if err != nil {
		Failure("Error: %v", err)
	}
}
