package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
)

// 阿里云短信的默认接口地址与地域
const (
	aliyunDefaultEndpoint = "dysmsapi.aliyuncs.com"
	aliyunDefaultRegion   = "cn-hangzhou"
)

func init() {
	register("aliyun", newAliyunProvider)
}

// aliyunProvider 是阿里云短信驱动
type aliyunProvider struct {
	client   *dysmsapi.Client
	signName string
	timeout  int // 毫秒
}

// newAliyunProvider 按账号配置构建阿里云短信客户端
func newAliyunProvider(account Account, timeout time.Duration) (provider, error) {
	accessKeyID := strings.TrimSpace(account.AccessKeyID)
	accessKeySecret := strings.TrimSpace(account.AccessKeySecret)
	signName := strings.TrimSpace(account.SignName)
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, errors.New("缺少 access_key_id 或 access_key_secret")
	}
	if signName == "" {
		return nil, errors.New("缺少 sign_name")
	}

	region := strings.TrimSpace(account.Region)
	if region == "" {
		region = aliyunDefaultRegion
	}
	endpoint := strings.TrimSpace(account.Endpoint)
	if endpoint == "" {
		endpoint = aliyunDefaultEndpoint
	}

	client, err := dysmsapi.NewClient(&openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		RegionId:        tea.String(region),
		Endpoint:        tea.String(endpoint),
	})
	if err != nil {
		return nil, fmt.Errorf("创建阿里云短信客户端失败: %w", err)
	}

	millis := int(timeout / time.Millisecond)
	if millis <= 0 {
		millis = 5000
	}
	return &aliyunProvider{client: client, signName: signName, timeout: millis}, nil
}

// send 调用阿里云 SendSms 接口；该 SDK 没有 context 版本，超时由 runtime 参数控制
func (p *aliyunProvider) send(ctx context.Context, phone, templateCode string, params map[string]string) (SendResult, error) {
	if err := ctx.Err(); err != nil {
		return SendResult{}, err
	}

	templateParam, err := json.Marshal(params)
	if err != nil {
		return SendResult{}, fmt.Errorf("序列化短信模板变量失败: %w", err)
	}

	request := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phone),
		SignName:      tea.String(p.signName),
		TemplateCode:  tea.String(templateCode),
		TemplateParam: tea.String(string(templateParam)),
	}
	runtime := &dara.RuntimeOptions{
		ConnectTimeout: tea.Int(p.timeout),
		ReadTimeout:    tea.Int(p.timeout),
		Autoretry:      tea.Bool(false),
	}

	response, err := p.client.SendSmsWithOptions(request, runtime)
	if err != nil {
		return SendResult{}, err
	}
	if response == nil || response.Body == nil {
		return SendResult{}, errors.New("阿里云短信返回为空")
	}
	// 接口可能返回 200 但业务失败，必须判断业务码
	code := tea.StringValue(response.Body.Code)
	if !strings.EqualFold(code, "OK") {
		return SendResult{}, fmt.Errorf("阿里云短信返回 %s: %s", code, tea.StringValue(response.Body.Message))
	}
	return SendResult{Provider: "aliyun", BizID: tea.StringValue(response.Body.BizId)}, nil
}
