package answer

import (
	answeruserauth "github.com/ggmolly/belfast/internal/answer/userauth"
	"github.com/ggmolly/belfast/internal/connection"
)

const (
	registerResultOK             = 0
	registerResultInvalidAccount = 1010
	registerResultAccountExists  = 1011
	registerResultNumericAccount = 1012
	registerResultDatabaseError  = 11
)

func RegisterAccount(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answeruserauth.RegisterAccount(buffer, client)
}
