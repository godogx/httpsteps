package httpsteps

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

// synchronized keeps exclusive access to the scenario steps.
type synchronized struct {
	mu        sync.Mutex
	locks     map[string]chan struct{}
	onRelease func(lockName string) error
	ctxKey    *struct{ _ int }
}

func newSynchronized(onRelease func(lockName string) error) *synchronized {
	return &synchronized{
		locks:     make(map[string]chan struct{}),
		onRelease: onRelease,
		ctxKey:    new(struct{ _ int }),
	}
}

// acquireLock acquires resource lock for the given key and returns true.
//
// If the lock is already held by another context, it waits for the lock to be released.
// It returns false is the lock is already held by this context.
// This function fails if the context is missing current lock.
func (s *synchronized) acquireLock(ctx context.Context, lockName string) (bool, error) {
	currentLock, ok := ctx.Value(s.ctxKey).(chan struct{})
	if !ok {
		return false, errMissingScenarioLock
	}

	s.mu.Lock()
	lock := s.locks[lockName]

	if lock == nil {
		if s.locks == nil {
			s.locks = make(map[string]chan struct{})
		}

		s.locks[lockName] = currentLock
	}

	s.mu.Unlock()

	// Wait for the alien lock to be released.
	if lock != nil && lock != currentLock {
		<-lock

		return s.acquireLock(ctx, lockName)
	}

	if lock == nil {
		return true, nil
	}

	return false, nil
}

// register adds hooks to scenario context.
func (s *synchronized) register(sc *godog.ScenarioContext) {
	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		lock := make(chan struct{})

		// Adding unique pointer to context to avoid collisions.
		return context.WithValue(ctx, s.ctxKey, lock), nil
	})

	sc.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		s.mu.Lock()
		defer s.mu.Unlock()

		// Releasing locks owned by scenario.
		currentLock, ok := ctx.Value(s.ctxKey).(chan struct{})
		if !ok {
			return ctx, errMissingScenarioLock
		}

		var errs []string

		for key, lock := range s.locks {
			if lock == currentLock {
				delete(s.locks, key)
			}

			if s.onRelease != nil {
				if err := s.onRelease(key); err != nil {
					errs = append(errs, err.Error())
				}
			}
		}

		close(currentLock)

		if len(errs) > 0 {
			return ctx, errors.New(strings.Join(errs, ", ")) // nolint:goerr113
		}

		return ctx, nil
	})
}
