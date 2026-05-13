package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	file    *os.File
	verbose bool
	mu      sync.Mutex
}

var (
	instance *Logger
	once     sync.Once
)

func GetLogger() *Logger {
	once.Do(func() {
		instance = &Logger{}
		instance.init()
	})
	return instance
}

func (l *Logger) init() {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".contextsync", "logs")
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "daemon.log")
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	l.file = file
}

func (l *Logger) log(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	if l.file != nil {
		l.file.WriteString(line)
	}

	if l.verbose || level == "ERROR" {
		fmt.Fprint(os.Stderr, line)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log("ERROR", format, args...)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log("DEBUG", format, args...)
}

func (l *Logger) SetVerbose(v bool) {
	l.verbose = v
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// GetLogPath returns the log file path
func GetLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".contextsync", "logs", "daemon.log")
}

// ReadLogs reads the last n lines of the log file
func ReadLogs(n int) string {
	logPath := GetLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "No logs available"
	}

	lines := splitLines(string(data))
	if len(lines) <= n {
		return string(data)
	}

	start := len(lines) - n
	result := ""
	for i := start; i < len(lines); i++ {
		result += lines[i] + "\n"
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
