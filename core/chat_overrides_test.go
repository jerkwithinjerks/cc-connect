package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// overridesAgent records which start method was used and with what overrides.
type overridesAgent struct {
	stubAgent
	mu          sync.Mutex
	plainCalls  []string
	optionCalls []SessionOverrides
	optionIDs   []string
	failResume  bool
}

func (a *overridesAgent) StartSession(_ context.Context, id string) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.plainCalls = append(a.plainCalls, id)
	return &stubAgentSession{}, nil
}

func (a *overridesAgent) StartSessionWithOptions(_ context.Context, id string, o SessionOverrides) (AgentSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.optionCalls = append(a.optionCalls, o)
	a.optionIDs = append(a.optionIDs, id)
	if a.failResume && id != "" {
		return nil, errors.New("resume failed")
	}
	return &stubAgentSession{}, nil
}

func newChatOverridesEngine(agent Agent) (*Engine, *stubPlatformEngine) {
	p := &stubPlatformEngine{n: "dingtalk"}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	e.SetChatOverrides([]ChatOverride{{
		Platform:  "DingTalk",
		ChatID:    "cidGroup==",
		Overrides: SessionOverrides{Mode: "plan", Model: "haiku", ReasoningEffort: "low"},
	}})
	return e, p
}

func TestChatOverrides_AppliedForPerUserAndSharedKeys(t *testing.T) {
	want := SessionOverrides{Mode: "plan", Model: "haiku", ReasoningEffort: "low"}
	for _, key := range []string{"dingtalk:g:cidGroup==:staff1", "dingtalk:g:cidGroup=="} {
		agent := &overridesAgent{}
		e, p := newChatOverridesEngine(agent)
		e.getOrCreateInteractiveStateWith(key, p, "ctx", &Session{}, e.sessions, nil, "")
		if len(agent.optionCalls) != 1 || agent.optionCalls[0] != want {
			t.Errorf("key %q: option calls = %+v, want one call with %+v", key, agent.optionCalls, want)
		}
		if len(agent.plainCalls) != 0 {
			t.Errorf("key %q: unexpected plain StartSession calls %v", key, agent.plainCalls)
		}
	}
}

func TestChatOverrides_UnmatchedChatUsesPlainStart(t *testing.T) {
	agent := &overridesAgent{}
	e, p := newChatOverridesEngine(agent)
	e.getOrCreateInteractiveStateWith("dingtalk:g:cidOther==:staff1", p, "ctx", &Session{}, e.sessions, nil, "")
	if len(agent.optionCalls) != 0 || len(agent.plainCalls) != 1 {
		t.Errorf("option calls %v, plain calls %v; want only one plain call", agent.optionCalls, agent.plainCalls)
	}
}

func TestChatOverrides_SameChatIDOnOtherPlatformNotMatched(t *testing.T) {
	agent := &overridesAgent{}
	e, _ := newChatOverridesEngine(agent)
	if _, ok := e.chatOverrideFor("feishu", "feishu:cidGroup==:u1"); ok {
		t.Error("override must be scoped to its platform")
	}
}

func TestChatOverrides_UsesCCSessionKeyInMultiWorkspace(t *testing.T) {
	agent := &overridesAgent{}
	e, p := newChatOverridesEngine(agent)
	// Interactive keys are workspace-prefixed; the chat is identified by ccSessionKey.
	e.getOrCreateInteractiveStateWith("/ws/a:dingtalk:g:cidGroup==:staff1", p, "ctx", &Session{}, e.sessions, nil, "dingtalk:g:cidGroup==:staff1")
	if len(agent.optionCalls) != 1 {
		t.Errorf("option calls = %v, want 1", agent.optionCalls)
	}
}

func TestChatOverrides_ResumeFallbackKeepsOverrides(t *testing.T) {
	agent := &overridesAgent{failResume: true}
	e, p := newChatOverridesEngine(agent)
	session := &Session{AgentSessionID: "old-id"}
	e.getOrCreateInteractiveStateWith("dingtalk:g:cidGroup==:staff1", p, "ctx", session, e.sessions, nil, "")
	if len(agent.optionIDs) != 2 || agent.optionIDs[0] != "old-id" || agent.optionIDs[1] != "" {
		t.Fatalf("start IDs = %q, want resume then fresh retry", agent.optionIDs)
	}
	if agent.optionCalls[1].Mode != "plan" {
		t.Errorf("fresh retry lost overrides: %+v", agent.optionCalls[1])
	}
	if len(agent.plainCalls) != 0 {
		t.Errorf("unexpected plain StartSession calls %v", agent.plainCalls)
	}
}

func TestChatOverrides_AgentWithoutSupportFallsBack(t *testing.T) {
	e, p := newChatOverridesEngine(&stubAgent{})
	state := e.getOrCreateInteractiveStateWith("dingtalk:g:cidGroup==:staff1", p, "ctx", &Session{}, e.sessions, nil, "")
	if state.agentSession == nil {
		t.Fatal("expected a session from plain StartSession fallback")
	}
	if !e.chatOverrideWarned[chatOverrideKey("dingtalk", "cidGroup==")] {
		t.Error("expected a one-time warning to be recorded")
	}
}

func TestChatOverrides_SetReplacesAndSkipsEmpty(t *testing.T) {
	e, _ := newChatOverridesEngine(&overridesAgent{})
	e.SetChatOverrides([]ChatOverride{
		{Platform: "dingtalk", ChatID: "cidNew==", Overrides: SessionOverrides{Model: "sonnet"}},
		{Platform: "dingtalk", ChatID: "cidEmpty=="},
	})
	if _, ok := e.chatOverrideFor("dingtalk", "dingtalk:g:cidGroup==:s"); ok {
		t.Error("old override should be gone after replace")
	}
	if o, ok := e.chatOverrideFor("dingtalk", "dingtalk:g:cidNew==:s"); !ok || o.Model != "sonnet" {
		t.Errorf("new override = %+v, %v", o, ok)
	}
	if _, ok := e.chatOverrideFor("dingtalk", "dingtalk:g:cidEmpty==:s"); ok {
		t.Error("entry without overrides must be ignored")
	}
	e.SetChatOverrides(nil)
	if _, ok := e.chatOverrideFor("dingtalk", "dingtalk:g:cidNew==:s"); ok {
		t.Error("nil should clear overrides")
	}
}

func TestChatOverrides_WhoamiAndModeNote(t *testing.T) {
	e, p := newChatOverridesEngine(&stubAgent{})
	msg := &Message{Platform: "dingtalk", SessionKey: "dingtalk:g:cidGroup==:staff1", UserID: "staff1", ReplyCtx: "ctx"}

	text := e.formatWhoamiText(msg)
	if !strings.Contains(text, "mode=plan, model=haiku, reasoning_effort=low") {
		t.Errorf("whoami text missing overrides:\n%s", text)
	}

	e.cmdMode(p, msg, nil)
	sent := p.getSent()
	if len(sent) == 0 || !strings.Contains(sent[len(sent)-1], "pinned by config (mode=plan") {
		t.Errorf("expected pinned note after /mode, got %v", sent)
	}

	other := &Message{Platform: "dingtalk", SessionKey: "dingtalk:g:cidOther==:staff1", ReplyCtx: "ctx"}
	if strings.Contains(e.formatWhoamiText(other), "Chat overrides") {
		t.Error("unconfigured chat must not show overrides")
	}
}
