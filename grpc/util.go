package grpc

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/rs/zerolog"

	"github.com/ggsrc/gglib/interceptor/grpc/metautils"
	"github.com/ggsrc/gglib/interceptor/metadata"
)

var (
	// CReg credID symbol regexp to find C_123.
	CReg *regexp.Regexp = regexp.MustCompile(`C_\d+`)
	// NReg number regexp to find number inside credential id.
	NReg *regexp.Regexp = regexp.MustCompile(`\d+`)
)

// DeviceType is the kind of device a request comes from.
type DeviceType = string

// Device types returned by GetDeviceType.
const (
	UNKNOWN       DeviceType = ""
	MOBILEANDROID DeviceType = "Mobile_android"
	MOBILEIPHONE  DeviceType = "Mobile_iphone"
	MOBILEOTHER   DeviceType = "Mobile_other"
	TABLETIPAD    DeviceType = "Tablet_ipad"
	TABLETOTHER   DeviceType = "Tablet_other"
	WEB           DeviceType = "Web"
	TV            DeviceType = "TV"
)

// GetDeviceType classifies a User-Agent header value.
func GetDeviceType(userAgentHeader string) DeviceType {
	if isUserAgent(userAgentHeader, "Android") {
		return MOBILEANDROID
	}
	if isUserAgent(userAgentHeader, "iPhone") {
		return MOBILEIPHONE
	}
	if isUserAgent(userAgentHeader, "webOS", "BlackBerry", "Windows Phone") {
		return MOBILEOTHER
	}
	if isUserAgent(userAgentHeader, "iPad", "iPod") {
		return TABLETIPAD
	}
	if isUserAgent(userAgentHeader, "tablet", "RX-34", "FOLIO") ||
		(isUserAgent(userAgentHeader, "Kindle", "Mac OS") && isUserAgent(userAgentHeader, "Silk")) ||
		(isUserAgent(userAgentHeader, "AppleWebKit") && isUserAgent(userAgentHeader, "Silk")) {
		return TABLETOTHER
	}
	if isUserAgent(userAgentHeader, "TV", "NetCast", "boxee", "Kylo", "Roku", "DLNADOC") {
		return TV
	}

	return WEB
}

func isUserAgent(userAgent string, userAgents ...string) bool {
	for _, v := range userAgents {
		if strings.Contains(userAgent, v) {
			return true
		}
	}
	return false
}

// GetIPFromRequest returns the first address in X-Forwarded-For, or the
// remote address when the header is absent. The header is trusted as is.
func GetIPFromRequest(r *http.Request) (string, error) {
	var ip string
	var err error
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ", ")
		ip = ips[0]
	} else {
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "", err
		}
	}
	if !isIP(ip) {
		// TODO: error handling improvement
		return "", fmt.Errorf("not a valid ip: %s", ip)
	}
	return ip, nil
}

func isIP(ipstr string) (b bool) {
	ip := net.ParseIP(ipstr)
	return ip != nil
}

// GetCredIDsFromFormula returns the numbers of all "C_<n>" tokens in formula,
// in order.
func GetCredIDsFromFormula(formula string) []int64 {
	credIDs := []int64{}
	var credID int64

	cRegFound := CReg.FindString(formula)
	for cRegFound != "" {
		nRegFound := NReg.FindString(cRegFound)
		credID, _ = strconv.ParseInt(nRegFound, 10, 64)
		credIDs = append(credIDs, credID)
		formula = strings.Replace(formula, cRegFound, "dummy", 1)
		// search next credID symbol
		cRegFound = CReg.FindString(formula)
	}

	return credIDs
}

// GetNumericHash returns the 32-bit FNV-1a hash of s.
func GetNumericHash(s string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(s)) // hash.Hash.Write never returns an error

	return hash.Sum32()
}

// GetMetadata returns the incoming gRPC metadata value under
// metadata.CTX_KEY_METADATA.
func GetMetadata(ctx context.Context) string {
	md := metautils.ExtractIncoming(ctx)
	return md.Get(metadata.CTX_KEY_METADATA)
}

// InterceptorLogger adapts zerolog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func InterceptorLogger(l zerolog.Logger) logging.Logger {
	return logging.LoggerFunc(
		func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
			l := l.With().Fields(fields).Logger()
			switch lvl {
			case logging.LevelDebug:
				l.Debug().Msg(msg)
			case logging.LevelInfo:
				l.Info().Msg(msg)
			case logging.LevelWarn:
				l.Warn().Msg(msg)
			case logging.LevelError:
				l.Error().Msg(msg)
			default:
				panic(fmt.Sprintf("unknown level %v", lvl))
			}
		},
	)
}
