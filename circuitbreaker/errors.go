package circuitbreaker

import "errors"

var (
	ErrRefuse = errors.New("request refused. the circuit is open")
)
