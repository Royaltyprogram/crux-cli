package cmd

import (
	"log/slog"
	"os"

	"github.com/liushuangls/go-server-template/configs"
	"github.com/liushuangls/go-server-template/pkg/xslog"
)

func NewDefaultSlog(conf *configs.Config) *slog.Logger {
	var extraWriters []xslog.ExtraWriter

	if conf.IsDebugMode() {
		conf.Log.Level = slog.LevelDebug
	}
	if conf.IsDebugMode() || conf.HTTP.LogToStdout {
		level := slog.LevelInfo
		if conf.IsDebugMode() {
			level = slog.LevelDebug
		}
		extraWriters = append(extraWriters, xslog.ExtraWriter{
			Writer: os.Stdout,
			Level:  level,
		})
	}

	conf.Log.ExtraWriters = extraWriters
	fileLogger := xslog.NewFileSlog(&conf.Log)
	slog.SetDefault(fileLogger)

	return fileLogger
}
