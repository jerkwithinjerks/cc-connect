package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestEngine_AdminCommands_GatesPromotedBuiltin(t *testing.T) {
	e := newTestEngine()
	e.SetAdminFrom("admin1")
	e.SetAdminCommands([]string{"/mode"})
	p := &stubPlatformEngine{n: "test"}

	msg := &Message{SessionKey: "test:u1", UserID: "user1", ReplyCtx: "ctx"}
	e.handleCommand(p, msg, "/mode")

	sent := p.getSent()
	if len(sent) != 1 || !strings.Contains(sent[0], "requires admin") {
		t.Errorf("non-admin should be blocked from promoted /mode, got: %v", sent)
	}
}

func TestEngine_AdminCommands_AdminCanRunPromoted(t *testing.T) {
	e := newTestEngine()
	e.SetAdminFrom("admin1")
	e.SetAdminCommands([]string{"mode"})
	p := &stubPlatformEngine{n: "test"}

	msg := &Message{SessionKey: "test:a1", UserID: "admin1", ReplyCtx: "ctx"}
	e.handleCommand(p, msg, "/mode")

	for _, s := range p.getSent() {
		if strings.Contains(s, "requires admin") {
			t.Errorf("admin should not be blocked from /mode, got: %s", s)
		}
	}
}

func TestEngine_AdminCommands_UnlistedStaysOpen(t *testing.T) {
	e := newTestEngine()
	e.SetAdminCommands([]string{"mode"})
	p := &stubPlatformEngine{n: "test"}

	msg := &Message{SessionKey: "test:u1", UserID: "user1", ReplyCtx: "ctx"}
	e.handleCommand(p, msg, "/help")

	sent := p.getSent()
	if len(sent) == 0 {
		t.Fatal("expected /help to produce a reply")
	}
	if strings.Contains(sent[0], "requires admin") {
		t.Errorf("/help should not require admin, got: %s", sent[0])
	}
}

func TestEngine_AdminCommands_GatesCustomPromptCommand(t *testing.T) {
	e := newTestEngine()
	e.SetAdminFrom("admin1")
	e.SetAdminCommands([]string{"review"})
	e.commands.Add("review", "", "please review", "", "", "config")
	p := &stubPlatformEngine{n: "test"}

	msg := &Message{SessionKey: "test:u1", UserID: "user1", ReplyCtx: "ctx"}
	e.handleCommand(p, msg, "/review")

	sent := p.getSent()
	if len(sent) != 1 || !strings.Contains(sent[0], "requires admin") {
		t.Errorf("non-admin should be blocked from promoted custom command, got: %v", sent)
	}
}

func TestEngine_AdminCommands_CannotDemoteBuiltinPrivileged(t *testing.T) {
	e := newTestEngine()
	e.SetAdminCommands(nil)

	if !e.requiresAdmin("shell", nil) {
		t.Error("/shell must stay admin-only even when admin_commands is empty")
	}
	if !e.requiresAdmin("commands", []string{"addexec"}) {
		t.Error("/commands addexec must stay admin-only even when admin_commands is empty")
	}
}

func TestEngine_AdminCommands_ResolvesAndReplaces(t *testing.T) {
	e := newTestEngine()
	e.SetAdminCommands([]string{"/MODE", "model"})
	if got, want := e.GetAdminCommands(), []string{"mode", "model"}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetAdminCommands() = %v, want %v", got, want)
	}

	// Reload replaces rather than merges.
	e.SetAdminCommands([]string{"model"})
	if e.requiresAdmin("mode", nil) {
		t.Error("/mode should no longer require admin after reload")
	}
	if !e.requiresAdmin("model", nil) {
		t.Error("/model should require admin")
	}
}
