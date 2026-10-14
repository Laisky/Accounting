package logger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gmw "github.com/Laisky/gin-middlewares/v7"
	gutils "github.com/Laisky/go-utils/v6"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
	"github.com/Laisky/zap/zapcore"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/config"
)

// TestUtilsLoggingCompatibility preserves levels, context identity, and structured local fields.
func TestUtilsLoggingCompatibility(t *testing.T) {
	previous := Logger
	t.Cleanup(func() { Logger = previous })
	for _, debug := range []bool{false, true} {
		log := Setup(debug)
		want := glog.LevelInfo
		if debug {
			want = glog.LevelDebug
		}
		require.Equal(t, want, log.Level())
		require.Same(t, log, FromContext(nil)) //nolint:staticcheck // Exercise the supported nil-context fallback explicitly.
		request := log.Named("request")
		require.Same(t, request, FromContext(gmw.SetLogger(context.Background(), request)))
	}
}

// TestUtilsAlertCompatibility verifies Accounting's configured alert hook sends only safe metadata.
func TestUtilsAlertCompatibility(t *testing.T) {
	previous := Logger
	t.Cleanup(func() { Logger = previous })
	messages := make(chan string, 4)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Variables map[string]json.RawMessage }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var message string
		if err := json.Unmarshal(body.Variables["msg"], &message); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case messages <- message:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"data\":{\"TelegramMonitorAlert\":{\"name\":\"synthetic\"}}}"))
	}))
	defer receiver.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.Config{}
	cfg.AlertPusher.API = receiver.URL
	cfg.AlertPusher.Type = "synthetic"
	cfg.AlertPusher.Token = "synthetic-only"
	Setup(false)
	log := SetupEnhanced(ctx, cfg)
	var localSecret string
	log = log.WithOptions(zap.HooksWithFields(func(_ zapcore.Entry, fields []zapcore.Field) error {
		for _, field := range fields {
			if field.Key == "session_token" {
				localSecret = field.String
			}
		}
		return nil
	}))
	log.Info("info control")
	log.Error("compatibility error", zap.String("session_token", "synthetic-secret"))
	select {
	case message := <-messages:
		require.Contains(t, message, "compatibility error")
		require.Contains(t, message, "error")
		require.NotContains(t, message, "info control")
		require.NotContains(t, message, "synthetic-secret")
		require.NotContains(t, message, "session_token")
	case <-time.After(3 * time.Second):
		t.Fatal("configured error alert was not delivered")
	}
	require.Equal(t, "synthetic-secret", localSecret)
	cancel()
}

// TestUtilsAlertRateLimiterCompatibility preserves the one-alert capacity and canceled lifecycle.
func TestUtilsAlertRateLimiterCompatibility(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	limiter, err := gutils.NewRateLimiter(ctx, gutils.RateLimiterArgs{Max: 1, NPerSec: 1})
	require.NoError(t, err)
	defer limiter.Close()
	defer cancel()
	require.True(t, limiter.Allow())
	require.False(t, limiter.Allow())
	cancel()
	require.False(t, limiter.Allow())
	invalid, err := gutils.NewRateLimiter(context.Background(), gutils.RateLimiterArgs{Max: 0, NPerSec: 1})
	require.Error(t, err)
	require.Nil(t, invalid)
}
