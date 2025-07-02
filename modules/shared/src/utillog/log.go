package utillog

import (
	"log/slog"
	"os"
)

func NewLogger() {
	slog.SetDefault(
		slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelDebug,
				},
			),
		),
	)
}
