package main

import "testing"

func TestMainCallsRunWorker(t *testing.T) {
	called := false
	original := runWorker
	runWorker = func() {
		called = true
	}
	defer func() {
		runWorker = original
	}()

	main()

	if !called {
		t.Fatal("expected main to call runWorker")
	}
}
