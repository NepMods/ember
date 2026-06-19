package ember

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// ModelEvent represents a model lifecycle event.
type ModelEvent string

const (
	// EventCreating is fired before a model is created.
	EventCreating ModelEvent = "creating"
	// EventCreated is fired after a model is created.
	EventCreated ModelEvent = "created"
	// EventUpdating is fired before a model is updated.
	EventUpdating ModelEvent = "updating"
	// EventUpdated is fired after a model is updated.
	EventUpdated ModelEvent = "updated"
	// EventSaving is fired before a model is saved (created or updated).
	EventSaving ModelEvent = "saving"
	// EventSaved is fired after a model is saved.
	EventSaved ModelEvent = "saved"
	// EventDeleting is fired before a model is deleted.
	EventDeleting ModelEvent = "deleting"
	// EventDeleted is fired after a model is deleted.
	EventDeleted ModelEvent = "deleted"
	// EventRestoring is fired before a soft-deleted model is restored.
	EventRestoring ModelEvent = "restoring"
	// EventRestored is fired after a soft-deleted model is restored.
	EventRestored ModelEvent = "restored"
)

// ─── Individual observer interfaces ──────────────────────────────────────────

// CreatingObserver is notified before a model is created.
type CreatingObserver interface {
	Creating(ctx context.Context, model interface{}) error
}

// CreatedObserver is notified after a model is created.
type CreatedObserver interface {
	Created(ctx context.Context, model interface{}) error
}

// UpdatingObserver is notified before a model is updated.
type UpdatingObserver interface {
	Updating(ctx context.Context, model interface{}) error
}

// UpdatedObserver is notified after a model is updated.
type UpdatedObserver interface {
	Updated(ctx context.Context, model interface{}) error
}

// SavingObserver is notified before a model is saved.
type SavingObserver interface {
	Saving(ctx context.Context, model interface{}) error
}

// SavedObserver is notified after a model is saved.
type SavedObserver interface {
	Saved(ctx context.Context, model interface{}) error
}

// DeletingObserver is notified before a model is deleted.
type DeletingObserver interface {
	Deleting(ctx context.Context, model interface{}) error
}

// DeletedObserver is notified after a model is deleted.
type DeletedObserver interface {
	Deleted(ctx context.Context, model interface{}) error
}

// RestoringObserver is notified before a soft-deleted model is restored.
type RestoringObserver interface {
	Restoring(ctx context.Context, model interface{}) error
}

// RestoredObserver is notified after a soft-deleted model is restored.
type RestoredObserver interface {
	Restored(ctx context.Context, model interface{}) error
}

// Observer is the combined interface for all lifecycle events.
type Observer interface {
	CreatingObserver
	CreatedObserver
	UpdatingObserver
	UpdatedObserver
	SavingObserver
	SavedObserver
	DeletingObserver
	DeletedObserver
	RestoringObserver
	RestoredObserver
}

// BaseObserver provides a no-op default for every method in Observer.
type BaseObserver struct{}

// BaseObserver.Creating
func (o *BaseObserver) Creating(ctx context.Context, model interface{}) error { return nil }

// Created
func (o *BaseObserver) Created(ctx context.Context, model interface{}) error { return nil }

// Updating
func (o *BaseObserver) Updating(ctx context.Context, model interface{}) error { return nil }

// Updated
func (o *BaseObserver) Updated(ctx context.Context, model interface{}) error { return nil }

// Saving
func (o *BaseObserver) Saving(ctx context.Context, model interface{}) error { return nil }

// Saved
func (o *BaseObserver) Saved(ctx context.Context, model interface{}) error { return nil }

// Deleting
func (o *BaseObserver) Deleting(ctx context.Context, model interface{}) error { return nil }

// Deleted
func (o *BaseObserver) Deleted(ctx context.Context, model interface{}) error { return nil }

// Restoring
func (o *BaseObserver) Restoring(ctx context.Context, model interface{}) error { return nil }

// Restored
func (o *BaseObserver) Restored(ctx context.Context, model interface{}) error { return nil }

// ObserverFunc is a function-based observer.
type ObserverFunc func(ctx context.Context, model interface{}) error

// ObserverFuncs allows registering individual observer functions.
type ObserverFuncs struct {
	CreatingFunc  ObserverFunc
	CreatedFunc   ObserverFunc
	UpdatingFunc  ObserverFunc
	UpdatedFunc   ObserverFunc
	SavingFunc    ObserverFunc
	SavedFunc     ObserverFunc
	DeletingFunc  ObserverFunc
	DeletedFunc   ObserverFunc
	RestoringFunc ObserverFunc
	RestoredFunc  ObserverFunc
}

// ObserverFuncs.Creating
func (o *ObserverFuncs) Creating(ctx context.Context, m interface{}) error {
	return callIfSet(o.CreatingFunc, ctx, m)
}

// Created
func (o *ObserverFuncs) Created(ctx context.Context, m interface{}) error {
	return callIfSet(o.CreatedFunc, ctx, m)
}

// Updating
func (o *ObserverFuncs) Updating(ctx context.Context, m interface{}) error {
	return callIfSet(o.UpdatingFunc, ctx, m)
}

// Updated
func (o *ObserverFuncs) Updated(ctx context.Context, m interface{}) error {
	return callIfSet(o.UpdatedFunc, ctx, m)
}

// Saving
func (o *ObserverFuncs) Saving(ctx context.Context, m interface{}) error {
	return callIfSet(o.SavingFunc, ctx, m)
}

// Saved
func (o *ObserverFuncs) Saved(ctx context.Context, m interface{}) error {
	return callIfSet(o.SavedFunc, ctx, m)
}

// Deleting
func (o *ObserverFuncs) Deleting(ctx context.Context, m interface{}) error {
	return callIfSet(o.DeletingFunc, ctx, m)
}

// Deleted
func (o *ObserverFuncs) Deleted(ctx context.Context, m interface{}) error {
	return callIfSet(o.DeletedFunc, ctx, m)
}

// Restoring
func (o *ObserverFuncs) Restoring(ctx context.Context, m interface{}) error {
	return callIfSet(o.RestoringFunc, ctx, m)
}

// Restored
func (o *ObserverFuncs) Restored(ctx context.Context, m interface{}) error {
	return callIfSet(o.RestoredFunc, ctx, m)
}

func callIfSet(fn ObserverFunc, ctx context.Context, model interface{}) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, model)
}

// ─── EventDispatcher ──────────────────────────────────────────────────────────

// EventDispatcher manages model lifecycle observers.
type EventDispatcher struct {
	mu        sync.RWMutex
	observers map[reflect.Type][]interface{}
	global    []interface{}
}

// NewEventDispatcher creates a new EventDispatcher.
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		observers: make(map[reflect.Type][]interface{}),
	}
}

// Observe registers an observer for a specific model type.
func (ed *EventDispatcher) Observe(model interface{}, obs interface{}) {
	if model == nil {
		return
	}
	ed.mu.Lock()
	defer ed.mu.Unlock()

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ed.observers[t] = append(ed.observers[t], obs)
}

// ObserveAll registers a global observer for all model types.
func (ed *EventDispatcher) ObserveAll(obs interface{}) {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	ed.global = append(ed.global, obs)
}

// Fire dispatches a model event to all registered observers.
func (ed *EventDispatcher) Fire(ctx context.Context, event ModelEvent, model interface{}) error {
	if model == nil {
		return nil
	}
	ed.mu.RLock()
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	typeObservers := make([]interface{}, len(ed.observers[t]))
	copy(typeObservers, ed.observers[t])
	globalObservers := make([]interface{}, len(ed.global))
	copy(globalObservers, ed.global)
	ed.mu.RUnlock()

	for _, obs := range typeObservers {
		if obs == nil {
			continue
		}
		if err := safeFireObserver(obs, event, ctx, model); err != nil {
			return err
		}
	}

	for _, obs := range globalObservers {
		if obs == nil {
			continue
		}
		if err := safeFireObserver(obs, event, ctx, model); err != nil {
			return err
		}
	}

	return nil
}

func safeFireObserver(obs interface{}, event ModelEvent, ctx context.Context, model interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ember: observer panic: %v", r)
		}
	}()
	switch event {
	case EventCreating:
		if o, ok := obs.(CreatingObserver); ok {
			return o.Creating(ctx, model)
		}
	case EventCreated:
		if o, ok := obs.(CreatedObserver); ok {
			return o.Created(ctx, model)
		}
	case EventUpdating:
		if o, ok := obs.(UpdatingObserver); ok {
			return o.Updating(ctx, model)
		}
	case EventUpdated:
		if o, ok := obs.(UpdatedObserver); ok {
			return o.Updated(ctx, model)
		}
	case EventSaving:
		if o, ok := obs.(SavingObserver); ok {
			return o.Saving(ctx, model)
		}
	case EventSaved:
		if o, ok := obs.(SavedObserver); ok {
			return o.Saved(ctx, model)
		}
	case EventDeleting:
		if o, ok := obs.(DeletingObserver); ok {
			return o.Deleting(ctx, model)
		}
	case EventDeleted:
		if o, ok := obs.(DeletedObserver); ok {
			return o.Deleted(ctx, model)
		}
	case EventRestoring:
		if o, ok := obs.(RestoringObserver); ok {
			return o.Restoring(ctx, model)
		}
	case EventRestored:
		if o, ok := obs.(RestoredObserver); ok {
			return o.Restored(ctx, model)
		}
	}
	return nil
}
