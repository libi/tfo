package wechat

import (
	"context"
	"testing"
	"time"

	"github.com/libi/tfo/internal/channel"
	"github.com/libi/tfo/internal/config"
)

func TestExtractText(t *testing.T) {
	tests := []struct {
		name string
		msg  WeixinMessage
		want string
	}{
		{
			name: "text message",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeText, TextItem: &TextItem{Text: "hello world"}},
				},
			},
			want: "hello world",
		},
		{
			name: "voice with text",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeVoice, VoiceItem: &VoiceItem{Text: "语音转文字"}},
				},
			},
			want: "语音转文字",
		},
		{
			name: "mixed items",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeText, TextItem: &TextItem{Text: "text part"}},
					{Type: ItemTypeImage}, // 图片忽略
					{Type: ItemTypeVoice, VoiceItem: &VoiceItem{Text: "voice part"}},
				},
			},
			want: "text part\nvoice part",
		},
		{
			name: "empty message",
			msg:  WeixinMessage{},
			want: "",
		},
		{
			name: "image only - no text",
			msg: WeixinMessage{
				ItemList: []MessageItem{
					{Type: ItemTypeImage},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.msg)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetMessageID(t *testing.T) {
	msg := WeixinMessage{MessageID: 12345}
	if id := GetMessageID(msg); id != "12345" {
		t.Errorf("GetMessageID() = %q, want %q", id, "12345")
	}
}

func TestWeChatAdapter_StartWithoutConfig(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{})
	err := a.Start(nil)
	if err == nil {
		t.Fatal("expected error for empty baseUrl")
	}
}

func TestWeChatAdapter_StartWithoutToken(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{BaseURL: "http://localhost"})
	err := a.Start(nil)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestWeChatAdapter_StateTransitions(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{})
	if a.State() != channel.ConnStateDisconnected {
		t.Fatalf("initial state should be disconnected, got %v", a.State())
	}
	if a.Name() != "wechat" {
		t.Fatalf("Name() should be wechat, got %q", a.Name())
	}
}

func TestWeChatAdapter_UpdateConfig(t *testing.T) {
	a := NewWeChatAdapter(config.WeChatConfig{BaseURL: "http://old"})
	a.UpdateConfig(config.WeChatConfig{BaseURL: "http://new", Token: "tok"})

	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cfg.BaseURL != "http://new" {
		t.Fatalf("expected updated baseUrl, got %q", a.cfg.BaseURL)
	}
}

func TestSleepCtx_Immediate(t *testing.T) {
	// sleepCtx 应该能正常完成一个极短的 sleep
	start := time.Now()
	sleepCtx(context.Background(), 1*time.Millisecond)
	if time.Since(start) > 1*time.Second {
		t.Fatal("sleepCtx took too long")
	}
}

func TestSleepCtx_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	start := time.Now()
	sleepCtx(ctx, 10*time.Second)
	if time.Since(start) > 1*time.Second {
		t.Fatal("sleepCtx should return immediately on cancelled context")
	}
}

func TestBuildParseReplyToken(t *testing.T) {
	msg := WeixinMessage{FromUserID: "user123", ContextToken: "ctx-abc"}
	token := buildReplyToken(msg)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	uid, ct, ok := parseReplyToken(token)
	if !ok {
		t.Fatal("parseReplyToken failed")
	}
	if uid != "user123" {
		t.Fatalf("userID = %q, want user123", uid)
	}
	if ct != "ctx-abc" {
		t.Fatalf("contextToken = %q, want ctx-abc", ct)
	}
}

func TestBuildReplyToken_NoContextToken(t *testing.T) {
	msg := WeixinMessage{FromUserID: "user1"}
	token := buildReplyToken(msg)
	if token == "" {
		t.Fatal("expected non-empty token even without ContextToken")
	}
	uid, ct, ok := parseReplyToken(token)
	if !ok {
		t.Fatal("parseReplyToken failed")
	}
	if uid != "user1" {
		t.Fatalf("userID = %q, want user1", uid)
	}
	if ct != "" {
		t.Fatalf("contextToken = %q, want empty", ct)
	}
}

func TestBuildReplyToken_Empty(t *testing.T) {
	msg := WeixinMessage{}
	if token := buildReplyToken(msg); token != "" {
		t.Fatalf("expected empty token for missing FromUserID, got %q", token)
	}
}

func TestParseReplyToken_Invalid(t *testing.T) {
	_, _, ok := parseReplyToken("")
	if ok {
		t.Fatal("expected false for empty token")
	}
	_, _, ok = parseReplyToken("no-separator")
	if ok {
		t.Fatal("expected false for token without separator")
	}
}
