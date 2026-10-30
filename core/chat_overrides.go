package core

import (
	"context"
	"log/slog"
	"strings"
)

// ChatOverride pins agent defaults for one conversation, identified by its
// platform name and chat ID (the "Chat ID" shown by /whoami). It applies to
// every session key in that chat, both per-user and shared.
type ChatOverride struct {
	Platform  string
	ChatID    string
	Overrides SessionOverrides
}

func chatOverrideKey(platform, chatID string) string {
	return strings.ToLower(strings.TrimSpace(platform)) + "\x00" + strings.TrimSpace(chatID)
}

// SetChatOverrides replaces the per-conversation agent defaults. Pass nil to
// clear them. Changes take effect when a chat's next agent process starts.
func (e *Engine) SetChatOverrides(overrides []ChatOverride) {
	m := make(map[string]SessionOverrides, len(overrides))
	for _, o := range overrides {
		if o.ChatID == "" || o.Overrides.IsZero() {
			continue
		}
		m[chatOverrideKey(o.Platform, o.ChatID)] = o.Overrides
	}
	e.chatOverridesMu.Lock()
	e.chatOverrides = m
	e.chatOverrideWarned = nil
	e.chatOverridesMu.Unlock()
}

// chatOverrideFor returns the override configured for the chat that
// sessionKey belongs to. platform falls back to the session key prefix.
func (e *Engine) chatOverrideFor(platform, sessionKey string) (SessionOverrides, bool) {
	if platform == "" {
		platform, _, _ = strings.Cut(sessionKey, ":")
	}
	chatID := extractChannelID(sessionKey)
	if chatID == "" {
		return SessionOverrides{}, false
	}
	e.chatOverridesMu.RLock()
	defer e.chatOverridesMu.RUnlock()
	o, ok := e.chatOverrides[chatOverrideKey(platform, chatID)]
	return o, ok
}

// startAgentSession starts an agent process for a chat, applying that chat's
// configured overrides when the agent supports them.
func (e *Engine) startAgentSession(ctx context.Context, agent Agent, platform, sessionKey, sessionID string) (AgentSession, error) {
	o, ok := e.chatOverrideFor(platform, sessionKey)
	if !ok {
		return agent.StartSession(ctx, sessionID)
	}
	if starter, ok := agent.(SessionOptionsStarter); ok {
		slog.Debug("starting session with chat overrides",
			"session_key", sessionKey, "mode", o.Mode, "model", o.Model, "reasoning_effort", o.ReasoningEffort)
		return starter.StartSessionWithOptions(ctx, sessionID, o)
	}
	e.warnChatOverrideUnsupported(agent, platform, sessionKey)
	return agent.StartSession(ctx, sessionID)
}

// formatSessionOverrides renders the set fields, e.g. "mode=plan, model=haiku".
func formatSessionOverrides(o SessionOverrides) string {
	var parts []string
	if o.Mode != "" {
		parts = append(parts, "mode="+o.Mode)
	}
	if o.Model != "" {
		parts = append(parts, "model="+o.Model)
	}
	if o.ReasoningEffort != "" {
		parts = append(parts, "reasoning_effort="+o.ReasoningEffort)
	}
	return strings.Join(parts, ", ")
}

// noteChatOverride tells the user that this chat's agent defaults are pinned
// by config, so /mode and /model changes to the shared agent won't apply here.
func (e *Engine) noteChatOverride(p Platform, msg *Message) {
	if o, ok := e.chatOverrideFor(msg.Platform, msg.SessionKey); ok {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgChatOverridePinned, formatSessionOverrides(o)))
	}
}

// warnChatOverrideUnsupported logs once per chat that the agent ignores overrides.
func (e *Engine) warnChatOverrideUnsupported(agent Agent, platform, sessionKey string) {
	key := chatOverrideKey(platform, extractChannelID(sessionKey))
	e.chatOverridesMu.Lock()
	defer e.chatOverridesMu.Unlock()
	if e.chatOverrideWarned[key] {
		return
	}
	if e.chatOverrideWarned == nil {
		e.chatOverrideWarned = make(map[string]bool)
	}
	e.chatOverrideWarned[key] = true
	slog.Warn("chat override configured but agent does not support per-session overrides; using agent defaults",
		"agent", agent.Name(), "session_key", sessionKey)
}
