package networking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/tahardi/bearclave/tee"
)

var (
	ErrClient               = errors.New("client")
	ErrClientNon200Response = fmt.Errorf("%w: non-200 response", ErrClient)
)

type Client struct {
	host   string
	client *http.Client
}

func NewClient(host string) *Client {
	client := &http.Client{}
	return NewClientWithClient(host, client)
}

func NewClientWithClient(
	host string,
	client *http.Client,
) *Client {
	return &Client{
		host:   host,
		client: client,
	}
}

func (c *Client) Attest(
	ctx context.Context,
	nonce []byte,
	userData []byte,
) (*tee.AttestResult, error) {
	attestUserDataRequest := AttestRequest{Nonce: nonce, UserData: userData}
	attestUserDataResponse := AttestResponse{}
	err := c.Do(
		ctx,
		"POST",
		AttestPath,
		attestUserDataRequest,
		&attestUserDataResponse,
	)
	if err != nil {
		return nil, fmt.Errorf("doing attest request: %w", err)
	}
	return attestUserDataResponse.Attestation, nil
}

func (c *Client) Do(
	ctx context.Context,
	method string,
	api string,
	apiReq any,
	apiResp any,
) error {
	bodyBytes, err := json.Marshal(apiReq)
	if err != nil {
		return clientError("marshaling request body", err)
	}

	url := c.host + api
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return clientError("creating request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	switch {
	case err != nil:
		return clientError("sending request", err)
	case resp.StatusCode != http.StatusOK:
		msg := strconv.Itoa(resp.StatusCode)
		return clientErrorNon200Response(msg, nil)
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return clientError("reading response body", err)
	}

	err = json.Unmarshal(bodyBytes, apiResp)
	if err != nil {
		return clientError("unmarshaling response", err)
	}
	return nil
}

func wrapClientError(clientErr error, msg string, err error) error {
	switch {
	case msg == "" && err == nil:
		return clientErr
	case msg != "" && err != nil:
		return fmt.Errorf("%w: %s: %w", clientErr, msg, err)
	case msg != "":
		return fmt.Errorf("%w: %s", clientErr, msg)
	default:
		return fmt.Errorf("%w: %w", clientErr, err)
	}
}

func clientError(msg string, err error) error {
	return wrapClientError(ErrClient, msg, err)
}

func clientErrorNon200Response(msg string, err error) error {
	return wrapClientError(ErrClientNon200Response, msg, err)
}
