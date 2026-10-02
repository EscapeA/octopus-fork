package helper

import (
	"context"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func TestFetchModelsReturnsMockCatalogWhenDevMockEnabled(t *testing.T) {
	t.Setenv("OCTOPUS_DEV_MOCK_SUCCESS", "true")

	models, err := FetchModels(context.Background(), model.Channel{})
	if err != nil {
		t.Fatalf("FetchModels() error = %v", err)
	}
	if len(models) == 0 {
		t.Fatal("FetchModels() returned no models in dev mock mode")
	}
}

func TestTestChannelReturnsMockSuccessWhenDevMockEnabled(t *testing.T) {
	t.Setenv("OCTOPUS_DEV_MOCK_SUCCESS", "true")

	summary, err := TestChannel(context.Background(), model.Channel{})
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}
	if !summary.Passed {
		t.Fatal("TestChannel() Passed = false, want true in dev mock mode")
	}
	if len(summary.Results) == 0 {
		t.Fatal("TestChannel() returned no results in dev mock mode")
	}
}
