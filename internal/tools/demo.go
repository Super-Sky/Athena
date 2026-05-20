// demo.go defines the repository's demo tools and their helper result structures.
// demo.go 定义仓库内置 demo tools 及其辅助结果结构。
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	tu "github.com/cloudwego/eino/components/tool/utils"
	"moss/internal/observability"
)

// UserRequest captures the minimal user_id input shared by demo tools.
// UserRequest 描述 demo tools 共用的最小 user_id 输入。
type UserRequest struct {
	UserID string `json:"user_id" jsonschema_description:"User identifier, such as u1001"`
}

// ProfileResult captures the mock profile payload returned by the demo profile tool.
// ProfileResult 描述 demo profile tool 返回的模拟档案结果。
type ProfileResult struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Level  string `json:"level"`
	City   string `json:"city"`
}

// OrdersResult captures the mock order payload returned by the demo orders tool.
// OrdersResult 描述 demo orders tool 返回的模拟订单结果。
type OrdersResult struct {
	UserID      string   `json:"user_id"`
	OpenOrders  []string `json:"open_orders"`
	LastOrderID string   `json:"last_order_id"`
}

// RiskResult captures the mock risk payload returned by the demo risk tool.
// RiskResult 描述 demo risk tool 返回的模拟风险结果。
type RiskResult struct {
	UserID string   `json:"user_id"`
	Flags  []string `json:"flags"`
	Score  string   `json:"score"`
}

// DemoTools returns the ordered demo tool slice used by the scaffold runtime.
// DemoTools 返回脚手架 runtime 使用的有序 demo tool 列表。
func DemoTools() ([]tool.BaseTool, error) {
	registry, err := DemoToolRegistry()
	if err != nil {
		return nil, err
	}

	result := make([]tool.BaseTool, 0, len(registry))
	for _, name := range []string{"lookup_profile", "lookup_orders", "lookup_risk_flags"} {
		result = append(result, registry[name])
	}
	return result, nil
}

// DemoToolRegistry returns the named demo tool registry keyed by tool name.
// DemoToolRegistry 返回按 tool 名称索引的 demo tool registry。
func DemoToolRegistry() (map[string]tool.BaseTool, error) {
	profileTool, err := tu.InferTool("lookup_profile",
		"Look up a user's basic profile. Use this when the user asks for profile, account, level, or city information.",
		lookupProfile)
	if err != nil {
		return nil, err
	}

	ordersTool, err := tu.InferTool("lookup_orders",
		"Look up a user's open orders. Use this when the user asks for order status, pending orders, or latest order information.",
		lookupOrders)
	if err != nil {
		return nil, err
	}

	riskTool, err := tu.InferTool("lookup_risk_flags",
		"Look up a user's risk flags. Use this when the user asks for fraud, review, compliance, or risk information.",
		lookupRiskFlags)
	if err != nil {
		return nil, err
	}

	return map[string]tool.BaseTool{
		"lookup_profile":    profileTool,
		"lookup_orders":     ordersTool,
		"lookup_risk_flags": riskTool,
	}, nil
}

func lookupProfile(ctx context.Context, input UserRequest) (ProfileResult, error) {
	logToolStart("lookup_profile", input.UserID)
	defer logToolEnd("lookup_profile", input.UserID)

	time.Sleep(2200 * time.Millisecond)

	return ProfileResult{
		UserID: input.UserID,
		Name:   "Moss Demo User",
		Level:  "gold",
		City:   "Shanghai",
	}, nil
}

func lookupOrders(ctx context.Context, input UserRequest) (OrdersResult, error) {
	logToolStart("lookup_orders", input.UserID)
	defer logToolEnd("lookup_orders", input.UserID)

	time.Sleep(2600 * time.Millisecond)

	return OrdersResult{
		UserID:      input.UserID,
		OpenOrders:  []string{"SO-1024", "SO-2048"},
		LastOrderID: "SO-4096",
	}, nil
}

func lookupRiskFlags(ctx context.Context, input UserRequest) (RiskResult, error) {
	logToolStart("lookup_risk_flags", input.UserID)
	defer logToolEnd("lookup_risk_flags", input.UserID)

	time.Sleep(1800 * time.Millisecond)

	return RiskResult{
		UserID: input.UserID,
		Flags:  []string{"manual_review", "high_value_account"},
		Score:  "medium",
	}, nil
}

func logToolStart(toolName, userID string) {
	observability.LogAction(observability.LogLevelDebug, observability.ActionLog{
		Module: "tools",
		Action: toolName,
		Step:   "start",
		Status: "running",
		Detail: map[string]any{
			"user_id": userID,
		},
	})
}

func logToolEnd(toolName, userID string) {
	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "tools",
		Action: toolName,
		Step:   "finish",
		Status: "ok",
		Detail: map[string]any{
			"user_id": userID,
		},
	})
}

// DemoQueryHint returns one Chinese demo query that exercises all demo tools for the given user.
// DemoQueryHint 返回一条用于触发全部 demo tools 的中文示例查询。
func DemoQueryHint(userID string) string {
	return fmt.Sprintf("请同时查询用户 %s 的档案、订单和风险标记，并给出汇总。", userID)
}
