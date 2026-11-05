// Package oauth 是邮箱 OAuth2 授权码流程的封装，
// 只实现 Google / Microsoft 个人账号需要的授权、换取与刷新，不依赖第三方 SDK
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 支持的 OAuth 服务商
const (
	// ProviderGoogle 是 Google
	ProviderGoogle = "google"
	// ProviderMicrosoft 是 Microsoft 个人账号
	ProviderMicrosoft = "microsoft"
)

// tokenTimeout 是换取与刷新令牌的请求超时
const tokenTimeout = 20 * time.Second

// googleScopes 是 Gmail 需要的授权范围
var googleScopes = []string{
	"https://mail.google.com/",
	"offline_access",
}

// microsoftScopes 是 Outlook 个人邮箱收信与发信需要的授权范围
var microsoftScopes = []string{
	"https://outlook.office.com/IMAP.AccessAsUser.All",
	"https://outlook.office.com/SMTP.Send",
	"offline_access",
}

// Config 是 OAuth2 客户端配置，由配置中心的 kefu.mail.oauth.* 提供
type Config struct {
	// 服务商, google/microsoft
	Provider string
	// 客户端 id
	ClientID string
	// 客户端密钥
	ClientSecret string
	// 回调地址, 必须与云平台登记一致
	RedirectURL string
}

// Token 是一次授权或刷新得到的令牌
type Token struct {
	// 访问令牌
	AccessToken string
	// 刷新令牌, 首次授权才会返回
	RefreshToken string
	// 访问令牌过期时间
	Expiry time.Time
}

// AuthURL 生成授权地址，后台打开新窗口
func AuthURL(cfg Config, state string) (string, error) {
	if cfg.ClientID == "" || cfg.RedirectURL == "" {
		return "", errors.New("OAuth2 客户端信息未配置")
	}
	endpoint, scopes, err := endpoints(cfg.Provider)
	if err != nil {
		return "", err
	}

	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("response_type", "code")
	values.Set("redirect_uri", cfg.RedirectURL)
	values.Set("scope", strings.Join(scopes, " "))
	values.Set("state", state)
	values.Set("access_type", "offline")
	values.Set("prompt", "consent")
	return endpoint + "?" + values.Encode(), nil
}

// Exchange 用回调的授权码换取令牌
func Exchange(ctx context.Context, cfg Config, code string) (*Token, error) {
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("client_secret", cfg.ClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", cfg.RedirectURL)
	return requestToken(ctx, cfg.Provider, values)
}

// Refresh 用 refresh token 换取新的访问令牌，刷新失败时保留原 refresh token
func Refresh(ctx context.Context, cfg Config, refreshToken string) (*Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("OAuth2 刷新令牌为空")
	}
	values := url.Values{}
	values.Set("client_id", cfg.ClientID)
	values.Set("client_secret", cfg.ClientSecret)
	values.Set("refresh_token", refreshToken)
	values.Set("grant_type", "refresh_token")
	return requestToken(ctx, cfg.Provider, values)
}

// tokenResponse 是令牌接口的返回
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// requestToken 调用令牌接口，按服务商地址提交表单
func requestToken(ctx context.Context, provider string, values url.Values) (*Token, error) {
	_, _, tokenURL, err := providerConfig(provider)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, tokenTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, tokenURL,
		strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 OAuth2 请求失败: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求 OAuth2 令牌失败: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 OAuth2 响应失败: %w", err)
	}
	result := new(tokenResponse)
	if err := json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("解析 OAuth2 响应失败: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("OAuth2 授权失败: %s %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return nil, errors.New("OAuth2 未返回访问令牌")
	}

	token := &Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}
	if result.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	}
	return token, nil
}

// endpoints 返回服务商的授权地址与授权范围
func endpoints(provider string) (string, []string, error) {
	authURL, scopes, _, err := providerConfig(provider)
	return authURL, scopes, err
}

// providerConfig 返回服务商的授权地址、授权范围与令牌地址
func providerConfig(provider string) (string, []string, string, error) {
	switch strings.ToLower(provider) {
	case ProviderGoogle:
		return "https://accounts.google.com/o/oauth2/v2/auth",
			googleScopes, "https://oauth2.googleapis.com/token", nil
	case ProviderMicrosoft:
		return "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
			microsoftScopes, "https://login.microsoftonline.com/consumers/oauth2/v2.0/token", nil
	default:
		return "", nil, "", fmt.Errorf("不支持的 OAuth2 服务商: %s", provider)
	}
}
