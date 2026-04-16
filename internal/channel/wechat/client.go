package wechat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	channelVersion         = "1.0.0"
	defaultBotType         = "0"
	defaultLongPollTimeout = 35 * time.Second
	defaultAPITimeout      = 15 * time.Second
)

// Client 微信 iLink Bot HTTP 客户端
type Client struct {
	httpClient *http.Client
}

// NewClient 创建客户端
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 0}, // 每次请求通过 context 控制超时
	}
}

func buildBaseInfo() BaseInfo {
	return BaseInfo{ChannelVersion: channelVersion}
}

// randomWechatUIN 生成 X-WECHAT-UIN header
func randomWechatUIN() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	n := binary.BigEndian.Uint32(buf[:])
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(n), 10)))
}

func buildHeaders(token string, bodyLen int) http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("Content-Length", strconv.Itoa(bodyLen))
	h.Set("X-WECHAT-UIN", randomWechatUIN())
	if strings.TrimSpace(token) != "" {
		h.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	return h
}

func ensureTrailingSlash(u string) string {
	if strings.HasSuffix(u, "/") {
		return u
	}
	return u + "/"
}

func (c *Client) apiPost(ctx context.Context, baseURL, endpoint string, body []byte, token string, timeout time.Duration) ([]byte, error) {
	base := ensureTrailingSlash(baseURL)
	u, err := url.JoinPath(base, endpoint)
	if err != nil {
		return nil, fmt.Errorf("weixin api url: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("weixin api request: %w", err)
	}
	for k, vs := range buildHeaders(token, len(body)) {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weixin api fetch: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("weixin api read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("weixin api %s %d: %s", endpoint, resp.StatusCode, string(raw))
	}
	return raw, nil
}

// GetUpdates 长轮询获取新消息
func (c *Client) GetUpdates(ctx context.Context, baseURL, token string, pollTimeout time.Duration, getUpdatesBuf string) (*GetUpdatesResponse, error) {
	if pollTimeout <= 0 {
		pollTimeout = defaultLongPollTimeout
	}
	body, err := json.Marshal(GetUpdatesRequest{
		GetUpdatesBuf: getUpdatesBuf,
		BaseInfo:      buildBaseInfo(),
	})
	if err != nil {
		return nil, err
	}
	raw, err := c.apiPost(ctx, baseURL, "ilink/bot/getupdates", body, token, pollTimeout+5*time.Second)
	if err != nil {
		if ctx.Err() != nil {
			return &GetUpdatesResponse{Ret: 0, Msgs: nil, GetUpdatesBuf: getUpdatesBuf}, nil
		}
		return nil, err
	}
	var resp GetUpdatesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("weixin getupdates decode: %w", err)
	}
	return &resp, nil
}

// SendMessage 发送消息（用于回执确认）
func (c *Client) SendMessage(ctx context.Context, baseURL, token string, msg SendMessageRequest) error {
	msg.BaseInfo = buildBaseInfo()
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	raw, err := c.apiPost(ctx, baseURL, "ilink/bot/sendmessage", body, token, defaultAPITimeout)
	if err != nil {
		return err
	}
	var resp SendMessageResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("weixin sendmessage decode: %w", err)
	}
	if resp.Ret != 0 || resp.ErrCode != 0 {
		return fmt.Errorf("weixin sendmessage: ret=%d errcode=%d errmsg=%s", resp.Ret, resp.ErrCode, strings.TrimSpace(resp.ErrMsg))
	}
	return nil
}

// FetchQRCode 获取登录二维码
func (c *Client) FetchQRCode(ctx context.Context, apiBaseURL string) (*QRCodeResponse, error) {
	base := ensureTrailingSlash(apiBaseURL)
	u := base + "ilink/bot/get_bot_qrcode?bot_type=" + url.QueryEscape(defaultBotType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weixin qrcode fetch: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weixin qrcode %d: %s", resp.StatusCode, string(raw))
	}
	var qr QRCodeResponse
	if err := json.Unmarshal(raw, &qr); err != nil {
		return nil, fmt.Errorf("weixin qrcode decode: %w", err)
	}
	return &qr, nil
}

// PollQRStatus 轮询扫码状态
func (c *Client) PollQRStatus(ctx context.Context, apiBaseURL, qrcode string) (*QRStatusResponse, error) {
	base := ensureTrailingSlash(apiBaseURL)
	u := base + "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("iLink-App-ClientVersion", "1")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return &QRStatusResponse{Status: "wait"}, nil
		}
		return nil, fmt.Errorf("weixin qrstatus fetch: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weixin qrstatus %d: %s", resp.StatusCode, string(raw))
	}
	var status QRStatusResponse
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("weixin qrstatus decode: %w", err)
	}
	return &status, nil
}
