package server

// Test harness: bridges GALA functional tests (func TestX(T) T) to Go testing.

import (
	"martianoff/gala/test"
	"testing"
)

func runGalaTest(t *testing.T, fn func(test.T) test.T) {
	t.Helper()
	gt := test.NewT(t)
	gt = fn(gt)
	gt.Finalize()
}
