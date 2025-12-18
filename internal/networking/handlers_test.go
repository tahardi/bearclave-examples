package networking_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"bearclave-examples/internal/networking"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tahardi/bearclave/mocks"
	"github.com/tahardi/bearclave/tee"
)

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

func TestMakeAttestAPICallHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		want := map[string]string{"status": "ok"}
		wantBytes, err := json.Marshal(want)
		require.NoError(t, err)
		wantHash := sha256.Sum256(wantBytes)

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

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		method := "GET"
		url := backend.URL
		recorder := httptest.NewRecorder()
		body := networking.AttestAPICallRequest{Method: method, URL: url}
		req := makeRequest(t, "POST", networking.AttestAPICallPath, body)

		handler := networking.MakeAttestAPICallHandler(
			networking.DefaultTimeout,
			attester,
			backend.Client(),
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(
			t,
			logBuffer.String(),
			base64.StdEncoding.EncodeToString(wantHash[:]),
		)

		response := networking.AttestAPICallResponse{}
		err = json.NewDecoder(recorder.Body).Decode(&response)
		require.NoError(t, err)

		verifier, err := tee.NewVerifier(tee.NoTEE)
		require.NoError(t, err)

		verified, err := verifier.Verify(response.Attestation)
		require.NoError(t, err)
		assert.Equal(t, wantHash[:], verified.UserData)
	})

	t.Run("error - decoding request", func(t *testing.T) {
		// given
		attester := mocks.NewAttester(t)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := []byte("invalid json")
		req := makeRequest(t, "POST", networking.AttestAPICallPath, body)

		handler := networking.MakeAttestAPICallHandler(
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
		wantHash := sha256.Sum256(wantBytes)

		backend := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(wantBytes)
			}),
		)
		defer backend.Close()

		attester := mocks.NewAttester(t)
		attester.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(nil, assert.AnError).Once()

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		method := "GET"
		url := backend.URL
		recorder := httptest.NewRecorder()
		body := networking.AttestAPICallRequest{Method: method, URL: url}
		req := makeRequest(t, "POST", networking.AttestAPICallPath, body)

		handler := networking.MakeAttestAPICallHandler(
			networking.DefaultTimeout,
			attester,
			backend.Client(),
			logger,
		)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(wantHash[:]))
		assert.Contains(t, recorder.Body.String(), "attesting")
	})
}

func TestMakeAttestUserDataHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		// given
		data := []byte("hello world")
		nonce := []byte("nonce")
		attestation := &tee.AttestResult{Report: []byte("attestation")}
		attester := mocks.NewAttester(t)
		attester.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(attestation, nil).Once()

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		recorder := httptest.NewRecorder()
		body := networking.AttestUserDataRequest{Nonce: nonce, UserData: data}
		req := makeRequest(t, "POST", networking.AttestUserDataPath, body)

		handler := networking.MakeAttestUserDataHandler(attester, logger)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(nonce))
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(data))

		response := networking.AttestUserDataResponse{}
		err := json.NewDecoder(recorder.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, attestation, response.Attestation)
	})

	t.Run("error - decoding request", func(t *testing.T) {
		// given
		attester := mocks.NewAttester(t)

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
		attester := mocks.NewAttester(t)
		attester.
			On("Attest", mock.AnythingOfType("[]attestation.AttestOption")).
			Return(nil, assert.AnError).Once()

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
