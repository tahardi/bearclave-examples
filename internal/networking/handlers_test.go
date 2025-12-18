package networking_test

import (
	"bytes"
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

func TestMakeAttestHandler(t *testing.T) {
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
		body := networking.AttestRequest{Nonce: nonce, UserData: data}
		req := makeRequest(t, "POST", networking.AttestPath, body)

		handler := networking.MakeAttestHandler(attester, logger)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(nonce))
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(data))

		response := networking.AttestResponse{}
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
		req := makeRequest(t, "POST", networking.AttestPath, body)

		handler := networking.MakeAttestHandler(attester, logger)

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
		body := networking.AttestRequest{Nonce: nonce, UserData: data}
		req := makeRequest(t, "POST", networking.AttestPath, body)

		handler := networking.MakeAttestHandler(attester, logger)

		// when
		handler.ServeHTTP(recorder, req)

		// then
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(nonce))
		assert.Contains(t, logBuffer.String(), base64.StdEncoding.EncodeToString(data))
		assert.Contains(t, recorder.Body.String(), "attesting")
	})
}
