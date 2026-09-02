package logger

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log = logrus.New()

var logDir string

func InitLogger() {
	var candidates []string

	// 1. User config directory
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		candidates = append(candidates, filepath.Join(dir, "EposProxy", "logs"))
	}

	// 2. ProgramData (system-wide on Windows, ideal for Windows services)
	if pd := os.Getenv("ProgramData"); pd != "" {
		candidates = append(candidates, filepath.Join(pd, "EposProxy", "logs"))
	}

	// 3. Executable directory
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), "logs"))
	}

	// 4. System temp directory
	candidates = append(candidates, filepath.Join(os.TempDir(), "EposProxy", "logs"))

	for _, cand := range candidates {
		if err := os.MkdirAll(cand, 0755); err == nil {
			logDir = cand
			break
		}
	}

	if logDir != "" {
		filename := filepath.Join(logDir, "epos-proxy.log")
		log.SetOutput(&lumberjack.Logger{
			Filename:   filename,
			MaxSize:    20, // MB
			MaxBackups: 5,  // keep last x files
			MaxAge:     5,  // days
			Compress:   false,
		})
	} else {
		log.SetOutput(os.Stderr)
	}

	// log.SetReportCaller(true)

	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	log.SetLevel(logrus.InfoLevel)
	Info("Logger initialized")
}

// Wrappers
func Info(args ...interface{})                  { log.Info(args...) }
func Infof(format string, args ...interface{})  { log.Infof(format, args...) }
func Warn(args ...interface{})                  { log.Warn(args...) }
func Warnf(format string, args ...interface{})  { log.Warnf(format, args...) }
func Error(args ...interface{})                 { log.Error(args...) }
func Errorf(format string, args ...interface{}) { log.Errorf(format, args...) }
func Fatal(args ...interface{})                 { log.Fatal(args...) }
func Fatalf(format string, args ...interface{}) { log.Fatalf(format, args...) }
func Debug(args ...interface{})                 { log.Debug(args...) }
func Debugf(format string, args ...interface{}) { log.Debugf(format, args...) }

func LogDirectory() string { return logDir }
