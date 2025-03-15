package apifox

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"github.com/xhy/api-pulse/config"
)

// Client Apifox API 客户端
type Client struct {
	config     *config.ApifoxConfig
	httpClient *resty.Client
	logger     *logrus.Logger
}

// NewClient 创建新的 Apifox 客户端
func NewClient(cfg *config.ApifoxConfig, logger *logrus.Logger) *Client {
	client := resty.New()

	// 添加请求/响应日志拦截器
	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		logger.WithFields(logrus.Fields{
			"url":     req.URL,
			"method":  req.Method,
			"headers": req.Header,
		}).Debug("发送 API 请求")
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logger.WithFields(logrus.Fields{
			"status":       resp.Status(),
			"response_len": len(resp.Body()),
			"time":         resp.Time(),
		}).Debug("收到 API 响应")

		// 仅在调试模式下记录完整响应体
		if logger.Level == logrus.DebugLevel && len(resp.Body()) < 5000 {
			logger.WithField("response", string(resp.Body())).Debug("响应内容")
		}

		return nil
	})

	return &Client{
		config:     cfg,
		httpClient: client,
		logger:     logger,
	}
}

// GetApiTreeList 获取项目的 API 树形列表
func (c *Client) GetApiTreeList() (*ApiTreeListResponse, error) {
	url := fmt.Sprintf("%s/projects/%s/api-tree-list?locale=zh-CN",
		c.config.BaseURL, c.config.ProjectID)

	c.logger.WithFields(logrus.Fields{
		"url":        url,
		"project_id": c.config.ProjectID,
		"branch_id":  c.config.BranchID,
	}).Info("正在获取 API 树形列表")

	// 添加更多诊断信息
	c.logger.WithFields(logrus.Fields{
		"auth_token_length": len(c.config.Authorization),
		"auth_token_prefix": c.config.Authorization[:10] + "...", // 只记录前10个字符
	}).Info("使用的认证信息")

	// 创建与curl命令类似的请求
	request := c.httpClient.R().
		SetHeader("authorization", fmt.Sprintf("Bearer %s", c.config.Authorization)).
		SetHeader("x-branch-id", c.config.BranchID).
		SetHeader("x-project-id", c.config.ProjectID).
		// 添加更多用户curl请求中使用的头信息
		SetHeader("accept", "*/*").
		SetHeader("accept-language", "zh-CN").
		SetHeader("access-control-allow-origin", "*").
		SetHeader("origin", "https://app.apifox.com").
		SetHeader("referer", "https://app.apifox.com/").
		SetHeader("sec-ch-ua", "\"Not(A:Brand\";v=\"99\", \"Microsoft Edge\";v=\"133\", \"Chromium\";v=\"133\"").
		SetHeader("sec-ch-ua-mobile", "?0").
		SetHeader("sec-ch-ua-platform", "\"macOS\"").
		SetHeader("sec-fetch-dest", "empty").
		SetHeader("sec-fetch-mode", "cors").
		SetHeader("sec-fetch-site", "same-site").
		SetHeader("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0").
		SetHeader("x-client-mode", "web").
		SetHeader("x-client-version", "2.7.2-alpha.2").
		SetHeader("x-device-id", "QYdpRHW1-OwOB-BN3F-lBDh-gRtHzeRe2ies")

	// 打印完整的请求头信息
	c.logger.WithField("headers", fmt.Sprintf("%v", request.Header)).Info("完整请求头")

	// 发送请求
	resp, err := request.Get(url)

	if err != nil {
		c.logger.WithError(err).Error("获取 API 列表失败")
		return nil, err
	}

	// 检查HTTP状态
	if resp.StatusCode() != 200 {
		c.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode(),
			"response":    string(resp.Body()),
		}).Error("API 返回非成功状态码")
		return nil, fmt.Errorf("API 请求失败: HTTP %d", resp.StatusCode())
	}

	// 记录完整的原始响应内容
	respBody := string(resp.Body())
	c.logger.WithField("raw_response", respBody).Info("API 树形列表原始响应")

	// 打印 JSON 原始解析结果
	var rawJson map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &rawJson); err != nil {
		c.logger.WithError(err).Error("解析原始 JSON 失败")
	} else {
		// 检查 data 字段的类型
		if data, ok := rawJson["data"]; ok {
			c.logger.WithFields(logrus.Fields{
				"data_type": fmt.Sprintf("%T", data),
				"data":      fmt.Sprintf("%v", data),
			}).Info("原始 data 字段")
		}
	}

	// 解析 JSON 响应
	var response ApiTreeListResponse
	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		c.logger.WithError(err).WithField("response", respBody).Error("解析 API 响应失败")
		return nil, err
	}

	// 使用更详细的日志输出
	c.logger.WithFields(logrus.Fields{
		"success":   response.Success,
		"data_type": fmt.Sprintf("%T", response.Data),
	}).Info("解析后的 API 树形列表基本信息")

	// 根据 Data 的具体类型添加特定日志
	switch data := response.Data.(type) {
	case map[string]interface{}:
		c.logger.WithField("map_keys", fmt.Sprintf("%v", getMapKeys(data))).Info("Data 是对象类型，包含的键")
	case []interface{}:
		c.logger.WithField("array_length", len(data)).Info("Data 是数组类型，包含的元素数量")
		// 如果是数组，记录第一个元素的类型和内容
		if len(data) > 0 {
			firstItem := data[0]
			c.logger.WithFields(logrus.Fields{
				"first_item_type": fmt.Sprintf("%T", firstItem),
			}).Debug("数组中第一个元素的信息")

			if m, ok := firstItem.(map[string]interface{}); ok {
				c.logger.WithFields(logrus.Fields{
					"first_item_keys": fmt.Sprintf("%v", getMapKeys(m)),
				}).Debug("第一个元素的键")
			}
		}
	default:
		c.logger.WithField("exact_type", fmt.Sprintf("%T", response.Data)).Info("Data 是其他类型")
	}

	// 数据详情
	dataJSON, _ := json.Marshal(response.Data)
	c.logger.WithField("data_json", string(dataJSON)).Info("API 树形列表数据内容")

	return &response, nil
}

// getMapKeys 获取 map 的所有键，辅助函数
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetApiDetail 获取单个 API 的详细信息
func (c *Client) GetApiDetail(apiKey string) (*ApiDetailResponse, error) {
	// 从 apiKey 中提取 ID，格式为 "apiDetail.ID"
	var apiID string
	_, err := fmt.Sscanf(apiKey, "apiDetail.%s", &apiID)
	if err != nil {
		c.logger.WithError(err).WithField("apiKey", apiKey).Error("解析 API ID 失败")
		return nil, fmt.Errorf("无效的 API Key 格式: %s", apiKey)
	}

	url := fmt.Sprintf("%s/projects/%s/http-apis/%s?locale=zh-CN",
		c.config.BaseURL, c.config.ProjectID, apiID)

	c.logger.WithFields(logrus.Fields{
		"url":        url,
		"project_id": c.config.ProjectID,
		"branch_id":  c.config.BranchID,
		"api_id":     apiID,
	}).Info("获取 API 详情")

	// 创建与树形列表请求相同格式的请求
	request := c.httpClient.R().
		SetHeader("authorization", fmt.Sprintf("Bearer %s", c.config.Authorization)).
		SetHeader("x-branch-id", c.config.BranchID).
		SetHeader("x-project-id", c.config.ProjectID).
		// 添加更多请求头
		SetHeader("accept", "*/*").
		SetHeader("accept-language", "zh-CN").
		SetHeader("access-control-allow-origin", "*").
		SetHeader("origin", "https://app.apifox.com").
		SetHeader("referer", "https://app.apifox.com/").
		SetHeader("sec-ch-ua", "\"Not(A:Brand\";v=\"99\", \"Microsoft Edge\";v=\"133\", \"Chromium\";v=\"133\"").
		SetHeader("sec-ch-ua-mobile", "?0").
		SetHeader("sec-ch-ua-platform", "\"macOS\"").
		SetHeader("sec-fetch-dest", "empty").
		SetHeader("sec-fetch-mode", "cors").
		SetHeader("sec-fetch-site", "same-site").
		SetHeader("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0").
		SetHeader("x-client-mode", "web").
		SetHeader("x-client-version", "2.7.2-alpha.2").
		SetHeader("x-device-id", "QYdpRHW1-OwOB-BN3F-lBDh-gRtHzeRe2ies")

	// 发送请求
	resp, err := request.Get(url)
	if err != nil {
		c.logger.WithError(err).WithField("apiKey", apiKey).Error("获取 API 详情失败")
		return nil, err
	}

	// 检查HTTP状态
	if resp.StatusCode() != 200 {
		c.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode(),
			"response":    string(resp.Body()),
		}).Error("API 详情请求返回非成功状态码")
		return nil, fmt.Errorf("API 详情请求失败: HTTP %d", resp.StatusCode())
	}

	// 记录响应内容
	respBody := string(resp.Body())
	c.logger.WithField("response", respBody).Info("API 详情原始响应")

	// 解析原始 JSON 响应
	var rawJson map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &rawJson); err != nil {
		c.logger.WithError(err).Error("解析原始 JSON 失败")
	} else {
		// 检查 data 字段的类型
		if data, ok := rawJson["data"]; ok {
			c.logger.WithFields(logrus.Fields{
				"data_type": fmt.Sprintf("%T", data),
				"data":      fmt.Sprintf("%v", data),
			}).Info("API 详情 data 字段")
		}
	}

	// 使用自定义解析逻辑检测空对象响应
	if respBody == "{\n\"success\": true,\n\"data\": {}\n}" {
		c.logger.Warn("API 详情返回空对象，可能需要不同的请求格式或认证")
	}

	// 使用 map 解析 JSON
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &rawResponse); err != nil {
		c.logger.WithError(err).Error("解析 API 详情响应为 map 失败")
		return nil, err
	}

	// 检查成功标志
	success, ok := rawResponse["success"].(bool)
	if !ok || !success {
		c.logger.Error("API 详情请求返回成功标志为 false 或格式不正确")
		return nil, fmt.Errorf("API 详情响应指示失败或格式不正确")
	}

	// 获取 data 字段
	dataRaw, exists := rawResponse["data"]
	if !exists {
		c.logger.Error("API 详情响应中不存在 data 字段")
		return nil, fmt.Errorf("API 详情响应中不存在 data 字段")
	}

	dataMap, ok := dataRaw.(map[string]interface{})
	if !ok {
		c.logger.Error("API 详情响应中 data 字段不是对象格式")
		return nil, fmt.Errorf("API 详情响应中 data 字段格式不正确")
	}

	// 是否为空对象
	if len(dataMap) == 0 {
		c.logger.Warn("API 详情响应中的 data 字段是空对象")
	}

	// 构建 ApiDetail 对象
	detail := ApiDetail{}

	// 提取基本字段
	if id, ok := dataMap["id"].(float64); ok {
		detail.ID = int(id)
	}
	if name, ok := dataMap["name"].(string); ok {
		detail.Name = name
	}
	if typ, ok := dataMap["type"].(string); ok {
		detail.Type = typ
	}
	if method, ok := dataMap["method"].(string); ok {
		detail.Method = method
	}
	if path, ok := dataMap["path"].(string); ok {
		detail.Path = path
	}
	if desc, ok := dataMap["description"].(string); ok {
		detail.Description = desc
	}
	if status, ok := dataMap["status"].(string); ok {
		detail.Status = status
	}
	if createdAt, ok := dataMap["createdAt"].(string); ok {
		detail.CreatedAt = createdAt
	}
	if updatedAt, ok := dataMap["updatedAt"].(string); ok {
		detail.UpdatedAt = updatedAt
	}
	if folderId, ok := dataMap["folderId"].(float64); ok {
		detail.FolderID = int(folderId)
	}
	if operationId, ok := dataMap["operationId"].(string); ok {
		detail.OperationID = operationId
	}
	if visibility, ok := dataMap["visibility"].(string); ok {
		detail.Visibility = visibility
	}
	if creatorId, ok := dataMap["creatorId"].(float64); ok {
		detail.CreatorID = int(creatorId)
	}
	if editorId, ok := dataMap["editorId"].(float64); ok {
		detail.EditorID = int(editorId)
	}

	// 处理 tags 字段
	if tagsRaw, exists := dataMap["tags"].([]interface{}); exists {
		for _, tag := range tagsRaw {
			if tagStr, ok := tag.(string); ok {
				detail.Tags = append(detail.Tags, tagStr)
			}
		}
	}

	// 处理 requestBody
	if rbRaw, exists := dataMap["requestBody"].(map[string]interface{}); exists {
		rb := RequestBody{}
		if rbType, ok := rbRaw["type"].(string); ok {
			rb.Type = rbType
		}
		if mediaType, ok := rbRaw["mediaType"].(string); ok {
			rb.MediaType = mediaType
		}
		// 直接保存 jsonSchema 作为接口类型
		if schema, exists := rbRaw["jsonSchema"]; exists {
			rb.JsonSchema = schema
		}
		// 处理 parameters
		if params, exists := rbRaw["parameters"].([]interface{}); exists {
			for _, p := range params {
				if paramMap, ok := p.(map[string]interface{}); ok {
					param := Parameter{}
					if id, ok := paramMap["id"].(string); ok {
						param.ID = id
					}
					if name, ok := paramMap["name"].(string); ok {
						param.Name = name
					}
					if required, ok := paramMap["required"].(bool); ok {
						param.Required = required
					}
					if desc, ok := paramMap["description"].(string); ok {
						param.Description = desc
					}
					if typ, ok := paramMap["type"].(string); ok {
						param.Type = typ
					}
					if enable, ok := paramMap["enable"].(bool); ok {
						param.Enable = enable
					}
					rb.Parameters = append(rb.Parameters, param)
				}
			}
		}
		detail.RequestBody = rb
	}

	// 处理 parameters
	if paramsRaw, exists := dataMap["parameters"].(map[string]interface{}); exists {
		params := Parameters{}

		// 处理 query 参数
		if queryParams, exists := paramsRaw["query"].([]interface{}); exists {
			for _, p := range queryParams {
				if paramMap, ok := p.(map[string]interface{}); ok {
					param := Parameter{}
					if id, ok := paramMap["id"].(string); ok {
						param.ID = id
					}
					if name, ok := paramMap["name"].(string); ok {
						param.Name = name
					}
					if required, ok := paramMap["required"].(bool); ok {
						param.Required = required
					}
					if desc, ok := paramMap["description"].(string); ok {
						param.Description = desc
					}
					if typ, ok := paramMap["type"].(string); ok {
						param.Type = typ
					}
					if enable, ok := paramMap["enable"].(bool); ok {
						param.Enable = enable
					}
					params.Query = append(params.Query, param)
				}
			}
		}

		// 处理 path 参数
		if pathParams, exists := paramsRaw["path"].([]interface{}); exists {
			for _, p := range pathParams {
				if paramMap, ok := p.(map[string]interface{}); ok {
					param := Parameter{}
					if id, ok := paramMap["id"].(string); ok {
						param.ID = id
					}
					if name, ok := paramMap["name"].(string); ok {
						param.Name = name
					}
					if required, ok := paramMap["required"].(bool); ok {
						param.Required = required
					}
					if desc, ok := paramMap["description"].(string); ok {
						param.Description = desc
					}
					if typ, ok := paramMap["type"].(string); ok {
						param.Type = typ
					}
					if enable, ok := paramMap["enable"].(bool); ok {
						param.Enable = enable
					}
					params.Path = append(params.Path, param)
				}
			}
		}

		detail.Parameters = params
	}

	// 处理 responses
	if respsRaw, exists := dataMap["responses"].([]interface{}); exists {
		for _, r := range respsRaw {
			if respMap, ok := r.(map[string]interface{}); ok {
				resp := Response{}
				if id, ok := respMap["id"].(float64); ok {
					resp.ID = int(id)
				}
				if name, ok := respMap["name"].(string); ok {
					resp.Name = name
				}
				if code, ok := respMap["code"].(float64); ok {
					resp.Code = int(code)
				}
				if contentType, ok := respMap["contentType"].(string); ok {
					resp.ContentType = contentType
				}
				if desc, ok := respMap["description"].(string); ok {
					resp.Description = desc
				}
				// 直接保存 jsonSchema 作为接口类型
				if schema, exists := respMap["jsonSchema"]; exists {
					resp.JsonSchema = schema
				}
				detail.Responses = append(detail.Responses, resp)
			}
		}
	}

	// 处理 commonParameters
	if cpRaw, exists := dataMap["commonParameters"].(map[string]interface{}); exists {
		cp := CommonParameters{}

		// 处理 header 参数
		if headers, exists := cpRaw["header"].([]interface{}); exists {
			for _, h := range headers {
				if headerMap, ok := h.(map[string]interface{}); ok {
					header := HeaderParam{}
					if name, ok := headerMap["name"].(string); ok {
						header.Name = name
					}
					cp.Header = append(cp.Header, header)
				}
			}
		}

		detail.CommonParameters = cp
	}

	// 构建并返回 ApiDetailResponse
	return &ApiDetailResponse{
		Success: success,
		Data:    detail,
	}, nil
}

// GetApiMappings 获取轻量级的API映射信息
// 此方法专门用于在收到webhook时快速获取所有API的基本映射信息
func (c *Client) GetApiMappings() (map[string]ApiBasic, error) {
	// 获取API树形列表
	resp, err := c.GetApiTreeList()
	if err != nil {
		c.logger.WithError(err).Error("获取API映射时无法获取API树形列表")
		return nil, err
	}

	if resp == nil || !resp.Success {
		c.logger.Error("获取API映射失败：API树形列表返回非成功状态")
		return nil, fmt.Errorf("API树形列表请求未成功")
	}

	// 结果映射，使用"method path"作为键
	mappings := make(map[string]ApiBasic)

	// 递归提取所有API基本信息
	c.extractApiMappingsFromTree(resp.Data, mappings)

	c.logger.WithField("mapping_count", len(mappings)).Info("成功获取API映射信息")

	return mappings, nil
}

// extractApiMappingsFromTree 从树形结构中递归提取API映射
func (c *Client) extractApiMappingsFromTree(data interface{}, mappings map[string]ApiBasic) {
	// 处理数组类型数据
	if items, ok := data.([]interface{}); ok {
		for _, item := range items {
			c.extractApiMappingsFromTree(item, mappings)
		}
		return
	}

	// 处理映射类型数据
	if item, ok := data.(map[string]interface{}); ok {
		// 检查是否是API类型
		if typeStr, ok := item["type"].(string); ok && typeStr == "apiDetail" {
			// 提取API基本信息
			if apiData, ok := item["api"].(map[string]interface{}); ok {
				var basic ApiBasic

				// 提取关键字段
				if id, ok := apiData["id"].(float64); ok {
					basic.ID = int(id)
				}
				if name, ok := apiData["name"].(string); ok {
					basic.Name = name
				}
				if method, ok := apiData["method"].(string); ok {
					basic.Method = method
				}
				if path, ok := apiData["path"].(string); ok {
					basic.Path = path
				}

				// 如果方法和路径都不为空，添加到映射中
				if basic.Method != "" && basic.Path != "" {
					key := strings.ToLower(basic.Method) + " " + basic.Path
					mappings[key] = basic
				}
			}
		}

		// 递归处理子项
		if children, ok := item["children"]; ok && children != nil {
			c.extractApiMappingsFromTree(children, mappings)
		}
	}
}
