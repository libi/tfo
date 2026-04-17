package wechat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libi/tfo/internal/channel"
	"github.com/libi/tfo/internal/config"
)

const (
	sessionExpiredErrCode  = -14
	maxConsecutiveFailures = 3
	backoffDelay           = 30 * time.Second
	retryDelay             = 2 * time.Second
	sessionPauseDuration   = 1 * time.Hour
)

// WeChatAdapter 微信 iLink Bot 适配器
// 通过 HTTP long-polling 接收消息
type WeChatAdapter struct {
	mu      sync.RWMutex
	cfg     config.WeChatConfig
	client  *Client
	handler channel.MessageHandler
	state   channel.ConnState
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewWeChatAdapter 创建微信适配器
func NewWeChatAdapter(cfg config.WeChatConfig) *WeChatAdapter {
	return &WeChatAdapter{
		cfg:    cfg,
		client: NewClient(),
		state:  channel.ConnStateDisconnected,
	}
}

func (a *WeChatAdapter) Name() string { return "wechat" }

func (a *WeChatAdapter) SetMessageHandler(h channel.MessageHandler) {
	a.mu.Lock()
	a.handler = h
	a.mu.Unlock()
}

func (a *WeChatAdapter) State() channel.ConnState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *WeChatAdapter) setState(s channel.ConnState) {
	a.mu.Lock()
	a.state = s
	a.mu.Unlock()
}

// Start 启动 long-polling 循环（非阻塞）
func (a *WeChatAdapter) Start(ctx context.Context) error {
	if strings.TrimSpace(a.cfg.BaseURL) == "" {
		return fmt.Errorf("wechat: baseUrl is required")
	}
	if strings.TrimSpace(a.cfg.Token) == "" {
		return fmt.Errorf("wechat: token is required")
	}

	a.mu.Lock()
	if a.state == channel.ConnStateConnected || a.state == channel.ConnStateConnecting {
		a.mu.Unlock()
		return nil // 已启动
	}
	pollCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.done = make(chan struct{})
	a.state = channel.ConnStateConnecting
	a.mu.Unlock()

	go a.pollLoop(pollCtx)
	return nil
}

// Stop 停止 long-polling
func (a *WeChatAdapter) Stop() error {
	a.mu.Lock()
	cancel := a.cancel
	done := a.done
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	a.setState(channel.ConnStateDisconnected)
	return nil
}

// pollLoop 长轮询主循环
func (a *WeChatAdapter) pollLoop(ctx context.Context) {
	defer close(a.done)

	var getUpdatesBuf string
	var consecutiveFailures int
	pollTimeout := time.Duration(a.cfg.PollTimeoutSeconds) * time.Second
	if pollTimeout <= 0 {
		pollTimeout = defaultLongPollTimeout
	}

	a.setState(channel.ConnStateConnected)
	log.Printf("[wechat] poll loop started, baseUrl=%s", a.cfg.BaseURL)

	for {
		select {
		case <-ctx.Done():
			log.Println("[wechat] poll loop stopped")
			return
		default:
		}

		resp, err := a.client.GetUpdates(ctx, a.cfg.BaseURL, a.cfg.Token, pollTimeout, getUpdatesBuf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			consecutiveFailures++
			log.Printf("[wechat] getupdates error: %v (failures=%d)", err, consecutiveFailures)
			if consecutiveFailures >= maxConsecutiveFailures {
				a.setState(channel.ConnStateReconnecting)
				consecutiveFailures = 0
				sleepCtx(ctx, backoffDelay)
			} else {
				sleepCtx(ctx, retryDelay)
			}
			continue
		}

		// 处理 API 错误
		if resp.Ret != 0 || resp.ErrCode != 0 {
			if resp.ErrCode == sessionExpiredErrCode || resp.Ret == sessionExpiredErrCode {
				log.Printf("[wechat] session expired, pausing %v", sessionPauseDuration)
				a.setState(channel.ConnStateError)
				sleepCtx(ctx, sessionPauseDuration)
				consecutiveFailures = 0
				continue
			}
			consecutiveFailures++
			log.Printf("[wechat] api error: ret=%d errcode=%d errmsg=%s", resp.Ret, resp.ErrCode, resp.ErrMsg)
			if consecutiveFailures >= maxConsecutiveFailures {
				a.setState(channel.ConnStateReconnecting)
				consecutiveFailures = 0
				sleepCtx(ctx, backoffDelay)
			} else {
				sleepCtx(ctx, retryDelay)
			}
			continue
		}

		// 成功
		consecutiveFailures = 0
		a.setState(channel.ConnStateConnected)
		if resp.GetUpdatesBuf != "" {
			getUpdatesBuf = resp.GetUpdatesBuf
		}

		// 处理消息
		for _, msg := range resp.Msgs {
			if msg.MessageType != MessageTypeUser {
				continue
			}
			a.processMessage(ctx, msg)
		}
	}
}

// processMessage 提取文本内容并交给 handler
func (a *WeChatAdapter) processMessage(ctx context.Context, msg WeixinMessage) {
	text := extractText(msg)
	if strings.TrimSpace(text) == "" {
		return
	}

	a.mu.RLock()
	handler := a.handler
	a.mu.RUnlock()

	if handler == nil {
		return
	}

	ts := time.Now()
	if msg.CreateTimeMs > 0 {
		ts = time.UnixMilli(msg.CreateTimeMs)
	}

	incoming := &channel.IncomingMessage{
		ChannelType: "wechat",
		SenderID:    strings.TrimSpace(msg.FromUserID),
		Content:     text,
		Type:        channel.MessageTypeText,
		Timestamp:   ts,
		ReplyToken:  buildReplyToken(msg),
	}

	if err := handler(ctx, incoming); err != nil {
		log.Printf("[wechat] handle message %d: %v", msg.MessageID, err)
	}
}

// extractText 从消息中提取文本内容（包括语音转文字）
func extractText(msg WeixinMessage) string {
	var parts []string
	for _, item := range msg.ItemList {
		switch item.Type {
		case ItemTypeText:
			if item.TextItem != nil && strings.TrimSpace(item.TextItem.Text) != "" {
				parts = append(parts, item.TextItem.Text)
			}
		case ItemTypeVoice:
			if item.VoiceItem != nil && strings.TrimSpace(item.VoiceItem.Text) != "" {
				parts = append(parts, item.VoiceItem.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// replyToken 格式: "userId|contextToken"（contextToken 可为空）
func buildReplyToken(msg WeixinMessage) string {
	if strings.TrimSpace(msg.FromUserID) == "" {
		return ""
	}
	return msg.FromUserID + "|" + msg.ContextToken
}

func parseReplyToken(token string) (userID, contextToken string, ok bool) {
	parts := strings.SplitN(token, "|", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// Reply 实现 MessageAdapter.Reply，向微信用户发送回复消息
func (a *WeChatAdapter) Reply(ctx context.Context, replyToken string, text string) error {
	userID, contextToken, ok := parseReplyToken(replyToken)
	if !ok {
		return fmt.Errorf("wechat: invalid reply token")
	}
	req := SendMessageRequest{
		Msg: WeixinMessage{
			ToUserID:     userID,
			ClientID:     generateClientID(),
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList: []MessageItem{
				{Type: ItemTypeText, TextItem: &TextItem{Text: text}},
			},
			ContextToken: contextToken,
		},
	}
	return a.client.SendMessage(ctx, a.cfg.BaseURL, a.cfg.Token, req)
}

// sleepCtx 可被 context 取消的 sleep
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// UpdateConfig 动态更新配置（热更新 token/baseUrl）
func (a *WeChatAdapter) UpdateConfig(cfg config.WeChatConfig) {
	a.mu.Lock()
	a.cfg = cfg
	a.mu.Unlock()
}

// GetQRCode 获取登录二维码（暴露给前端）
func (a *WeChatAdapter) GetQRCode(ctx context.Context, apiBaseURL string) (*QRCodeResponse, error) {
	return a.client.FetchQRCode(ctx, apiBaseURL)
}

// PollQRStatus 轮询扫码状态（暴露给前端）
func (a *WeChatAdapter) PollQRStatus(ctx context.Context, apiBaseURL, qrcode string) (*QRStatusResponse, error) {
	return a.client.PollQRStatus(ctx, apiBaseURL, qrcode)
}

// LoginWithQRResult 用扫码结果更新配置并启动
func (a *WeChatAdapter) LoginWithQRResult(ctx context.Context, result *QRStatusResponse) error {
	if result == nil || result.Status != "confirmed" {
		return fmt.Errorf("wechat: invalid QR result status: %v", result)
	}
	a.mu.Lock()
	a.cfg.BaseURL = result.BaseURL
	a.cfg.Token = result.BotToken
	a.cfg.Enabled = true
	a.mu.Unlock()

	// 如果正在运行，先停止
	if a.State() == channel.ConnStateConnected || a.State() == channel.ConnStateReconnecting {
		a.Stop()
	}

	return a.Start(ctx)
}

// GetMessageID 从 WeixinMessage 生成唯一消息 ID
func GetMessageID(msg WeixinMessage) string {
	return strconv.FormatInt(msg.MessageID, 10)
}

// generateClientID 生成唯一的客户端消息 ID
func generateClientID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "tfo-wechat-" + hex.EncodeToString(b)
}
