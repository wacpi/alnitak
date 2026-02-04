package routes

import (
	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/api/v1"
	"interastral-peace.com/alnitak/internal/middleware"
)

func CollectPGCRoutes(r *gin.RouterGroup) {
	pgcGroup := r.Group("pgc")
	{
		pgcGroup.GET("list", api.GetPGCList)
		pgcGroup.GET("detail", api.GetPGCDetail)
		pgcGroup.GET("/:pgc_id/episodes", api.GetPGCEpisodes)
		pgcGroup.GET("search", api.SearchPGC)
		pgcGroup.GET("type/:type", api.GetPGCByType)
		pgcGroup.GET("ongoing", api.GetOngoingPGC)
		pgcGroup.GET("recommended", api.GetRecommendedPGC)
		pgcGroup.GET("detail-with-episodes", api.GetPGCDetailWithEpisodes)

		pgcAuth := pgcGroup.Group("")
		pgcAuth.Use(middleware.Auth(), middleware.RequirePGC())
		{
			pgcAuth.POST("create", api.CreatePGC)
			pgcAuth.PUT("update", api.UpdatePGC)
			pgcAuth.DELETE(":pgc_id", api.DeletePGC)
			pgcAuth.POST("/:pgc_id/episodes/add", api.AddPGCEpisode)
			pgcAuth.DELETE("/:pgc_id/episodes/:id", api.DeletePGCEpisode)
		}
	}
}
