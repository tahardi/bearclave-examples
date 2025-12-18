package networking

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/tahardi/bearclave/tee"
)

const (
	AttestAPICallPath  = "/attest-api-call"
	AttestUserDataPath = "/attest-user-data"
	DefaultTimeout     = 15 * time.Second
)

type AttestAPICallRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type AttestAPICallResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
	Response    []byte            `json:"response"`
}

func MakeAttestAPICallHandler(
	ctxTimeout time.Duration,
	attester tee.Attester,
	client *http.Client,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiCallReq := AttestAPICallRequest{}
		err := json.NewDecoder(r.Body).Decode(&apiCallReq)
		if err != nil {
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), ctxTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, apiCallReq.Method, apiCallReq.URL, nil)
		if err != nil {
			WriteError(w, fmt.Errorf("creating request: %w", err))
			return
		}

		logger.Info(
			"making API call",
			slog.String("method", apiCallReq.Method),
			slog.String("url", apiCallReq.URL),
		)
		resp, err := client.Do(req)
		if err != nil {
			WriteError(w, fmt.Errorf("sending request: %w", err))
			return
		}
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			WriteError(w, fmt.Errorf("reading response body: %w", err))
			return
		}

		hash := sha256.Sum256(respBytes)
		logger.Info(
			"attesting api call",
			slog.String("hash", base64.StdEncoding.EncodeToString(hash[:])),
		)
		attestation, err := attester.Attest(tee.WithAttestUserData(hash[:]))
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		apiCallResp := AttestAPICallResponse{
			Attestation: attestation,
			Response:    respBytes,
		}
		WriteResponse(w, apiCallResp)
	}
}

type AttestUserDataRequest struct {
	Nonce    []byte `json:"nonce,omitempty"`
	UserData []byte `json:"userdata,omitempty"`
}
type AttestUserDataResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
}

func MakeAttestUserDataHandler(
	attester tee.Attester,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := AttestUserDataRequest{}
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		logger.Info(
			"attesting",
			slog.String("nonce", base64.StdEncoding.EncodeToString(req.Nonce)),
			slog.String("userdata", base64.StdEncoding.EncodeToString(req.UserData)),
		)
		att, err := attester.Attest(
			tee.WithAttestNonce(req.Nonce),
			tee.WithAttestUserData(req.UserData),
		)
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}
		WriteResponse(w, AttestUserDataResponse{Attestation: att})
	}
}

func WriteError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func WriteResponse(w http.ResponseWriter, out any) {
	data, err := json.Marshal(out)
	if err != nil {
		WriteError(w, fmt.Errorf("marshaling response: %w", err))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		WriteError(w, fmt.Errorf("writing response: %w", err))
		return
	}
}
