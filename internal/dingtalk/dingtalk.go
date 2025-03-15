package dingtalk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"github.com/xhy/api-pulse/internal/apifox"
)

// NotifyService 钉钉通知服务
type NotifyService struct {
	webhookURL string
	client     *resty.Client
	logger     *logrus.Logger
}

// MarkdownMessage 钉钉 markdown 消息结构
type MarkdownMessage struct {
	MsgType  string `json:"msgtype"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown"`
	At struct {
		AtMobiles []string `json:"atMobiles"`
		AtUserIds []string `json:"atUserIds"`
		IsAtAll   bool     `json:"isAtAll"`
	} `json:"at"`
}

// NewNotifyService 创建新的钉钉通知服务
func NewNotifyService(webhookURL string, logger *logrus.Logger) *NotifyService {
	return &NotifyService{
		webhookURL: webhookURL,
		client:     resty.New(),
		logger:     logger,
	}
}

// SendApiChangedNotification 发送 API 变更通知
func (s *NotifyService) SendApiChangedNotification(diff apifox.ApiDiff) error {
	// 构建 Markdown 消息内容
	title := "API 变更通知"
	text := s.buildApiDiffMarkdown(diff)

	message := MarkdownMessage{
		MsgType: "markdown",
	}
	message.Markdown.Title = title
	message.Markdown.Text = text
	message.At.IsAtAll = false

	// 将消息序列化为 JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		s.logger.WithError(err).Error("序列化钉钉消息失败")
		return err
	}

	// 发送请求
	resp, err := s.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(jsonData).
		Post(s.webhookURL)

	if err != nil {
		s.logger.WithError(err).Error("发送钉钉通知失败")
		return err
	}

	if resp.StatusCode() != 200 {
		s.logger.WithField("status", resp.Status()).
			WithField("response", string(resp.Body())).
			Error("钉钉服务器返回错误")
		return fmt.Errorf("钉钉服务器返回错误: %s", resp.Status())
	}

	s.logger.Info("成功发送 API 变更通知到钉钉")
	return nil
}

// buildApiDiffMarkdown 构建 API 差异的 Markdown 内容
func (s *NotifyService) buildApiDiffMarkdown(diff apifox.ApiDiff) string {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("### API变更通知: %s\n\n", diff.Name))
	buffer.WriteString(fmt.Sprintf("**接口ID:** %d\n\n", diff.ApiID))
	buffer.WriteString(fmt.Sprintf("**请求方法:** %s\n\n", diff.Method))

	// 方法变更
	if diff.MethodDiff {
		buffer.WriteString("#### 请求方法变更\n\n")
		buffer.WriteString(fmt.Sprintf("- 旧方法: `%s`\n", strings.ToUpper(diff.OldMethod)))
		buffer.WriteString(fmt.Sprintf("- 新方法: `%s`\n\n", strings.ToUpper(diff.Method)))
	}

	// 路径变更
	if diff.PathDiff {
		buffer.WriteString("#### 路径变更\n\n")
		buffer.WriteString(fmt.Sprintf("- 旧路径: `%s`\n", diff.OldPath))
		buffer.WriteString(fmt.Sprintf("- 新路径: `%s`\n\n", diff.NewPath))
	}

	// 请求体变更
	if diff.RequestBodyDiff {
		buffer.WriteString("#### 请求体变更\n\n")

		// 处理请求体详情，移除空行和多余的换行符，确保格式一致
		bodyDetail := diff.RequestBodyDetail
		bodyDetail = strings.TrimSpace(bodyDetail)

		// 确保请求体变更内容正确显示
		buffer.WriteString("```\n")

		// 如果详情为空，添加默认信息
		if bodyDetail == "" {
			buffer.WriteString("请求体发生变更\n")
		} else {
			buffer.WriteString(bodyDetail)
			// 确保结尾没有多余的换行符
			if !strings.HasSuffix(bodyDetail, "\n") {
				buffer.WriteString("\n")
			}
		}

		buffer.WriteString("```\n\n")
	}

	// 参数变更
	if diff.ParametersDiff {
		buffer.WriteString("#### 参数变更\n\n")
		buffer.WriteString(fmt.Sprintf("```\n%s\n```\n\n", diff.ParametersDetail))
	}

	// 响应变更
	if diff.ResponsesDiff {
		buffer.WriteString("#### 响应变更\n\n")
		buffer.WriteString(fmt.Sprintf("```\n%s\n```\n\n", diff.ResponsesDetail))
	}

	// 修改者信息
	buffer.WriteString(fmt.Sprintf("**修改者:** %s\n\n", diff.ModifierName))
	buffer.WriteString(fmt.Sprintf("**修改时间:** %s\n\n", diff.ModifiedTime))

	return buffer.String()
}

// ExtractNameTimeFromContent 从 webhook 内容中提取修改者姓名和时间
func ExtractNameTimeFromContent(content string) (string, string) {
	lines := strings.Split(content, "\n")
	var name, timeStr string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "修改者：") {
			name = strings.TrimPrefix(line, "修改者：")
		} else if strings.HasPrefix(line, "修改时间：") {
			timeStr = strings.TrimPrefix(line, "修改时间：")
		}
	}

	return name, timeStr
}
