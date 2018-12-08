package dlx

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/nuclio/logger"
)

type Handler struct {
	logger          logger.Logger
	handleFunc      func(http.ResponseWriter, *http.Request)
	functionStarter *FunctionStarter
}

func NewHandler(logger logger.Logger, functionStarter *FunctionStarter) (Handler, error) {
	h := Handler{
		logger:          logger,
		functionStarter: functionStarter,
	}
	h.handleFunc = h.handleRequest
	return h, nil
}

func (h *Handler) handleRequest(res http.ResponseWriter, req *http.Request) {
	var targetURL *url.URL
	var err error
	var functionName string

	responseChannel := make(chan FunctionStatusResult, 1)
	defer close(responseChannel)

	// first try to see if our request came from ingress controller
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	forwardedPort := req.Header.Get("X-Forwarded-Port")
	originalURI := req.Header.Get("X-Original-Uri")
	functionName = req.Header.Get("X-Service-Name")

	if forwardedHost != "" && forwardedPort != "" && functionName != "" {
		targetURL, err = url.Parse(fmt.Sprintf("http://%s:%s/%s", forwardedHost, forwardedPort, originalURI))
		if err != nil {
			h.logger.Warn("Failed to parse url for function",
				"functionName", functionName,
				"err", err)
			res.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		functionName := req.Header.Get("X-nuclio-target")
		if functionName == "" {
			h.logger.Warn("When ingress not set, must pass X-nuclio-target header value")
			res.WriteHeader(http.StatusBadRequest)
			return
		}
		targetURL, err = url.Parse(fmt.Sprintf("http://%s:8080", functionName))
		if err != nil {
			h.logger.Warn("Failed to parse url for function",
				"targetURL", targetURL,
				"err", err)
			res.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	h.functionStarter.handleFunctionStart(functionName, responseChannel)
	statusResult := <-responseChannel

	if statusResult.Error != nil {
		h.logger.WarnWith("Failed to forward request to function",
			"function", statusResult.FunctionName,
			"err", statusResult.Error)
		res.WriteHeader(statusResult.Status)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(res, req)
}
