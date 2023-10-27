package api

import (
	"net/http"

	"github.com/ehgks0000/rf-server-go/middleware"
	"github.com/gin-gonic/gin"
)

func (server *Server) setupGroupRoutes(r *gin.Engine) {
	server.setupPostRoutes(r, "/api/v1/posts")
}

func (server *Server) setupPostRoutes(r *gin.Engine, route string) {
	postGroups := r.Group(route)

	postGroups.GET("/public", server.GetRandomPostPublic)

}

func (server *Server) setupRoutes() {
	router := gin.Default()

	router.Use(middleware.Logger())

	// /ping 엔드포인트를 설정합니다. GET 요청에 대해 응답합니다.
	router.GET("/ping", func(c *gin.Context) {
		// JSON 응답을 반환합니다.
		c.JSON(200, gin.H{
			"message": "gin-gonic pong",
		})
	})

	router.POST("/ping", func(ctx *gin.Context) {
		var req createUserRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		ctx.JSON(http.StatusOK, req)
	})

	server.setupGroupRoutes(router)

	server.router = router
}
