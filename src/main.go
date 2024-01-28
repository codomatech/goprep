package main

import (
	"bytes"
	_ "encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/lmittmann/tint"
	"go.starlark.net/starlark"
)

var logger = slog.New(tint.NewHandler(os.Stderr, nil))

var scripts []*ProxyScript

func main() {
	// setup the environment
	//

	sourceURL := os.Getenv("SOURCE_URL")

	source, err := url.Parse(sourceURL)

	if err != nil {
		logger.Info("Invalid SOURCE_URL", "source_url", sourceURL)
		return
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 6350
	}

	scriptsDir := os.Getenv("SCRIPTS_DIR")
	if len(scriptsDir) == 0 {
		scriptsDir = "/etc/goprep/scripts/"
	}
	if !strings.HasSuffix(scriptsDir, "/") {
		scriptsDir += "/"
	}

	fObjs, err := os.ReadDir(scriptsDir)
	if err != nil || len(fObjs) == 0 {
		logger.Error("No scripts found", "scripts_dir", scriptsDir)
		return
	}

	files := make([]string, len(fObjs))

	for _, obj := range fObjs {
		files = append(files, obj.Name())
	}
	sort.Strings(files)

	scripts = make([]*ProxyScript, 0, len(files))

	for _, fname := range files {
		if len(fname) == 0 {
			continue
		}
		if strings.HasSuffix(fname, ".star") {
			filePath := scriptsDir + fname

			compiled, err := loadStarlakScript(filePath)
			if err != nil {
				logger.Error("Error loading or compiling script", "file_path", filePath, "error", err)
				continue
			}

			scripts = append(scripts, compiled)
			logger.Info("Script loaded and compiled successfully", "file_path", filePath)
		} else {
			logger.Debug("Skipping file (not a .star file)", "file_path", fname)
		}
	}

	// setup the proxy
	//

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(source)
			r.Out.Host = r.In.Host // if desired
		},
	}

	//proxy := httputil.NewSingleHostReverseProxy(source)

	proxy.ModifyResponse = modifyResponse

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// run
	//

	logger.Info("listening to requests", "port", port)

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		logger.Error("error starting server", "error", err)
	}
}

func clearMap(m map[string]string) {
	for k := range m {
		delete(m, k)
	}
}

func modifyResponse(resp *http.Response) (err error) {

	var body string = ""
	bodyIsSet := false
	reqData := make(map[string]string)
	respData := make(map[string]string)

	headerOverride := make(map[string]string)

	clearMap(reqData)
	clearMap(respData)
	httpRequestToMap(resp.Request, reqData)
	httpResponseToMap(resp, respData)

	//origBody := make([]byte, 0)
	origBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	respData["Body"] = string(origBody)
	for _, script := range scripts {

		r, err := script.ExecuteModifyFunction(reqData, respData)
		if err != nil {
			logger.Error("Error executing script", "error", err)
			continue
		}
		result := r.(*starlark.Dict)

		bodyValue, found, _ := result.Get(starlark.String("body"))
		if found {
			bodyIsSet = true
			body, _ = strconv.Unquote(bodyValue.String())
		}

		headersValue, found, _ := result.Get(starlark.String("headers"))
		if found {
			//logger.Info("found headers: %+v", headersValue)
			headers := headersValue.(*starlark.Dict)
			for _, item := range headers.Items() {
				k, _ := strconv.Unquote(item.Index(0).String())
				v, _ := strconv.Unquote(item.Index(1).String())
				headerOverride[k] = v
			}
		}
	}

	for k, v := range headerOverride {
		logger.Debug("setting header", "name", k, "value", v)
		resp.Header.Set(k, v)
		resp.Header.Set(k, v)
	}
	//logger.Info("response headers now are: %+v", resp.Header)

	if bodyIsSet {
		resp.Body = io.NopCloser(bytes.NewReader([]byte(body)))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	} else {
		resp.Body = io.NopCloser(bytes.NewReader([]byte(origBody)))
	}

	return nil
}
