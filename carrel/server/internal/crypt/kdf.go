package crypt

import (
	"context"
)

const kdfConcurrency = 4

var kdfSem = make(chan struct{}, kdfConcurrency)

func acquireKDF(ctx context.Context) error {
	select {
	case kdfSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseKDF() {
	<-kdfSem
}
