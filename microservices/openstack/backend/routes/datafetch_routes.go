package routes

import (
	"PoolManagerVM/backend/controllers"
	"PoolManagerVM/backend/middlewares"

	"github.com/gin-gonic/gin"
)

func DatafetchRoutes(r *gin.Engine) {
	datafetch := r.Group("/datafetch")
	{
		datafetch.GET("flavor", middlewares.AuthMiddleware(), controllers.GetallFlavors)
		datafetch.GET("networks", middlewares.AuthMiddleware(), controllers.GetAllNetworks)
		datafetch.POST("imagegroup", middlewares.AuthMiddleware(), controllers.GetGroupeImage)
		datafetch.GET("groupimagesname", middlewares.AuthMiddleware(), controllers.GetGroupeImagename)
	}
}
