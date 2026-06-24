package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Middleware is the standard decorator signature.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares to h in order, so the first listed runs outermost.
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

type ctxKey int

const interactionIDKey ctxKey = iota

// FAPIHeader is the OBIE/FAPI interaction id header. ASPSPs must echo a
// client-supplied value back and generate one when absent.
const FAPIHeader = "x-fapi-interaction-id"

// InteractionID returns the FAPI interaction id associated with the request
// context, or "" if none was set.
func InteractionID(ctx context.Context) string {
	id, _ := ctx.Value(interactionIDKey).(string)
	return id
}

// FAPIInteractionID echoes a caller-supplied x-fapi-interaction-id or mints a
// new UUID, storing it on the context and the response headers. This is a
// mandatory OBIE behaviour for correlating requests end to end.
func FAPIInteractionID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(FAPIHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(FAPIHeader, id)
		ctx := context.WithValue(r.Context(), interactionIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger emits a structured access log line per request, including the FAPI
// interaction id and latency.
func Logger(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"interaction_id", InteractionID(r.Context()),
			)
		})
	}
}

// Recoverer converts a panic into an OBIE 500 so a single bad request can never
// take down the service.
func Recoverer(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "error", rec, "path", r.URL.Path)
					RespondError(w, Internal("An internal error occurred"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
