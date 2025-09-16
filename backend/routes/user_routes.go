package routes

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/controllers"
	"PoolManagerVM/backend/middlewares"

	"github.com/gin-gonic/gin"
)

func UserRoutes(r *gin.Engine) {
	users := r.Group("/users")
	{
		users.GET("", controllers.GetUsers)
		users.GET("/me", middlewares.AuthMiddleware(), controllers.GetProfile)
		users.POST("", controllers.CreateUser(config.Database))
		// users.DELETE("/", controllers.DeleteUser)
	}
}
