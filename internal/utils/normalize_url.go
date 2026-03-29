package utils

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"
)

func NormalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)

	if err != nil {
		return "", fmt.Errorf("could not parse URL : %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("URL has invalid scheme")
	}

	parsed.Fragment = ""

	if parsed.Host == "" {
		return "", fmt.Errorf("URL has no field 'Host'")
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if port := parsed.Port(); port != "" {
		parsed.Host = net.JoinHostPort(host, port)
	} else {
		parsed.Host = host
	}

	cleanPath := path.Clean(parsed.Path)
	if cleanPath == "." {
		cleanPath = ""
	}
	parsed.Path = strings.TrimSuffix(cleanPath, "/")
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.RawPath = ""

	return parsed.String(), nil

}
