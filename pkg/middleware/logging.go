package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerLogger returns a grpc.UnaryServerInterceptor that logs every RPC call.
func UnaryServerLogger(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		code := status.Code(err)
		level := slog.LevelInfo
		switch {
		case code == codes.OK:
			level = slog.LevelInfo
		case code == codes.Internal, code == codes.Unknown, code == codes.DataLoss:
			level = slog.LevelError
		default:
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.String("kind", "grpc"),
			slog.String("method", info.FullMethod),
			slog.String("code", code.String()),
			slog.Duration("duration", dur),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		logger.LogAttrs(ctx, level, "grpc_call", attrs...)
		return resp, err
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	n, err := sr.ResponseWriter.Write(b)
	sr.bytes += n
	return n, err
}

// HTTPLogger wraps an http.Handler with structured access logging.
func HTTPLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			dur := time.Since(start)

			level := slog.LevelInfo
			switch {
			case rec.status >= 500:
				level = slog.LevelError
			case rec.status >= 400:
				level = slog.LevelWarn
			}

			logger.LogAttrs(r.Context(), level, "http_call",
				slog.String("kind", "http"),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", dur),
				slog.String("remote", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)
		})
	}
}
