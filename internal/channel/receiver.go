package channel

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
)

// NoteCreator 笔记创建接口（解耦对 note.Service 的依赖）
type NoteCreator interface {
	Create(ctx context.Context, content string) (interface{}, error)
}

// ReceiptText 回执确认文本，可自定义
var ReceiptText = "✅ 已记录"

// Receiver 统一消息接收管理器
type Receiver struct {
	mu          sync.RWMutex
	adapters    []MessageAdapter
	adapterMap  map[string]MessageAdapter // name → adapter，用于回复路由
	noteCreator NoteCreator
	enableReply bool // 是否在创建笔记后自动回复
}

// NewReceiver 创建消息接收器
func NewReceiver(noteCreator NoteCreator) *Receiver {
	return &Receiver{
		noteCreator: noteCreator,
		adapterMap:  make(map[string]MessageAdapter),
		enableReply: true,
	}
}

// SetEnableReply 设置是否自动回复
func (r *Receiver) SetEnableReply(enable bool) {
	r.mu.Lock()
	r.enableReply = enable
	r.mu.Unlock()
}

// RegisterAdapter 注册一个消息适配器，自动注入 handleMessage 回调
func (r *Receiver) RegisterAdapter(adapter MessageAdapter) {
	adapter.SetMessageHandler(r.handleMessage)
	r.mu.Lock()
	r.adapters = append(r.adapters, adapter)
	r.adapterMap[adapter.Name()] = adapter
	r.mu.Unlock()
}

// StartAll 启动所有已注册的适配器
func (r *Receiver) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []string
	for _, a := range r.adapters {
		if err := a.Start(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", a.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("start adapters: %s", strings.Join(errs, "; "))
	}
	return nil
}

// StopAll 优雅关闭所有适配器
func (r *Receiver) StopAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []string
	for _, a := range r.adapters {
		if err := a.Stop(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", a.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("stop adapters: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetAdapterStates 返回所有适配器的连接状态
func (r *Receiver) GetAdapterStates() map[string]ConnState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	states := make(map[string]ConnState, len(r.adapters))
	for _, a := range r.adapters {
		states[a.Name()] = a.State()
	}
	return states
}

// handleMessage 统一消息处理：格式化内容 → 创建笔记
func (r *Receiver) handleMessage(ctx context.Context, msg *IncomingMessage) error {
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return nil
	}

	// 格式化：附加来源标记
	content := formatMessageContent(msg)

	_, err := r.noteCreator.Create(ctx, content)
	if err != nil {
		log.Printf("[receiver] create note from %s: %v", msg.ChannelType, err)
		return err
	}

	log.Printf("[receiver] note created from %s, sender=%s", msg.ChannelType, msg.SenderID)

	// 自动回复确认
	r.mu.RLock()
	reply := r.enableReply
	adapter := r.adapterMap[msg.ChannelType]
	r.mu.RUnlock()

	if reply && adapter != nil && msg.ReplyToken != "" {
		if err := adapter.Reply(ctx, msg.ReplyToken, ReceiptText); err != nil {
			log.Printf("[receiver] reply to %s: %v", msg.ChannelType, err)
			// 回复失败不影响笔记创建结果
		}
	}

	return nil
}

// formatMessageContent 格式化外部消息为笔记内容
func formatMessageContent(msg *IncomingMessage) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(msg.Content))

	// 附加来源标签
	tag := "#" + msg.ChannelType
	sb.WriteString("\n\n")
	sb.WriteString(tag)

	return sb.String()
}
