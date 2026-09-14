package chat

import (
	"my-project/internal/agent"
)

type ControllerV1 struct {
	agent *agent.Agent
}

func NewV1(agentInstance *agent.Agent) *ControllerV1 {
	return &ControllerV1{
		agent: agentInstance,
	}
}
