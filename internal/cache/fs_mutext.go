package cache

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// TODO: make sensible
const lockStaleAfter = 10 * time.Minute

type FSMutex interface {
	Lock(lockTryLimit int8) error
	Unlock() 
}

type fsMutex struct {
	lockPath string
	locked bool
}

func (mu *fsMutex) Lock(lockTryLimit int8) error {
	tries := 0
	for {
		tries++
		if int(lockTryLimit) > 0 && tries > int(lockTryLimit) {
			return errors.New("can't acquire lock")
		}

		now := time.Now()

		f, err := os.OpenFile(mu.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			// We acquired the lock. Stamp metadata.
			_, _ = f.WriteString(fmt.Sprintf("%d\n%d\n", os.Getpid(), now.Unix()))
			_ = f.Close()
			mu.locked = true
			return nil
		}

		// If it's some error other than "already exists", bail.
		if !errors.Is(err, os.ErrExist) {
			return err
		}

		// Lock exists: check if it's stale.
		info, statErr := os.Stat(mu.lockPath)
		if statErr != nil {
			// If vanished between calls, just retry.
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return statErr
		}

		if age := time.Since(info.ModTime()); age > lockStaleAfter {
			// TODO: notify user about stale builds
			// Consider stale. Best-effort remove.
			_ = os.Remove(mu.lockPath)
			// Next loop iteration will try to acquire again.
			continue
		}

		// Not stale yet → wait and retry.
		time.Sleep(50 * time.Millisecond)
	}
}

func (mu *fsMutex) Unlock() {
	if !mu.locked {
		return
	}
	_ = os.Remove(mu.lockPath)
	mu.locked = false
}

func NewFSMutex(lockPath string) FSMutex {
	return &fsMutex{lockPath: lockPath, locked: false}
}
