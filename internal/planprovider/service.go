package planprovider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
)

const planGroupName = "Plan"

// ListProviders 列出所有 Plan Provider
func ListProviders(ctx context.Context, providerType model.PlanProviderType) ([]model.PlanProviderListItem, error) {
	var providers []model.PlanProvider
	if err := db.GetDB().WithContext(ctx).
		Where("provider_type = ?", providerType).
		Order("id ASC").
		Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("list plan providers: %w", err)
	}

	result := make([]model.PlanProviderListItem, 0, len(providers))
	for _, p := range providers {
		item := model.PlanProviderListItem{PlanProvider: p}
		if p.ChannelID > 0 {
			channel, err := op.ChannelGet(p.ChannelID, ctx)
			if err == nil {
				item.Models = channel.Model
				item.ChannelName = channel.Name
				item.ChannelEnabled = channel.Enabled
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// AddProvider 添加 Plan Provider：查询余额 → 创建 Channel → 加入 Plan 分组
func AddProvider(ctx context.Context, category model.PlanProviderCategory, apiKey, customName string) (*model.PlanProvider, error) {
	info := getCategoryInfo(category)
	if info == nil {
		return nil, fmt.Errorf("unknown plan provider category: %s", category)
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	name := customName
	if name == "" {
		name = info.Name
	}

	// 1. 查询余额 / TokenPlan
	var balance, balanceUsed float64
	var quotaTotal, quotaUsed, weeklyTotal, weeklyUsed float64
	var quotaResetAt, weeklyResetAt *string

	if info.Type == model.PlanProviderTypeBalance {
		result, err := QueryBalance(ctx, category, apiKey, info.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("query balance: %w", err)
		}
		balance = result.Balance
		balanceUsed = result.BalanceUsed
	} else {
		result, err := QueryTokenPlan(ctx, category, apiKey, info.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("query tokenplan: %w", err)
		}
		quotaTotal = result.QuotaTotal
		quotaUsed = result.QuotaUsed
		if result.QuotaResetAt != nil {
			s := result.QuotaResetAt.Format("2006-01-02 15:04:05")
			quotaResetAt = &s
		}
		weeklyTotal = result.WeeklyTotal
		weeklyUsed = result.WeeklyUsed
		if result.WeeklyResetAt != nil {
			s := result.WeeklyResetAt.Format("2006-01-02 15:04:05")
			weeklyResetAt = &s
		}
	}

	// 2. 确保 Plan 分组存在
	groupID, err := ensurePlanGroup(ctx)
	if err != nil {
		return nil, fmt.Errorf("ensure plan group: %w", err)
	}

	// 3. 创建 Channel
	channelName := fmt.Sprintf("[%s] %s", info.Name, name)
	if customName != "" {
		channelName = fmt.Sprintf("[%s] %s", info.Name, customName)
	}

	channel := &model.Channel{
		Name:    channelName,
		Type:    outbound.OutboundTypeOpenAIChat,
		Enabled: true,
		BaseUrls: []model.BaseUrl{
			{URL: info.BaseURL, Delay: 0},
		},
		Keys: []model.ChannelKey{
			{Enabled: true, ChannelKey: apiKey},
		},
		Model:     info.Models,
		AutoSync:  false,
		AutoGroup: model.AutoGroupTypeNone,
	}
	if err := op.ChannelCreate(channel, ctx); err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	// 4. 加入 Plan 分组
	if err := addChannelToGroup(ctx, channel.ID, channel.Model, groupID); err != nil {
		return nil, fmt.Errorf("add channel to plan group: %w", err)
	}

	// 5. 持久化 PlanProvider
	now := time.Now()
	provider := &model.PlanProvider{
		Name:         name,
		Category:     category,
		ProviderType: info.Type,
		APIKey:       apiKey,
		BaseURL:      info.BaseURL,
		ChannelID:    channel.ID,
		Balance:      balance,
		BalanceUsed:  balanceUsed,
		QuotaTotal:   quotaTotal,
		QuotaUsed:    quotaUsed,
		WeeklyTotal:  weeklyTotal,
		WeeklyUsed:   weeklyUsed,
	}

	if quotaResetAt != nil {
		if t, err := time.Parse("2006-01-02 15:04:05", *quotaResetAt); err == nil {
			provider.QuotaResetAt = &t
		}
	}
	if weeklyResetAt != nil {
		if t, err := time.Parse("2006-01-02 15:04:05", *weeklyResetAt); err == nil {
			provider.WeeklyResetAt = &t
		}
	}

	provider.LastRefresh = &now
	provider.CreatedAt = now
	provider.UpdatedAt = now

	if err := db.GetDB().WithContext(ctx).Create(provider).Error; err != nil {
		return nil, fmt.Errorf("create plan provider record: %w", err)
	}

	return provider, nil
}

// RefreshProvider 刷新 Plan Provider 的余额 / TokenPlan
func RefreshProvider(ctx context.Context, id int) (*model.PlanProvider, error) {
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("find plan provider: %w", err)
	}

	if provider.ProviderType == model.PlanProviderTypeBalance {
		result, err := QueryBalance(ctx, provider.Category, provider.APIKey, provider.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("refresh balance: %w", err)
		}
		provider.Balance = result.Balance
		provider.BalanceUsed = result.BalanceUsed
	} else {
		result, err := QueryTokenPlan(ctx, provider.Category, provider.APIKey, provider.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("refresh tokenplan: %w", err)
		}
		provider.QuotaTotal = result.QuotaTotal
		provider.QuotaUsed = result.QuotaUsed
		if result.QuotaResetAt != nil {
			provider.QuotaResetAt = result.QuotaResetAt
		}
		provider.WeeklyTotal = result.WeeklyTotal
		provider.WeeklyUsed = result.WeeklyUsed
		if result.WeeklyResetAt != nil {
			provider.WeeklyResetAt = result.WeeklyResetAt
		}
	}

	now := time.Now()
	provider.LastRefresh = &now
	provider.UpdatedAt = now

	if err := db.GetDB().WithContext(ctx).Save(&provider).Error; err != nil {
		return nil, fmt.Errorf("save plan provider: %w", err)
	}

	return &provider, nil
}

// DeleteProvider 删除 Plan Provider，同时删除关联的 Channel
func DeleteProvider(ctx context.Context, id int) error {
	var provider model.PlanProvider
	if err := db.GetDB().WithContext(ctx).First(&provider, id).Error; err != nil {
		return fmt.Errorf("find plan provider: %w", err)
	}

	// 删除关联的 GroupItems
	if provider.ChannelID > 0 {
		if err := db.GetDB().WithContext(ctx).Where("channel_id = ?", provider.ChannelID).Delete(&model.GroupItem{}).Error; err != nil {
			return fmt.Errorf("delete group items: %w", err)
		}
		// 删除 Channel
		if err := db.GetDB().WithContext(ctx).Delete(&model.Channel{}, provider.ChannelID).Error; err != nil {
			return fmt.Errorf("delete channel: %w", err)
		}
		// 删除 ChannelKeys
		if err := db.GetDB().WithContext(ctx).Where("channel_id = ?", provider.ChannelID).Delete(&model.ChannelKey{}).Error; err != nil {
			return fmt.Errorf("delete channel keys: %w", err)
		}
	}

	if err := db.GetDB().WithContext(ctx).Delete(&provider).Error; err != nil {
		return fmt.Errorf("delete plan provider: %w", err)
	}

	return nil
}

// GetCategories 获取所有支持的厂商分类
func GetCategories(providerType model.PlanProviderType) []model.PlanProviderCategoryInfo {
	result := make([]model.PlanProviderCategoryInfo, 0)
	for _, c := range model.PlanProviderCategories {
		if c.Type == providerType {
			result = append(result, c)
		}
	}
	return result
}

// --- internal helpers ---

func getCategoryInfo(category model.PlanProviderCategory) *model.PlanProviderCategoryInfo {
	for _, c := range model.PlanProviderCategories {
		if c.Category == category {
			return &c
		}
	}
	return nil
}

func ensurePlanGroup(ctx context.Context) (int, error) {
	// 查找已有 Plan 分组
	var group model.Group
	err := db.GetDB().WithContext(ctx).Where("name = ?", planGroupName).First(&group).Error
	if err == nil {
		return group.ID, nil
	}

	// 创建 Plan 分组
	newGroup := &model.Group{
		Name:         planGroupName,
		EndpointType: "*",
		Mode:         model.GroupModeRoundRobin,
	}
	if err := op.GroupCreate(newGroup, ctx); err != nil {
		return 0, fmt.Errorf("create plan group: %w", err)
	}
	return newGroup.ID, nil
}

func addChannelToGroup(ctx context.Context, channelID int, modelList string, groupID int) error {
	models := strings.Split(modelList, ",")
	// If model list is "*", use a wildcard
	if len(models) == 1 && models[0] == "*" {
		models = []string{"*"}
	}

	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		item := model.GroupItem{
			GroupID:   groupID,
			ChannelID: channelID,
			ModelName: modelName,
			Priority:  0,
			Weight:    1,
		}
		if err := db.GetDB().WithContext(ctx).Create(&item).Error; err != nil {
			return fmt.Errorf("add group item (model=%s): %w", modelName, err)
		}
	}

	// Refresh group cache
	if err := op.GroupRefreshCacheByIDs([]int{groupID}, ctx); err != nil {
		return fmt.Errorf("refresh group cache: %w", err)
	}
	return nil
}
