package relay

import (
	"testing"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/relay/balancer"
)

func TestFinalChannelFallsBackToSkippedAttempt(t *testing.T) {
	attempts := []model.ChannelAttempt{
		{
			ChannelID:   56,
			ChannelName: "test-channel",
			Status:      model.AttemptCircuitBreak,
			Msg:         "circuit breaker tripped",
		},
	}

	channelID, channelName := finalChannel(attempts)
	if channelID != 56 || channelName != "test-channel" {
		t.Fatalf("finalChannel() = (%d, %q), want (56, %q)", channelID, channelName, "test-channel")
	}
}

func TestFinalChannelPrefersLastForwardedFailure(t *testing.T) {
	attempts := []model.ChannelAttempt{
		{
			ChannelID:   11,
			ChannelName: "failed-channel",
			Status:      model.AttemptFailed,
		},
		{
			ChannelID:   56,
			ChannelName: "skipped-channel",
			Status:      model.AttemptCircuitBreak,
		},
	}

	channelID, channelName := finalChannel(attempts)
	if channelID != 11 || channelName != "failed-channel" {
		t.Fatalf("finalChannel() = (%d, %q), want (11, %q)", channelID, channelName, "failed-channel")
	}
}

func TestRouteIteratorAttemptsCarrySuccessfulChannel(t *testing.T) {
	iter := &balancer.Iterator{}
	span := iter.StartAttempt(23, 7, "mimo-channel", "Mimo-v2.5-pro-codeplan")
	span.End(model.AttemptSuccess, 200, "")

	channelID, channelName := finalChannel(iter.Attempts())
	if channelID != 23 || channelName != "mimo-channel" {
		t.Fatalf("finalChannel(iter.Attempts()) = (%d, %q), want (%d, %q)", channelID, channelName, 23, "mimo-channel")
	}
}
