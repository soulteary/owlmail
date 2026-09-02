package mcpserver

import (
	"context"
	"testing"
)

func TestRunStdioRejectsUninitializedService(t *testing.T) {
	var service *Service
	if err := service.RunStdio(context.Background()); err == nil {
		t.Fatal("uninitialized service was accepted")
	}
}
