package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSchedulerSpacesRequests(t *testing.T) {
	scheduler := Scheduler{Interval: 15 * time.Millisecond}
	if err := scheduler.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := scheduler.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 10*time.Millisecond {
		t.Fatalf("second request waited %s, want at least 10ms", elapsed)
	}
}

func TestSchedulerHonorsCancellation(t *testing.T) {
	scheduler := Scheduler{Interval: time.Hour}
	if err := scheduler.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := scheduler.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected scheduler error: %v", err)
	}
}
