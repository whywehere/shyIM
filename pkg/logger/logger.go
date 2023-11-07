package logger

import (
	"log/slog"
	"os"
	"time"
)

var Slog *slog.Logger

func init() {
	file, err := os.OpenFile("./logfile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	Slog = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				if t, ok := attr.Value.Any().(time.Time); ok {
					attr.Value = slog.StringValue(t.Format(time.DateTime))
				}
			}
			return attr
		},
	}))
	Slog.Info("Slog initialized")
}
