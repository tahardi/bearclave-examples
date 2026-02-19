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

	"github.com/tahardi/bearclave-examples/internal/engine"

	"github.com/tahardi/bearclave/tee"
)

const (
	AttestCertPath      = "/attest-cert"
	AttestCELPath       = "/attest-cel"
	AttestExprPath      = "/attest-expr"
	AttestHTTPCallPath  = "/attest-http-call"
	AttestHTTPSCallPath = "/attest-https-call"
	AttestUserDataPath  = "/attest-user-data"
	DefaultTimeout      = 15 * time.Second
)

type AttestCertRequest struct{}
type AttestCertResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
}

func MakeAttestCertHandler(
	attester *tee.Attester,
	certProvider tee.CertProvider,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("received attest cert request")
		certReq := AttestCertRequest{}
		err := json.NewDecoder(r.Body).Decode(&certReq)
		if err != nil {
			logger.Error("decoding request", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		cert, err := certProvider.GetCert(r.Context())
		if err != nil {
			logger.Error("getting cert", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("getting cert: %w", err))
			return
		}

		chainDER := append([][]byte{}, cert.Certificate...)
		chainJSON, err := json.Marshal(chainDER)
		if err != nil {
			logger.Error("marshaling chain", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("marshaling chain: %w", err))
			return
		}

		logger.Info("attesting cert")
		att, err := attester.Attest(tee.WithAttestUserData(chainJSON))
		if err != nil {
			logger.Error("attesting", slog.String("error", err.Error()))
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}
		tee.WriteResponse(w, AttestCertResponse{Attestation: att})
	}
}

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
		logger.Info("received attest CEL request")
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
		logger.Info("received attest expr request")
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
		logger.Info("received attest HTTP call request")
		httpCallReq := AttestHTTPCallRequest{}
		err := json.NewDecoder(r.Body).Decode(&httpCallReq)
		if err != nil {
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), ctxTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, httpCallReq.Method, httpCallReq.URL, nil)
		if err != nil {
			WriteError(w, fmt.Errorf("creating request: %w", err))
			return
		}

		logger.Info(
			"making HTTP call",
			slog.String("method", httpCallReq.Method),
			slog.String("URL", httpCallReq.URL),
		)
		// G704 - potential for Server-Side Request Forgery (SSRF)
		//nolint:gosec
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

		logger.Info("attesting HTTP call")
		attestation, err := attester.Attest(tee.WithAttestUserData(respBytes))
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		httpCallResp := AttestHTTPCallResponse{
			Attestation: attestation,
		}
		WriteResponse(w, httpCallResp)
	}
}

type AttestHTTPSCallRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}
type AttestHTTPSCallResponse struct {
	Attestation *tee.AttestResult `json:"attestation"`
}

func MakeAttestHTTPSCallHandler(
	ctxTimeout time.Duration,
	attester *tee.Attester,
	client *http.Client,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("received attest HTTPS call request")
		httpsCallReq := AttestHTTPSCallRequest{}
		err := json.NewDecoder(r.Body).Decode(&httpsCallReq)
		if err != nil {
			WriteError(w, fmt.Errorf("decoding request: %w", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), ctxTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, httpsCallReq.Method, httpsCallReq.URL, nil)
		if err != nil {
			WriteError(w, fmt.Errorf("creating request: %w", err))
			return
		}

		logger.Info(
			"making HTTPS call",
			slog.String("method", httpsCallReq.Method),
			slog.String("URL", httpsCallReq.URL),
		)
		// G704 - potential for Server-Side Request Forgery (SSRF)
		//nolint:gosec
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

		logger.Info("attesting HTTPS call")
		attestation, err := attester.Attest(tee.WithAttestUserData(respBytes))
		if err != nil {
			WriteError(w, fmt.Errorf("attesting: %w", err))
			return
		}

		httpsCallResp := AttestHTTPSCallResponse{
			Attestation: attestation,
		}
		WriteResponse(w, httpsCallResp)
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
		logger.Info("received attest user data request")
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
