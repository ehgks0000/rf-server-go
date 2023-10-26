package main

import (
	"net/http"

	"github.com/ehgks0000/rf-server-go/middleware"
	"github.com/gin-gonic/gin"
)

type createUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

func main() {
	// Gin 엔진 인스턴스를 생성합니다.
	r := gin.Default()

	r.Use(middleware.Logger())

	// /ping 엔드포인트를 설정합니다. GET 요청에 대해 응답합니다.
	r.GET("/ping", func(c *gin.Context) {
		// JSON 응답을 반환합니다.
		c.JSON(200, gin.H{
			"message": "gin-gonic pong",
		})
	})

	r.POST("/ping", func(ctx *gin.Context) {
		var req createUserRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		ctx.JSON(http.StatusOK, req)
	})

	// 3001 포트에서 서버를 실행합니다.
	r.Run(":3001")
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
