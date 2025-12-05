package logger

import (
	"io"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

var (
	Info  *log.Logger
	Error *log.Logger
)

func Init() {
	Info = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime)
	Error = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)

	gin.DefaultWriter = &logWriter{Info}
	gin.DefaultErrorWriter = &logWriter{Error}
}

type logWriter struct {
	logger *log.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Print(string(p))
	return len(p), nil
}

func GinLogger() gin.HandlerFunc {
	return gin.LoggerWithWriter(io.MultiWriter(&logWriter{Info}))
}
