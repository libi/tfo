package channel

import (
	"context"
	"time"
)

// MessageType 消息类型
type MessageType int

const (
	MessageTypeText  MessageType = iota // 纯文本
	MessageTypeVoice                    // 语音转文字
	MessageTypeImage                    // 图片 (预留)
)

// IncomingMessage 从外部通道接收到的原始消息
type IncomingMessage struct {
	ChannelType string      `json:"channelType"` // "wechat", "feishu" 等
	SenderID    string      `json:"senderId"`
	Content     string      `json:"content"`
	Type        MessageType `json:"type"`
	Timestamp   time.Time   `json:"timestamp"`
	ReplyToken  string      `json:"-"` // 适配器专用回复上下文，不序列化
}

// ConnState 通道连接状态
type ConnState int

const (
	ConnStateDisconnected ConnState = iota // 未连接
	ConnStateConnecting                    // 连接中
	ConnStateConnected                     // 已连接
	ConnStateReconnecting                  // 重连中
	ConnStateError                         // 错误
)

// String 返回连接状态的字符串表示
func (s ConnState) String() string {
	switch s {
	case ConnStateDisconnected:
		return "disconnected"
	case ConnStateConnecting:
		return "connecting"
	case ConnStateConnected:
		return "connected"
	case ConnStateReconnecting:
		return "reconnecting"
	case ConnStateError:
		return "error"
	default:
		return "unknown"
	}
}

// MessageHandler 消息处理回调
type MessageHandler func(ctx context.Context, msg *IncomingMessage) error

// MessageAdapter 多通道消息适配器接口
type MessageAdapter interface {
	// Name 返回通道名称
	Name() string

	// Start 启动通道（非阻塞，内部管理 goroutine）
	Start(ctx context.Context) error

	// Stop 优雅关闭
	Stop() error

	// State 返回当前连接状态
	State() ConnState

	// SetMessageHandler 注入消息处理回调
	SetMessageHandler(handler MessageHandler)

	// Reply 向消息发送者回复文本（需要 IncomingMessage.ReplyToken）
	Reply(ctx context.Context, replyToken string, text string) error
}
