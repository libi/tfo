package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetUpdates(t *testing.T) {
	// 模拟 iLink API 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}

		resp := GetUpdatesResponse{
			Ret: 0,
			Msgs: []WeixinMessage{
				{
					MessageID:   100,
					FromUserID:  "user1",
					MessageType: MessageTypeUser,
					ItemList: []MessageItem{
						{Type: ItemTypeText, TextItem: &TextItem{Text: "测试消息"}},
					},
					CreateTimeMs: time.Now().UnixMilli(),
				},
			},
			GetUpdatesBuf: "buf-001",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.GetUpdates(context.Background(), server.URL, "test-token", 5*time.Second, "")
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if resp.Ret != 0 {
		t.Fatalf("expected ret=0, got %d", resp.Ret)
	}
	if len(resp.Msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Msgs))
	}
	if resp.Msgs[0].ItemList[0].TextItem.Text != "测试消息" {
		t.Fatalf("unexpected message text: %s", resp.Msgs[0].ItemList[0].TextItem.Text)
	}
	if resp.GetUpdatesBuf != "buf-001" {
		t.Fatalf("expected buf=buf-001, got %s", resp.GetUpdatesBuf)
	}
}

func TestClient_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}

		// 验证 auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("unexpected auth: %s", auth)
		}

		resp := SendMessageResponse{Ret: 0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient()
	req := SendMessageRequest{
		Msg: WeixinMessage{
			ToUserID:    "user1",
			MessageType: MessageTypeBot,
			ItemList: []MessageItem{
				{Type: ItemTypeText, TextItem: &TextItem{Text: "✅ 已存入电脑"}},
			},
		},
	}
	err := client.SendMessage(context.Background(), server.URL, "test-token", req)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
}

func TestClient_SendMessage_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SendMessageResponse{Ret: -1, ErrCode: 100, ErrMsg: "error"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient()
	req := SendMessageRequest{
		Msg: WeixinMessage{ToUserID: "user1"},
	}
	err := client.SendMessage(context.Background(), server.URL, "token", req)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestClient_FetchQRCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		resp := QRCodeResponse{
			QRCode:           "qr-data-123",
			QRCodeImgContent: "base64-image-data",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient()
	qr, err := client.FetchQRCode(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchQRCode: %v", err)
	}
	if qr.QRCode != "qr-data-123" {
		t.Fatalf("expected qr-data-123, got %s", qr.QRCode)
	}
}

func TestEnsureTrailingSlash(t *testing.T) {
	if got := ensureTrailingSlash("http://localhost"); got != "http://localhost/" {
		t.Fatalf("expected trailing slash, got %q", got)
	}
	if got := ensureTrailingSlash("http://localhost/"); got != "http://localhost/" {
		t.Fatalf("expected no double slash, got %q", got)
	}
}
