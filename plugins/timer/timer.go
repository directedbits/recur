package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
)

// TickEvent represents a single timer firing.
type TickEvent struct {
	TickCount        string
	TimeSinceStarted string
}

// StartCron sets up a cron-scheduled event source.
// Returns an event channel, a stop function, and any setup error.
func StartCron(expression, timezone string, fireOnStart bool) (<-chan TickEvent, func(), error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	c := cron.New(cron.WithLocation(loc))

	events := make(chan TickEvent, 16)
	startTime := time.Now()
	var count uint64

	_, err = c.AddFunc(expression, func() {
		count++
		events <- TickEvent{
			TickCount:        strconv.FormatUint(count, 10),
			TimeSinceStarted: time.Since(startTime).Truncate(time.Second).String(),
		}
	})
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cron expression %q: %w", expression, err)
	}

	if fireOnStart {
		count++
		events <- TickEvent{
			TickCount:        strconv.FormatUint(count, 10),
			TimeSinceStarted: "0s",
		}
	}

	c.Start()

	stop := func() {
		c.Stop()
		close(events)
	}

	return events, stop, nil
}

// StartInterval sets up a fixed-interval event source using time.Ticker.
// Returns an event channel, a stop function, and any setup error.
func StartInterval(every string, fireOnStart bool) (<-chan TickEvent, func(), error) {
	duration, err := time.ParseDuration(every)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid duration %q: %w", every, err)
	}
	if duration <= 0 {
		return nil, nil, fmt.Errorf("duration must be positive, got %s", duration)
	}

	events := make(chan TickEvent, 16)
	startTime := time.Now()
	done := make(chan struct{})

	var count uint64

	if fireOnStart {
		count++
		events <- TickEvent{
			TickCount:        strconv.FormatUint(count, 10),
			TimeSinceStarted: "0s",
		}
	}

	ticker := time.NewTicker(duration)

	go func() {
		defer close(events)
		for {
			select {
			case <-ticker.C:
				count++
				events <- TickEvent{
					TickCount:        strconv.FormatUint(count, 10),
					TimeSinceStarted: time.Since(startTime).Truncate(time.Second).String(),
				}
			case <-done:
				return
			}
		}
	}()

	stop := func() {
		ticker.Stop()
		close(done)
	}

	return events, stop, nil
}
