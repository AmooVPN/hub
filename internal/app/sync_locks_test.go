package app

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestWithPanelSyncLockAcquiresAndReleases(t *testing.T) {
	server := miniredis.RunT(t)
	runner := &Runner{redis: redis.NewClient(&redis.Options{Addr: server.Addr()})}
	t.Cleanup(func() { _ = runner.redis.Close() })

	called := false
	if err := runner.withPanelSyncLock(context.Background(), 42, func() error {
		called = true
		if got, err := runner.redis.Get(context.Background(), panelSyncLockKey(42)).Result(); err != nil || got == "" {
			t.Fatalf("expected lock to exist while held, got token=%q err=%v", got, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("expected lock acquisition to succeed: %v", err)
	}
	if !called {
		t.Fatal("expected lock callback to run")
	}
	if _, err := runner.redis.Get(context.Background(), panelSyncLockKey(42)).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected lock to be released, got err=%v", err)
	}
}

func TestWithPanelSyncLockPreventsConcurrentUse(t *testing.T) {
	server := miniredis.RunT(t)
	runner := &Runner{redis: redis.NewClient(&redis.Options{Addr: server.Addr()})}
	t.Cleanup(func() { _ = runner.redis.Close() })

	started := make(chan struct{})
	release := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.withPanelSyncLock(context.Background(), 7, func() error {
			close(started)
			<-release
			return nil
		})
	}()

	<-started
	if err := runner.withPanelSyncLock(context.Background(), 7, func() error { return nil }); err == nil || err.Error() != "panel sync already running" {
		t.Fatalf("expected concurrent lock attempt to fail, got %v", err)
	}

	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("expected first lock holder to finish cleanly, got %v", err)
	}
}

func TestReleasePanelSyncLockRequiresOwnerToken(t *testing.T) {
	server := miniredis.RunT(t)
	runner := &Runner{redis: redis.NewClient(&redis.Options{Addr: server.Addr()})}
	t.Cleanup(func() { _ = runner.redis.Close() })

	ctx := context.Background()
	key := panelSyncLockKey(99)
	if ok, err := runner.redis.SetNX(ctx, key, "owner-token", 0).Result(); err != nil || !ok {
		t.Fatalf("expected test lock to be created, ok=%v err=%v", ok, err)
	}

	if err := runner.releasePanelSyncLock(ctx, key, "wrong-token"); err != nil {
		t.Fatalf("expected wrong-token release attempt to be ignored, got %v", err)
	}
	if got, err := runner.redis.Get(ctx, key).Result(); err != nil || got != "owner-token" {
		t.Fatalf("expected lock to remain after wrong-token release, got token=%q err=%v", got, err)
	}

	if err := runner.releasePanelSyncLock(ctx, key, "owner-token"); err != nil {
		t.Fatalf("expected owner-token release to succeed, got %v", err)
	}
	if _, err := runner.redis.Get(ctx, key).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected lock to be removed, got err=%v", err)
	}
}
