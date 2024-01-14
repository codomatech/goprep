package main

import (
	"bytes"
	_ "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"go.starlark.net/starlark"
)

type ProxyScript struct {
	FilePath string
	Script   starlark.StringDict
	Thread   *starlark.Thread
}

func (script *ProxyScript) ExecuteModifyFunction(reqData map[string]string, respData map[string]string) (starlark.Value, error) {
	arg1 := map2dict(reqData)
	arg2 := map2dict(respData)
	result, err := starlark.Call(script.Thread, script.Script["modify"], starlark.Tuple{arg1, arg2}, nil)

	if err != nil {
		return nil, err
	}
	return result, err
}

func loadStarlakScript(filePath string) (*ProxyScript, error) {
	thread := &starlark.Thread{Name: fmt.Sprintf("thead: %s", filePath)}
	script, err := starlark.ExecFile(thread, filePath, nil, nil)
	if err != nil {
		return nil, err
	}

	if !script.Has("modify") {
		return nil, errors.New("no `modify` function defined")
	}

	return &ProxyScript{filePath, script, thread}, nil
}

var scripts []*ProxyScript

func main() {
	// setup the environment
	//

	sourceURL := os.Getenv("SOURCE_URL")

	source, err := url.Parse(sourceURL)

	if err != nil {
		fmt.Printf("Invalid SOURCE_URL: `%s`", sourceURL)
		return
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		fmt.Printf("Please set the environment variable PORT to a valid integer")
		return
	}

	scriptsDir := os.Getenv("SCRIPTS_DIR")
	if len(scriptsDir) == 0 {
		scriptsDir = "/etc/goprep/scripts"
	}
	fObjs, err := os.ReadDir(scriptsDir)
	if err != nil || len(fObjs) == 0 {
		fmt.Printf("No scripts found in `%s`", scriptsDir)
		return
	}

	files := make([]string, len(fObjs))

	for _, obj := range fObjs {
		files = append(files, obj.Name())
	}
	sort.Strings(files)

	scripts = make([]*ProxyScript, len(files))

	for _, fname := range files {
		if strings.HasSuffix(fname, ".star") {
			filePath := scriptsDir + "/" + fname

			compiled, err := loadStarlakScript(filePath)
			if err != nil {
				fmt.Printf("Error loading or compiling script %s: %v\n", filePath, err)
				continue
			}

			scripts = append(scripts, compiled)
			fmt.Printf("Script %s loaded and compiled successfully\n", filePath)
		} else {
			fmt.Printf("Skipping file %s (not a .star file)\n", fname)
		}
	}

	// setup the proxy
	//

	proxy := httputil.NewSingleHostReverseProxy(source)

	proxy.ModifyResponse = modifyResponse

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// run
	//

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Println(err)
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
	for _, script := range scripts {
		clearMap(reqData)
		clearMap(respData)
		httpRequestToMap(resp.Request, reqData)
		httpResponseToMap(resp, respData)

		r, err := script.ExecuteModifyFunction(reqData, respData)
		if err != nil {
			// TODO report
			continue
		}
		result := r.(*starlark.Dict)

		bodyValue, found, _ := result.Get(starlark.String("body"))
		if found {
			bodyIsSet = true
			body = bodyValue.String()
		}

		headersValue, found, _ := result.Get(starlark.String("headers"))
		if found {
			headers := headersValue.(*starlark.Dict)
			for _, item := range headers.Items() {
				k := item.Index(0).String()
				v := item.Index(1).String()
				resp.Header.Set(k, v)
			}
		}
	}
	if bodyIsSet {
		resp.Body = io.NopCloser(bytes.NewReader([]byte(body)))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}

	return nil
}

func httpRequestToMap(request *http.Request, reqData map[string]string) {

	reqData["Method"] = request.Method
	reqData["URL"] = request.URL.String()
	reqData["Proto"] = request.Proto
	reqData["ProtoMajor"] = fmt.Sprintf("%d", request.ProtoMajor)
	reqData["ProtoMinor"] = fmt.Sprintf("%d", request.ProtoMinor)

	for key, values := range request.Header {
		reqData[fmt.Sprintf("Header_%s", key)] = fmt.Sprintf("%v", values)
	}

	reqData["ContentLength"] = fmt.Sprintf("%d", request.ContentLength)
	reqData["TransferEncoding"] = fmt.Sprintf("%v", request.TransferEncoding)
	reqData["Host"] = request.Host
	reqData["Form"] = fmt.Sprintf("%v", request.Form)
	reqData["PostForm"] = fmt.Sprintf("%v", request.PostForm)
	reqData["MultipartForm"] = fmt.Sprintf("%v", request.MultipartForm)
	reqData["Trailer"] = fmt.Sprintf("%v", request.Trailer)
	reqData["RemoteAddr"] = request.RemoteAddr
	reqData["RequestURI"] = request.RequestURI
	reqData["TLS"] = fmt.Sprintf("%v", request.TLS)
}

func httpResponseToMap(resp *http.Response, respData map[string]string) {

	respData["Status"] = resp.Status
	respData["StatusCode"] = fmt.Sprintf("%d", resp.StatusCode)
	respData["Proto"] = resp.Proto
	respData["ProtoMajor"] = fmt.Sprintf("%d", resp.ProtoMajor)
	respData["ProtoMinor"] = fmt.Sprintf("%d", resp.ProtoMinor)

	for key, values := range resp.Header {
		respData[fmt.Sprintf("Header_%s", key)] = fmt.Sprintf("%v", values)
	}

	respData["ContentLength"] = fmt.Sprintf("%d", resp.ContentLength)
	respData["TransferEncoding"] = fmt.Sprintf("%v", resp.TransferEncoding)
	respData["Uncompressed"] = fmt.Sprintf("%v", resp.Uncompressed)
	respData["Request"] = fmt.Sprintf("%v", resp.Request)
	respData["TLS"] = fmt.Sprintf("%v", resp.TLS)
}

func map2dict(goMap map[string]string) *starlark.Dict {
	var err error
	dict := starlark.NewDict(len(goMap))
	for key, value := range goMap {
		k := starlark.String(key)
		v := starlark.String(value)
		err = dict.SetKey(k, v)
		if err != nil {
			fmt.Printf("error making dict: %+v\n", err)
			panic("starlak VM is unusable, aborting")
		}
	}
	return dict
}
