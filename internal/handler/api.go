package handler

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/xhy/api-pulse/internal/apifox"
	"github.com/xhy/api-pulse/internal/storage"
)

// ApiHandler API处理器
type ApiHandler struct {
	logger  *logrus.Logger
	apifox  *apifox.Client
	storage *storage.ApiStore
}

// NewApiHandler 创建新的API处理器
func NewApiHandler(logger *logrus.Logger, client *apifox.Client, storage *storage.ApiStore) *ApiHandler {
	return &ApiHandler{
		logger:  logger,
		apifox:  client,
		storage: storage,
	}
}

// InitializeApiList 初始化 API 列表
func (h *ApiHandler) InitializeApiList(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("开始初始化 API 列表")

	// 获取 API 树形列表
	tree, err := h.apifox.GetApiTreeList()
	if err != nil {
		h.logger.WithError(err).Error("无法获取 API 树形列表")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "无法获取 API 树形列表: " + err.Error(),
		})
		return
	}

	if tree == nil || len(tree.Data) == 0 {
		h.logger.Warn("API 树形列表为空")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "API 树形列表为空，没有需要初始化的 API",
		})
		return
	}

	h.logger.WithField("api_count", len(tree.Data)).Info("成功获取 API 树形列表")

	successCount := 0
	failureCount := 0
	var failedApis []string

	// 对每个 API，获取并存储详细信息
	for _, item := range tree.Data {
		h.processApiTreeItem(item, &successCount, &failureCount, &failedApis)
	}

	h.logger.WithFields(logrus.Fields{
		"success_count": successCount,
		"failure_count": failureCount,
	}).Info("API 列表初始化完成")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "API 列表初始化完成",
		"success_count": successCount,
		"failure_count": failureCount,
		"failed_apis":   failedApis,
	})
}

// processApiTreeItem 处理 API 树节点，递归处理子节点
func (h *ApiHandler) processApiTreeItem(item apifox.ApiTreeItem, successCount, failureCount *int, failedApis *[]string) {
	// 如果是 API 节点，处理 API
	if item.Type == "apiDetail" && item.Api != nil {
		h.logger.WithField("api_name", item.Name).Debug("处理 API")

		// 获取 API 详情
		apiDetails, err := h.apifox.GetApiDetail(item.Key)
		if err != nil {
			h.logger.WithError(err).WithField("api_name", item.Name).Error("获取 API 详情失败")
			*failureCount++
			*failedApis = append(*failedApis, item.Name)
			return
		}

		// 存储 API 详情
		apiInfo := apifox.StoredApiInfo{
			ApiKey:    item.Key,
			ApiID:     apiDetails.Data.ID,
			Detail:    apiDetails.Data,
			UpdatedAt: apiDetails.Data.UpdatedAt,
		}
		if err := h.storage.SaveApi(apiInfo); err != nil {
			h.logger.WithError(err).WithField("api_name", item.Name).Error("存储 API 详情失败")
			*failureCount++
			*failedApis = append(*failedApis, item.Name)
			return
		}

		*successCount++
		h.logger.WithField("api_name", item.Name).Info("成功初始化 API")
	}

	// 处理子节点
	if len(item.Children) > 0 {
		// 将 Children JSON 解析为适当的类型
		var children []apifox.ApiTreeItem
		if err := json.Unmarshal(item.Children, &children); err != nil {
			h.logger.WithError(err).WithField("item_name", item.Name).Warn("解析子节点失败")
			return
		}

		for _, child := range children {
			h.processApiTreeItem(child, successCount, failureCount, failedApis)
		}
	}
}
