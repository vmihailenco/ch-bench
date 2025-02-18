package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-faster/errors"
	"golang.org/x/sync/errgroup"

	"github.com/go-faster/ch"
	"github.com/go-faster/ch/proto"
)

func run(ctx context.Context) error {
	var arg struct {
		Workers int
	}
	flag.IntVar(&arg.Workers, "j", 1, "concurrent workers to use")
	flag.Parse()

	var (
		rows       uint64
		totalBytes uint64
	)

	start := time.Now()
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < arg.Workers; i++ {
		g.Go(func() error {
			c, err := ch.Dial(ctx, ch.Options{})
			if err != nil {
				return errors.Wrap(err, "dial")
			}
			defer func() {
				_ = c.Close()
			}()

			var data proto.ColUInt64
			if err := c.Do(ctx, ch.Query{
				Body: "SELECT number FROM system.numbers_mt LIMIT 500000000",
				OnProgress: func(ctx context.Context, p proto.Progress) error {
					atomic.AddUint64(&totalBytes, p.Bytes)
					return nil
				},
				OnResult: func(ctx context.Context, block proto.Block) error {
					atomic.AddUint64(&rows, uint64(block.Rows))
					return nil
				},
				Result: proto.Results{
					{Name: "number", Data: &data},
				},
			}); err != nil {
				return errors.Wrap(err, "query")
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "wait")
	}

	duration := time.Since(start)
	fmt.Println(duration.Round(time.Millisecond), rows, "rows",
		humanize.Bytes(totalBytes),
		humanize.Bytes(uint64(float64(totalBytes)/duration.Seconds()))+"/s",
		arg.Workers, "jobs",
	)

	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(2)
	}
}
