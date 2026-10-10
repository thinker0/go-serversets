package serversets

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
)

// An Endpoint is a service (host and port) registered on Zookeeper
// to be discovered by clients/watchers.
type Endpoint struct {
	*ServerSet
	PingRate   time.Duration // default/initial is 1 second
	CloseEvent chan struct{}

	done chan struct{}
	wg   sync.WaitGroup

	host string
	port int

	key   string
	ping  func() error
	alive bool
}

// RegisterEndpoint registers a host and port as alive. It creates the appropriate
// Zookeeper nodes and watchers will be notified this server/endpoint is available.
func (ss *ServerSet) RegisterEndpoint(host string, port int, ping func() error) (*Endpoint, error) {
	endpoint := &Endpoint{
		ServerSet:  ss,
		PingRate:   time.Second,
		CloseEvent: make(chan struct{}, 1),
		done:       make(chan struct{}),
		host:       host,
		port:       port,
		ping:       ping,
		alive:      true,
	}

	if ping != nil {
		endpoint.alive = endpoint.ping() == nil
	}

	connection, sessionEvents, err := ss.connectToZookeeper()
	if err != nil {
		return nil, err
	}

	err = endpoint.update(connection)
	if err != nil {
		connection.Close()
		return nil, err
	}

	// spawn goroutine to deal with connection/session issues.
	endpoint.wg.Add(1)
	go func() {
		defer endpoint.wg.Done()
		for {
			select {
			case event := <-sessionEvents:
				if event.Type == zk.EventSession && event.State == zk.StateExpired {
					if connection != nil {
						connection.Close()
						connection = nil
					}
				}
			case <-endpoint.done:
				if connection != nil {
					connection.Close()
				}
				return
			}

			for connection == nil {
				select {
				case <-endpoint.done:
					return
				default:
				}

				connection, sessionEvents, err = ss.connectToZookeeper()
				if err != nil {
					log.Printf("unable to reconnect to zookeeper after session expired: %v", err)
					connection = nil
					time.Sleep(1 * time.Second)
					continue
				}

				err = endpoint.update(connection)
				if err != nil {
					log.Printf("unable to reregister endpoint after session expired: %v", err)
					connection.Close()
					connection = nil
					time.Sleep(1 * time.Second)
					continue
				}
			}
		}
	}()

	if ping != nil {
		endpoint.wg.Add(1)
		go func() {
			defer endpoint.wg.Done()
			ticker := time.NewTicker(endpoint.PingRate)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
				case <-endpoint.done:
					return
				}

				alive := endpoint.ping() == nil
				if alive != endpoint.alive {
					endpoint.alive = alive
					if connection != nil {
						err := endpoint.update(connection)
						if err != nil {
							log.Printf("unable to update endpoint status after ping change: %v", err)
						}
					}
				}
			}
		}()
	}

	return endpoint, nil
}

// Close blocks until the client connection to Zookeeper is closed.
// If already called, will simply return, even if in the process of closing.
func (ep *Endpoint) Close() {
	select {
	case <-ep.done:
		return
	default:
	}

	close(ep.done)
	ep.wg.Wait()
	ep.CloseEvent <- struct{}{}
}

func (ep *Endpoint) update(connection *zk.Conn) error {
	// don't create/remove the node if we're dead
	if !ep.alive {
		if ep.key != "" {
			err := connection.Delete(ep.key, 0)
			ep.key = ""
			return err
		}

		return nil
	}

	record := ep.ServerSet.ZKFmt.Create(ep.host, ep.port)
	recordData, err := record.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal zk record: %w", err)
	}

	ep.key, err = ep.ServerSet.registerEndpoint(connection, recordData)
	return err
}

func (ss *ServerSet) registerEndpoint(connection *zk.Conn, data []byte) (string, error) {
	err := ss.createFullPath(connection)
	if err != nil {
		return "", err
	}

	return connection.Create(
		ss.directoryPath()+"/"+ss.ZKFmt.Prefix(),
		data,
		zk.FlagEphemeral|zk.FlagSequence,
		zk.WorldACL(zk.PermAll))
}
