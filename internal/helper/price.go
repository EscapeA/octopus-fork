package helper

import (
	"context"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/llm"
)

// LLMPriceAddToDB 为渠道同步新发现的模型建价格条目。
// 价格一律写 0（未定价，不参与计费），由用户在模型管理页按人民币（¥/M tokens）
// 手工填写；不再从任何外部/内置价格源自动填价。
func LLMPriceAddToDB(modelNames []string, ctx context.Context) error {
	newLLMInfos := make([]model.LLMInfo, 0, len(modelNames))
	for _, modelName := range modelNames {
		if modelName == "" {
			continue
		}
		newLLMInfos = append(newLLMInfos, model.LLMInfo{Name: modelName})
	}
	if len(newLLMInfos) > 0 {
		return llm.BatchCreate(newLLMInfos, ctx)
	}
	return nil
}

func LLMPriceDeleteFromDBWithNoPrice(modelNames []string, ctx context.Context) error {
	if len(modelNames) == 0 {
		return nil
	}
	// 手动设置过价格的模型不自动删除（用户明确创建/编辑过）。
	manualSet, err := loadManualPriceModelSet(ctx)
	if err != nil {
		return err
	}
	needDeleteModelNames := make([]string, 0, len(modelNames))
	for _, modelName := range modelNames {
		if modelName == "" {
			continue
		}
		if _, ok := manualSet[modelName]; ok {
			continue
		}
		modelPrice, err := llm.Get(modelName)
		if err != nil {
			return err
		}
		if modelPrice.Input != 0 || modelPrice.Output != 0 || modelPrice.CacheRead != 0 || modelPrice.CacheWrite != 0 {
			continue
		}
		needDeleteModelNames = append(needDeleteModelNames, modelName)
	}
	if len(needDeleteModelNames) > 0 {
		return llm.BatchDelete(needDeleteModelNames, ctx)
	}
	return nil
}

// loadManualPriceModelSet 查询所有手动设置价格的模型名集合（price_manual=true）。
// modelCache 只缓存价格不缓存 manual 标记，因此需直查 DB。
func loadManualPriceModelSet(ctx context.Context) (map[string]struct{}, error) {
	var names []string
	if err := db.GetDB().WithContext(ctx).Model(&model.LLMInfo{}).
		Where("price_manual = ?", true).
		Pluck("name", &names).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	return set, nil
}
