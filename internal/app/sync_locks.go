package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AmooVPM/hub/internal/security"
)

func (r *Runner) withPanelSyncLock(ctx context.Context, panelID int64, fn func() error) error {
	if r.redis == nil {
		return fn()
	}
	key := panelSyncLockKey(panelID)
	token, err := security.RandomToken(16)
	if err != nil {
		return err
	}
	ok, err := r.redis.SetNX(ctx, key, token, 5*time.Minute).Result()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("panel sync already running")
	}
	defer func() { _ = r.releasePanelSyncLock(context.Background(), key, token) }()
	return fn()
}

func (r *Runner) releasePanelSyncLock(ctx context.Context, key, token string) error {
	if r.redis == nil {
		return nil
	}
	script := `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`
	return r.redis.Eval(ctx, script, []string{key}, token).Err()
}

func panelSyncLockKey(panelID int64) string {
	return fmt.Sprintf("hub:lock:sync:panel:%d", panelID)
}
