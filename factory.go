package marketconnector

import (
	"fmt"
	"sync"

	"github.com/sunnyme20/marketconnector/brokers/angelone"
	"github.com/sunnyme20/marketconnector/brokers/zerodha"
	"github.com/sunnyme20/marketconnector/model"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[model.BrokerName]func() Broker)
)

func init() {
	Register(model.BrokerAngelOne, func() Broker { return &angelone.Angelone{} })
	Register(model.BrokerZerodha, func() Broker { return &zerodha.Zerodha{} })
}

// Register makes a broker available to [NewBroker] under the given name. The
// builder is a function that constructs a new Broker instance. Register panics
// if the name is already registered or if builder is nil.
//
// It is safe to call concurrently with [NewBroker].
func Register(name model.BrokerName, builder func() Broker) {
	if builder == nil {
		panic("marketconnector: Register called with nil builder")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic("marketconnector: Register called twice for broker " + name)
	}
	registry[name] = builder
}

// NewBroker returns a new broker instance registered under the given name,
// or an error if no such broker is registered.
func NewBroker(name model.BrokerName) (Broker, error) {
	registryMu.RLock()
	builder, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("marketconnector: unknown broker %q", name)
	}
	return builder(), nil
}
