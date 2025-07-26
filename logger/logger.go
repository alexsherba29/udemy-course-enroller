package logger

import (
	"io"
	"log"
	"os"
)

type Logger struct {
	info  *log.Logger
	error *log.Logger
	file  *os.File
}

func New(logFile string, level string) (*Logger, error) {
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	// Add file output if specified
	var file *os.File
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, err
		}
		file = f
		writers = append(writers, f)
	}

	multiWriter := io.MultiWriter(writers...)

	return &Logger{
		info:  log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		error: log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		file:  file,
	}, nil
}

func (l *Logger) Info(v ...interface{}) {
	l.info.Println(v...)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.info.Printf(format, v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.error.Println(v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.error.Printf(format, v...)
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}