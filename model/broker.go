package model

// BrokerName identifies a supported broker. A broker becomes selectable through
// marketconnector.NewBroker only after a constant is defined here and the
// corresponding package is registered in the factory.
//
// The zero value ("") is not a valid broker; always pass one of the constants
// below (or a custom constant for a runtime-registered broker).
type BrokerName string

const (
	// BrokerAngelOne is the Angel One SmartAPI broker.
	BrokerAngelOne BrokerName = "angelone"

	// BrokerZerodha is the Zerodha Kite Connect broker.
	BrokerZerodha BrokerName = "zerodha"

	// Future brokers should add constants here, e.g.:
	// BrokerUpstox BrokerName = "upstox"
)
