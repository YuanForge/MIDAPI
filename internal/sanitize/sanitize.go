package sanitize

import (
	"fmt"
	"net/url"
	"strings"
)

var sensitiveNames = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"x-api-key":           true,
	"x-goog-api-key":      true,
	"api-key":             true,
	"apikey":              true,
	"key":                 true,
	"token":               true,
	"access_token":        true,
	"refresh_token":       true,
	"secret":              true,
	"password":            true,
	"cookie":              true,
	"set-cookie":          true,
}

func IsSensitiveName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if sensitiveNames[normalized] {
		return true
	}
	return strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "access_token") ||
		strings.Contains(normalized, "refresh_token")
}

func MaskString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

func RedactHeaderMap(headers map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(headers))
	for k, v := range headers {
		if IsSensitiveName(k) {
			out[k] = MaskString(fmt.Sprint(v))
			continue
		}
		out[k] = v
	}
	return out
}

func RedactStringHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if IsSensitiveName(k) {
			out[k] = MaskString(v)
			continue
		}
		out[k] = v
	}
	return out
}

func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return raw
	}
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	q := u.Query()
	changed := false
	for k, vals := range q {
		if !IsSensitiveName(k) {
			continue
		}
		for i := range vals {
			vals[i] = MaskString(vals[i])
		}
		q[k] = vals
		changed = true
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func RedactJSONMap(input map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(input))
	for k, v := range input {
		if IsSensitiveName(k) {
			out[k] = MaskString(fmt.Sprint(v))
			continue
		}
		switch vv := v.(type) {
		case map[string]interface{}:
			if strings.EqualFold(k, "_headers") {
				out[k] = RedactHeaderMap(vv)
			} else {
				out[k] = RedactJSONMap(vv)
			}
		case map[string]string:
			out[k] = RedactStringHeaders(vv)
		case string:
			if strings.EqualFold(k, "_url") || strings.Contains(strings.ToLower(k), "url") {
				out[k] = RedactURL(vv)
			} else {
				out[k] = vv
			}
		default:
			out[k] = vv
		}
	}
	return out
}
