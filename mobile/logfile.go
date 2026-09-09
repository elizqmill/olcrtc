package mobile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/logger"
)

var (
	logMu       sync.Mutex
	logFile     *os.File
	logFilePath string
)

// InitLogFile starts logging to a file in the given directory.
// Returns the absolute path of the log file, or empty string on error.
func InitLogFile(dir string) string {
	logMu.Lock()
	defer logMu.Unlock()

	if logFile != nil {
		return logFilePath
	}

	path := filepath.Join(dir, "olcrtc.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return ""
	}

	logFile = f
	logFilePath = path

	logger.SetLogCallback(func(line string) {
		logMu.Lock()
		defer logMu.Unlock()
		if logFile != nil {
			fmt.Fprintf(logFile, "%s %s\n", time.Now().Format("15:04:05.000"), line)
			logFile.Sync()
		}
	})

	return path
}

// ReadLogTail returns the last n bytes of the log file.
func ReadLogTail(n int64) string {
	logMu.Lock()
	path := logFilePath
	logMu.Unlock()

	if path == "" {
		return ""
	}

	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return ""
	}

	size := stat.Size()
	if size == 0 {
		return ""
	}
	if n > size {
		n = size
	}

	buf := make([]byte, n)
	_, err = f.ReadAt(buf, size-n)
	if err != nil {
		return ""
	}

	return string(buf)
}

// ReadLogLines returns the last maxLines log lines.
func ReadLogLines(maxLines int) string {
	logMu.Lock()
	path := logFilePath
	logMu.Unlock()

	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// ClearLog truncates the log file.
func ClearLog() {
	logMu.Lock()
	defer logMu.Unlock()

	if logFile != nil {
		logFile.Truncate(0)
		logFile.Seek(0, 0)
	}
}
