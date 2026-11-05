package sms

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

// 腾讯云短信的默认地域
const tencentDefaultRegion = "ap-guangzhou"

func init() {
	register("tencent", newTencentProvider)
}

// tencentProvider 是腾讯云短信驱动
type tencentProvider struct {
	client   *tsms.Client
	signName string
	sdkAppID string
}

// newTencentProvider 按账号配置构建腾讯云短信客户端
func newTencentProvider(account Account, timeout time.Duration) (provider, error) {
	accessKeyID := strings.TrimSpace(account.AccessKeyID)
	accessKeySecret := strings.TrimSpace(account.AccessKeySecret)
	signName := strings.TrimSpace(account.SignName)
	sdkAppID := strings.TrimSpace(account.SDKAppID)
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, errors.New("缺少 access_key_id（SecretId）或 access_key_secret（SecretKey）")
	}
	if signName == "" {
		return nil, errors.New("缺少 sign_name")
	}
	if sdkAppID == "" {
		return nil, errors.New("缺少 sdk_app_id")
	}

	region := strings.TrimSpace(account.Region)
	if region == "" {
		region = tencentDefaultRegion
	}

	credential := common.NewCredential(accessKeyID, accessKeySecret)
	clientProfile := profile.NewClientProfile()
	// 腾讯云 SDK 的超时单位是秒
	seconds := int(timeout / time.Second)
	if seconds <= 0 {
		seconds = 5
	}
	clientProfile.HttpProfile.ReqTimeout = seconds

	client, err := tsms.NewClient(credential, region, clientProfile)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云短信客户端失败: %w", err)
	}
	return &tencentProvider{client: client, signName: signName, sdkAppID: sdkAppID}, nil
}

// send 调用腾讯云 SendSms 接口，号码需要带国际区号
func (p *tencentProvider) send(ctx context.Context, phone, templateCode string, params map[string]string) (SendResult, error) {
	request := tsms.NewSendSmsRequest()
	request.PhoneNumberSet = common.StringPtrs([]string{withChinaCode(phone)})
	request.SmsSdkAppId = common.StringPtr(p.sdkAppID)
	request.SignName = common.StringPtr(p.signName)
	request.TemplateId = common.StringPtr(templateCode)
	request.TemplateParamSet = common.StringPtrs(templateValues(params))

	response, err := p.client.SendSmsWithContext(ctx, request)
	if err != nil {
		return SendResult{}, err
	}
	if response == nil || response.Response == nil {
		return SendResult{}, errors.New("腾讯云短信返回为空")
	}

	// 接口调用失败会由 SDK 返回 error，这里只判断每条号码的业务码
	if len(response.Response.SendStatusSet) == 0 {
		return SendResult{}, errors.New("腾讯云短信未返回发送状态")
	}
	bizID := ""
	for _, status := range response.Response.SendStatusSet {
		if status == nil {
			continue
		}
		code := derefString(status.Code)
		if !strings.EqualFold(code, "Ok") {
			return SendResult{}, fmt.Errorf("腾讯云短信返回 %s: %s", code, derefString(status.Message))
		}
		if bizID == "" {
			bizID = derefString(status.SerialNo)
		}
	}
	return SendResult{Provider: "tencent", BizID: bizID}, nil
}

// withChinaCode 给不带国际区号的大陆手机号补上 +86
func withChinaCode(phone string) string {
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	return "+86" + phone
}

// templateValues 把模板变量按名称字典序转成腾讯云要求的数组；
// 腾讯云模板按变量顺序传参，多变量场景需保证变量名排序与模板变量顺序一致
func templateValues(params map[string]string) []string {
	if len(params) == 0 {
		return nil
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, params[key])
	}
	return values
}

// derefString 取指针字符串的值，为空时返回空串
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
