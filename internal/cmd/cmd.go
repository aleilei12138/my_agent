package cmd

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"

	"my-project/internal/controller/chat"
	"my-project/internal/controller/hello"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			agentInstance, err := newAgentFromConfig(ctx)
			if err != nil {
				return err
			}
			g.Log().Info(ctx, "agent initialized successfully")
			_ = agentInstance // Use the agentInstance as needed
			chatController := chat.NewV1(agentInstance)
			s := g.Server()
			s.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(ghttp.MiddlewareHandlerResponse)
				group.Bind(
					hello.NewV1(),
					chatController,
				)
			})
			s.Run()
			return nil
		},
	}
)
