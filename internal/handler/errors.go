package handler

import (
	"connectrpc.com/connect"

	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

// connectError はドメインエラーを適切な Connect のエラーコードへ変換する。
// 認可エラーは PermissionDenied、存在しない場合は NotFound、検証エラーは InvalidArgument、それ以外は Internal。
func connectError(err error) error {
	switch {
	case domain.IsForbiddenError(err):
		return connect.NewError(connect.CodePermissionDenied, err)
	case domain.IsNotFoundError(err):
		return connect.NewError(connect.CodeNotFound, err)
	case domain.IsValidationError(err):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
