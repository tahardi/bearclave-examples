package networking_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahardi/bearclave-examples/internal/networking"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tahardi/bearclave"
	"github.com/tahardi/bearclave/mocks"
	"github.com/tahardi/bearclave/tee"
)

func writeError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeResponse(t *testing.T, w http.ResponseWriter, out any) {
	t.Helper()
	data, err := json.Marshal(out)
	require.NoError(t, err)

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	require.NoError(t, err)
}

func TestClient_AttestHTTPCall(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		ctx := context.Background()
		method := http.MethodGet
		url := "http://httpbin.org/get"
		want := &tee.AttestResult{Base: &bearclave.AttestResult{Report: []byte("attestation")}}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, networking.AttestHTTPCallPath)

			req := networking.AttestHTTPCallRequest{}
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, method, req.Method)
			assert.Equal(t, url, req.URL)

			resp := networking.AttestHTTPCallResponse{Attestation: want}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		got, err := client.AttestHTTPCall(ctx, method, url)

		// then
		require.NoError(t, err)
		assert.Equal(t, want, got.Attestation)
	})

	t.Run("error - doing attest http call request", func(t *testing.T) {
		// given
		ctx := context.Background()
		method := http.MethodGet
		url := "http://httpbin.org/get"

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, assert.AnError)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		_, err := client.AttestHTTPCall(ctx, method, url)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "doing attest http call request")
	})
}

func TestClient_AttestExpr(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		ctx := context.Background()
		env := map[string]any{
			"targetUrl": "http://httpbin.org/get",
		}
		expression := `httpGet(targetUrl).url == targetUrl ? "URL Match Success" : "URL Mismatch"`
		want := &tee.AttestResult{Base: &bearclave.AttestResult{Report: []byte("attestation")}}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, networking.AttestExprPath)

			req := networking.AttestExprRequest{}
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, expression, req.Expression)
			assert.Equal(t, env, req.Env)

			resp := networking.AttestExprResponse{Attestation: want}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		got, err := client.AttestExpr(ctx, expression, env)

		// then
		require.NoError(t, err)
		assert.Equal(t, want, got.Attestation)
	})

	t.Run("error - doing attest request", func(t *testing.T) {
		// given
		ctx := context.Background()
		env := map[string]any{
			"targetUrl": "http://httpbin.org/get",
		}
		expression := `httpGet(targetUrl).url == targetUrl ? "URL Match Success" : "URL Mismatch"`

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, assert.AnError)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		_, err := client.AttestExpr(ctx, expression, env)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "doing attest expr request")
	})
}

func TestClient_AttestUserData(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		ctx := context.Background()
		data := []byte("hello world")
		nonce := []byte("nonce")
		want := &tee.AttestResult{Base: &bearclave.AttestResult{Report: []byte("attestation")}}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, networking.AttestUserDataPath)

			req := networking.AttestUserDataRequest{}
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, data, req.UserData)

			resp := networking.AttestUserDataResponse{Attestation: want}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		got, err := client.AttestUserData(ctx, nonce, data)

		// then
		require.NoError(t, err)
		assert.Equal(t, want, got.Attestation)
	})

	t.Run("error - doing attest user data request", func(t *testing.T) {
		// given
		ctx := context.Background()
		data := []byte("data")
		nonce := []byte("nonce")

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, assert.AnError)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		_, err := client.AttestUserData(ctx, nonce, data)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "doing attest user data request")
	})
}

type doRequest struct {
	Data []byte `json:"data"`
}

type doResponse struct {
	Data []byte `json:"data"`
}

func TestClient_Do(t *testing.T) {
	t.Run("happy path - GET", func(t *testing.T) {
		// given
		ctx := context.Background()
		want := []byte("data")
		method := http.MethodGet
		api := "/"
		apiReq := &doRequest{}
		apiResp := &doResponse{}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, method, r.Method)

			resp := doResponse{Data: want}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		require.NoError(t, err)
		assert.Equal(t, want, apiResp.Data)
	})

	t.Run("happy path - POST", func(t *testing.T) {
		// given
		ctx := context.Background()
		want := []byte("data")
		method := http.MethodPost
		api := "/"
		apiReq := &doRequest{Data: want}
		apiResp := &doResponse{}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, method, r.Method)

			req := doRequest{}
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, want, req.Data)

			resp := doResponse{}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		assert.NoError(t, err)
	})

	t.Run("error - context canceled", func(t *testing.T) {
		// given
		ctx, cancel := context.WithCancel(context.Background())
		want := []byte("data")
		method := http.MethodGet
		api := "/"
		apiReq := &doRequest{}
		apiResp := &doResponse{}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, method, r.Method)

			resp := doResponse{Data: want}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		cancel()
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "context canceled")
	})

	t.Run("error - creating request", func(t *testing.T) {
		// given
		ctx := context.Background()
		method := "invalid method"
		api := "/"
		apiReq := &doRequest{}
		apiResp := &doResponse{}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, method, r.Method)

			resp := doResponse{}
			writeResponse(t, w, resp)
		})

		server := httptest.NewServer(handler)
		defer server.Close()

		client := networking.NewClientWithClient(server.URL, server.Client())

		// when
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "creating request")
	})

	t.Run("error - sending request", func(t *testing.T) {
		// given
		ctx := context.Background()
		method := http.MethodPost
		api := "/"
		apiReq := &doRequest{}
		apiResp := &doResponse{}

		roundTripper := mocks.NewRoundTripper(t)
		roundTripper.On("RoundTrip", mock.Anything).Return(nil, assert.AnError)

		httpClient := &http.Client{Transport: roundTripper}
		client := networking.NewClientWithClient("127.0.0.1", httpClient)

		// when
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "sending request")
	})

	t.Run("error - received non-200 response", func(t *testing.T) {
		// given
		ctx := context.Background()
		method := http.MethodPost
		api := "/"
		apiReq := &doRequest{}
		apiResp := &doResponse{}

		httpResp := &http.Response{
			StatusCode: http.StatusInternalServerError,
		}

		roundTripper := mocks.NewRoundTripper(t)
		roundTripper.On("RoundTrip", mock.Anything).Return(httpResp, nil)

		httpClient := &http.Client{Transport: roundTripper}
		client := networking.NewClientWithClient("127.0.0.1", httpClient)

		// when
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		require.ErrorIs(t, err, networking.ErrClientNon200Response)
		assert.ErrorContains(t, err, "non-200 response")
	})

	t.Run("error - reading response body", func(t *testing.T) {
		// given
		ctx := context.Background()
		method := http.MethodPost
		api := "/"
		apiReq := &doRequest{}
		apiResp := &doResponse{}

		readCloser := mocks.NewReadCloser(t)
		readCloser.On("Read", mock.Anything).Return(0, assert.AnError)
		readCloser.On("Close").Return(nil)

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       readCloser,
		}

		roundTripper := mocks.NewRoundTripper(t)
		roundTripper.On("RoundTrip", mock.Anything).Return(httpResp, nil)

		httpClient := &http.Client{Transport: roundTripper}
		client := networking.NewClientWithClient("127.0.0.1", httpClient)

		// when
		err := client.Do(ctx, method, api, apiReq, apiResp)

		// then
		require.ErrorIs(t, err, networking.ErrClient)
		assert.ErrorContains(t, err, "reading response body")
	})
}
