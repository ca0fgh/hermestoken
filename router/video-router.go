package router

import (
	"github.com/ca0fgh/hermestoken/controller"
	"github.com/ca0fgh/hermestoken/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	// Video proxy: accepts either session auth (dashboard) or token auth (API clients)
	rootVideoProxyRouter := router.Group("")
	rootVideoProxyRouter.Use(middleware.RouteTag("relay"))
	rootVideoProxyRouter.Use(middleware.CanonicalOpenAIPath())
	rootVideoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		rootVideoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
	}

	videoProxyRouter := router.Group("/v1")
	videoProxyRouter.Use(middleware.RouteTag("relay"))
	videoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
	}

	rootVideoRouter := router.Group("")
	rootVideoRouter.Use(middleware.RouteTag("relay"))
	rootVideoRouter.Use(middleware.CanonicalOpenAIPath())
	rootVideoRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		rootVideoRouter.POST("/video/generations", controller.RelayTask)
		rootVideoRouter.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		rootVideoRouter.POST("/videos/:video_id/remix", controller.RelayTask)
		rootVideoRouter.POST("/videos", controller.RelayTask)
		rootVideoRouter.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}
	// openai compatible API video routes
	// docs: https://platform.openai.com/docs/api-reference/videos/create
	{
		videoV1Router.POST("/videos", controller.RelayTask)
		videoV1Router.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	// Jimeng official API routes - direct mapping to official API format
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		// Maps to: /?Action=CVSync2AsyncSubmitTask&Version=2022-08-31 and /?Action=CVSync2AsyncGetResult&Version=2022-08-31
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}
}
