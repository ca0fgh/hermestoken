package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The playground relays an upstream's HTTP status verbatim on a route that is
// itself authenticated with the operator's session cookie, so a channel whose
// provider key was revoked answers 401 on a perfectly valid session. Reaching this
// handler means UserAuth already passed, so nothing it returns is ever a statement
// about the caller's session — and the body has to say so, or the browser resets
// auth and logs the operator out mid-request.
func TestPlaygroundMarksItsErrorsAsComingFromUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	ctx.Set("use_access_token", true)

	Playground(ctx)

	var body struct {
		Error       map[string]any `json:"error"`
		ErrorSource string         `json:"error_source"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))

	assert.Equal(t, "upstream", body.ErrorSource, "the browser distinguishes a relayed status from its own session expiring by this field")
	assert.NotEmpty(t, body.Error, "the OpenAI-shaped error body must be preserved for the playground to display")
}
