package httpserver

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestSPADevProxyRejectsCONNECT verifies the patched proxy rejects tunnels before forwarding.
func TestSPADevProxyRejectsCONNECT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("frontend"))
	}))
	t.Cleanup(upstream.Close)
	router := gin.New()
	require.NoError(t, registerDevProxy(router, upstream.URL))

	control := httptest.NewRecorder()
	getContext, cancelGET := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelGET()
	router.ServeHTTP(control, httptest.NewRequestWithContext(getContext, http.MethodGet, "/frontend", nil))
	require.Equal(t, http.StatusOK, control.Code)
	require.Equal(t, "frontend", control.Body.String())
	require.Equal(t, int32(1), calls.Load())

	response := httptest.NewRecorder()
	connectContext, cancelCONNECT := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelCONNECT()
	router.ServeHTTP(response, httptest.NewRequestWithContext(connectContext, http.MethodConnect, "http://frontend.invalid/", nil))
	require.Equal(t, http.StatusMethodNotAllowed, response.Code)
	require.Equal(t, int32(1), calls.Load(), "CONNECT must not reach the upstream")
}

// TestSPAStaticIgnoresExcessiveRanges exercises the actual asset and index routes.
func TestSPAStaticIgnoresExcessiveRanges(t *testing.T) {
	t.Setenv("GODEBUG", "")
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "assets"), 0o750))
	content := strings.Repeat("0123456789abcdef", 64)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte(content), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "large.txt"), []byte(content), 0o600))
	router := gin.New()
	require.NoError(t, registerStaticSPA(router, dir))

	ranges := make([]string, 201)
	for index := range ranges {
		offset := strconv.Itoa(index)
		ranges[index] = offset + "-" + offset
	}

	for _, path := range []string{"/assets/large.txt", "/reports"} {
		t.Run(path, func(t *testing.T) {
			control := httptest.NewRequest(http.MethodGet, path, nil)
			control.Header.Set("Accept", "text/html")
			control.Header.Set("Range", "bytes=0-0")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, control)
			require.Equal(t, http.StatusPartialContent, response.Code)
			require.Equal(t, content[:1], response.Body.String())
			require.Equal(t, "bytes 0-0/1024", response.Header().Get("Content-Range"))

			boundary := httptest.NewRequest(http.MethodGet, path, nil)
			boundary.Header.Set("Accept", "text/html")
			boundary.Header.Set("Range", "bytes="+strings.Join(ranges[:200], ","))
			response = httptest.NewRecorder()
			router.ServeHTTP(response, boundary)
			require.Equal(t, http.StatusPartialContent, response.Code)
			mediaType, parameters, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
			require.NoError(t, err)
			require.Equal(t, "multipart/byteranges", mediaType)
			require.NotEmpty(t, parameters["boundary"])
			parts := multipart.NewReader(response.Body, parameters["boundary"])
			for index := range 200 {
				part, err := parts.NextPart()
				require.NoError(t, err)
				require.Equal(t, "bytes "+strconv.Itoa(index)+"-"+strconv.Itoa(index)+"/1024", part.Header.Get("Content-Range"))
				body, err := io.ReadAll(part)
				require.NoError(t, err)
				require.Equal(t, content[index:index+1], string(body))
				require.NoError(t, part.Close())
			}
			_, err = parts.NextPart()
			require.ErrorIs(t, err, io.EOF)

			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Accept", "text/html")
			request.Header.Set("Range", "bytes="+strings.Join(ranges, ","))
			response = httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			require.Equal(t, content, response.Body.String())
			require.Empty(t, response.Header().Get("Content-Range"))
		})
	}
}
