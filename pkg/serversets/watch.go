package serversets

import (
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
)

const (
	// SOH control character
	SOH = "\x01"
)

// A Watch keeps tabs on a server set in Zookeeper and notifies
// via the Event() channel when the list of servers changes.
// The list of servers is updated automatically and will be up to date when the Event is sent.
type Watch struct {
	serverSet *ServerSet

	LastEvent  time.Time
	EventCount int
	event      chan struct{}

	done chan struct{} // used for closing
	wg   sync.WaitGroup

	// lock for read/writing the records slice
	lock    sync.RWMutex
	records []ZKRecord
}

// Watch creates a new watch on this server set. Changes to the set will
// update watch.Endpoints() and an event will be sent to watch.Event right after that happens.
func (ss *ServerSet) Watch() (*Watch, error) {
	watch := &Watch{
		serverSet: ss,
		done:      make(chan struct{}),
		event:     make(chan struct{}, 1),
	}

	connection, sessionEvents, err := ss.connectToZookeeper()
	if err != nil {
		return nil, err
	}

	// Ensure path exists before watching
	err = ss.createFullPath(connection)
	if err != nil {
		connection.Close()
		return nil, err
	}

	keys, watchEvents, err := watch.watch(connection)
	if err != nil {
		connection.Close()
		return nil, err
	}

	watch.records, err = watch.updateRecords(connection, keys)
	if err != nil {
		connection.Close()
		return nil, err
	}

	// spawn a goroutine to deal with session disconnects and watch events
	watch.wg.Add(1)
	go func() {
		defer watch.wg.Done()
		for {
			// Connection was closed try to reconnect
			if connection == nil {
				time.Sleep(5 * time.Second)
				connection, sessionEvents, err = ss.connectToZookeeper()
				if err != nil {
					log.Printf("unable to reconnect to zookeeper after session expired: %v", err)
					watchEvents = nil
					continue
				}
			}

			// Starting the watch has failed, retry
			if watchEvents == nil {
				keys, watchEvents, err = watch.watch(connection)
				if err != nil {
					time.Sleep(1 * time.Second)
					log.Printf("unable to reregister endpoint after session expired: %v", err)
					continue
				}

				watch.records, err = watch.updateRecords(connection, keys)
				if err != nil {
					time.Sleep(1 * time.Second)
					log.Printf("unable to update endpoint list after session expired: %v", err)
					watchEvents = nil
					continue
				}
				watch.triggerEvent()
			}

			select {
			case event := <-sessionEvents:
				if event.Type == zk.EventSession && event.State == zk.StateExpired {
					connection.Close()
					connection = nil
				}
			case <-watchEvents:
				watchEvents = nil

				keys, watchEvents, err = watch.watch(connection)
				if err != nil {
					log.Printf("unable to reregister watch after znode event: %v", err)
					break
				}

				records, err := watch.updateRecords(connection, keys)
				if err != nil {
					log.Printf("unable to update endpoint list after znode event: %v", err)
					watchEvents = nil
					break
				}

				watch.lock.Lock()
				watch.records = records
				watch.lock.Unlock()

				watch.triggerEvent()
			case <-watch.done:
				connection.Close()
				return
			}
		}
	}()

	return watch, nil
}

// Endpoints returns a slice of the current list of endpoints associated with this watch,
// sorted in alphabetical order.
func (w *Watch) Endpoints() []string {
	w.lock.RLock()
	defer w.lock.RUnlock()
	endpoints := make([]string, 0, len(w.records))
	for _, r := range w.records {
		endpoints = append(endpoints, r.Endpoint())
	}

	sort.Strings(endpoints)
	return endpoints
}

// EndpointRecords returns a slice of the current list of ZKRecords associated with this watch.
func (w *Watch) EndpointRecords() []ZKRecord {
	w.lock.RLock()
	defer w.lock.RUnlock()

	records := make([]ZKRecord, len(w.records))
	copy(records, w.records)
	return records
}

// EndpointEntities returns a slice of the current list of Entities associated with this watch, collected at the last event.
// For non-Finagle records, entries that cannot be converted to Entity are skipped.
func (w *Watch) EndpointEntities() []Entity {
	w.lock.RLock()
	defer w.lock.RUnlock()

	entities := make([]Entity, 0, len(w.records))
	for _, r := range w.records {
		if fr, ok := r.(*FinagleRecord); ok {
			entities = append(entities, *fr)
		}
	}

	return entities
}

// Event returns the event channel. This channel will get an object
// whenever something changes with the list of endpoints.
func (w *Watch) Event() <-chan struct{} {
	return w.event
}

// IsClosed returns true if this watch is closed.
func (w *Watch) IsClosed() bool {
	select {
	case <-w.done:
		return true
	default:
	}

	return false
}

// Close closes the watch.
func (w *Watch) Close() {
	select {
	case <-w.done:
		return
	default:
	}

	close(w.done)
	w.wg.Wait()
}

func (w *Watch) watch(connection *zk.Conn) ([]string, <-chan zk.Event, error) {
	children, _, events, err := connection.ChildrenW(w.serverSet.directoryPath())
	return children, events, err
}

func (w *Watch) updateRecords(connection *zk.Conn, keys []string) ([]ZKRecord, error) {
	records := make([]ZKRecord, 0, len(keys))

	prefix := w.serverSet.ZKFmt.Prefix()
	for _, k := range keys {
		if !strings.HasPrefix(k, prefix) {
			continue
		}

		r, err := w.getRecord(connection, k)
		if err != nil {
			return nil, err
		}

		if r == nil {
			// znode not found
			continue
		}

		if r.IsAlive() {
			records = append(records, r)
		}
	}

	return records, nil
}

func (w *Watch) getRecord(connection *zk.Conn, key string) (ZKRecord, error) {
	data, _, err := connection.Get(w.serverSet.directoryPath() + "/" + key)
	if err == zk.ErrNoNode {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	// FIXME handle very rare cases where Get returns the
	// SOH control character instead of the actual value
	if string(data) == SOH {
		return w.getRecord(connection, key)
	}

	r, err := w.serverSet.ZKFmt.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// triggerEvent will queue up something in the Event channel if there isn't already something there.
func (w *Watch) triggerEvent() {
	w.EventCount++
	w.LastEvent = time.Now()

	select {
	case w.event <- struct{}{}:
	default:
	}
}
