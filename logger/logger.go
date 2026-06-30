package logger

import (
	"epos-proxy/buildinfo"
	"epos-proxy/config"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log = logrus.New()

var logDir string
var supportMode bool

func InitLogger(cfg *config.Manager) {
	logDir = filepath.Join(cfg.ConfigDirectory(), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		Fatalf("Failed to create log directory: %v", err)
	}

	filename := filepath.Join(logDir, "epos-proxy.log")

	log.SetOutput(&lumberjack.Logger{
		Filename:   filename,
		MaxSize:    20, // MB
		MaxBackups: 5,  // keep last x files
		MaxAge:     5,  // days
		Compress:   false,
	})
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	log.SetLevel(logrus.InfoLevel)
	if cfg.HasArgs("--verbose", "--support-mode") || cfg.Data.SupportMode {
		log.SetLevel(logrus.DebugLevel)
		supportMode = true
	}

	Infof("Starting Epos Proxy v: %s", buildinfo.GetVersionInfo())
}

func SetSupportMode(enabled bool) {
	supportMode = enabled
	if enabled {
		log.SetLevel(logrus.DebugLevel)
		Infof("Support mode enabled: verbose logging is active")
	} else {
		log.SetLevel(logrus.InfoLevel)
		Infof("Support mode disabled: concise logging is active")
	}
}

func IsSupportMode() bool {
	return supportMode
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
