package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xhy/api-pulse/internal/apifox"
	"github.com/xhy/api-pulse/internal/dingtalk"
	"github.com/xhy/api-pulse/internal/service"
	"github.com/xhy/api-pulse/internal/storage"
)

// ApiNotifyHandler Webhook 处理器
type ApiNotifyHandler struct {
	apifoxClient  *apifox.Client
	diffService   *apifox.DiffService
	notifyService *dingtalk.NotifyService
	apiStore      *storage.ApiStore
	logger        *logrus.Logger
	apiService    *service.ApiService
}

// NewApiNotifyHandler 创建新的 Webhook 处理器
func NewApiNotifyHandler(
	apifoxClient *apifox.Client,
	diffService *apifox.DiffService,
	notifyService *dingtalk.NotifyService,
	apiStore *storage.ApiStore,
	logger *logrus.Logger,
	apiService *service.ApiService,
) *ApiNotifyHandler {
	return &ApiNotifyHandler{
		apifoxClient:  apifoxClient,
		diffService:   diffService,
		notifyService: notifyService,
		apiStore:      apiStore,
		logger:        logger,
		apiService:    apiService,
	}
}

// HandleWebhook 处理 API 变更的 Webhook
func (h *ApiNotifyHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var payload apifox.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.WithError(err).Error("解析 Webhook 请求体失败")
		http.Error(w, "解析请求失败", http.StatusBadRequest)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"event":   payload.Event,
		"title":   payload.Title,
		"content": payload.Content,
	}).Info("接收到 Webhook")

	// 检查事件类型
	if payload.Event != "API_UPDATED" && payload.Event != "API_CREATED" {
		h.logger.WithField("event", payload.Event).Info("忽略非 API 更新/创建事件")
		w.WriteHeader(http.StatusOK)
		return
	}

	// 标记是否为新创建的 API
	isNewApi := payload.Event == "API_CREATED"

	// 解析 webhook 内容获取 API 名称和路径
	apiName, apiPath, err := apifox.ParseWebhookContent(payload.Content)
	if err != nil {
		h.logger.WithError(err).Error("解析 Webhook 内容失败")
		http.Error(w, "解析 Webhook 内容失败", http.StatusBadRequest)
		return
	}

	// 提取修改者信息
	modifierName, modifiedTime := dingtalk.ExtractNameTimeFromContent(payload.Content)

	// 从路径提取 HTTP 方法
	method := apifox.ExtractMethodFromPath(apiPath)
	if method == "" {
		h.logger.WithError(fmt.Errorf("无法从路径提取 HTTP 方法: %s", apiPath)).Error("解析 API 路径失败")
		http.Error(w, "无法从路径提取 HTTP 方法", http.StatusBadRequest)
		return
	}

	// 提取实际路径（不包含方法）
	path := strings.TrimPrefix(apiPath, method+" ")
	path = strings.TrimSpace(path)

	h.logger.WithFields(logrus.Fields{
		"api_name": apiName,
		"method":   method,
		"path":     path,
	}).Debug("已解析 API 信息")

	// 步骤1: 获取最新的API映射信息
	h.logger.Info("正在获取最新的 API 映射信息以匹配更改")
	apiMappings, err := h.apifoxClient.GetApiMappings()
	if err != nil {
		h.logger.WithError(err).Error("获取 API 映射信息失败")
		http.Error(w, "无法获取最新 API 信息", http.StatusInternalServerError)
		return
	}

	// 步骤2: 使用方法和路径查找对应的API
	split := strings.Split(path, " ")
	lookupKey := strings.ToLower(split[0]) + " " + split[1]
	apiBasic, exists := apiMappings[lookupKey]

	if !exists {
		h.logger.WithFields(logrus.Fields{
			"method": method,
			"path":   path,
		}).Warn("在最新的 API 映射中未找到匹配的 API")

		// 尝试从存储中查找，以防是路径变更
		oldApiInfo, oldExists := h.apiStore.GetApiByPath(method, path)
		if !oldExists {
			h.logger.Error("无法找到对应的 API 信息，无法处理变更")
			http.Error(w, "未找到对应的 API", http.StatusNotFound)
			return
		}

		// 使用存储中的信息继续处理
		h.logger.WithField("api_key", oldApiInfo.ApiKey).Info("使用存储的 API 信息处理变更")

		// 获取API详情
		apiDetailResp, err := h.apifoxClient.GetApiDetail(oldApiInfo.ApiKey)
		if err != nil {
			h.logger.WithError(err).Error("获取 API 详情失败")
			http.Error(w, "无法获取 API 详情", http.StatusInternalServerError)
			return
		}

		// 检查责任人过滤
		if h.apifoxClient.GetConfig().ResponsibleId != apiDetailResp.Data.ResponsibleID {

			h.logger.WithFields(logrus.Fields{
				"api_name":              oldApiInfo.Name,
				"api_id":                oldApiInfo.ApiID,
				"config_responsible_id": h.apifoxClient.GetConfig().ResponsibleId,
				"api_responsible_id":    apiDetailResp.Data.ResponsibleID,
			}).Info("API负责人与配置的负责人不匹配，跳过通知")

			// 仍然保存API信息，但不发送通知
			apiInfo := apifox.StoredApiInfo{
				ApiKey:    oldApiInfo.ApiKey,
				ApiID:     apiDetailResp.Data.ID,
				Name:      oldApiInfo.Name,
				Method:    strings.ToLower(apiDetailResp.Data.Method),
				ApiPath:   apiDetailResp.Data.Path,
				Detail:    apiDetailResp.Data,
				UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
			}

			if err := h.apiStore.SaveApi(apiInfo); err != nil {
				h.logger.WithError(err).WithField("apiKey", oldApiInfo.ApiKey).Error("更新 API 信息失败")
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		// 比较差异
		diff := h.diffService.CompareApis(oldApiInfo.Detail, apiDetailResp.Data, modifierName, modifiedTime)

		// 检查是否有差异
		if diff.PathDiff || diff.MethodDiff || diff.RequestBodyDiff || diff.ParametersDiff || diff.ResponsesDiff {
			// 发送通知
			if err := h.notifyService.SendApiChangedNotification(*diff); err != nil {
				h.logger.WithError(err).Error("发送 API 变更通知失败")
				http.Error(w, "发送通知失败", http.StatusInternalServerError)
				return
			}

			// 更新存储的 API 信息
			apiInfo := apifox.StoredApiInfo{
				ApiKey:    oldApiInfo.ApiKey,
				ApiID:     apiDetailResp.Data.ID,
				Name:      oldApiInfo.Name,
				Method:    strings.ToLower(apiDetailResp.Data.Method),
				ApiPath:   apiDetailResp.Data.Path,
				Detail:    apiDetailResp.Data,
				UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
			}

			if err := h.apiStore.SaveApi(apiInfo); err != nil {
				h.logger.WithError(err).WithField("apiKey", oldApiInfo.ApiKey).Error("更新 API 信息失败")
			}
		} else {
			h.logger.WithField("apiKey", oldApiInfo.ApiKey).Info("API 没有实质性变更，不发送通知")
		}
	} else {
		// 使用新找到的API信息
		h.logger.WithFields(logrus.Fields{
			"api_id":   apiBasic.ID,
			"api_name": apiBasic.Name,
		}).Info("在最新映射中找到匹配的 API")

		// 构建API Key
		apiKey := fmt.Sprintf("apiDetail.%d", apiBasic.ID)

		// 检查旧API信息
		var oldApiInfo apifox.StoredApiInfo
		var oldExists bool

		// 先尝试通过新API ID查找
		oldApiInfo, oldExists = h.apiStore.GetApi(apiKey)

		// 如果没找到，再通过方法和路径查找
		if !oldExists {
			oldApiInfo, oldExists = h.apiStore.GetApiByPath(method, path)
		}

		// 获取API详情
		apiDetailResp, err := h.apifoxClient.GetApiDetail(apiKey)
		if err != nil {
			h.logger.WithError(err).Error("获取 API 详情失败")
			http.Error(w, "无法获取 API 详情", http.StatusInternalServerError)
			return
		}

		// 检查责任人过滤
		if h.apifoxClient.GetConfig().ResponsibleId != apiDetailResp.Data.ResponsibleID {
			h.logger.WithFields(logrus.Fields{
				"api_name":              apiBasic.Name,
				"api_id":                apiBasic.ID,
				"config_responsible_id": h.apifoxClient.GetConfig().ResponsibleId,
				"api_responsible_id":    apiDetailResp.Data.ResponsibleID,
			}).Info("API负责人与配置的负责人不匹配，跳过通知")

			// 仍然保存API信息，但不发送通知
			apiInfo := apifox.StoredApiInfo{
				ApiKey:    apiKey,
				ApiID:     apiBasic.ID,
				Name:      apiBasic.Name,
				Method:    strings.ToLower(apiBasic.Method),
				ApiPath:   apiBasic.Path,
				Detail:    apiDetailResp.Data,
				UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
			}

			if err := h.apiStore.SaveApi(apiInfo); err != nil {
				h.logger.WithError(err).WithField("apiKey", apiKey).Error("更新/保存 API 信息失败")
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		// 如果找到旧信息，则比较差异
		if oldExists {
			// 比较差异
			diff := h.diffService.CompareApis(oldApiInfo.Detail, apiDetailResp.Data, modifierName, modifiedTime)

			// 检查是否有差异
			if diff.PathDiff || diff.MethodDiff || diff.RequestBodyDiff || diff.ParametersDiff || diff.ResponsesDiff {
				// 发送通知
				if err := h.notifyService.SendApiChangedNotification(*diff); err != nil {
					h.logger.WithError(err).Error("发送 API 变更通知失败")
					http.Error(w, "发送通知失败", http.StatusInternalServerError)
					return
				}
			} else {
				h.logger.WithField("apiKey", apiKey).Info("API 没有实质性变更，不发送通知")
			}
		} else {
			// 这是一个新API
			h.logger.WithField("api_name", apiBasic.Name).Info("检测到新 API")

			// 如果是 API_CREATED 事件，发送 API 创建通知
			if isNewApi {
				// 创建一个包含新API信息的差异对象
				createdDiff := &apifox.ApiDiff{
					ApiID:        apiBasic.ID,
					Name:         apiBasic.Name,
					NewPath:      apiBasic.Path,
					Method:       apiBasic.Method,
					ModifierName: modifierName,
					ModifiedTime: modifiedTime,
					IsNewApi:     true,
				}

				// 发送API创建通知
				if err := h.notifyService.SendApiCreatedNotification(*createdDiff); err != nil {
					h.logger.WithError(err).Error("发送 API 创建通知失败")
					http.Error(w, "发送通知失败", http.StatusInternalServerError)
					return
				}
			} else {
				h.logger.WithField("api_name", apiBasic.Name).Info("新 API 未通过创建事件通知，仅保存信息不发送通知")
			}
		}

		// 无论如何，都更新/保存最新的API信息
		apiInfo := apifox.StoredApiInfo{
			ApiKey:    apiKey,
			ApiID:     apiBasic.ID,
			Name:      apiBasic.Name,
			Method:    strings.ToLower(apiBasic.Method),
			ApiPath:   apiBasic.Path,
			Detail:    apiDetailResp.Data,
			UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}

		if err := h.apiStore.SaveApi(apiInfo); err != nil {
			h.logger.WithError(err).WithField("apiKey", apiKey).Error("更新/保存 API 信息失败")
		}
	}

	w.WriteHeader(http.StatusOK)
}

// HealthCheck 健康检查
func (h *ApiNotifyHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}
