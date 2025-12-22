package networking

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"bearclave-examples/internal/engine"

	"github.com/tahardi/bearclave/tee"
)

const (
	AttestCELPath      = "/attest-cel"
	AttestExprPath     = "/attest-expr"
	AttestHTTPCallPath = "/attest-http-call"
	AttestUserDataPath = "/attest-user-data"
	DefaultTimeout     = 15 * time.Second
)

type AttestCELRequest struct {
	Expression string         `json:"expression"`
	Env        map[string]any `json:"env"`
}

type AttestedCEL struct {
	Expression string `json:"expression"`
	Env        any    `json:"env"`
	Output     any    `json:"output"`
}

type AttestCELResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
}

func MakeAttestCELHandler(
	celEngine *engine.CELEngine,
	celTimeout time.Duration,
	attester *tee.Attester,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exprReq := AttestCELRequest{}
		err := json.NewDecoder(r.Body).Decode(&exprReq)
		if err != nil {
			logger.Error("decoding request", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), celTimeout)
		defer cancel()

		logger.Info("executing cel", slog.String("expression", exprReq.Expression))
		output, err := celEngine.Execute(ctx, exprReq.Expression, exprReq.Env)
		if err != nil {
			logger.Error("executing expression", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("executing expression: %w", err))
			return
		}

		result := AttestedCEL{
			Expression: exprReq.Expression,
			Env:        exprReq.Env,
			Output:     output,
		}
		resBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("marshaling result", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("marshaling result: %w", err))
			return
		}

		logger.Info("attesting cel", slog.Any("result", result))
		attestation, err := attester.Attest(tee.WithAttestUserData(resBytes))
		if err != nil {
			logger.Error("attesting", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		apiCallResp := AttestCELResponse{
			Attestation: attestation,
		}
		WriteResponse(w, apiCallResp)
	}
}

type AttestExprRequest struct {
	Expression string         `json:"expression"`
	Env        map[string]any `json:"env"`
}

type AttestedExpr struct {
	Expression string `json:"expression"`
	Env        any    `json:"env"`
	Output     any    `json:"output"`
}

type AttestExprResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
}

func MakeAttestExprHandler(
	exprEngine *engine.ExprEngine,
	exprTimeout time.Duration,
	attester *tee.Attester,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exprReq := AttestExprRequest{}
		err := json.NewDecoder(r.Body).Decode(&exprReq)
		if err != nil {
			logger.Error("decoding request", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), exprTimeout)
		defer cancel()

		logger.Info("executing expr", slog.String("expression", exprReq.Expression))
		output, err := exprEngine.Execute(ctx, exprReq.Expression, exprReq.Env)
		if err != nil {
			logger.Error("executing expression", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("executing expression: %w", err))
			return
		}

		result := AttestedExpr{
			Expression: exprReq.Expression,
			Env:        exprReq.Env,
			Output:     output,
		}
		resBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("marshaling result", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("marshaling result: %w", err))
			return
		}

		logger.Info("attesting expr", slog.Any("result", result))
		attestation, err := attester.Attest(tee.WithAttestUserData(resBytes))
		if err != nil {
			logger.Error("attesting", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		apiCallResp := AttestExprResponse{
			Attestation: attestation,
		}
		WriteResponse(w, apiCallResp)
	}
}

type AttestHTTPCallRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type AttestHTTPCallResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
}

func MakeAttestHTTPCallHandler(
	ctxTimeout time.Duration,
	attester *tee.Attester,
	client *http.Client,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiCallReq := AttestHTTPCallRequest{}
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
			"making http call",
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

		logger.Info("attesting http call")
		attestation, err := attester.Attest(tee.WithAttestUserData(respBytes))
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		apiCallResp := AttestHTTPCallResponse{
			Attestation: attestation,
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
	attester *tee.Attester,
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
