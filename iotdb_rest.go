package taos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	iotdbQueryEndpoint    = "/rest/table/v1/query"
	iotdbNonQueryEndpoint = "/rest/table/v1/nonQuery"
	iotdbPingEndpoint     = "/ping"
)

type iotdbRESTEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type iotdbQueryRequest struct {
	Database string `json:"database"`
	SQL      string `json:"sql"`
	RowLimit int    `json:"row_limit"`
}

type iotdbNonQueryRequest struct {
	Database string `json:"database,omitempty"`
	SQL      string `json:"sql"`
}

type iotdbQueryResponse struct {
	Code        int                 `json:"code"`
	Message     string              `json:"message"`
	ColumnNames []string            `json:"column_names"`
	DataTypes   []string            `json:"data_types"`
	Values      [][]json.RawMessage `json:"values"`
}

// IoTDBRESTError carries the database code and HTTP status returned by IoTDB.
type IoTDBRESTError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e *IoTDBRESTError) Error() string {
	message := e.Message
	if message == "" {
		message = "request failed"
	}
	if e.Code != 0 {
		return fmt.Sprintf("IoTDB REST error code %d (HTTP %d): %s", e.Code, e.HTTPStatus, message)
	}
	return fmt.Sprintf("IoTDB REST request failed (HTTP %d): %s", e.HTTPStatus, message)
}

func (c *iotdbConn) postJSON(ctx context.Context, endpoint string, payload interface{}, output interface{}, requireSuccessCode bool) error {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode IoTDB REST request: %w", err)
	}

	body, statusCode, err := c.doHTTP(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody), true)
	if err != nil {
		return err
	}

	envelope := iotdbRESTEnvelope{}
	_ = json.Unmarshal(body, &envelope)
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return &IoTDBRESTError{HTTPStatus: statusCode, Code: envelope.Code, Message: envelope.Message}
	}
	if envelope.Code != 0 && envelope.Code != http.StatusOK {
		return &IoTDBRESTError{HTTPStatus: statusCode, Code: envelope.Code, Message: envelope.Message}
	}
	if requireSuccessCode && envelope.Code != http.StatusOK {
		return &IoTDBRESTError{HTTPStatus: statusCode, Message: "response did not contain code 200"}
	}
	if output != nil {
		if err := json.Unmarshal(body, output); err != nil {
			return fmt.Errorf("decode IoTDB REST response: %w", err)
		}
	}
	return nil
}

func (c *iotdbConn) ping(ctx context.Context) error {
	_, statusCode, err := c.doHTTP(ctx, http.MethodGet, iotdbPingEndpoint, nil, false)
	if err != nil {
		return err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return &IoTDBRESTError{HTTPStatus: statusCode, Message: "health check failed"}
	}
	return nil
}

func (c *iotdbConn) doHTTP(ctx context.Context, method, endpoint string, body io.Reader, authenticate bool) ([]byte, int, error) {
	if err := c.checkOpen(); err != nil {
		return nil, 0, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	target := *c.config.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + endpoint
	target.RawPath = ""
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, 0, fmt.Errorf("create IoTDB REST request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticate {
		request.SetBasicAuth(c.config.username, c.config.password)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(io.LimitReader(response.Body, maxIoTDBResponseLength+1))
	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("read IoTDB REST response: %w", err)
	}
	if int64(len(data)) > maxIoTDBResponseLength {
		return nil, response.StatusCode, fmt.Errorf("IoTDB REST response exceeds %d bytes", maxIoTDBResponseLength)
	}
	return data, response.StatusCode, nil
}
