package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xhy/api-pulse/internal/apifox"
	"github.com/xhy/api-pulse/internal/storage"
)

// ApiService API服务
type ApiService struct {
	logger        *logrus.Logger
	apifox        *apifox.Client
	storage       *storage.ApiStore
	diffService   *apifox.DiffService
	syncInterval  time.Duration
	stopSync      chan struct{}
	isSyncRunning bool
	syncMutex     sync.Mutex
}

// NewApiService 创建新的API服务
func NewApiService(logger *logrus.Logger, client *apifox.Client, storage *storage.ApiStore, diffService *apifox.DiffService) *ApiService {
	return &ApiService{
		logger:        logger,
		apifox:        client,
		storage:       storage,
		diffService:   diffService,
		syncInterval:  time.Hour, // 默认1小时同步一次
		stopSync:      make(chan struct{}),
		isSyncRunning: false,
	}
}

// SetSyncInterval 设置同步间隔
func (s *ApiService) SetSyncInterval(interval time.Duration) {
	s.syncInterval = interval
}

// StartSync 开始周期性同步
func (s *ApiService) StartSync() {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	if s.isSyncRunning {
		s.logger.Info("同步任务已在运行中")
		return
	}

	s.isSyncRunning = true
	s.stopSync = make(chan struct{})

	go func() {
		ticker := time.NewTicker(s.syncInterval)
		defer ticker.Stop()

		// 立即执行一次同步
		s.SyncAllAPIs()

		for {
			select {
			case <-ticker.C:
				s.SyncAllAPIs()
			case <-s.stopSync:
				s.logger.Info("停止API同步任务")
				return
			}
		}
	}()

	s.logger.WithField("interval", s.syncInterval.String()).Info("已启动API定时同步")
}

// StopSync 停止周期性同步
func (s *ApiService) StopSync() {
	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	if !s.isSyncRunning {
		return
	}

	close(s.stopSync)
	s.isSyncRunning = false
	s.logger.Info("API同步任务已停止")
}

// SyncAllAPIs 同步所有API信息
func (s *ApiService) SyncAllAPIs() {
	s.logger.Info("开始同步所有API信息")

	// 获取API树形列表
	resp, err := s.apifox.GetApiTreeList()
	if err != nil {
		s.logger.WithError(err).Error("获取API树形列表失败")
		return
	}

	if resp == nil || !resp.Success {
		s.logger.Warn("API树形列表返回非成功状态")
		return
	}

	// 提取所有API项
	apiItems := s.ExtractApiItems(resp.Data)
	s.logger.WithField("api_count", len(apiItems)).Info("同步：已提取API项")

	// 过滤出类型为apiDetail的项
	var validApiItems []ApiItem
	for _, item := range apiItems {
		if item.Type == "apiDetail" && item.Key != "" {
			validApiItems = append(validApiItems, item)
		}
	}
	s.logger.WithField("valid_api_count", len(validApiItems)).Info("同步：有效API数量")

	// 获取当前存储的API信息
	currentApis := s.storage.GetAllApis()
	s.logger.WithField("current_count", len(currentApis)).Info("当前缓存的API数量")

	// 使用WaitGroup等待所有同步完成
	var wg sync.WaitGroup
	wg.Add(len(validApiItems))

	// 限制并发数
	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)

	// 统计
	var mutex sync.Mutex
	updatedCount := 0
	unchangedCount := 0
	newCount := 0
	errorCount := 0

	// 并发处理每个API项
	for _, item := range validApiItems {
		go func(item ApiItem) {
			defer wg.Done()

			// 占用并发槽
			sem <- struct{}{}
			defer func() { <-sem }()

			apiKey := item.Key
			s.logger.WithField("api_key", apiKey).Debug("同步处理API")

			// 获取API详情
			apiDetailResp, err := s.apifox.GetApiDetail(apiKey)
			if err != nil {
				s.logger.WithError(err).WithField("api_key", apiKey).Error("获取API详情失败")
				mutex.Lock()
				errorCount++
				mutex.Unlock()
				return
			}

			if !apiDetailResp.Success || isEmptyApiDetail(apiDetailResp.Data) {
				s.logger.WithField("api_key", apiKey).Warn("API详情无效")
				mutex.Lock()
				errorCount++
				mutex.Unlock()
				return
			}

			// 查找存储中是否已有此API
			oldApiInfo, exists := currentApis[apiKey]

			// 准备新的API信息
			newApiInfo := apifox.StoredApiInfo{
				ApiKey:    apiKey,
				ApiID:     apiDetailResp.Data.ID,
				Name:      apiDetailResp.Data.Name,
				Method:    strings.ToLower(apiDetailResp.Data.Method),
				ApiPath:   apiDetailResp.Data.Path,
				Detail:    apiDetailResp.Data,
				UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
			}

			if exists {
				// 比较差异
				diff := s.diffService.CompareApis(oldApiInfo.Detail, apiDetailResp.Data, "", "")

				// 检查是否有实质性变更
				if diff.PathDiff || diff.MethodDiff || diff.RequestBodyDiff || diff.ParametersDiff || diff.ResponsesDiff {
					s.logger.WithFields(logrus.Fields{
						"api_key":     apiKey,
						"api_name":    newApiInfo.Name,
						"path_diff":   diff.PathDiff,
						"method_diff": diff.MethodDiff,
						"body_diff":   diff.RequestBodyDiff,
						"params_diff": diff.ParametersDiff,
						"resp_diff":   diff.ResponsesDiff,
					}).Info("检测到API变更")

					mutex.Lock()
					updatedCount++
					mutex.Unlock()
				} else {
					mutex.Lock()
					unchangedCount++
					mutex.Unlock()
				}
			} else {
				// 这是一个新API
				s.logger.WithField("api_name", newApiInfo.Name).Info("发现新API")
				mutex.Lock()
				newCount++
				mutex.Unlock()
			}

			// 无论是否有变更，都保存最新信息
			if err := s.storage.SaveApi(newApiInfo); err != nil {
				s.logger.WithError(err).WithField("api_key", apiKey).Error("保存API信息失败")
			}
		}(item)
	}

	// 等待所有goroutine完成
	wg.Wait()

	s.logger.WithFields(logrus.Fields{
		"total":     len(validApiItems),
		"updated":   updatedCount,
		"unchanged": unchangedCount,
		"new":       newCount,
		"error":     errorCount,
	}).Info("API同步完成")
}

// InitializeApiList 初始化API列表
func (s *ApiService) InitializeApiList() (int, int, []string, error) {
	s.logger.Info("开始初始化 API 列表")

	// 获取 API 树形列表
	resp, err := s.apifox.GetApiTreeList()
	if err != nil {
		s.logger.WithError(err).Error("无法获取 API 树形列表")
		return 0, 0, nil, err
	}

	if resp == nil || !resp.Success {
		s.logger.Warn("API 树形列表返回非成功状态")
		return 0, 0, nil, fmt.Errorf("API 树形列表请求未成功")
	}

	// 提取所有 API 项
	apiItems := s.ExtractApiItems(resp.Data)
	s.logger.WithField("api_count", len(apiItems)).Info("已提取 API 项")

	// 过滤出类型为 apiDetail 的项
	var validApiItems []ApiItem
	for _, item := range apiItems {
		if item.Type == "apiDetail" && item.Key != "" {
			validApiItems = append(validApiItems, item)
		}
	}
	s.logger.WithField("valid_api_count", len(validApiItems)).Info("有效 API 数量")

	// 初始化计数器和通道
	successCount := 0
	failureCount := 0
	var failedApis []string
	var mutex sync.Mutex // 用于保护计数器和失败API列表

	// 使用 WaitGroup 等待所有 goroutine 完成
	var wg sync.WaitGroup
	wg.Add(len(validApiItems))

	// 限制并发数
	maxConcurrency := 5
	sem := make(chan struct{}, maxConcurrency)

	s.logger.Info("开始并发获取 API 详情")

	// 并发处理每个 API 项
	for _, item := range validApiItems {
		// 获取 API 的基本信息（从API树形列表中提取）
		apiBasicInfo := findApiBasicInfoFromTreeItem(item, resp.Data)

		// 启动 goroutine 获取 API 详情
		go func(item ApiItem, basicInfo *apifox.ApiBasic) {
			defer wg.Done()

			// 占用并发槽
			sem <- struct{}{}
			defer func() { <-sem }()

			s.logger.WithFields(logrus.Fields{
				"key":  item.Key,
				"name": item.Name,
			}).Debug("处理 API")

			// 获取 API 详情
			apiDetails, err := s.apifox.GetApiDetail(item.Key)

			// 创建 API 信息对象，准备存储
			var apiInfo apifox.StoredApiInfo
			apiInfo.ApiKey = item.Key
			apiInfo.Name = item.Name
			apiInfo.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

			// 如果有基本信息，先设置方法和路径
			if basicInfo != nil {
				apiInfo.ApiID = basicInfo.ID
				apiInfo.Method = strings.ToLower(basicInfo.Method)
				apiInfo.ApiPath = basicInfo.Path
			}

			// 如果成功获取到详情且不为空对象，使用详情信息
			if err == nil && apiDetails != nil && apiDetails.Success && !isEmptyApiDetail(apiDetails.Data) {
				apiInfo.ApiID = apiDetails.Data.ID
				apiInfo.Method = strings.ToLower(apiDetails.Data.Method)
				apiInfo.ApiPath = apiDetails.Data.Path
				apiInfo.Detail = apiDetails.Data

				mutex.Lock()
				successCount++
				mutex.Unlock()

				s.logger.WithFields(logrus.Fields{
					"api_name": item.Name,
					"path":     apiDetails.Data.Path,
					"method":   apiDetails.Data.Method,
				}).Info("成功初始化 API")
			} else {
				// 记录错误或空对象情况
				mutex.Lock()
				failureCount++
				failedApis = append(failedApis, item.Name)
				mutex.Unlock()

				if err != nil {
					s.logger.WithError(err).WithField("api_name", item.Name).Warn("获取 API 详情失败，使用基本信息")
				} else {
					s.logger.WithField("api_name", item.Name).Warn("API 详情为空对象，使用基本信息")
				}
			}

			// 无论如何，都保存 API 信息到存储
			if apiInfo.Method != "" && apiInfo.ApiPath != "" {
				if err := s.storage.SaveApi(apiInfo); err != nil {
					s.logger.WithError(err).WithField("api_name", item.Name).Error("存储 API 信息失败")
				} else {
					s.logger.WithFields(logrus.Fields{
						"api_name": item.Name,
						"method":   apiInfo.Method,
						"path":     apiInfo.ApiPath,
					}).Debug("API 信息已保存")
				}
			} else {
				s.logger.WithField("api_name", item.Name).Warn("API 方法或路径为空，无法保存")
			}
		}(item, apiBasicInfo)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	s.logger.WithFields(logrus.Fields{
		"success_count": successCount,
		"failure_count": failureCount,
	}).Info("API 列表初始化完成")

	return successCount, failureCount, failedApis, nil
}

// isEmptyApiDetail 检查 API 详情是否为空对象
func isEmptyApiDetail(detail apifox.ApiDetail) bool {
	return detail.ID == 0 && detail.Name == "" && detail.Path == "" && detail.Method == ""
}

// findApiBasicInfoFromTreeItem 从 API 树形列表中查找 API 的基本信息
func findApiBasicInfoFromTreeItem(item ApiItem, treeData interface{}) *apifox.ApiBasic {
	// 从数组中查找
	if data, ok := treeData.([]interface{}); ok {
		for _, d := range data {
			if m, ok := d.(map[string]interface{}); ok {
				// 检查是否匹配当前项
				if keyStr, ok := m["key"].(string); ok && keyStr == item.Key {
					// 尝试获取 api 字段
					if apiData, ok := m["api"].(map[string]interface{}); ok {
						basic := &apifox.ApiBasic{}

						// 提取基本字段
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

						return basic
					}
				}

				// 递归检查 children 字段
				if children, ok := m["children"]; ok && children != nil {
					if result := findApiBasicInfoFromTreeItem(item, children); result != nil {
						return result
					}
				}
			}
		}
	}

	return nil
}

// ExtractApiItems 从数据中提取 API 项
func (s *ApiService) ExtractApiItems(data interface{}) []ApiItem {
	var result []ApiItem

	// 处理数组类型数据
	if items, ok := data.([]interface{}); ok {
		for _, item := range items {
			result = append(result, s.ExtractApiItems(item)...)
		}
		return result
	}

	// 处理映射类型数据
	if item, ok := data.(map[string]interface{}); ok {
		// 获取关键字段
		var apiItem ApiItem

		if key, ok := item["key"].(string); ok {
			apiItem.Key = key
		}
		if typ, ok := item["type"].(string); ok {
			apiItem.Type = typ
		}
		if name, ok := item["name"].(string); ok {
			apiItem.Name = name
		}

		// 如果是有效的API项，添加到结果中
		if apiItem.Key != "" && apiItem.Type != "" {
			result = append(result, apiItem)
		}

		// 递归处理子项
		if children, ok := item["children"]; ok && children != nil {
			childResults := s.ExtractApiItems(children)
			result = append(result, childResults...)
		}
	}

	return result
}

// ApiItem 表示一个 API 项
type ApiItem struct {
	Key  string
	Type string
	Name string
}
