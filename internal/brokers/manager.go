// Package brokers contains the broker object definitions, implementations and manager that will be used by the daemon
// for authentication.
package brokers

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/ubuntu/authd/internal/log"
	"github.com/ubuntu/decorate"
)

// Manager is the object that manages the available brokers and the session->broker and user->broker relationships.
type Manager struct {
	brokers      map[string]*Broker
	brokersOrder []string

	usersToBroker   map[string]*Broker
	usersToBrokerMu sync.RWMutex

	transactionsToBroker   map[string]*Broker
	transactionsToBrokerMu sync.RWMutex

	cleanup func()
}

// Option is the function signature used to tweak the daemon creation.
type Option func(*options)

type options struct {
	rootDir      string
	brokerCfgDir string
}

// WithRootDir uses a dedicated path for our root.
func WithRootDir(p string) func(o *options) {
	return func(o *options) {
		o.rootDir = p
	}
}

// NewManager creates a new broker manager object.
func NewManager(ctx context.Context, configuredBrokers []string, args ...Option) (m *Manager, err error) {
	defer decorate.OnError(&err /*i18n.G(*/, "can't create brokers detection object") //)

	log.Debug(ctx, "Building broker detection")

	// Set default options.
	opts := options{
		rootDir:      "/",
		brokerCfgDir: "etc/authd/broker.d",
	}
	// Apply given args.
	for _, f := range args {
		f(&opts)
	}

	brokersConfPath := filepath.Join(opts.rootDir, opts.brokerCfgDir)

	brokersConfPathWithExample, cleanup, err := useExampleBrokers()
	if err != nil {
		return nil, err
	} else if brokersConfPathWithExample != "" {
		brokersConfPath = brokersConfPathWithExample
	}

	// Connect to the system bus
	// Don't call dbus.SystemBus which caches globally system dbus (issues in tests)
	bus, err := dbus.ConnectSystemBus()
	if err != nil {
		return m, err
	}

	// Select all brokers in ascii order if none is configured
	if len(configuredBrokers) == 0 {
		log.Debug(ctx, "Auto-detecting brokers")

		entries, err := os.ReadDir(brokersConfPath)
		if errors.Is(err, fs.ErrNotExist) {
			log.Warningf(ctx, "Broker configuration directory %q does not exist, only local broker will be available", brokersConfPath)
		} else if err != nil {
			return m, fmt.Errorf("could not read brokers directory to detect brokers: %v", err)
		}

		for _, e := range entries {
			if !e.Type().IsRegular() {
				continue
			}
			configuredBrokers = append(configuredBrokers, e.Name())
		}
	}

	brokers := make(map[string]*Broker)
	var brokersOrder []string

	// First broker is always the local one.
	b, err := newBroker(ctx, localBrokerName, "", nil)
	brokersOrder = append(brokersOrder, b.ID)
	brokers[b.ID] = &b

	// Load brokers configuration
	for _, n := range configuredBrokers {
		configFile := filepath.Join(brokersConfPath, n)
		b, err := newBroker(ctx, n, configFile, bus)
		if err != nil {
			log.Warningf(ctx, "Skipping broker %q is not correctly configured: %v", n, err)
			continue
		}
		brokersOrder = append(brokersOrder, b.ID)
		brokers[b.ID] = &b
	}

	return &Manager{
		brokers:      brokers,
		brokersOrder: brokersOrder,

		usersToBroker:        make(map[string]*Broker),
		transactionsToBroker: make(map[string]*Broker),

		cleanup: cleanup,
	}, nil
}

// AvailableBrokers returns currently loaded and available brokers in preference order.
func (m *Manager) AvailableBrokers() (r []*Broker) {
	for _, id := range m.brokersOrder {
		r = append(r, m.brokers[id])
	}
	return r
}

// SetDefaultBrokerForUser memorizes which broker was used for which user.
func (m *Manager) SetDefaultBrokerForUser(brokerID, username string) error {
	broker, err := m.brokerFromID(brokerID)
	if err != nil {
		return fmt.Errorf("invalid broker: %v", err)
	}

	m.usersToBrokerMu.Lock()
	defer m.usersToBrokerMu.Unlock()
	m.usersToBroker[username] = broker
	return nil
}

// BrokerForUser returns any previously selected broker for a given user, if any.
func (m *Manager) BrokerForUser(username string) (broker *Broker) {
	m.usersToBrokerMu.RLock()
	defer m.usersToBrokerMu.RUnlock()
	return m.usersToBroker[username]
}

// BrokerFromSessionID returns broker currently in use for a given transaction sessionID.
func (m *Manager) BrokerFromSessionID(id string) (broker *Broker, err error) {
	m.transactionsToBrokerMu.RLock()
	defer m.transactionsToBrokerMu.RUnlock()

	// no session ID means local broker
	if id == "" {
		return m.brokerFromID(localBrokerName)
	}

	broker, exists := m.transactionsToBroker[id]
	if !exists {
		return nil, fmt.Errorf("no broker found for session %q", id)
	}

	return broker, nil
}

// NewSession create a new session for the broker and store the sesssionID on the manager.
func (m *Manager) NewSession(brokerID, username, lang string) (sessionID string, encryptionKey string, err error) {
	broker, err := m.brokerFromID(brokerID)
	if err != nil {
		return "", "", fmt.Errorf("invalid broker: %v", err)
	}

	sessionID, encryptionKey, err = broker.newSession(context.Background(), username, lang)
	if err != nil {
		return "", "", err
	}

	m.transactionsToBrokerMu.Lock()
	defer m.transactionsToBrokerMu.Unlock()
	m.transactionsToBroker[sessionID] = broker
	return sessionID, encryptionKey, nil
}

// EndSession signals the end of the session to the broker associated with the sessionID and then removes the
// session -> broker mapping.
func (m *Manager) EndSession(sessionID string) error {
	b, err := m.BrokerFromSessionID(sessionID)
	if err != nil {
		return err
	}

	if err = b.endSession(context.Background(), sessionID); err != nil {
		return err
	}

	m.transactionsToBrokerMu.Lock()
	delete(m.transactionsToBroker, sessionID)
	m.transactionsToBrokerMu.Unlock()
	return nil
}

// brokerFromID returns the broker matching this brokerID.
func (m *Manager) brokerFromID(id string) (broker *Broker, err error) {
	broker, exists := m.brokers[id]
	if !exists {
		return nil, fmt.Errorf("no broker found matching %q", id)
	}

	return broker, nil
}
