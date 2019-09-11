package rungo

import (
	"bytes"
	"fmt"
	"github.com/fpawel/gotools/internal/loggo/data"
	"github.com/fpawel/gotools/pkg/ccolor"
	"github.com/maruel/panicparse/stack"
	"github.com/powerman/structlog"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Cmd struct {
	ExeName   string
	ExeArgs   string
	UseGUI    bool
	NotifyGUI NotifyGUI
}

type NotifyGUI struct {
	MsgCodeConsole uintptr
	MsgCodePanic   uintptr
	WindowClass    string
}

func (c Cmd) Exec() error {
	panicOutput := bytes.NewBuffer(nil)
	db := data.New(data.KindText, c.ExeName)
	defer log.ErrIfFail(db.Close)
	writers := []io.Writer{panicOutput, db}

	var notifier notifyWriter

	if c.UseGUI {
		notifier = c.NotifyGUI.newWriter()
		writers = append(writers, notifier)
	}

	cmd := exec.Command(c.ExeName, strings.Fields(c.ExeArgs)...)
	cmd.Stderr = io.MultiWriter(append(writers, ccolor.NewWriter(os.Stderr))...)
	cmd.Stdout = io.MultiWriter(append(writers, ccolor.NewWriter(os.Stdout))...)
	if err := cmd.Start(); err != nil {
		return err
	}

	err := cmd.Wait()
	if err == nil {
		return nil
	}
	panicContentBuff := bytes.NewBuffer(nil)
	if err := parseDump(panicOutput, panicContentBuff); err != nil {
		return fmt.Errorf("unknown panic: %v", err)
	}
	panicContent := panicContentBuff.String()
	if c.UseGUI {
		go notifier.w.NotifyStr(c.NotifyGUI.MsgCodePanic, panicContent)
	}
	if _, err := cmd.Stderr.Write([]byte(panicContent)); err != nil {
		return err
	}
	return nil
}

func parseDump(in io.Reader, out io.Writer) error {
	// Optional: Check for GOTRACEBACK being set, in particular if there is only
	// one goroutine returned.
	c, err := stack.ParseDump(in, out, true)
	if err != nil {
		return err
	}

	// Find out similar goroutine traces and group them into buckets.
	buckets := stack.Aggregate(c.Goroutines, stack.AnyValue)

	// Calculate alignment.
	srcLen := 0
	pkgLen := 0
	for _, bucket := range buckets {
		for _, line := range bucket.Signature.Stack.Calls {
			if l := len(line.SrcLine()); l > srcLen {
				srcLen = l
			}
			if l := len(line.Func.PkgName()); l > pkgLen {
				pkgLen = l
			}
		}
	}

	for _, bucket := range buckets {
		// Print the goroutine header.
		extra := ""
		if s := bucket.SleepString(); s != "" {
			extra += " [" + s + "]"
		}
		if bucket.Locked {
			extra += " [locked]"
		}
		if c := bucket.CreatedByString(false); c != "" {
			extra += " [Created by " + c + "]"
		}
		if _, err := fmt.Fprintf(out, "%d: %s%s\n", len(bucket.IDs), bucket.State, extra); err != nil {
			return err
		}

		// Print the stack lines.
		for _, line := range bucket.Stack.Calls {
			if _, err := fmt.Fprintf(out,
				"    %-*s %-*s %s(%s)\n",
				pkgLen, line.Func.PkgName(), srcLen, line.SrcLine(),
				line.Func.Name(), &line.Args); err != nil {
				return err
			}
		}
		if bucket.Stack.Elided {
			if _, err := fmt.Fprintf(out, "    (...)\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

var log = structlog.New()
