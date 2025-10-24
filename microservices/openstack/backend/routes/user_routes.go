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
		users.GET("/me", middlewares.AuthMiddleware(), controllers.GetProfile)
		users.POST("", controllers.CreateUser(config.Database))
		users.DELETE("/me", middlewares.AuthMiddleware(), controllers.DeleteUser)
		users.GET("/me/configs", middlewares.AuthMiddleware(), controllers.GetUserConfigs)
		users.POST("/me/configs", middlewares.AuthMiddleware(), controllers.CreateUserConfig(config.Database))
		users.DELETE("/me/configs/:config_id", middlewares.AuthMiddleware(), controllers.DeleteUserConfig(config.Database))
		users.POST("/me/configs/:config_id", middlewares.AuthMiddleware(), controllers.UpdateUserConfig(config.Database))
	}
}
