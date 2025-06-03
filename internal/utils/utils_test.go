package utils_test

import (
	"sync"
	"testing"
	"time"

	"github.com/speakeasy-api/speakeasy/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestSingleSlotQueue(t *testing.T) {
	log := []int{}
	var logMu sync.Mutex

	bump := func() {
		logMu.Lock()
		log = append(log, -1)
		logMu.Unlock()
	}

	queue := utils.SingleSlotQueue(bump)

	fn := func(id int) func() {
		return func() {
			logMu.Lock()
			log = append(log, id)
			logMu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			queue(fn(id))
			wg.Done()
		}(i)
		time.Sleep(10 * time.Millisecond)
	}
	wg.Wait()

	queue(fn(3))

	// f0 got executed without bump since the queue was initially empty
	// both f1 and f2 were queued with a bump (-1)
	// f2 replaced f1 in the queue before it could execute
	// f3 got executed without bump since the queue had been cleared
	logMu.Lock()
	assert.Equal(t, []int{0, -1, -1, 2, 3}, log)
	logMu.Unlock()
}
