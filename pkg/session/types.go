package session

import (
	"io"

	"github.com/shayne/derpcat/pkg/telemetry"
)

type ListenConfig struct {
	Emitter       *telemetry.Emitter
	TokenSink     chan<- string
	StdioOut      io.Writer
	ForceRelay    bool
	UsePublicDERP bool
}

type SendConfig struct {
	Token         string
	Emitter       *telemetry.Emitter
	StdioIn       io.Reader
	ForceRelay    bool
	UsePublicDERP bool
}

type ShareConfig struct {
	Emitter       *telemetry.Emitter
	TokenSink     chan<- string
	TargetAddr    string
	ForceRelay    bool
	UsePublicDERP bool
}

type OpenConfig struct {
	Token         string
	BindAddr      string
	BindAddrSink  chan<- string
	Emitter       *telemetry.Emitter
	ForceRelay    bool
	UsePublicDERP bool
}

type State string

const (
	StateWaiting  State = "waiting-for-claim"
	StateClaimed  State = "claimed"
	StateProbing  State = "probing-direct"
	StateDirect   State = "connected-direct"
	StateRelay    State = "connected-relay"
	StateComplete State = "stream-complete"
)

func emitStatus(emitter *telemetry.Emitter, state State) {
	if emitter != nil {
		emitter.Status(string(state))
	}
}
