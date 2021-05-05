package serversets

import (
	"encoding/json"
	"log"
	"net"
	"sort"
	"strconv"
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

	// lock for read/writing the endpoints slice
	lock      sync.RWMutex
	endpoints []Entity
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

	keys, watchEvents, err := watch.watch(connection)
	if err != nil {
		defer connection.Close()
		return nil, err
	}

	watch.endpoints, err = watch.updateEndpoints(connection, keys)
	if err != nil {
		defer connection.Close()
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

				watch.endpoints, err = watch.updateEndpoints(connection, keys)
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
				keys, watchEvents, err = watch.watch(connection)
				if err != nil {
					log.Printf("unable to rewatch endpoint after znode event: %v", err)
					break
				}

				endpoints, err := watch.updateEndpoints(connection, keys)
				if err != nil {
					log.Printf("unable to updated endpoint list after znode event: %v", err)
					watchEvents = nil
					break
				}

				watch.lock.Lock()
				watch.endpoints = endpoints
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

// Endpoints returns a slice of the current list of servers/endpoints associated with this watch.
func (w *Watch) Endpoints() []string {
	w.lock.RLock()
	defer w.lock.RUnlock()
	endpoints := make([]string, 0, len(w.endpoints))
	for _, e := range w.endpoints {
		endpoints = append(endpoints, net.JoinHostPort(e.ServiceEndpoint.Host, strconv.Itoa(e.ServiceEndpoint.Port)))
	}

	sort.Strings(endpoints)
	return endpoints
}

// EndpointEntities returns a slice of the current list of Entites associated with this watch, collected at the last event.
func (w *Watch) EndpointEntities() []Entity {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.endpoints
}

// Event returns the event channel. This channel will get an object
// whenever something changes with the list of endpoints.
func (w *Watch) Event() <-chan struct{} {
	return w.event
}

// Close blocks until the underlying Zookeeper connection is closed.
func (w *Watch) Close() {
	select {
	case <-w.done:
		w.wg.Wait()
		return
	default:
	}

	close(w.done)
	w.wg.Wait()

	// the goroutine watching for events must be terminted
	// before we close this channel, since it might still be sending events.
	close(w.event)
	return
}

// IsClosed returns if this watch has been closed. This is a way for libraries wrapping
// this package to know if their underlying watch is closed and should stop looking for events.
func (w *Watch) IsClosed() bool {
	select {
	case <-w.done:
		return true
	default:
	}

	return false
}

// watch creates the actual Zookeeper watch.
func (w *Watch) watch(connection *zk.Conn) ([]string, <-chan zk.Event, error) {
	err := w.serverSet.checkExistsFullPath(connection)
	if err != nil {
		return nil, nil, err
	}

	children, _, events, err := connection.ChildrenW(w.serverSet.directoryPath())
	return children, events, err
}

func (w *Watch) updateEndpoints(connection *zk.Conn, keys []string) ([]Entity, error) {
	endpoints := make([]Entity, 0, len(keys))

	for _, k := range keys {
		if !strings.HasPrefix(k, MemberPrefix) {
			continue
		}

		e, err := w.getEndpoint(connection, k)
		if err != nil {
			return nil, err
		}

		if e == nil {
			// znode not found
			continue
		}

		if e.Status == statusAlive {
			endpoints = append(endpoints, *e)
		}
	}

	return endpoints, nil

}

func (w *Watch) getEndpoint(connection *zk.Conn, key string) (*Entity, error) {

	data, _, err := connection.Get(w.serverSet.directoryPath() + "/" + key)
	if err == zk.ErrNoNode {
		return nil, nil
	}

	if err != nil {
		// most likely some sort of zk connection error
		return nil, err
	}

	// Found this SOH check while browsing the docker/libkv source
	// https://github.com/docker/libkv/commit/035e8143a336ceb29760c07278ef930f49767377

	// FIXME handle very rare cases where Get returns the
	// SOH control character instead of the actual value
	if string(data) == SOH {
		return w.getEndpoint(connection, key)
	}

	e := &Entity{}
	err = json.Unmarshal(data, &e)
	if err != nil {
		return nil, err
	}

	return e, err
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
