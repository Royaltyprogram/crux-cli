package common

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/Royaltyprogram/aiops/pkg/ecode"
)

func EchoErrorHandler(c *echo.Context, err error) {
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}

	var (
		sErr *ecode.Error
		eErr *echo.HTTPError
		sc   echo.HTTPStatusCoder
	)
	if errors.As(err, &sErr) {
	} else if errors.As(err, &eErr) {
		code := eErr.StatusCode()
		sErr = mapHTTPStatusError(code, eErr.Unwrap())
	} else if errors.As(err, &sc) {
		code := sc.StatusCode()
		sErr = mapHTTPStatusError(code, err)
	} else {
		sErr = ecode.InternalServerErr.WithCause(fmt.Errorf("%s received unknown error: %w", c.Path(), err))
	}

	_ = c.JSON(NewResp(nil, sErr))
}

func mapHTTPStatusError(code int, cause error) *ecode.Error {
	switch code {
	case http.StatusBadRequest:
		return ecode.InvalidParams.WithHttpCodeCause(code, cause)
	case http.StatusNotFound:
		return ecode.NotFound.WithHttpCodeCause(code, cause)
	case http.StatusTooManyRequests:
		return ecode.TooManyRequest.WithCause(cause)
	default:
		if code >= 400 && code < 500 {
			return ecode.New(code, code, http.StatusText(code)).WithCause(cause)
		}
		return ecode.New(ecode.UnknownCode, code, http.StatusText(code)).WithCause(cause)
	}
}
