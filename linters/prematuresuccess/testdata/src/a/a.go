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

// Each case is its own function: an earlier matching call in the same function makes a case
// unsupported (case 16 and 18), so cases must not share one.

func c1(c Client) error {
	log.Printf("config saved to disk") // 1 candidate: := with err
	err := c.SaveConfig()
	return err
}

func c2(c Client) error {
	err := c.SaveConfig() // 2 fixed twin: the call first
	log.Printf("config saved to disk")
	return err
}

func c3(c Client) error {
	log.Print("config saved to disk") // 3 candidate: the error has another name
	failure := c.SaveConfig()
	return failure
}

func c4(c Client) error {
	log.Print("config saved to disk") // 4 candidate: if init
	if err := c.SaveConfig(); err != nil {
		return err
	}
	return nil
}

func c5(c Client) {
	log.Print("configs counted") // 5 none: an int named err is not an error
	err := c.CountConfigs()
	_ = err
}

func c6(c Client) {
	log.Print("config saved") // 6 none: the error is discarded
	_ = c.SaveConfig()
}

func c7(c Client) error {
	log.Print("cache warmed") // 7 none: the message is about something else
	return c.SaveConfig()
}

func c8(c Client) error {
	log.Println() // 8 unsupported: no constant message (and no crash)
	return c.SaveConfig()
}

func c9(c Client, err error) error {
	logf.Error(err) // 9 none: a field constructor is not a log call
	return c.SaveConfig()
}

func c10(ctx context.Context, c Client, l *logf.Logger) error {
	l.Info(ctx, "config saved to disk") // 10 candidate: a typed logger
	_, err := c.SaveConfigCopy()
	return err
}

func c11(c Client) error {
	slog.Error("config save failed") // 11 none: error level reports no success
	return c.SaveConfig()
}

func c12(c Client) error {
	func() { log.Print("config saved") }() // 12 none: the log is inside a closure
	return c.SaveConfig()
}

func c13(c Client, err error) error {
	switch {
	case err == nil:
		slog.Info("config saved to disk") // 13 candidate: a case clause body
		return c.SaveConfig()
	}
	return nil
}

func c14(c Client) error {
	log.Printf(msg) // 14 unsupported: the message is a variable
	return c.SaveConfig()
}

func c15(c Client) error {
	log.Print("config saved to disk") // 15 candidate: return passes the error up
	return c.SaveConfig()
}

func c16(a, b Client) error {
	if err := a.SaveConfig(); err != nil {
		return err
	}
	log.Print("config saved to disk") // 16 unsupported: may report the earlier call
	return b.SaveConfig()
}

func c17(c Client) {
	log.Print("config saved to disk") // 17 none: go starts work, the error is not kept
	go c.SaveConfig()
}

func c18(a, b Client) error {
	err := a.SaveConfig()
	if err != nil {
		return err
	}
	log.Print("config saved to disk") // 18 unsupported: earlier call with a separate error check
	return b.SaveConfig()
}

func c19(a, b Client) error {
	if err := a.SaveConfig(); err == nil {
		log.Print("config saved to disk") // 19 unsupported: the earlier call is in the enclosing block
		return b.SaveConfig()
	}
	return nil
}

func c20(c Client) error {
	go func() { _ = c.SaveConfig() }()
	log.Print("config saved to disk") // 20 candidate: a call inside a closure is not an earlier call of this function
	return c.SaveConfig()
}
