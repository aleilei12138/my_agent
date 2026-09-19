package agent

import (
	"context"
	"fmt"
)

const defaultMaxTurns = 8

func (a *Agent) Run(
	ctx context.Context,
	messages []Message,
) (Message, error) {
	tools := a.registry.Definitions()

	for turn := 0; turn < defaultMaxTurns; turn++ {
		response, err := a.llm.Chat(
			ctx,
			messages,
			tools,
		)
		if err != nil {
			return Message{}, fmt.Errorf(
				"agent: llm chat: %w",
				err,
			)
		}

		// 无论有没有 ToolCall，都先把 Assistant 消息
		// 放进完整对话历史。
		messages = append(
			messages,
			response,
		)

		// 没有 ToolCall，说明模型已经给出最终回答。
		if len(response.ToolCalls) == 0 {
			return response, nil
		}

		// 执行模型请求的工具。
		toolMessages, err := a.executeToolCall(
			ctx,
			response.ToolCalls,
		)
		if err != nil {
			return Message{}, fmt.Errorf(
				"agent: execute tools: %w",
				err,
			)
		}

		// 把所有工具结果加入消息历史，
		// 下一轮重新发送给 LLM。
		messages = append(
			messages,
			toolMessages...,
		)
	}

	return Message{}, fmt.Errorf(
		"agent: maximum turns exceeded: %d",
		defaultMaxTurns,
	)
}
