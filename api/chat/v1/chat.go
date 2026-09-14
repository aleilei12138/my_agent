package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type ChatReq struct {
	g.Meta  `path:"/chat" method:"post" tags:"Chat" summary:"Chat with AI Agent"`
	Message string `json:"message" v:"required#message is required"`
}

type ChatRes struct {
	g.Meta `mime:"application/json"`
	Reply  string `json:"reply"`
}
