package taos

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultIoTDBRESTPort   = "18080"
	defaultIoTDBTimeout    = 15 * time.Second
	defaultIoTDBRowLimit   = 10000
	maxIoTDBSQLLength      = 1 << 20
	maxIoTDBResponseLength = 64 << 20
)

type iotdbConfig struct {
	baseURL         *url.URL
	username        string
	password        string
	database        string
	timeout         time.Duration
	rowLimit        int
	location        *time.Location
	interpolateArgs bool
}

func parseIoTDBDSN(dsn string) (*iotdbConfig, error) {
	var cfg *iotdbConfig
	var rawQuery string
	var err error

	if strings.Contains(dsn, "://") {
		cfg, rawQuery, err = parseIoTDBURL(dsn)
	} else {
		cfg, rawQuery, err = parseIoTDBLegacyDSN(dsn)
	}
	if err != nil {
		return nil, err
	}

	cfg.timeout = defaultIoTDBTimeout
	cfg.rowLimit = defaultIoTDBRowLimit
	cfg.location = time.UTC
	cfg.interpolateArgs = true

	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid IoTDB DSN query parameters")
	}
	for key, values := range params {
		if len(values) != 1 {
			return nil, fmt.Errorf("IoTDB DSN parameter %q must appear once", key)
		}
		value := values[0]
		switch key {
		case "timeout":
			duration, parseErr := time.ParseDuration(value)
			if parseErr != nil || duration <= 0 {
				return nil, fmt.Errorf("invalid IoTDB DSN timeout")
			}
			cfg.timeout = duration
		case "row_limit":
			limit, parseErr := strconv.Atoi(value)
			if parseErr != nil || limit <= 0 {
				return nil, fmt.Errorf("invalid IoTDB DSN row_limit")
			}
			cfg.rowLimit = limit
		case "loc":
			location, parseErr := time.LoadLocation(value)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid IoTDB DSN loc")
			}
			cfg.location = location
		case "interpolateParams":
			interpolate, parseErr := strconv.ParseBool(value)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid IoTDB DSN interpolateParams")
			}
			cfg.interpolateArgs = interpolate
		case "model":
			if !strings.EqualFold(value, "table") {
				return nil, fmt.Errorf("IoTDB REST driver supports only the Table model")
			}
		default:
			return nil, fmt.Errorf("unknown IoTDB DSN parameter %q", key)
		}
	}

	return cfg, nil
}

func parseIoTDBURL(dsn string) (*iotdbConfig, string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, "", fmt.Errorf("invalid IoTDB DSN URL")
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "iotdb", "http":
		scheme = "http"
	case "iotdbs", "https":
		scheme = "https"
	default:
		return nil, "", fmt.Errorf("IoTDB DSN scheme must be iotdb, iotdbs, http, or https")
	}

	if parsed.User == nil {
		return nil, "", fmt.Errorf("IoTDB DSN must include username and password")
	}
	password, hasPassword := parsed.User.Password()
	if !hasPassword || parsed.User.Username() == "" {
		return nil, "", fmt.Errorf("IoTDB DSN must include username and password")
	}

	host, port, err := normalizeIoTDBAddress(parsed.Hostname(), parsed.Port())
	if err != nil {
		return nil, "", err
	}
	baseURL := &url.URL{Scheme: scheme, Host: net.JoinHostPort(host, port)}

	database, err := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	if err != nil || strings.Contains(database, "/") {
		return nil, "", fmt.Errorf("invalid IoTDB database in DSN")
	}

	return &iotdbConfig{baseURL: baseURL, username: parsed.User.Username(), password: password, database: database}, parsed.RawQuery, nil
}

func parseIoTDBLegacyDSN(dsn string) (*iotdbConfig, string, error) {
	query := ""
	if index := strings.IndexByte(dsn, '?'); index >= 0 {
		query = dsn[index+1:]
		dsn = dsn[:index]
	}

	separator := strings.Index(dsn, ")/")
	if separator < 0 || separator+2 > len(dsn) {
		return nil, "", fmt.Errorf("invalid IoTDB DSN; expected user:password@http(host:port)/database")
	}
	authority := dsn[:separator+1]
	database, err := url.PathUnescape(dsn[separator+2:])
	if err != nil || strings.Contains(database, "/") {
		return nil, "", fmt.Errorf("invalid IoTDB database in DSN")
	}

	at := strings.LastIndexByte(authority, '@')
	if at < 0 {
		return nil, "", fmt.Errorf("IoTDB DSN must include username and password")
	}
	userInfo := authority[:at]
	colon := strings.IndexByte(userInfo, ':')
	if colon < 0 {
		return nil, "", fmt.Errorf("IoTDB DSN must include username and password")
	}
	username, err := url.QueryUnescape(userInfo[:colon])
	if err != nil || username == "" {
		return nil, "", fmt.Errorf("invalid IoTDB DSN username")
	}
	password, err := url.QueryUnescape(userInfo[colon+1:])
	if err != nil {
		return nil, "", fmt.Errorf("invalid IoTDB DSN password encoding")
	}

	protocolAddress := authority[at+1:]
	openParen := strings.IndexByte(protocolAddress, '(')
	if openParen < 0 || protocolAddress[len(protocolAddress)-1] != ')' {
		return nil, "", fmt.Errorf("invalid IoTDB DSN network address")
	}
	scheme := strings.ToLower(protocolAddress[:openParen])
	if scheme != "http" && scheme != "https" {
		return nil, "", fmt.Errorf("IoTDB DSN network must be http or https")
	}
	host, port, err := net.SplitHostPort(protocolAddress[openParen+1 : len(protocolAddress)-1])
	if err != nil {
		return nil, "", fmt.Errorf("invalid IoTDB DSN host and port")
	}
	host, port, err = normalizeIoTDBAddress(host, port)
	if err != nil {
		return nil, "", err
	}

	return &iotdbConfig{
		baseURL:  &url.URL{Scheme: scheme, Host: net.JoinHostPort(host, port)},
		username: username,
		password: password,
		database: database,
	}, query, nil
}

func normalizeIoTDBAddress(host, port string) (string, string, error) {
	if host == "" {
		return "", "", fmt.Errorf("IoTDB DSN host is required")
	}
	if port == "" {
		port = defaultIoTDBRESTPort
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", "", fmt.Errorf("invalid IoTDB REST port")
	}
	return host, strconv.Itoa(portNumber), nil
}
