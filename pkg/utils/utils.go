package utils

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ContextKey string

const (
	RequestIDKey ContextKey = "reqId"
)

func GetLocalIP() (string, error) {
	// Connect to a dummy address; doesn't have to be reachable
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// GetClientIP does not consider reverse proxies or load balancers
func GetClientIP(r *http.Request) (string, error) {
	slog.Debug("Getting client IP", "remoteAddr", r.RemoteAddr)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func GetClientHostname(r *http.Request) (string, error) {
	ip, err := GetClientIP(r)

	if err != nil {
		return "", err
	}

	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		slog.Debug("Failed to get client hostname", "error", err)
		return ip, nil // fallback to IP if no hostname found
	}

	// names may contain trailing dot
	return strings.TrimSuffix(names[0], "."), nil
}

func ParseRangeHeader(header string, size int64) (start, end int64, err error) {
	if header == "" {
		return 0, 0, nil
	}
	re := regexp.MustCompile(`^bytes=(\d*)-(\d*)$`)

	matches := re.FindStringSubmatch(header)

	errMsg := fmt.Errorf("invalid range header: %q for size %d", header, size)

	if len(matches) < 3 || (matches[1] == "" && matches[2] == "") {
		return 0, 0, errMsg
	}

	if matches[1] != "" {
		start, _ = strconv.ParseInt(matches[1], 10, 64)
	}

	if matches[2] != "" {
		end, _ = strconv.ParseInt(matches[2], 10, 64)
	}

	if matches[1] == "" {
		start = size - end
		end = size - 1
	}

	if matches[2] == "" {
		end = size - 1
	}

	if start > end || end >= size {
		return 0, 0, errMsg
	}

	slog.Info("Parsed range header", "start", start, "end", end, "size", size)

	return start, end, nil
}

var ErrForbiddenPath = errors.New("forbidden path")

// SecureJoin ensures that the joined path is within the base directory
func SecureJoin(base, path string) (string, error) {
	targetPath := filepath.Join(base, path)

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}

	parentPath := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)

	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil && os.IsNotExist(err) {
		evalParent, err := filepath.EvalSymlinks(parentPath)
		if err == nil {
			absPath = filepath.Join(evalParent, fileName)
		}
	} else if err != nil {
		return "", err
	}

	absPath = filepath.Clean(absPath)

	// prevent prefix matching for path traversal
	if !strings.HasPrefix(absPath, base+(string(filepath.Separator))) && absPath != base {
		return "", ErrForbiddenPath
	}

	return absPath, nil
}

func ValidAbsDir(path string) (string, error) {
	path, err := filepath.Abs(path)

	if err != nil {
		return "", err
	}

	path, err = filepath.EvalSymlinks(path)

	if err != nil {
		return "", err
	}
	path = filepath.Clean(path)

	info, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", path)
		}
		return "", err
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", path)
	}

	return path, nil
}

// ReplaceHome replaces the home directory in the given path with a tilde (~).
func ReplaceHome(dir string) string {
	if home := os.Getenv("HOME"); home != "" {
		dir = strings.ReplaceAll(dir, home, "~")
	}
	return dir
}

func HumanizeSize(size int64) string {
	if size == 0 {
		return ""
	}

	const unit = 1024.0
	if size < unit {
		return ""
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	exp := 0
	val := float64(size)

	for val >= unit && exp < len(units)-1 {
		val /= unit
		exp++
	}

	if val < 10 {
		// cause sprintf would convert 1.0 to 1
		whole := int(val)
		frac := int((val - float64(whole)) * 10)
		if frac == 0 {
			return fmt.Sprintf("%d %s", whole, units[exp])
		}
		return fmt.Sprintf("%d.%d %s", whole, frac, units[exp])
	}

	return fmt.Sprintf("%d %s", int(val), units[exp])
}

func ThrottleC[T any](in <-chan T, t time.Duration) chan T {
	out := make(chan T)

	go func() {
		ticker := time.NewTicker(t)
		defer ticker.Stop()
		defer close(out)

		var latest T
		var hasLatest bool

		for {
			select {
			case <-ticker.C:
				if hasLatest {
					out <- latest
					hasLatest = false
				}
			case val, ok := <-in:
				if !ok {
					return
				}
				hasLatest = true
				latest = val
			}
		}
	}()

	return out
}
