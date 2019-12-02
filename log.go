package main

import (
	"fmt"
	"os"
	"time"
)

const (
	DateFormatYmdHis = "2006-01-02 15:04:05"
)

type Logger struct {
	logDir string

	appFile      *os.File
	errorFile    *os.File
	rawFile      *os.File
	userLogFiles map[string]*os.File
}

func NewLogger(logFilesDir string) *Logger {
	r := Logger{logDir: logFilesDir}

	f, err := os.OpenFile(r.logDir+"/raw.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("cannot open raw.log: %s\n", err)
		os.Exit(1)
	}
	r.rawFile = f

	f, err = os.OpenFile(r.logDir+"/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("cannot open app.log: %s\n", err)
		os.Exit(1)
	}
	r.appFile = f

	f, err = os.OpenFile(r.logDir+"/app.err", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("cannot open app.err: %s\n", err)
		os.Exit(1)
	}
	r.errorFile = f

	r.userLogFiles = make(map[string]*os.File)

	return &r
}

func (l *Logger) Raw(text string) {
	_, _ = fmt.Fprintf(l.rawFile, "[%s] %s\n", time.Now().Format(DateFormatYmdHis), text)
}

func (l *Logger) App(text string) {
	_, _ = fmt.Fprintf(l.appFile, "[%s] %s\n", time.Now().Format(DateFormatYmdHis), text)
}

func (l *Logger) Error(text string) {
	_, _ = fmt.Fprintf(l.errorFile, "[%s] %s\n", time.Now().Format(DateFormatYmdHis), text)
}

func (l *Logger) User(userId, text string) {
	_, ok := l.userLogFiles[userId]

	if !ok {
		filename := l.logDir + "/" + userId + ".log"

		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic("cannot open " + filename)
		}

		l.userLogFiles[userId] = f
	}

	_, _ = fmt.Fprintln(l.userLogFiles[userId], text)
}
