package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

type Config struct {
	Token  string `json:"token"`
	ChatID string `json:"chat_id"`
}

type chunkRef struct {
	MessageID int64  `json:"message_id"`
	FileID    string `json:"file_id"`
}

type Client struct {
	chatID     string
	baseAPIURL string
	fileBase   string
	httpClient *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	return NewClientWithBaseURL(cfg, "https://api.telegram.org")
}

func NewClientWithBaseURL(cfg Config, baseURL string) (*Client, error) {
	cfg, err := ValidateConfig(cfg)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(baseURL, "/")
	return &Client{
		chatID:     cfg.ChatID,
		baseAPIURL: fmt.Sprintf("%s/bot%s", base, cfg.Token),
		fileBase:   fmt.Sprintf("%s/file/bot%s", base, cfg.Token),
		httpClient: http.DefaultClient,
	}, nil
}

func ValidateConfig(cfg Config) (Config, error) {
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.ChatID = strings.TrimSpace(cfg.ChatID)
	if strings.TrimSpace(cfg.Token) == "" {
		return Config{}, fmt.Errorf("telegram token is required")
	}
	if strings.TrimSpace(cfg.ChatID) == "" {
		return Config{}, fmt.Errorf("telegram chat_id is required")
	}
	return cfg, nil
}

func ParseConfig(raw string) (Config, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func EncodeConfig(cfg Config) (string, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *Client) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer func() { _ = pw.Close() }()
		defer func() { _ = writer.Close() }()
		if err := writer.WriteField("chat_id", c.chatID); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		part, err := writer.CreateFormFile("document", name)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err = io.Copy(part, r); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseAPIURL+"/sendDocument", pr)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("telegram sendDocument failed: status=%d body=%s", resp.StatusCode, string(data))
	}

	var parsed struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
			Document  struct {
				FileID string `json:"file_id"`
			} `json:"document"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if !parsed.OK || parsed.Result.Document.FileID == "" || parsed.Result.MessageID == 0 {
		return "", fmt.Errorf("telegram sendDocument invalid response")
	}
	refJSON, err := json.Marshal(chunkRef{MessageID: parsed.Result.MessageID, FileID: parsed.Result.Document.FileID})
	if err != nil {
		return "", err
	}
	return string(refJSON), nil
}

func (c *Client) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	var ref chunkRef
	if err := json.Unmarshal([]byte(name), &ref); err != nil {
		return nil, err
	}
	if ref.FileID == "" {
		return nil, fmt.Errorf("telegram chunk file_id is empty")
	}
	form := url.Values{}
	form.Set("file_id", ref.FileID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseAPIURL+"/getFile", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram getFile failed: status=%d body=%s", resp.StatusCode, string(data))
	}
	var parsed struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	_ = resp.Body.Close()
	if !parsed.OK || parsed.Result.FilePath == "" {
		return nil, fmt.Errorf("telegram getFile invalid response")
	}
	fileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.fileBase+"/"+parsed.Result.FilePath, nil)
	if err != nil {
		return nil, err
	}
	fileResp, err := c.httpClient.Do(fileReq)
	if err != nil {
		return nil, err
	}
	if fileResp.StatusCode != http.StatusOK {
		defer func() { _ = fileResp.Body.Close() }()
		data, _ := io.ReadAll(fileResp.Body)
		return nil, fmt.Errorf("telegram file download failed: status=%d body=%s", fileResp.StatusCode, string(data))
	}
	return fileResp.Body, nil
}

func (c *Client) Delete(ctx context.Context, name string) error {
	var ref chunkRef
	if err := json.Unmarshal([]byte(name), &ref); err != nil {
		return err
	}
	if ref.MessageID == 0 {
		return fmt.Errorf("telegram chunk message_id is empty")
	}
	form := url.Values{}
	form.Set("chat_id", c.chatID)
	form.Set("message_id", fmt.Sprintf("%d", ref.MessageID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseAPIURL+"/deleteMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram deleteMessage failed: status=%d body=%s", resp.StatusCode, string(data))
	}
	var parsed struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if !parsed.OK {
		return fmt.Errorf("telegram deleteMessage invalid response")
	}
	return nil
}

func (c *Client) CheckChat(ctx context.Context) error {
	form := url.Values{}
	form.Set("chat_id", c.chatID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseAPIURL+"/getChat", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram getChat failed: status=%d body=%s", resp.StatusCode, string(data))
	}
	var parsed struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if !parsed.OK {
		return fmt.Errorf("telegram getChat invalid response")
	}
	return nil
}
