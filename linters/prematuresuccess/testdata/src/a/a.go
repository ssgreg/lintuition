package a

import (
	"context"
	"log"
	"log/slog"

	"github.com/ssgreg/logf"
)

type Client interface {
	SaveConfig() error
	CountConfigs() int
	SaveConfigCopy() (int, error)
}

var msg = "config saved"

func cases(ctx context.Context, c Client, l *logf.Logger) error {
	log.Printf("config saved to disk") // 1 candidate: := with err
	err := c.SaveConfig()

	err = c.SaveConfig() // 2 fixed twin: the call first
	log.Printf("config saved to disk")

	log.Print("config saved to disk") // 3 candidate: the error has another name
	failure := c.SaveConfig()

	_ = ctx
	log.Print("config saved to disk") // 4 candidate: if init
	if err := c.SaveConfig(); err != nil {
		return err
	}

	log.Print("configs counted") // 5 none: an int named err is not an error
	{
		err := c.CountConfigs()
		_ = err
	}

	log.Print("config saved") // 6 none: the error is discarded
	_ = c.SaveConfig()

	log.Print("cache warmed") // 7 none: the message is about something else
	err = c.SaveConfig()

	log.Println() // 8 unsupported: no constant message (and no crash)
	err = c.SaveConfig()

	logf.Error(err) // 9 none: a field constructor is not a log call
	err = c.SaveConfig()

	_ = ctx
	l.Info(ctx, "config saved to disk") // 10 candidate: a typed logger
	_, err = c.SaveConfigCopy()

	slog.Error("config save failed") // 11 none: error level reports no success
	err = c.SaveConfig()

	func() { log.Print("config saved") }() // 12 none: the log is inside a closure
	err = c.SaveConfig()

	switch {
	case err == nil:
		slog.Info("config saved to disk") // 13 candidate: a case clause body
		err = c.SaveConfig()
	}

	log.Printf(msg) // 14 unsupported: the message is a variable
	err = c.SaveConfig()

	err = c.SaveConfig()
	log.Print("config saved to disk") // 16 unsupported: may report the call before it
	err = c.SaveConfig()

	log.Print("config saved to disk") // 17 none: go starts work, the error is not kept
	go c.SaveConfig()

	_ = failure
	log.Print("config saved to disk") // 15 candidate: return passes the error up
	return c.SaveConfig()
}
