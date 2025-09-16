package routes

import (
	"PoolManagerVM/backend/controllers"

	"github.com/gin-gonic/gin"
)

func LoginRoutes(r *gin.Engine) {
	login := r.Group("/login")
	{
		login.POST("", controllers.LoginUser)
	}
}
