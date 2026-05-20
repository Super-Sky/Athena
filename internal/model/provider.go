// provider.go defines the runtime model provider abstraction and provider-facing execution config.
// provider.go 定义 runtime 模型供应商抽象及其面向执行侧的配置结构。
package model

import (
	"context"

	einomodel "github.com/cloudwego/eino/components/model"
)

// Provider builds one concrete chat model from a resolved runtime chat config.
// Provider 会根据解析后的运行时聊天配置构建一个具体的 chat model。
type Provider interface {
	NewChatModel(context.Context, ChatConfig) (einomodel.ToolCallingChatModel, error)
}

// DefaultProvider adapts the repository's supported protocols into concrete Eino chat models.
// DefaultProvider 会把仓库当前支持的协议适配成具体的 Eino chat model。
type DefaultProvider struct{}

// NewProvider creates the default runtime model provider.
// NewProvider 会创建默认运行时模型提供方。
func NewProvider() Provider {
	return DefaultProvider{}
}

// NewChatModel creates one concrete chat model from the resolved runtime config.
// NewChatModel 会根据解析后的运行时配置创建一个具体 chat model。
func (DefaultProvider) NewChatModel(ctx context.Context, cfg ChatConfig) (einomodel.ToolCallingChatModel, error) {
	return NewChatModelWithContext(ctx, cfg)
}
