package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
)

type pendingCleanupWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (w *pendingCleanupWorker) Close() error { w.once.Do(func() { w.cancel(); <-w.done }); return nil }

// startPendingCleanup runs recovery and one cleanup before listeners can open.
// The worker is registered after authority and before API; isolated LIFO
// shutdown cancels cleanup before closing its database.
func startPendingCleanup(server *mailserver.MailServer, cfg *config.Config) error {
	if cfg.Mode != "multiuser" {
		return nil
	}
	control := server.MailboxControl()
	if control == nil {
		return fmt.Errorf("pending cleanup requires mailbox authority")
	}
	interval, err := time.ParseDuration(cfg.PendingCleanupInterval)
	if err != nil || interval <= 0 {
		return fmt.Errorf("invalid pending cleanup interval")
	}
	retention := time.Duration(cfg.PendingRetentionDays) * 24 * time.Hour
	emptyRetention := time.Duration(cfg.PendingEmptyRetentionHours) * time.Hour
	apply := func(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, error) {
		return server.RetainPending(ctx, item)
	}
	sweepNow := time.Now()
	initial, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = control.RunPendingRetention(initial, apply)
	if err == nil {
		err = control.RunPendingMerges(initial, func(ctx context.Context, item mailbox.Merge) error {
			return server.MergeOwnership(ctx, item.SourceID, item.TargetID)
		})
	}
	if err == nil {
		err = control.CleanupPending(initial, sweepNow, retention, emptyRetention, apply)
	}
	cancel()
	if err != nil {
		return fmt.Errorf("recover pending retention: %w", err)
	}
	ctx, stop := context.WithCancel(context.Background())
	worker := &pendingCleanupWorker{cancel: stop, done: make(chan struct{})}
	go func() {
		defer close(worker.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		maintenanceTicker := time.NewTicker(250 * time.Millisecond)
		defer maintenanceTicker.Stop()
		cursor, retentionCursor := "", ""
		// Bound durable deletions and release authority/storage locks between
		// batches. The deadline is cooperative, not a hard filesystem/lock SLA.
		retainBatch := func() {
			batchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			next, retentionErr := control.CleanupPendingBatch(batchCtx, sweepNow, retention, emptyRetention, retentionCursor, func(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, bool, error) {
				return server.RetainPendingBatch(ctx, item, 32)
			})
			cancel()
			retentionCursor = next
			if retentionErr != nil && ctx.Err() == nil {
				common.Error("Pending mailbox retention batch failed: %v", retentionErr)
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-maintenanceTicker.C:
				batchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				next, mergeErr := control.RunMergeBatch(batchCtx, "", cursor, func(ctx context.Context, item mailbox.Merge) (bool, error) {
					return server.MergeOwnershipBatch(ctx, item.SourceID, item.TargetID, 32)
				})
				cancel()
				cursor = next
				if mergeErr != nil && ctx.Err() == nil {
					common.Error("Pending mailbox merge batch failed: %v", mergeErr)
				}
				retainBatch()
			case now := <-ticker.C:
				sweepNow = now
				retainBatch()
			}
		}
	}()
	if err := server.AddCloser(worker); err != nil {
		_ = worker.Close()
		return err
	}
	return nil
}

// configureStoragePolicyForMode keeps legacy eviction out of isolated storage.
// Global multiuser limits are admission quotas, never automatic personal deletion.
func configureStoragePolicyForMode(server *mailserver.MailServer, cfg *config.Config) error {
	if server.RequiresOwnership() != (cfg.Mode == "multiuser") {
		return fmt.Errorf("storage mode does not match server ownership mode")
	}
	if cfg.Mode == "multiuser" {
		return nil
	}
	policy, err := setupStoragePolicy(cfg)
	if err != nil {
		return err
	}
	if err = server.ConfigureStoragePolicy(policy); err != nil {
		return fmt.Errorf("configure storage policy: %w", err)
	}
	return nil
}
