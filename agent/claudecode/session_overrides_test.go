package claudecode

import (
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestApplySessionOverrides(t *testing.T) {
	tests := []struct {
		name                        string
		o                           core.SessionOverrides
		wantMode, wantModel, wantEf string
	}{
		{"empty keeps defaults", core.SessionOverrides{}, "default", "provider-model", "high"},
		{"mode alias normalized", core.SessionOverrides{Mode: "yolo"}, "bypassPermissions", "provider-model", "high"},
		{"model beats provider model", core.SessionOverrides{Model: "haiku"}, "default", "haiku", "high"},
		{"effort alias normalized", core.SessionOverrides{ReasoningEffort: "med"}, "default", "provider-model", "medium"},
		{"all set", core.SessionOverrides{Mode: "plan", Model: "sonnet", ReasoningEffort: "low"}, "plan", "sonnet", "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// "provider-model" stands for the model already resolved from the active provider.
			mode, model, effort := applySessionOverrides("default", "provider-model", "high", tt.o)
			if mode != tt.wantMode || model != tt.wantModel || effort != tt.wantEf {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", mode, model, effort, tt.wantMode, tt.wantModel, tt.wantEf)
			}
		})
	}
}

func TestStartSessionWithOptions_DoesNotMutateAgent(t *testing.T) {
	a := &Agent{mode: "default", model: "agent-model", reasoningEffort: "high"}
	var _ core.SessionOptionsStarter = a
	// The override is computed from a snapshot; agent fields stay as configured.
	applySessionOverrides(a.mode, a.model, a.reasoningEffort, core.SessionOverrides{Mode: "plan", Model: "haiku"})
	if a.GetMode() != "default" || a.GetModel() != "agent-model" {
		t.Errorf("agent mutated: mode=%q model=%q", a.GetMode(), a.GetModel())
	}
}
