package controller

import (
	"errors"
	"fmt"

	"github.com/ca0fgh/hermestoken/middleware"
	"github.com/ca0fgh/hermestoken/model"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	var hermesTokenError *types.HermesTokenError

	defer func() {
		if hermesTokenError != nil {
			c.JSON(hermesTokenError.StatusCode, gin.H{
				"error": hermesTokenError.ToOpenAIError(),
				// This route is authenticated with the operator's session cookie and
				// then relays an upstream's status verbatim, so a channel whose key was
				// revoked answers 401 on a perfectly valid session. Reaching this
				// handler at all means UserAuth already passed, so nothing it returns is
				// ever a statement about the caller's session — and the browser needs to
				// be told that, or it logs the operator out of a session that is fine.
				"error_source": "upstream",
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		hermesTokenError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, nil, nil)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		return
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	Relay(c, types.RelayFormatOpenAI)
}
