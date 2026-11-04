// Package notify provides transport-independent notification delivery.
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	neturl "net/url"
	"strings"
	"time"
)

type Message struct {
	Title    string    `json:"title"`
	Text     string    `json:"text"`
	Severity string    `json:"severity"`
	EventID  string    `json:"event_id"`
	Status   string    `json:"status"`
	Host     string    `json:"host"`
	Rule     string    `json:"rule"`
	At       time.Time `json:"at"`
}
type Channel interface {
	Type() string
	Send(context.Context, Config, Message) error
}
type Config struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Enabled  bool              `json:"enabled"`
	Settings map[string]string `json:"settings"`
}
type Registry struct{ channels map[string]Channel }

func NewRegistry() *Registry {
	r := &Registry{channels: map[string]Channel{}}
	r.Register(DingTalk{})
	r.Register(Email{})
	r.Register(Webhook{})
	r.Register(Feishu{})
	r.Register(WeCom{})
	r.Register(Slack{})
	r.Register(Telegram{})
	return r
}
func (r *Registry) Register(c Channel) { r.channels[c.Type()] = c }
func (r *Registry) Send(ctx context.Context, c Config, m Message) error {
	ch, ok := r.channels[c.Type]
	if !ok {
		return fmt.Errorf("unsupported notification channel %q", c.Type)
	}
	return ch.Send(ctx, c, m)
}

type DingTalk struct{}

func (DingTalk) Type() string { return "dingtalk" }
func (DingTalk) Send(ctx context.Context, c Config, m Message) error {
	url := c.Settings["webhook_url"]
	if url == "" {
		return fmt.Errorf("dingtalk webhook_url is required")
	}
	payload := map[string]any{"msgtype": "markdown", "markdown": map[string]string{"title": m.Title, "text": fmt.Sprintf("### %s\n\n- 状态：%s\n- 主机：%s\n- 规则：%s\n- 级别：%s\n\n%s", m.Title, m.Status, m.Host, m.Rule, m.Severity, m.Text)}}
	if secret := c.Settings["secret"]; secret != "" {
		timestamp := fmt.Sprint(time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "\n" + secret))
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		parsed, err := neturl.Parse(url)
		if err != nil {
			return err
		}
		q := parsed.Query()
		q.Set("timestamp", timestamp)
		q.Set("sign", sign)
		parsed.RawQuery = q.Encode()
		url = parsed.String()
	}
	return postJSON(ctx, url, payload)
}

type Webhook struct{}

func (Webhook) Type() string { return "webhook" }
func (Webhook) Send(ctx context.Context, c Config, m Message) error {
	url := c.Settings["url"]
	if url == "" {
		return fmt.Errorf("webhook url is required")
	}
	return postJSON(ctx, url, m)
}

type Email struct{}

func (Email) Type() string { return "email" }
func (Email) Send(ctx context.Context, c Config, m Message) error {
	host, port, user, pass := c.Settings["smtp_host"], c.Settings["smtp_port"], c.Settings["username"], c.Settings["password"]
	from, to := c.Settings["from"], c.Settings["to"]
	if host == "" || port == "" || from == "" || to == "" {
		return fmt.Errorf("smtp_host, smtp_port, from and to are required")
	}
	addr := net.JoinHostPort(host, port)
	dialer := net.Dialer{Timeout: 8 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if strings.EqualFold(c.Settings["tls"], "true") {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return err
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()
	if !strings.EqualFold(c.Settings["tls"], "true") {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	rawRecipients := strings.Split(to, ",")
	recipients := make([]string, 0, len(rawRecipients))
	for _, recipient := range rawRecipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			continue
		}
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
		recipients = append(recipients, recipient)
	}
	if len(recipients) == 0 {
		return fmt.Errorf("at least one valid recipient is required")
	}
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(m.Title)
	msg := []byte("To: " + strings.Join(recipients, ",") + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + m.Text + "\r\n")
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func postJSON(ctx context.Context, url string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 8 * time.Second}
	resp, e := cl.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("notification endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		OK  *bool  `json:"ok"`
		Err string `json:"description"`
	}
	if json.Unmarshal(body, &result) == nil && result.OK != nil && !*result.OK {
		return fmt.Errorf("notification provider rejected message: %s", result.Err)
	}
	return nil
}

// Feishu sends a text message to a Feishu custom bot.
type Feishu struct{}

func (Feishu) Type() string { return "feishu" }
func (Feishu) Send(ctx context.Context, c Config, m Message) error {
	endpoint := c.Settings["webhook_url"]
	if endpoint == "" {
		return fmt.Errorf("feishu webhook_url is required")
	}
	payload := map[string]any{"msg_type": "text", "content": map[string]string{"text": fmt.Sprintf("[%s] %s\n主机：%s\n规则：%s\n状态：%s\n%s", strings.ToUpper(m.Severity), m.Title, m.Host, m.Rule, m.Status, m.Text)}}
	if secret := c.Settings["secret"]; secret != "" {
		ts := fmt.Sprint(time.Now().Unix())
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(ts + "\n" + secret))
		payload["timestamp"] = ts
		payload["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return postJSON(ctx, endpoint, payload)
}

// WeCom sends Markdown to a WeCom group bot.
type WeCom struct{}

func (WeCom) Type() string { return "wecom" }
func (WeCom) Send(ctx context.Context, c Config, m Message) error {
	endpoint := c.Settings["webhook_url"]
	if endpoint == "" {
		return fmt.Errorf("wecom webhook_url is required")
	}
	return postJSON(ctx, endpoint, map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": fmt.Sprintf("### [%s] %s\n>主机：%s\n>规则：%s\n>状态：%s\n\n%s", strings.ToUpper(m.Severity), m.Title, m.Host, m.Rule, m.Status, m.Text)}})
}

// Slack sends a simple incoming-webhook message.
type Slack struct{}

func (Slack) Type() string { return "slack" }
func (Slack) Send(ctx context.Context, c Config, m Message) error {
	endpoint := c.Settings["webhook_url"]
	if endpoint == "" {
		return fmt.Errorf("slack webhook_url is required")
	}
	return postJSON(ctx, endpoint, map[string]string{"text": fmt.Sprintf("*%s* · %s\nHost: %s | Rule: %s | Status: %s\n%s", strings.ToUpper(m.Severity), m.Title, m.Host, m.Rule, m.Status, m.Text)})
}

// Telegram uses a bot token and target chat ID.
type Telegram struct{}

func (Telegram) Type() string { return "telegram" }
func (Telegram) Send(ctx context.Context, c Config, m Message) error {
	bot := c.Settings["bot_token"]
	chat := c.Settings["chat_id"]
	if bot == "" || chat == "" {
		return fmt.Errorf("telegram bot_token and chat_id are required")
	}
	endpoint := "https://api.telegram.org/bot" + bot + "/sendMessage"
	return postJSON(ctx, endpoint, map[string]string{"chat_id": chat, "text": fmt.Sprintf("[%s] %s\nHost: %s\nRule: %s\nStatus: %s\n%s", strings.ToUpper(m.Severity), m.Title, m.Host, m.Rule, m.Status, m.Text)})
}
