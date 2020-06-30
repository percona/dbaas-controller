package servers

import (
	"context"
	"runtime/debug"
	"runtime/pprof"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/logger"
)

// logGRPCRequest wraps f (gRPC handler) invocation with logging and panic recovery.
func logGRPCRequest(l logger.Logger, prefix string, warnD time.Duration, f func() error) (err error) {
	start := time.Now()
	l.Infof("Starting %s ...", prefix)

	defer func() {
		dur := time.Since(start)

		if p := recover(); p != nil {
			// Always log with %+v - there can be inner stacktraces
			// produced by panic(errors.WithStack(err)).
			// Also always log debug.Stack() for all panics.
			l.Debugf("%s done in %s with panic: %+v\nStack: %s", prefix, dur, p, debug.Stack())

			err = status.Error(codes.Internal, "Internal server error.")
			return
		}

		// log gRPC errors as warning, not errors, even if they are wrapped
		_, gRPCError := status.FromError(errors.Cause(err))
		switch {
		case err == nil:
			if warnD == 0 || dur < warnD {
				l.Infof("%s done in %s.", prefix, dur)
			} else {
				l.Warnf("%s done in %s (quite long).", prefix, dur)
			}
		case gRPCError:
			// %+v for inner stacktraces produced by errors.WithStack(err)
			l.Warnf("%s done in %s with gRPC error: %+v", prefix, dur, err)
		default:
			// %+v for inner stacktraces produced by errors.WithStack(err)
			l.Errorf("%s done in %s with unexpected error: %+v", prefix, dur, err)
			err = status.Error(codes.Internal, "Internal server error.")
		}
	}()

	err = f()
	return //nolint:nakedret
}

// unaryLoggingInterceptor returns a new unary server interceptor that logs incoming requests.
func unaryLoggingInterceptor(warnDuration time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) { //nolint:lll
		// add pprof labels for more useful profiles
		defer pprof.SetGoroutineLabels(ctx)
		ctx = pprof.WithLabels(ctx, pprof.Labels("method", info.FullMethod))
		pprof.SetGoroutineLabels(ctx)

		// make context with logger
		var l logger.Logger
		ctx, l = getCtxForRequest(ctx)

		var res interface{}
		err := logGRPCRequest(l, "RPC "+info.FullMethod, warnDuration, func() error {
			var origErr error
			res, origErr = handler(ctx, req)
			return origErr
		})

		// err is already logged by logRequest
		l.Debugf("\nRequest:\n%s\nResponse:\n%s\n", req, res)

		return res, err
	}
}

// streamLoggingInterceptor returns a new stream server interceptor that logs incoming messages.
func streamLoggingInterceptor(warnDuration time.Duration) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// add pprof labels for more useful profiles
		defer pprof.SetGoroutineLabels(ctx)
		ctx = pprof.WithLabels(ctx, pprof.Labels("method", info.FullMethod))
		pprof.SetGoroutineLabels(ctx)

		// make context with logger
		var l logger.Logger
		ctx, l = getCtxForRequest(ctx)

		err := logGRPCRequest(l, "Stream "+info.FullMethod, warnDuration, func() error {
			wrapped := grpc_middleware.WrapServerStream(ss)
			wrapped.WrappedContext = ctx
			return handler(srv, ss)
		})

		return err
	}
}
