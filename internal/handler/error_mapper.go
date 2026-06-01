package handler

import (
	connectrpc "connectrpc.com/connect"
	"github.com/saitamau-maximum/maxicloud/internal/domain"
)

type codeClassifier func(err error) (connectrpc.Code, bool)

func toConnectError(err error) error {
	return toError(err, classifyDomainError)
}

func classifyDomainError(err error) (connectrpc.Code, bool) {
	if domain.IsValidationError(err) {
		return connectrpc.CodeInvalidArgument, true
	}
	return 0, false
}

func toError(err error, classifiers ...codeClassifier) error {
	for _, classify := range classifiers {
		code, ok := classify(err)
		if ok {
			return connectrpc.NewError(code, err)
		}
	}
	return connectrpc.NewError(connectrpc.CodeInternal, err)
}
