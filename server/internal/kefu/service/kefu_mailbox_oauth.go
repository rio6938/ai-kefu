package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"admin/server/model"
	"admin/server/pkg/encrypt"
	"admin/server/pkg/oauth"
	rdb "admin/server/pkg/redis"
	"admin/server/pkg/setting"
)

// mailOAuthState 是 OAuth2 一次性 state 的内容，绑定当前用户与渠道防 CSRF
type mailOAuthState struct {
	// 发起授权的用户id
	UserID int64 `json:"userId"`
	// 渠道id
	ChannelID int64 `json:"channelId"`
	// OAuth 服务商
	Provider string `json:"provider"`
}

// OAuthURL 生成邮箱的 OAuth2 授权地址，state 存 redis 10 分钟且只能使用一次
func (s *KefuMailboxService) OAuthURL(
	channelID int64, userID int64,
) (*model.KefuMailOAuthURLResult, error) {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}
	config, err := mailOAuthConfig(mailbox.OAuthProvider)
	if err != nil {
		return nil, err
	}
	if rdb.Client == nil {
		return nil, errors.New("redis 不可用，无法发起授权")
	}

	state := randomHex(16)
	payload, err := json.Marshal(mailOAuthState{
		UserID:    userID,
		ChannelID: channelID,
		Provider:  mailbox.OAuthProvider,
	})
	if err != nil {
		return nil, err
	}
	if err := rdb.Client.Set(context.Background(),
		mailOAuthStatePrefix+state, string(payload), mailOAuthStateTTL).Err(); err != nil {
		return nil, fmt.Errorf("保存授权状态失败: %w", err)
	}

	url, err := oauth.AuthURL(config, state)
	if err != nil {
		return nil, err
	}
	return &model.KefuMailOAuthURLResult{URL: url}, nil
}

// OAuthCallback 处理 OAuth2 回调：校验一次性 state，换取并加密保存令牌；
// 回调免登录，state 绑定发起授权的用户与渠道，过期或复用直接拒绝
func (s *KefuMailboxService) OAuthCallback(ctx context.Context, state, code string) error {
	state = strings.TrimSpace(state)
	code = strings.TrimSpace(code)
	if state == "" || code == "" {
		return ErrMailOAuthStateInvalid
	}
	if rdb.Client == nil {
		return errors.New("redis 不可用，无法校验授权状态")
	}

	key := mailOAuthStatePrefix + state
	value, err := rdb.Client.Get(ctx, key).Result()
	if err != nil {
		return ErrMailOAuthStateInvalid
	}
	// state 一次性使用，取到后立即删除，复用会被拒
	if err := rdb.Client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("删除授权状态失败: %w", err)
	}

	payload := new(mailOAuthState)
	if err := json.Unmarshal([]byte(value), payload); err != nil {
		return ErrMailOAuthStateInvalid
	}
	mailbox, err := s.mailboxRepository.GetByChannelID(payload.ChannelID)
	if err != nil {
		return err
	}
	config, err := mailOAuthConfig(payload.Provider)
	if err != nil {
		return err
	}

	token, err := oauth.Exchange(ctx, config, code)
	if err != nil {
		return err
	}
	return s.saveOAuthToken(mailbox, token)
}

// RevokeOAuth 解除授权，清空令牌并停用邮箱，避免无意义的重试
func (s *KefuMailboxService) RevokeOAuth(channelID int64) error {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return err
	}
	return s.mailboxRepository.UpdateFields(mailbox.MailboxID, map[string]any{
		"oauth_access_token":  "",
		"oauth_refresh_token": "",
		"oauth_expire_time":   nil,
		"status":              model.KefuMailboxStatusDisabled,
		"last_error":          "已解除 OAuth2 授权",
		"update_time":         model.Now(),
	})
}

// refreshAccessToken 用 refresh token 刷新访问令牌，刷新失败只记 last_error，
// 不清空 refresh token，便于人工重新授权后继续使用
func (s *KefuMailboxService) refreshAccessToken(
	ctx context.Context, mailbox *model.KefuMailbox,
) (*model.KefuMailbox, error) {
	refreshToken, err := encrypt.Decrypt(mailbox.OAuthRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 刷新令牌解密失败: %w", err)
	}
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("邮箱未完成 OAuth2 授权")
	}
	config, err := mailOAuthConfig(mailbox.OAuthProvider)
	if err != nil {
		return nil, err
	}
	token, err := oauth.Refresh(ctx, config, refreshToken)
	if err != nil {
		if updateErr := s.mailboxRepository.UpdateFields(mailbox.MailboxID, map[string]any{
			"last_error":  "OAuth2 授权刷新失败，请重新授权",
			"update_time": model.Now(),
		}); updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	// 刷新接口通常不返回新的 refresh token，沿用原来的
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	if err := s.saveOAuthToken(mailbox, token); err != nil {
		return nil, err
	}
	return s.mailboxRepository.GetByID(mailbox.MailboxID)
}

// saveOAuthToken 加密保存 OAuth2 令牌
func (s *KefuMailboxService) saveOAuthToken(
	mailbox *model.KefuMailbox, token *oauth.Token,
) error {
	if token == nil || token.AccessToken == "" {
		return errors.New("OAuth2 未返回访问令牌")
	}
	accessToken, err := encrypt.Encrypt(token.AccessToken)
	if err != nil {
		return err
	}
	refreshToken := mailbox.OAuthRefreshToken
	if token.RefreshToken != "" {
		refreshToken, err = encrypt.Encrypt(token.RefreshToken)
		if err != nil {
			return err
		}
	}

	fields := map[string]any{
		"oauth_access_token":  accessToken,
		"oauth_refresh_token": refreshToken,
		"auth_type":           model.KefuMailAuthOAuth2,
		"oauth_provider":      mailbox.OAuthProvider,
		"last_error":          "",
		"update_time":         model.Now(),
	}
	if !token.Expiry.IsZero() {
		fields["oauth_expire_time"] = model.Time{Time: token.Expiry}
	}
	return s.mailboxRepository.UpdateFields(mailbox.MailboxID, fields)
}

// mailOAuthConfig 按服务商读取配置中心的 OAuth2 客户端信息
func mailOAuthConfig(provider string) (oauth.Config, error) {
	switch provider {
	case oauth.ProviderGoogle:
		return oauthConfigOf(oauth.ProviderGoogle,
			setting.GetString(configKeyMailOAuthGoogleClientID, ""),
			setting.GetString(configKeyMailOAuthGoogleClientSecret, ""))
	case oauth.ProviderMicrosoft:
		return oauthConfigOf(oauth.ProviderMicrosoft,
			setting.GetString(configKeyMailOAuthMicrosoftClientID, ""),
			setting.GetString(configKeyMailOAuthMicrosoftClientSecret, ""))
	default:
		return oauth.Config{}, ErrMailOAuthNotConfigured
	}
}

// oauthConfigOf 组装单个服务商的 OAuth2 配置，缺项时返回明确错误
func oauthConfigOf(provider, clientID, clientSecret string) (oauth.Config, error) {
	redirectURL := strings.TrimSpace(setting.GetString(configKeyMailOAuthRedirectURL, ""))
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" ||
		redirectURL == "" {
		return oauth.Config{}, ErrMailOAuthNotConfigured
	}
	return oauth.Config{
		Provider:     provider,
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		RedirectURL:  redirectURL,
	}, nil
}

// mailboxOfChannel 按渠道加载启用中的邮箱配置，供收信与发信复用
func (s *KefuMailboxService) mailboxOfChannel(channelID int64) (*model.KefuMailbox, error) {
	mailbox, err := s.mailboxRepository.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}
	if mailbox.Status != model.KefuMailboxStatusNormal {
		return nil, ErrMailSendDisabled
	}
	return mailbox, nil
}
