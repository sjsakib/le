package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.sakib.dev/le/pkg/cfg"
	"go.sakib.dev/le/pkg/utils"
	"go.sakib.dev/le/pkg/zip"
)

const downloadProgressLogInterval = 500 * time.Millisecond // Log download progress every 500 milliseconds

type handler struct {
	defaultServer http.Handler
	root          http.Dir
	ch            chan<- ServerEvent
	isStaticSite  bool
}

func newHandler(c *cfg.Config, ch chan<- ServerEvent) (http.Handler, error) {
	isStaticSite := false

	if _, err := os.Stat(path.Join(c.Dir, "index.html")); err == nil && c.StaticSiteMode == cfg.StaticSiteModeAuto {
		slog.Info("index.html detected, starting static site mode")
		isStaticSite = true
	}

	h := &handler{
		defaultServer: http.FileServer(http.Dir(c.Dir)),
		root:          http.Dir(c.Dir),
		ch:            ch,
		isStaticSite:  isStaticSite,
	}
	return h, nil
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	reqHelper := newReqHelper(w, r, h.ch)

	reqHelper.attachReqId()

	if h.isStaticSite {
		slog.InfoContext(reqHelper.ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		h.defaultServer.ServeHTTP(w, r)
		return
	}

	clientIP, err := utils.GetClientIP(r)
	if err != nil {
		slog.Warn("Failed to get client IP", "error", err)
		clientIP = "unknown"
	}
	reqHelper.clientIP = clientIP

	clientHost, err := utils.GetClientHostname(r)
	if err != nil {
		clientHost = "unknown"
	}
	reqHelper.clientHost = clientHost

	slog.InfoContext(reqHelper.ctx, "REQUEST",
		"clientIP", clientIP,
		"clientHost", clientHost,
		"userAgent", r.UserAgent(),
		"method", r.Method,
		"path", r.URL.Path)

	defer reqHelper.publishConnClose()

	isHead := r.Method == http.MethodHead

	if r.Method != http.MethodGet && !isHead {
		reqHelper.error("Method Not Allowed", nil, http.StatusMethodNotAllowed)
		return
	}

	absPath, err := utils.SecureJoin(string(h.root), r.URL.Path)
	reqHelper.absPath = absPath

	slog.Debug("Secure Join", "path", absPath, "root", string(h.root), "path", r.URL.Path, "error", err)

	if errors.Is(err, utils.ErrForbiddenPath) {
		reqHelper.error("NOT FOUND", err, http.StatusNotFound)
		return
	} else if err != nil {
		reqHelper.internalServerError(err)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			reqHelper.error("NOT FOUND", err, http.StatusNotFound)
			return
		}
		reqHelper.internalServerError(err)
		return
	}

	isArchive := r.URL.Query().Get("archive") == "true"

	if info.IsDir() && !isArchive {
		// check if request is coming from a browser
		acceptHeader := r.Header.Get("Accept")
		isBrowser := strings.Contains(acceptHeader, "text/html")

		if isBrowser {
			slog.InfoContext(reqHelper.ctx, "OK - Serving directory with pretty UI", "path", r.URL.Path)
			h.serveDirectory(w, r, absPath)
		} else {
			slog.InfoContext(reqHelper.ctx, "OK - Serving directory default file server", "path", r.URL.Path)
			h.defaultServer.ServeHTTP(w, r)
		}
		return
	}

	var source downloadSource

	if isArchive && info.IsDir() {
		source = zip.New(absPath, r.URL.Query().Get("compressed") == "true")
	} else {
		file, err := os.Open(absPath)
		if err != nil {
			reqHelper.internalServerError(err)
			return
		}
		defer file.Close()

		source = &fileSource{file, info}

	}

	reqHelper.serveSource(source)
}
