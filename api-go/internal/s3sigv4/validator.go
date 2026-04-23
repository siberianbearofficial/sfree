package s3sigv4

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMissingAuth          = errors.New("sigv4: missing Authorization header or presign query parameters")
	ErrUnsupportedAlgorithm = errors.New("sigv4: unsupported algorithm")
	ErrInvalidAuthorization = errors.New("sigv4: invalid Authorization header format")
	ErrInvalidPresign       = errors.New("sigv4: invalid presign query")
	ErrInvalidCredential    = errors.New("sigv4: invalid credential scope")
	ErrInvalidSignedHeaders = errors.New("sigv4: invalid SignedHeaders")
	ErrMissingSignedHeader  = errors.New("sigv4: request missing a header listed in SignedHeaders")
	ErrSignatureMismatch    = errors.New("sigv4: signature mismatch")
	ErrExpired              = errors.New("sigv4: request expired")
	ErrClockSkew            = errors.New("sigv4: request time outside allowed skew")
	ErrMissingPayloadHash   = errors.New("sigv4: missing X-Amz-Content-Sha256 for request body")
)

type Validator struct {
	Now           func() time.Time
	MaxSkew       time.Duration
	MaxPresignTTL time.Duration
}

type ValidatedRequest struct {
	Mode             string
	Algorithm        string
	AccessKey        string
	Date             string
	Region           string
	Service          string
	AmzDate          time.Time
	Scope            string
	SignedHeaders    []string
	Signature        string
	CanonicalRequest string
	StringToSign     string
	PayloadHash      string
	Request          *http.Request
}

func (v *Validator) nowUTC() time.Time {
	if v.Now != nil {
		return v.Now().UTC()
	}
	return time.Now().UTC()
}

func (v *Validator) skew() time.Duration {
	if v.MaxSkew <= 0 {
		return 15 * time.Minute
	}
	return v.MaxSkew
}

func (v *Validator) maxPresignTTL() time.Duration {
	if v.MaxPresignTTL <= 0 {
		return 7 * 24 * time.Hour
	}
	return v.MaxPresignTTL
}

func AccessKeyFromRequest(r *http.Request) (string, error) {
	if r == nil {
		return "", ErrMissingAuth
	}
	if isPresign(r.URL) {
		return parseAccessKey(r.URL.Query().Get("X-Amz-Credential"))
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return "", ErrMissingAuth
	}

	algo, params, err := parseAuthorizationHeader(auth)
	if err != nil {
		return "", err
	}
	if algo != "AWS4-HMAC-SHA256" {
		return "", ErrUnsupportedAlgorithm
	}

	cred := params["Credential"]
	if cred == "" {
		return "", ErrInvalidAuthorization
	}

	return parseAccessKey(cred)
}

func parseAccessKey(credential string) (string, error) {
	ak, _, _, _, _, err := parseCredential(credential)
	return ak, err
}

func (v *Validator) Validate(ctx context.Context, r *http.Request, accessKey, secretKey string) (*ValidatedRequest, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	if r == nil {
		return nil, errors.New("sigv4: nil request")
	}
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("sigv4: accessKey/secretKey required")
	}

	if isPresign(r.URL) {
		return v.validatePresign(ctx, r, accessKey, secretKey)
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
		return v.validateHeaderAuth(ctx, r, accessKey, secretKey)
	}
	return nil, ErrMissingAuth
}

func isPresign(u *url.URL) bool {
	if u == nil {
		return false
	}
	q := u.Query()
	return strings.EqualFold(q.Get("X-Amz-Algorithm"), "AWS4-HMAC-SHA256") &&
		q.Get("X-Amz-Credential") != "" &&
		q.Get("X-Amz-SignedHeaders") != "" &&
		q.Get("X-Amz-Date") != "" &&
		q.Get("X-Amz-Signature") != ""
}

func (v *Validator) validateHeaderAuth(ctx context.Context, r *http.Request, accessKey, secretKey string) (*ValidatedRequest, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	algo, params, err := parseAuthorizationHeader(auth)
	if err != nil {
		return nil, err
	}
	if algo != "AWS4-HMAC-SHA256" {
		return nil, ErrUnsupportedAlgorithm
	}

	cred := params["Credential"]
	signedHeadersStr := params["SignedHeaders"]
	signature := params["Signature"]
	if cred == "" || signedHeadersStr == "" || signature == "" {
		return nil, ErrInvalidAuthorization
	}

	ak, scope, date, region, service, err := parseCredential(cred)
	if err != nil {
		return nil, err
	}
	if ak != accessKey {
		return nil, fmt.Errorf("sigv4: access key mismatch: got %q", ak)
	}

	amzDate, err := parseAmzDateFromRequest(r, "")
	if err != nil {
		return nil, err
	}
	if amzDate.Format("20060102") != date {
		return nil, ErrInvalidCredential
	}

	now := v.nowUTC()
	if amzDate.Before(now.Add(-v.skew())) || amzDate.After(now.Add(v.skew())) {
		return nil, ErrClockSkew
	}

	signedHeaders, err := normalizeAndValidateSignedHeaders(signedHeadersStr)
	if err != nil {
		return nil, err
	}
	if !contains(signedHeaders, "host") {
		return nil, fmt.Errorf("%w: SignedHeaders must include host", ErrInvalidSignedHeaders)
	}

	payloadHash, err := payloadHashForHeaderAuth(r)
	if err != nil {
		return nil, err
	}

	canonReq, _, err := buildCanonicalRequest(r, signedHeaders, payloadHash, false)
	if err != nil {
		return nil, err
	}

	stringToSign := buildStringToSign(algo, amzDate, scope, canonReq)
	expectedSig := computeSignature(secretKey, date, region, service, stringToSign)

	if !constantTimeHexEquals(expectedSig, signature) {
		return nil, ErrSignatureMismatch
	}

	return &ValidatedRequest{
		Mode:             "header",
		Algorithm:        algo,
		AccessKey:        ak,
		Date:             date,
		Region:           region,
		Service:          service,
		AmzDate:          amzDate,
		Scope:            scope,
		SignedHeaders:    signedHeaders,
		Signature:        strings.ToLower(signature),
		CanonicalRequest: canonReq,
		StringToSign:     stringToSign,
		PayloadHash:      payloadHash,
		Request:          r,
	}, nil
}

func parseAuthorizationHeader(h string) (string, map[string]string, error) {
	if h == "" {
		return "", nil, ErrMissingAuth
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return "", nil, ErrInvalidAuthorization
	}
	algo := strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(parts[1])

	params := map[string]string{}
	for _, p := range splitCommaParams(rest) {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 {
			return "", nil, ErrInvalidAuthorization
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		params[k] = v
	}
	return algo, params, nil
}

func splitCommaParams(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		t := strings.TrimSpace(r)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (v *Validator) validatePresign(ctx context.Context, r *http.Request, accessKey, secretKey string) (*ValidatedRequest, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	q := r.URL.Query()

	algo := q.Get("X-Amz-Algorithm")
	if algo != "AWS4-HMAC-SHA256" {
		return nil, ErrUnsupportedAlgorithm
	}

	cred := q.Get("X-Amz-Credential")
	signedHeadersStr := q.Get("X-Amz-SignedHeaders")
	signature := q.Get("X-Amz-Signature")
	amzDateStr := q.Get("X-Amz-Date")
	expiresStr := q.Get("X-Amz-Expires")

	if cred == "" || signedHeadersStr == "" || signature == "" || amzDateStr == "" || expiresStr == "" {
		return nil, ErrInvalidPresign
	}

	ak, scope, date, region, service, err := parseCredential(cred)
	if err != nil {
		return nil, err
	}
	if ak != accessKey {
		return nil, fmt.Errorf("sigv4: access key mismatch: got %q", ak)
	}

	amzDate, err := parseAmzDate(amzDateStr)
	if err != nil {
		return nil, err
	}
	if amzDate.Format("20060102") != date {
		return nil, ErrInvalidCredential
	}

	expiresSec, err := strconv.Atoi(expiresStr)
	if err != nil || expiresSec <= 0 {
		return nil, fmt.Errorf("%w: invalid X-Amz-Expires", ErrInvalidPresign)
	}
	ttl := time.Duration(expiresSec) * time.Second
	if ttl > v.maxPresignTTL() {
		return nil, fmt.Errorf("%w: presign ttl exceeds max (%s)", ErrInvalidPresign, v.maxPresignTTL())
	}

	now := v.nowUTC()
	if now.Before(amzDate.Add(-v.skew())) {
		return nil, ErrClockSkew
	}
	if now.After(amzDate.Add(ttl)) {
		return nil, ErrExpired
	}

	signedHeaders, err := normalizeAndValidateSignedHeaders(signedHeadersStr)
	if err != nil {
		return nil, err
	}
	if !contains(signedHeaders, "host") {
		return nil, fmt.Errorf("%w: SignedHeaders must include host", ErrInvalidSignedHeaders)
	}

	payloadHash := q.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		if h := strings.TrimSpace(r.Header.Get("X-Amz-Content-Sha256")); h != "" {
			payloadHash = h
		} else if strings.EqualFold(service, "s3") {
			payloadHash = "UNSIGNED-PAYLOAD"
		} else {
			payloadHash, err = payloadHashForHeaderAuth(r)
			if err != nil {
				return nil, err
			}
		}
	}

	canonReq, _, err := buildCanonicalRequest(r, signedHeaders, payloadHash, true)
	if err != nil {
		return nil, err
	}

	stringToSign := buildStringToSign(algo, amzDate, scope, canonReq)
	expectedSig := computeSignature(secretKey, date, region, service, stringToSign)

	if !constantTimeHexEquals(expectedSig, signature) {
		return nil, ErrSignatureMismatch
	}

	return &ValidatedRequest{
		Mode:             "presign",
		Algorithm:        algo,
		AccessKey:        ak,
		Date:             date,
		Region:           region,
		Service:          service,
		AmzDate:          amzDate,
		Scope:            scope,
		SignedHeaders:    signedHeaders,
		Signature:        strings.ToLower(signature),
		CanonicalRequest: canonReq,
		StringToSign:     stringToSign,
		PayloadHash:      payloadHash,
		Request:          r,
	}, nil
}

func buildCanonicalRequest(r *http.Request, signedHeaders []string, payloadHash string, presign bool) (string, string, error) {
	method := r.Method
	if method == "" {
		method = "GET"
	}

	canonicalURI := canonicalizeURI(r.URL)
	canonicalQuery := canonicalizeQuery(r.URL, presign)

	canonicalHeaders, signedHeadersStr, err := canonicalizeHeaders(r, signedHeaders)
	if err != nil {
		return "", "", err
	}

	var sb strings.Builder
	sb.WriteString(method)
	sb.WriteByte('\n')
	sb.WriteString(canonicalURI)
	sb.WriteByte('\n')
	sb.WriteString(canonicalQuery)
	sb.WriteByte('\n')
	sb.WriteString(canonicalHeaders)
	sb.WriteByte('\n')
	sb.WriteString(signedHeadersStr)
	sb.WriteByte('\n')
	sb.WriteString(payloadHash)

	return sb.String(), canonicalHeaders, nil
}

func canonicalizeURI(u *url.URL) string {
	if u == nil {
		return "/"
	}

	path := u.EscapedPath()
	if u.Opaque != "" {
		parts := strings.Split(u.Opaque, "/")
		if len(parts) >= 4 {
			path = "/" + strings.Join(parts[3:], "/")
		}
	}
	if path == "" {
		path = "/"
	}

	decoded, err := url.PathUnescape(path)
	if err != nil {
		return awsPercentEncodePath(path)
	}
	return awsPercentEncodePath(decoded)
}

func awsPercentEncodePath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		if isUnreserved(c) || c == '/' {
			b.WriteByte(c)
		} else {
			_, _ = fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func canonicalizeQuery(u *url.URL, presign bool) string {
	if u == nil {
		return ""
	}
	type kv struct {
		k string
		v string
	}
	var items []kv
	for _, part := range strings.Split(u.RawQuery, "&") {
		if part == "" {
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		decodedName := queryPartUnescape(name)
		if presign && strings.EqualFold(decodedName, "X-Amz-Signature") {
			continue
		}
		items = append(items, kv{
			k: awsPercentEncodeQuery(decodedName),
			v: awsPercentEncodeQuery(queryPartUnescape(value)),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].k == items[j].k {
			return items[i].v < items[j].v
		}
		return items[i].k < items[j].k
	})

	var sb strings.Builder
	for i, it := range items {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(it.k)
		sb.WriteByte('=')
		sb.WriteString(it.v)
	}
	return sb.String()
}

func queryPartUnescape(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}

func awsPercentEncodeQuery(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
		} else {
			_, _ = fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func canonicalizeHeaders(r *http.Request, signedHeaders []string) (string, string, error) {
	var sb strings.Builder
	for _, h := range signedHeaders {
		sb.WriteString(h)
		sb.WriteByte(':')

		if h == "host" {
			host, err := canonicalHost(r)
			if err != nil {
				return "", "", err
			}
			sb.WriteString(canonicalizeHeaderValue(host))
			sb.WriteByte('\n')
			continue
		}

		values := r.Header.Values(h)
		if len(values) == 0 {
			v := r.Header.Get(h)
			if strings.TrimSpace(v) == "" {
				return "", "", fmt.Errorf("%w: %s", ErrMissingSignedHeader, h)
			}
			values = []string{v}
		}

		for i, v := range values {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(canonicalizeHeaderValue(v))
		}
		sb.WriteByte('\n')
	}

	return sb.String(), strings.Join(signedHeaders, ";"), nil
}

func canonicalHost(r *http.Request) (string, error) {
	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	if host == "" {
		host = r.Header.Get("Host")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("%w: host", ErrMissingSignedHeader)
	}

	scheme := strings.ToLower(r.URL.Scheme)
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	if h, p, err := net.SplitHostPort(host); err == nil {
		if (p == "80" && scheme == "http") || (p == "443" && scheme == "https") {
			return h, nil
		}
		return net.JoinHostPort(h, p), nil
	}

	return host, nil
}

func canonicalizeHeaderValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(v))
	inWS := false
	for i := 0; i < len(v); i++ {
		c := v[i]
		isWS := c == ' ' || c == '\t' || c == '\n' || c == '\r'
		if isWS {
			inWS = true
			continue
		}
		if inWS {
			b.WriteByte(' ')
			inWS = false
		}
		b.WriteByte(c)
	}
	return b.String()
}

func parseCredential(cred string) (accessKey, scope, date, region, service string, err error) {
	parts := strings.Split(cred, "/")
	if len(parts) != 5 {
		return "", "", "", "", "", ErrInvalidCredential
	}
	accessKey = parts[0]
	date = parts[1]
	region = parts[2]
	service = parts[3]
	term := parts[4]
	if accessKey == "" || date == "" || region == "" || service == "" || term != "aws4_request" {
		return "", "", "", "", "", ErrInvalidCredential
	}
	if len(date) != 8 {
		return "", "", "", "", "", ErrInvalidCredential
	}
	scope = strings.Join(parts[1:], "/")
	return accessKey, scope, date, region, service, nil
}

func parseAmzDateFromRequest(r *http.Request, fallback string) (time.Time, error) {
	s := strings.TrimSpace(r.Header.Get("X-Amz-Date"))
	if s == "" {
		s = strings.TrimSpace(r.Header.Get("Date"))
	}
	if s == "" && fallback != "" {
		s = fallback
	}
	if s == "" {
		return time.Time{}, errors.New("sigv4: missing X-Amz-Date/Date")
	}

	if t, err := parseAmzDate(s); err == nil {
		return t, nil
	}
	if t, err := http.ParseTime(s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("sigv4: invalid date format")
}

func parseAmzDate(s string) (time.Time, error) {
	t, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("sigv4: invalid X-Amz-Date: %w", err)
	}
	return t.UTC(), nil
}

func normalizeAndValidateSignedHeaders(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, ErrInvalidSignedHeaders
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, p := range parts {
		h := strings.ToLower(strings.TrimSpace(p))
		if h == "" {
			return nil, ErrInvalidSignedHeaders
		}
		if h == "authorization" {
			return nil, fmt.Errorf("%w: authorization cannot be signed", ErrInvalidSignedHeaders)
		}
		for i := 0; i < len(h); i++ {
			c := h[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return nil, fmt.Errorf("%w: invalid header name %q", ErrInvalidSignedHeaders, h)
			}
		}
		if _, ok := seen[h]; ok {
			return nil, fmt.Errorf("%w: duplicate header %q", ErrInvalidSignedHeaders, h)
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}

	sorted := append([]string(nil), out...)
	sort.Strings(sorted)
	if strings.Join(sorted, ";") != strings.Join(out, ";") {
		return nil, fmt.Errorf("%w: SignedHeaders must be sorted", ErrInvalidSignedHeaders)
	}
	return out, nil
}

func payloadHashForHeaderAuth(r *http.Request) (string, error) {
	if h := strings.TrimSpace(r.Header.Get("X-Amz-Content-Sha256")); h != "" {
		return h, nil
	}

	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		empty := sha256.Sum256(nil)
		return hex.EncodeToString(empty[:]), nil
	}
	if r.ContentLength < 0 {
		var b [1]byte
		n, err := r.Body.Read(b[:])
		if n == 0 && err == io.EOF {
			empty := sha256.Sum256(nil)
			return hex.EncodeToString(empty[:]), nil
		}
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("sigv4: read body prefix: %w", err)
		}
	}
	return "", ErrMissingPayloadHash
}

func buildStringToSign(algorithm string, amzDate time.Time, scope string, canonicalRequest string) string {
	crHash := sha256.Sum256([]byte(canonicalRequest))
	var sb strings.Builder
	sb.WriteString(algorithm)
	sb.WriteByte('\n')
	sb.WriteString(amzDate.UTC().Format("20060102T150405Z"))
	sb.WriteByte('\n')
	sb.WriteString(scope)
	sb.WriteByte('\n')
	sb.WriteString(hex.EncodeToString(crHash[:]))
	return sb.String()
}

func computeSignature(secretKey, dateYYYYMMDD, region, service, stringToSign string) string {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateYYYYMMDD))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	sig := hmacSHA256(kSigning, []byte(stringToSign))
	return hex.EncodeToString(sig)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func constantTimeHexEquals(aHex, bHex string) bool {
	a := strings.ToLower(strings.TrimSpace(aHex))
	b := strings.ToLower(strings.TrimSpace(bHex))
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
