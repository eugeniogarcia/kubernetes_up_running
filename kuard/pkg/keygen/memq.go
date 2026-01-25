package keygen

import (
	"context"
	"fmt"
	"os"
	"time"

	memqclient "kuard/pkg/memq/client"
)

type memQWorker struct {
	c    Config
	ctx  context.Context
	out  func(string)
	memq memqclient.Client
}

func newMemQWorker(ctx context.Context, c Config, out func(string)) *memQWorker {
	w := &memQWorker{
		c:   c,
		ctx: ctx,
		out: out,
		memq: memqclient.Client{
			BaseServerURL: c.MemQServer,
		},
	}
	return w
}

func (w *memQWorker) startWork() {
	w.log("MemQ Worker starting")
	for !w.isDone() {
		m, err := w.memq.Dequeue(w.c.MemQQueue)
		if err != nil {
			w.logf("Error talking to server: %v. Retrying after 1s.", err)
			time.Sleep(time.Second)
			continue
		}

		if m == nil {
			// Queue is empty.  Exit if necessary. Otherwise sleep.
			if w.c.ExitOnComplete {
				os.Exit(w.c.ExitCode)
			}
			w.logf("Queue is empty. Retrying after 1s.")
			time.Sleep(time.Second)
			continue
		}

		w.itemDone(generateKey())
	}
}

func (w *memQWorker) isDone() bool {
	select {
	case <-w.ctx.Done():
		w.log("MemQ Worker shutting down")
		return true
	default:
	}
	return false
}

func (w memQWorker) itemDone(desc string) {

	if len(desc) > 0 {
		desc = ": " + desc
	}

	w.logf("Item done%s", desc)
}

func (w *memQWorker) done() {

}
func (w *memQWorker) log(s string) {
	w.out(s)
}

func (w *memQWorker) logf(format string, v ...interface{}) {
	w.out(fmt.Sprintf(format, v...))
}
