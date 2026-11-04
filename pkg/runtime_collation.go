package quint

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Collators retain mutable comparison iterators. Each normalization or
// serialization owns one lease; the bounded idle pool only shares them between
// completed operations. Acquisition never waits, including recursive display.
var runtimeCollators = runtimeCollatorPool{idle: make(chan *runtimeCollator, 32)}

const runtimeCollatorMaxInput = 4096

type runtimeCollatorPool struct {
	idle chan *runtimeCollator
}

type runtimeCollator struct {
	value    *collate.Collator
	reusable bool
}

func (p *runtimeCollatorPool) acquire() *runtimeCollator {
	select {
	case comparison := <-p.idle:
		return comparison
	default:
		return &runtimeCollator{value: collate.New(language.AmericanEnglish), reusable: true}
	}
}

func (c *runtimeCollator) CompareString(left, right string) int {
	if len(left) > runtimeCollatorMaxInput || len(right) > runtimeCollatorMaxInput {
		c.reusable = false
	}
	return c.value.CompareString(left, right)
}

func (p *runtimeCollatorPool) release(comparison *runtimeCollator) {
	if comparison == nil || !comparison.reusable {
		return
	}
	// CompareString resets both iterators to its inputs. Clear their references
	// before retaining the collator; long inputs also discard grown scratch
	// buffers instead of keeping them in the pool. Only CompareString is used,
	// so the collator's separate sorting/key buffers never retain caller data.
	comparison.value.CompareString("", "")
	select {
	case p.idle <- comparison:
	default:
	}
}
