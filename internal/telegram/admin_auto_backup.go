package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"locationbot/internal/adminbackup"
)

const automaticBackupKeep = 14

func (b *Bot) autoBackupDirectory() string {
	return filepath.Join(b.store.DataDir(), "backups")
}
func (b *Bot) autoBackupList() ([]string, error) {
	dir := b.autoBackupDirectory()
	items, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, item := range items {
		if item.Type().IsRegular() && strings.HasPrefix(item.Name(), "auto-") && strings.HasSuffix(item.Name(), ".zip") {
			result = append(result, item.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

// The snapshot and live restore take the same lock. Stored customer details
// never leave the private runtime volume unless an admin explicitly exports.

// snapshotBackup serializes all state writers for a consistent ZIP.
// backupMu is held by the caller, which also excludes concurrent restorations.
func (b *Bot) snapshotBackup() ([]byte, error) {
	if b.leads != nil {
		b.leads.mu.Lock()
		defer b.leads.mu.Unlock()
	}
	if b.favorites != nil {
		b.favorites.mu.Lock()
		defer b.favorites.mu.Unlock()
	}
	if b.visitors != nil {
		b.visitors.mu.Lock()
		defer b.visitors.mu.Unlock()
	}
	var raw []byte
	err := b.metrics.WithExclusiveFile(func() error {
		var e error
		raw, e = adminbackup.Build(b.store.DataDir(), b.store.List())
		return e
	})
	return raw, err
}

func (b *Bot) runDailyBackup(ctx context.Context, now time.Time) error {
	if b.store == nil {
		return nil
	}
	b.backupMu.Lock()
	defer b.backupMu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	name := "auto-" + now.UTC().Format("2006-01-02") + ".zip"
	path := filepath.Join(b.autoBackupDirectory(), name)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := b.snapshotBackup()
	if err != nil {
		return err
	}
	if _, err = adminbackup.Examine(data); err != nil {
		return fmt.Errorf("self-check: %w", err)
	}
	if err = atomicAdminFile(path, data); err != nil {
		return err
	}
	list, err := b.autoBackupList()
	if err != nil {
		return err
	}
	for len(list) > automaticBackupKeep {
		if err := os.Remove(filepath.Join(b.autoBackupDirectory(), list[0])); err != nil {
			return err
		}
		list = list[1:]
	}
	return nil
}

func (b *Bot) autoBackupLoop(ctx context.Context) {
	if err := b.runDailyBackup(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
		log.Printf("Automatic Karty backup: %v", err)
	}
	for ctx.Err() == nil {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := b.runDailyBackup(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
			log.Printf("Automatic Karty backup: %v", err)
		}
	}
}
