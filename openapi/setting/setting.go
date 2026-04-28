package setting

import (
	"github.com/gin-gonic/gin"
	"github.com/yaoapp/yao/openapi/oauth/authorized"
	oauthTypes "github.com/yaoapp/yao/openapi/oauth/types"
	"github.com/yaoapp/yao/openapi/response"
	"github.com/yaoapp/yao/setting"
)

// Attach registers all /setting/* routes under the given group.
// Currently only System Info routes are wired; other groups will be
// added incrementally.
func Attach(group *gin.RouterGroup, oauth oauthTypes.OAuth) {
	group.Use(oauth.Guard)

	sys := group.Group("/system")
	sys.GET("", handleSystemInfo)
	sys.POST("/check-update", handleSystemCheckUpdate)
}

// resolveOwner extracts the authenticated user/team from the Gin context
// and returns a setting.ScopeID suitable for registry operations.
func resolveOwner(c *gin.Context) setting.ScopeID {
	info := authorized.GetInfo(c)
	return setting.ScopeID{
		Scope:  setting.ScopeUser,
		TeamID: info.TeamID,
		UserID: info.UserID,
	}
}

// respondError is a thin helper that writes a JSON error via the shared
// response package.
func respondError(c *gin.Context, status int, msg string) {
	response.RespondWithError(c, status, &response.ErrorResponse{
		Code:             "server_error",
		ErrorDescription: msg,
	})
}
