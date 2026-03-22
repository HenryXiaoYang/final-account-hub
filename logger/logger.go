package logger

import (
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

	// Gin already formats its own log lines with timestamps and prefixes,
	// so write directly to stdout/stderr to avoid double prefixes.
	gin.DefaultWriter = os.Stdout
	gin.DefaultErrorWriter = os.Stderr
}

func GinLogger() gin.HandlerFunc {
	return gin.LoggerWithWriter(os.Stdout)
}
