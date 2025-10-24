package routes

import (
	"PoolManagerVM/backend/controllers"
	"PoolManagerVM/backend/middlewares"

	"github.com/gin-gonic/gin"
)

func ServerpoolRoutes(r *gin.Engine) {
	serverpool := r.Group("/serverpool")
	{
		serverpool.POST("", middlewares.AuthMiddleware(), controllers.CreateServerpool)
		serverpool.DELETE("/:id", middlewares.AuthMiddleware(), controllers.DeleteServerpool)
		serverpool.GET("mysp", middlewares.AuthMiddleware(), controllers.GetMyServerpools)
		serverpool.GET("mysp/:id", middlewares.AuthMiddleware(), controllers.GetServersInServerpool)
		serverpool.POST("rebuild", middlewares.AuthMiddleware(), controllers.RebuildServer)
	}
}
