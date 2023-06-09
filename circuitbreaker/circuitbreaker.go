package circuitbreaker

import (
	"circuitbreaker/pkg/literal"
	"sync"
	"time"
)

type Circuitbreaker interface {
	Execute(func() (interface{}, error)) (interface{}, error)
	State() string
}

type Policy int
type State string

const (
	// MaxFails specifies the maximum non-consecutive fails which are allowed
	// in the "Closed" state before the state is changd to "Open".
	MaxFails Policy = iota

	// MaxConsecutiveFails specifies the maximum consecutive fails which are allowed
	// in the "Closed" state before the state is changed to "Open".
	MaxConsecutiveFails
)

const (
	open     State = "open"
	closed   State = "closed"
	halfOpen State = "half-open"
)

type ExtraOptions struct {
	// Policy determines how the fails should be incremented
	Policy Policy

	// MaxFails specifies the maximum non-consecutive fails which are allowed
	// in the "Closed" state before the state is changd to "Open".
	MaxFails *uint64

	// MaxConsecutiveFails specifies the maximum consecutive fails which are allowed
	// in the "Closed" state before the state is changed to "Open".
	MaxConsecutiveFails *uint64

	OpenInterval *time.Duration
}

type circuitbreaker struct {
	policy              Policy
	maxFails            uint64
	maxConsecutiveFails uint64
	openInterval        time.Duration

	// fails is the number of failed requets for the current "Closed" state,
	// resets after a successful transition from half-open to closed.
	fails uint64

	// current state of the circuit
	state State

	// openChannel handles the event transfer mechanism for the open state
	openChannel chan struct{}

	mutex sync.Mutex
}

func New(opts ...ExtraOptions) Circuitbreaker {
	var opt ExtraOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	if opt.MaxFails == nil {
		opt.MaxFails = literal.ToPointer(uint64(5))
	}

	if opt.MaxConsecutiveFails == nil {
		opt.MaxConsecutiveFails = literal.ToPointer(uint64(5))
	}

	if opt.OpenInterval == nil {
		opt.OpenInterval = literal.ToPointer(5 * time.Second)
	}

	cb := &circuitbreaker{
		policy:              opt.Policy,
		maxFails:            *opt.MaxFails,
		maxConsecutiveFails: *opt.MaxConsecutiveFails,
		openInterval:        *opt.OpenInterval,
		openChannel:         make(chan struct{}),
	}

	go cb.openWatcher()

	return cb
}

func (cb *circuitbreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	err := cb.doPreRequest()
	if err != nil {
		return nil, err
	}

	res, err := req()

	err = cb.doPostRequest(err)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (cb *circuitbreaker) State() string {
	return string(cb.state)
}

func (cb *circuitbreaker) doPreRequest() error {
	if cb.state == open {
		return ErrRefuse
	}

	return nil
}

func (cb *circuitbreaker) doPostRequest(err error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err == nil {
		if cb.policy == MaxConsecutiveFails {
			cb.fails = 0
		}
		cb.state = closed
		return nil
	}

	if cb.state == halfOpen {
		cb.state = open
		cb.openChannel <- struct{}{}
		return err
	}

	cb.fails++
	if cb.failsExcceededThreshod() {
		cb.state = open
		cb.openChannel <- struct{}{}
	}

	return err
}

func (cb *circuitbreaker) failsExcceededThreshod() bool {
	switch cb.policy {
	case MaxConsecutiveFails:
		return cb.fails >= cb.maxConsecutiveFails
	case MaxFails:
		return cb.fails >= cb.maxFails
	default:
		return false
	}
}

func (cb *circuitbreaker) openWatcher() {
	for range cb.openChannel {
		time.Sleep(cb.openInterval)
		cb.mutex.Lock()
		cb.state = halfOpen
		cb.fails = 0
		cb.mutex.Unlock()
	}
}
