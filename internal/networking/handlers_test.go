package networking_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"bearclave-examples/internal/engine"
	"bearclave-examples/internal/networking"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tahardi/bearclave/mocks"
	"github.com/tahardi/bearclave/tee"
)

var (
	defaultTimeout           = networking.DefaultTimeout
	errHTTPGet               = errors.New("http get")
	errHTTPGetMissingURL     = fmt.Errorf("%w: missing url", errHTTPGet)
	errHTTPGetWrongURLType   = fmt.Errorf("%w: url should be a string", errHTTPGet)
	errHTTPGetNon200Response = fmt.Errorf("%w: non-200 response", errHTTPGet)
)

func makeHTTPGet(client *http.Client) func(params ...any) (any, error) {
	return func(params ...any) (any, error) {
		if len(params) < 1 {
			return nil, errHTTPGetMissingURL
		}

		url, ok := params[0].(string)
		if !ok {
			return nil, errHTTPGetWrongURLType
		}

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating GET req: %w", err)
		}

		resp, err := client.Do(req)
		switch {
		case err != nil:
			return nil, fmt.Errorf("making GET req to '%s': %w", url, err)
		case resp.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("%w: %s", errHTTPGetNon200Response, resp.Status)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading body: %w", err)
		}

		var result any
		if err = json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, fmt.Errorf("decoding JSON response: %w", err)
		}
		return result, nil
	}
}

func makeRequest(
	t *testing.T,
	method string,
	path string,
	body any,
) *http.Request {
	t.Helper()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	// nolint:noctx
	req, err := http.NewRequest(method, path, bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	return req
}

func TestMakeAttestCELHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		wantOutput := "Hello, CEL"
		wantBytes, err := json.Marshal(wantOutput)
		require.NoError(t, err)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)
		verifier, err := tee.NewVerifier(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		whitelist := map[string]engine.CELEngineFn{
			"httpGet": makeHTTPGet(backend.Client()),
		}
		celEngine, err := engine.NewCELEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		expression := `httpGet(targetUrl)`
		env := map[string]any{
			"targetUrl": backend.URL,
		}
		recorder := httptest.NewRecorder()
		body := networking.AttestCELRequest{Expression: expression, Env: env}
		req := makeRequest(t, "POST", networking.AttestCELPath, body)

		handler := networking.MakeAttestCELHandler(
			celEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)

		response := networking.AttestCELResponse{}
		err = json.NewDecoder(recorder.Body).Decode(&response)
		require.NoError(t, err)

		verified, err := verifier.Verify(response.Attestation)
		require.NoError(t, err)

		got := networking.AttestedCEL{}
		err = json.Unmarshal(verified.UserData, &got)
		require.NoError(t, err)
		assert.Equal(t, expression, got.Expression)
		assert.Equal(t, env, got.Env)

		gotOutput, ok := got.Output.(string)
		require.True(t, ok)
		assert.Equal(t, wantOutput, gotOutput)
	})

	t.Run("error - decoding request", func(t *testing.T) {
		// given
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := []byte("invalid json")
		req := makeRequest(t, "POST", networking.AttestCELPath, body)

		celEngine, err := engine.NewCELEngine()
		require.NoError(t, err)

		handler := networking.MakeAttestCELHandler(
			celEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "decoding request")
	})

	t.Run("error - evaluating expr", func(t *testing.T) {
		// given
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		expression := `httpGet(targetUrl)`
		env := map[string]any{
			"targetUrl": "http://thiswontbecalled.org",
		}
		recorder := httptest.NewRecorder()
		body := networking.AttestCELRequest{Expression: expression, Env: env}
		req := makeRequest(t, "POST", networking.AttestCELPath, body)

		celEngine, err := engine.NewCELEngine()
		require.NoError(t, err)

		handler := networking.MakeAttestCELHandler(
			celEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "executing expression")
	})

	t.Run("error - attesting expr", func(t *testing.T) {
		// given
		want := map[string]string{"status": "ok"}
		wantBytes, err := json.Marshal(want)
		require.NoError(t, err)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		base := mocks.NewAttester(t)
		base.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(nil, assert.AnError).Once()
		attester, err := tee.NewAttesterWithBase(base)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		whitelist := map[string]engine.CELEngineFn{
			"httpGet": makeHTTPGet(backend.Client()),
		}
		celEngine, err := engine.NewCELEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		expression := `httpGet(targetUrl)`
		env := map[string]any{
			"targetUrl": backend.URL,
		}
		recorder := httptest.NewRecorder()
		body := networking.AttestCELRequest{Expression: expression, Env: env}
		req := makeRequest(t, "POST", networking.AttestCELPath, body)

		handler := networking.MakeAttestCELHandler(
			celEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "attesting")
	})
}

func TestMakeAttestExprHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		wantOutput := "Hello, Expr"
		wantBytes, err := json.Marshal(wantOutput)
		require.NoError(t, err)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)
		verifier, err := tee.NewVerifier(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		whitelist := map[string]engine.ExprEngineFn{
			"httpGet": makeHTTPGet(backend.Client()),
		}
		exprEngine, err := engine.NewExprEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		expression := `httpGet(targetUrl)`
		env := map[string]any{
			"targetUrl": backend.URL,
		}
		recorder := httptest.NewRecorder()
		body := networking.AttestExprRequest{Expression: expression, Env: env}
		req := makeRequest(t, "POST", networking.AttestExprPath, body)

		handler := networking.MakeAttestExprHandler(
			exprEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)

		response := networking.AttestExprResponse{}
		err = json.NewDecoder(recorder.Body).Decode(&response)
		require.NoError(t, err)

		verified, err := verifier.Verify(response.Attestation)
		require.NoError(t, err)

		got := networking.AttestedExpr{}
		err = json.Unmarshal(verified.UserData, &got)
		require.NoError(t, err)
		assert.Equal(t, expression, got.Expression)
		assert.Equal(t, env, got.Env)

		gotOutput, ok := got.Output.(string)
		require.True(t, ok)
		assert.Equal(t, wantOutput, gotOutput)
	})

	t.Run("error - decoding request", func(t *testing.T) {
		// given
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := []byte("invalid json")
		req := makeRequest(t, "POST", networking.AttestExprPath, body)

		exprEngine, err := engine.NewExprEngine()
		require.NoError(t, err)

		handler := networking.MakeAttestExprHandler(
			exprEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "decoding request")
	})

	t.Run("error - evaluating expr", func(t *testing.T) {
		// given
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		expression := `httpGet(targetUrl)`
		env := map[string]any{
			"targetUrl": "http://thiswontbecalled.org",
		}
		recorder := httptest.NewRecorder()
		body := networking.AttestExprRequest{Expression: expression, Env: env}
		req := makeRequest(t, "POST", networking.AttestExprPath, body)

		exprEngine, err := engine.NewExprEngine()
		require.NoError(t, err)

		handler := networking.MakeAttestExprHandler(
			exprEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "executing expression")
	})

	t.Run("error - attesting expr", func(t *testing.T) {
		// given
		want := map[string]string{"status": "ok"}
		wantBytes, err := json.Marshal(want)
		require.NoError(t, err)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		base := mocks.NewAttester(t)
		base.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(nil, assert.AnError).Once()
		attester, err := tee.NewAttesterWithBase(base)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		whitelist := map[string]engine.ExprEngineFn{
			"httpGet": makeHTTPGet(backend.Client()),
		}
		exprEngine, err := engine.NewExprEngineWithWhitelist(whitelist)
		require.NoError(t, err)

		expression := `httpGet(targetUrl)`
		env := map[string]any{
			"targetUrl": backend.URL,
		}
		recorder := httptest.NewRecorder()
		body := networking.AttestExprRequest{Expression: expression, Env: env}
		req := makeRequest(t, "POST", networking.AttestExprPath, body)

		handler := networking.MakeAttestExprHandler(
			exprEngine,
			defaultTimeout,
			attester,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "attesting")
	})
}

func TestMakeAttestHTTPCallHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		want := map[string]string{"status": "ok"}
		wantBytes, err := json.Marshal(want)
		require.NoError(t, err)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)
		verifier, err := tee.NewVerifier(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		method := "GET"
		url := backend.URL
		recorder := httptest.NewRecorder()
		body := networking.AttestHTTPCallRequest{Method: method, URL: url}
		req := makeRequest(t, "POST", networking.AttestHTTPCallPath, body)

		handler := networking.MakeAttestHTTPCallHandler(
			networking.DefaultTimeout,
			attester,
			backend.Client(),
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)

		response := networking.AttestHTTPCallResponse{}
		err = json.NewDecoder(recorder.Body).Decode(&response)
		require.NoError(t, err)

		verified, err := verifier.Verify(response.Attestation)
		require.NoError(t, err)

		var got map[string]string
		err = json.Unmarshal(verified.UserData, &got)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("error - decoding request", func(t *testing.T) {
		// given
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := []byte("invalid json")
		req := makeRequest(t, "POST", networking.AttestHTTPCallPath, body)

		handler := networking.MakeAttestHTTPCallHandler(
			networking.DefaultTimeout,
			attester,
			nil,
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Equal(t, 0, logBuffer.Len())
		assert.Contains(t, recorder.Body.String(), "decoding request")
	})

	t.Run("error - attesting userdata", func(t *testing.T) {
		// given
		want := map[string]string{"status": "ok"}
		wantBytes, err := json.Marshal(want)
		require.NoError(t, err)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		base := mocks.NewAttester(t)
		base.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(nil, assert.AnError).Once()
		attester, err := tee.NewAttesterWithBase(base)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		method := "GET"
		url := backend.URL
		recorder := httptest.NewRecorder()
		body := networking.AttestHTTPCallRequest{Method: method, URL: url}
		req := makeRequest(t, "POST", networking.AttestHTTPCallPath, body)

		handler := networking.MakeAttestHTTPCallHandler(
			networking.DefaultTimeout,
			attester,
			backend.Client(),
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "attesting")
	})
}

func TestMakeAttestUserDataHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		userData := []byte("hello world")
		nonce := []byte("nonce")
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)
		verifier, err := tee.NewVerifier(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := networking.AttestUserDataRequest{Nonce: nonce, UserData: userData}
		req := makeRequest(t, "POST", networking.AttestUserDataPath, body)

		handler := networking.MakeAttestUserDataHandler(attester, logger)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(nonce))
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(userData))

		response := networking.AttestUserDataResponse{}
		err = json.NewDecoder(recorder.Body).Decode(&response)
		require.NoError(t, err)

		verified, err := verifier.Verify(response.Attestation, tee.WithVerifyNonce(nonce))
		require.NoError(t, err)
		assert.Equal(t, userData, verified.UserData)
	})

	t.Run("error - decoding request", func(t *testing.T) {
		// given
		attester, err := tee.NewAttester(tee.NoTEE)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := []byte("invalid json")
		req := makeRequest(t, "POST", networking.AttestUserDataPath, body)

		handler := networking.MakeAttestUserDataHandler(attester, logger)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Equal(t, 0, logBuffer.Len())
		assert.Contains(t, recorder.Body.String(), "decoding request")
	})

	t.Run("error - attesting userdata", func(t *testing.T) {
		// given
		data := []byte("hello world")
		nonce := []byte("nonce")
		base := mocks.NewAttester(t)
		base.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(nil, assert.AnError).Once()
		attester, err := tee.NewAttesterWithBase(base)
		require.NoError(t, err)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := networking.AttestUserDataRequest{Nonce: nonce, UserData: data}
		req := makeRequest(t, "POST", networking.AttestUserDataPath, body)

		handler := networking.MakeAttestUserDataHandler(attester, logger)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(nonce))
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(data))
		assert.Contains(t, recorder.Body.String(), "attesting")
	})
}
