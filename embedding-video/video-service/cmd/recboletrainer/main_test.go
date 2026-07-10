package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainRegistersTrainerThroughEnvGate(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	source := string(data)
	if !strings.Contains(source, "recboletrainer.Register(app, cfg)") {
		t.Fatalf("main.go must use recboletrainer.Register(app, cfg) so RECBOLE_TRAINER_ENABLED is honored")
	}
	if strings.Contains(source, "recboletrainer.RegisterScheduler(app, cfg)") {
		t.Fatalf("main.go bypasses RECBOLE_TRAINER_ENABLED by calling RegisterScheduler directly")
	}
}
