package adversarial

import (
	"fmt"
	"sync/atomic"
	"time"
)

var runIDCounter atomic.Uint64

func defaultRunID(t time.Time) string {
	return "adv-" + t.UTC().Format("20060102T150405.000000000Z") + fmt.Sprintf("-%06d", runIDCounter.Add(1))
}
