// Package wechat 实现微信 iLink Bot 通道适配器。
// 通过 HTTP long-polling (getupdates) 接收消息，将文本消息转为碎片笔记。
package wechat

// iLink 消息 Item 类型常量
const (
	ItemTypeText  = 1
	ItemTypeImage = 2
	ItemTypeVoice = 3
	ItemTypeFile  = 4
	ItemTypeVideo = 5
)

// iLink 消息发送者类型
const (
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

// MessageState 消息生命周期
const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

// TextItem 文本内容
type TextItem struct {
	Text string `json:"text,omitempty"`
}

// VoiceItem 语音内容（含语音转文字）
type VoiceItem struct {
	Text string `json:"text,omitempty"` // speech-to-text 结果
}

// MessageItem 消息中的单个条目
type MessageItem struct {
	Type      int        `json:"type,omitempty"`
	TextItem  *TextItem  `json:"text_item,omitempty"`
	VoiceItem *VoiceItem `json:"voice_item,omitempty"`
}

// WeixinMessage iLink 协议的消息结构
type WeixinMessage struct {
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
	CreateTimeMs int64         `json:"create_time_ms,omitempty"`
}

// BaseInfo 公共请求元数据
type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

// GetUpdatesRequest getupdates 请求体
type GetUpdatesRequest struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      BaseInfo `json:"base_info,omitempty"`
}

// GetUpdatesResponse getupdates 响应体
type GetUpdatesResponse struct {
	Ret           int             `json:"ret"`
	ErrCode       int             `json:"errcode,omitempty"`
	ErrMsg        string          `json:"errmsg,omitempty"`
	Msgs          []WeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf string          `json:"get_updates_buf,omitempty"`
}

// SendMessageRequest sendmessage 请求体（用于发送回执）
type SendMessageRequest struct {
	Msg      WeixinMessage `json:"msg"`
	BaseInfo BaseInfo      `json:"base_info,omitempty"`
}

// SendMessageResponse sendmessage 响应体
type SendMessageResponse struct {
	Ret     int    `json:"ret"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

// QRCodeResponse 扫码登录二维码响应
type QRCodeResponse struct {
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

// QRStatusResponse 扫码状态响应
type QRStatusResponse struct {
	Status     string `json:"status"` // wait, scaned, confirmed, expired
	BotToken   string `json:"bot_token,omitempty"`
	ILinkBotID string `json:"ilink_bot_id,omitempty"`
	BaseURL    string `json:"baseurl,omitempty"`
}
