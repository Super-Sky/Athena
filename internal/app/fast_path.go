// fast_path.go defines the app-layer fast path extension point and its short-circuit result types.
// fast_path.go 定义 app 层 fast path 扩展点及其短路结果类型。
package app

import (
	"context"

	"moss/internal/runtime"
	"moss/internal/session"
)

// FastPathResult describes a short-circuit path chosen by the app layer.
// FastPathResult 描述 app 层命中的短路执行路径及其原因。
type FastPathResult struct {
	Matched  bool
	Name     string
	Reason   string
	Prepared *runtime.PreparedExecution
}

// FastPathEvaluator decides whether a request can bypass the standard runtime path.
// FastPathEvaluator 决定一个请求是否可以绕过标准 runtime 主链。
type FastPathEvaluator interface {
	Evaluate(context.Context, *session.Session, ChatRequest) (*FastPathResult, error)
}

// NoopFastPathEvaluator keeps the extension point wired without introducing business shortcuts yet.
// NoopFastPathEvaluator 仅保留扩展点，不提前引入任何业务 fast path。
type NoopFastPathEvaluator struct{}

// Evaluate always reports "miss" so the scaffold keeps using the standard runtime path.
// Evaluate 始终返回未命中，使脚手架继续走标准 runtime 主链。
func (NoopFastPathEvaluator) Evaluate(_ context.Context, _ *session.Session, _ ChatRequest) (*FastPathResult, error) {
	return &FastPathResult{}, nil
}
