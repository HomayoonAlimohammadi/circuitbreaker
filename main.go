package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"circuitbreaker/circuitbreaker"
	"circuitbreaker/pkg/literal"
)

func main() {
	cbOpts := circuitbreaker.ExtraOptions{
		Policy:              circuitbreaker.MaxFails,
		MaxFails:            literal.ToPointer(uint64(5)),
		MaxConsecutiveFails: literal.ToPointer(uint64(5)),
		OpenInterval:        literal.ToPointer(50 * time.Millisecond),
	}
	cb := circuitbreaker.New(cbOpts)
	wg := &sync.WaitGroup{}
	for i := 1; i < 30; i += 1 {
		wg.Add(1)
		go makeServiceCall(i, cb, wg)
		time.Sleep(10 * time.Millisecond)
	}

	log.Println("sent all the requests")
	wg.Wait()
	log.Println("got all the responses, exiting.")
}

func serviceMethod(id int) (string, error) {
	if val := rand.Float64(); val <= 0.5 {
		return "", errors.New("failed")
	}
	return fmt.Sprintf("[id: %d] done.", id), nil
}

func makeServiceCall(id int, cb circuitbreaker.Circuitbreaker, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := cb.Execute(func() (interface{}, error) {
		return serviceMethod(id)
	})
	if err != nil {
		log.Printf("[id %d] got err: %s", id, err.Error())
	} else {
		log.Printf("[id %d] success: %s", id, resp)
	}
}
