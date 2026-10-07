package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func LoadWebOrigins() ([]string, error) {
	origins := []string{}
	for _, raw := range strings.Split(os.Getenv("WEB_ALLOWED_ORIGINS"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
			u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
			strings.ContainsAny(raw, "*?#\\ \t\r\n") || strings.HasSuffix(u.Host, ":") {
			return nil, fmt.Errorf("WEB_ALLOWED_ORIGINS must contain exact http/https origins without paths or wildcards")
		}
		if port := u.Port(); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return nil, fmt.Errorf("WEB_ALLOWED_ORIGINS contains an invalid port")
			}
			if (u.Scheme == "http" && n == 80) || (u.Scheme == "https" && n == 443) {
				u.Host = u.Hostname()
				if net.ParseIP(u.Host) != nil && strings.Contains(u.Host, ":") {
					u.Host = "[" + u.Host + "]"
				}
			}
		}
		origin := u.Scheme + "://" + strings.ToLower(u.Host)
		duplicate := false
		for _, existing := range origins {
			duplicate = duplicate || existing == origin
		}
		if !duplicate {
			origins = append(origins, origin)
		}
	}
	return origins, nil
}
