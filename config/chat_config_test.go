package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestValidateChatConfigs(t *testing.T) {
	tests := []struct {
		name    string
		chats   []ChatConfig
		wantErr string
	}{
		{"none", nil, ""},
		{"valid", []ChatConfig{{Platform: "dingtalk", ChatID: "cid1==", Mode: "plan"}}, ""},
		{"same chat on two platforms", []ChatConfig{
			{Platform: "dingtalk", ChatID: "c1", Mode: "plan"},
			{Platform: "feishu", ChatID: "c1", Model: "haiku"},
		}, ""},
		{"missing platform", []ChatConfig{{ChatID: "cid1==", Mode: "plan"}}, "projects[0].chats[0].platform is required"},
		{"missing chat_id", []ChatConfig{{Platform: "dingtalk", Mode: "plan"}}, "projects[0].chats[0].chat_id is required"},
		{"no override", []ChatConfig{{Platform: "dingtalk", ChatID: "cid1=="}}, "sets no override"},
		{"duplicate (case-insensitive platform)", []ChatConfig{
			{Platform: "dingtalk", ChatID: "cid1==", Mode: "plan"},
			{Platform: "DingTalk", ChatID: "cid1==", Model: "haiku"},
		}, "projects[0].chats[1] duplicates projects[0].chats[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Projects: []ProjectConfig{{
				Name:      "p1",
				Agent:     AgentConfig{Type: "claudecode"},
				Platforms: []PlatformConfig{{Type: "dingtalk"}},
				Chats:     tt.chats,
			}}}
			err := cfg.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestChatConfig_ParsesFromTOML(t *testing.T) {
	src := `
[[projects]]
name = "p1"
[projects.agent]
type = "claudecode"
[[projects.platforms]]
type = "dingtalk"
[[projects.chats]]
platform = "dingtalk"
chat_id = "cidAbC123=="
mode = "plan"
model = "haiku"
reasoning_effort = "low"
`
	var cfg Config
	if _, err := toml.Decode(src, &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := ChatConfig{Platform: "dingtalk", ChatID: "cidAbC123==", Mode: "plan", Model: "haiku", ReasoningEffort: "low"}
	if len(cfg.Projects[0].Chats) != 1 || cfg.Projects[0].Chats[0] != want {
		t.Fatalf("chats = %+v, want [%+v]", cfg.Projects[0].Chats, want)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
