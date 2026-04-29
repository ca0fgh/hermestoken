package router

import (
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/controller"
	"github.com/gin-gonic/gin"
)

func openAIModelsHandler(c *gin.Context) {
	switch {
	case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
		controller.ListModels(c, constant.ChannelTypeAnthropic)
	case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "":
		controller.RetrieveModel(c, constant.ChannelTypeGemini)
	default:
		controller.ListModels(c, constant.ChannelTypeOpenAI)
	}
}

func openAIModelHandler(c *gin.Context) {
	switch {
	case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
		controller.RetrieveModel(c, constant.ChannelTypeAnthropic)
	default:
		controller.RetrieveModel(c, constant.ChannelTypeOpenAI)
	}
}
