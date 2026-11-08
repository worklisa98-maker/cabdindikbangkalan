package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	debugOnce    sync.Once
	debugEnabled bool
)

func DebugEnabled() bool {
	debugOnce.Do(func() {
		env := strings.TrimSpace(os.Getenv("APP_ENV"))
		if strings.EqualFold(env, "production") {
			debugEnabled = false
			return
		}
		debugEnabled = strings.EqualFold(strings.TrimSpace(os.Getenv("APP_DEBUG")), "true")
	})
	return debugEnabled
}

func Debugf(module, action, format string, args ...any) {
	if !DebugEnabled() {
		return
	}
	logf("DEBUG", module, action, format, args...)
}

func Debugln(module, action string, args ...any) {
	if !DebugEnabled() {
		return
	}
	msg := strings.TrimSpace(fmt.Sprint(args...))
	logLine("DEBUG", module, action, msg)
}

func Infof(module, action, format string, args ...any) {
	logf("INFO", module, action, format, args...)
}

func Warnf(module, action, format string, args ...any) {
	logf("WARN", module, action, format, args...)
}

func Errorf(module, action, format string, args ...any) {
	logf("ERROR", module, action, format, args...)
}

func Fatalf(module, action, format string, args ...any) {
	logf("FATAL", module, action, format, args...)
	os.Exit(1)
}

func logf(level, module, action, format string, args ...any) {
	msg := ""
	if format != "" {
		msg = fmt.Sprintf(format, args...)
	}
	logLine(level, module, action, msg)
}

func logLine(level, module, action, msg string) {
	if strings.TrimSpace(module) == "" {
		module = "app"
	}
	if strings.TrimSpace(action) == "" {
		action = "unknown"
	}
	if strings.TrimSpace(msg) == "" {
		log.Printf("[%s] [%s] action=%s", level, module, action)
		return
	}
	log.Printf("[%s] [%s] action=%s %s", level, module, action, msg)
}
