package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"connectrpc.com/connect"
	"github.com/emillamm/envx"
)

const (
	RpcErrorTag = "X-RPC-Error-Tag"
	RpcErrorMsg = "X-RPC-Error-Msg"
)

// NOTE: Consider moving this to logic to a dedicate repo that can be shared among services.

// RPC error that
// * is logged internally
// * does not provide any error information in the http header
func InternalRpcError(
	ctx context.Context,
	underlying error,
) error {
	httpErrTag := ""
	httpErrMsg := ""
	exposeErr := false
	logErr := true

	return RpcError(ctx, underlying, connect.CodeInternal, httpErrTag, httpErrMsg, exposeErr, logErr)
}

// RPC error that
// * is logged internally
// * allows an error tag in the http header if EXPOSE_INTERNAL_RPC_ERROR_TAG is set
func InternalTaggedRpcError(
	ctx context.Context,
	underlying error,
	httpErrTag string,
) error {
	var env envx.EnvX = os.Getenv

	httpErrMsg := ""
	exposeErr, _ := env.Bool("EXPOSE_INTERNAL_RPC_ERROR_TAG").Value()
	logErr := true

	return RpcError(ctx, underlying, connect.CodeInternal, httpErrTag, httpErrMsg, exposeErr, logErr)
}

// RPC error that
// * is not logged internally
// * allows an error tag and message in the http header
func ExposedRpcError(
	ctx context.Context,
	underlying error,
	httpErrTag string,
	httpErrMsg string,
) error {
	exposeErr := true
	logErr := false
	return RpcError(ctx, underlying, connect.CodeInvalidArgument, httpErrTag, httpErrMsg, exposeErr, logErr)
}

func RpcError(
	ctx context.Context,
	underlying error,
	connectCode connect.Code,
	httpErrTag string,
	httpErrMsg string,
	exposeHttpErr bool,
	logErr bool,
) error {
	// if EXPOSE_UNDERLYING_ERROR is set, then return underlying in the connect error.
	// NOTE: This should NOT be set in production.
	var exposedUnderlyingErr error = nil
	var env envx.EnvX = os.Getenv
	exposeUnderlyingError, _ := env.Bool("EXPOSE_UNDERLYING_ERROR").Value()
	if exposeUnderlyingError {
		exposedUnderlyingErr = underlying
	}
	err := connect.NewError(
		connectCode,
		exposedUnderlyingErr,
	)

	if exposeHttpErr {
		if httpErrTag != "" {
			err.Meta().Add(RpcErrorTag, httpErrTag)
		}
		if httpErrMsg != "" {
			err.Meta().Add(RpcErrorMsg, httpErrMsg)
		}
	}
	if logErr {
		logErrorWithCaller(ctx, underlying, 1, httpErrTag, httpErrMsg)
	}
	return err
}

// LogError logs an error with context, capturing the caller location
func LogError(ctx context.Context, err error) {
	logErrorWithCaller(ctx, err, 1, "", "")
}

// logErrorWithCaller is the internal logging function that captures caller information
// skip determines how many stack frames to skip when capturing the caller location
func logErrorWithCaller(ctx context.Context, err error, skip int, httpErrTag string, httpErrMsg string) {
	location := "n/a"
	if _, file, line, ok := runtime.Caller(skip + 2); ok {
		location = fmt.Sprintf("%v:%v", filepath.Base(file), line)
	}

	attrs := []interface{}{
		"location", location,
		"err", err,
	}

	if httpErrTag != "" {
		attrs = append(attrs, "httpErrTag", httpErrTag)
	}
	if httpErrMsg != "" {
		attrs = append(attrs, "httpErrMsg", httpErrMsg)
	}

	slog.ErrorContext(ctx, "internal server error", attrs...)
}

func GetErrorTag(rpcError error) string {
	target, ok := rpcError.(*connect.Error)
	if !ok {
		return ""
	}
	return target.Meta().Get(RpcErrorTag)
}
