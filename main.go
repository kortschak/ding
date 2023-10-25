// Copyright ©2023 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ding is a differential time stamped ping.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

func main() {
	addrs := make(set)
	flag.Var(addrs, "a", "set of addresses to ping (comma separated)")
	interval := flag.Duration("i", 10*time.Second, "interval between pings for each address")
	batch := flag.Duration("b", time.Minute, "length of time for each batch of pings")
	priv := flag.Bool("priv", true, "has access to raw network (requires setcap cap_net_raw=+ep or equivalent)")
	n := flag.Int("n", 5, "number of ICMP packets for each batch of pings")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	var wg sync.WaitGroup
	for {
		for addr := range addrs {
			addr := addr
			wg.Add(1)
			go func() {
				defer wg.Done()

				start, stats, err := ping(addr, *n, *interval, *batch, *priv)
				if err != nil {
					log.LogAttrs(ctx, slog.LevelError, "ping",
						slog.String("addr", addr),
						slog.Time("start", start),
						slog.Any("error", err),
					)
					return
				}
				log.LogAttrs(ctx, slog.LevelInfo, "ping",
					slog.String("addr", addr),
					slog.Time("start", start),
					slog.Int("sent", stats.PacketsSent),
					slog.Float64("loss", stats.PacketLoss),
					slog.Duration("min_rtt", stats.MinRtt),
					slog.Duration("max_rtt", stats.MaxRtt),
					slog.Duration("avg_rtt", stats.AvgRtt),
					slog.Duration("stdev_rtt", stats.StdDevRtt),
				)
			}()
		}
		wg.Wait()
	}
}

func ping(addr string, n int, interval, timeout time.Duration, priv bool) (time.Time, *probing.Statistics, error) {
	start := time.Now()
	p, err := probing.NewPinger(addr)
	if err != nil {
		return start, nil, err
	}
	p.SetPrivileged(priv)
	p.Count = n
	p.Interval = interval
	p.Timeout = timeout
	err = p.Run()
	return start, p.Statistics(), err
}

type set map[string]bool

func (s set) Set(v string) error {
	for _, y := range strings.Split(v, ",") {
		if y == "" {
			return errors.New("empty string target")
		}
		s[y] = true
	}
	return nil
}

func (s set) String() string {
	p := make([]string, 0, len(s))
	for y := range s {
		p = append(p, y)
	}
	sort.Strings(p)
	return strings.Join(p, ",")
}
