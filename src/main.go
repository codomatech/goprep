// TODO use starlark instead of tengo
package main

import (
	"bytes"
	_ "encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/d5/tengo/v2"
)

func loadTengoScript(filePath string) (*tengo.Script, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	script := tengo.NewScript(content)
	_, err = script.Compile()
	if err != nil {
		return nil, err
	}

	return script, nil
}

var scripts []*tengo.Script

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
	fObjs, err := ioutil.ReadDir(scriptsDir)
	if err != nil || len(fObjs) == 0 {
		fmt.Printf("No scripts found in `%s`", scriptsDir)
		return
	}

	files := make([]string, len(fObjs))

	for _, obj := range fObjs {
		files = append(files, obj.Name())
	}
	sort.Strings(files)

	scripts = make([]*tengo.Script, len(files))

	for _, fname := range files {
		if strings.HasSuffix(fname, ".tengo") {
			filePath := scriptsDir + "/" + fname

			compiled, err := loadTengoScript(filePath)
			if err != nil {
				fmt.Printf("Error loading or compiling script %s: %v\n", filePath, err)
				continue
			}

			scripts = append(scripts, compiled)
			fmt.Printf("Script %s loaded and compiled successfully\n", filePath)
		} else {
			fmt.Printf("Skipping file %s (not a .tengo file)\n", fname)
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

		// TODO we need to clone script, otherwise wrong semantics with concurrent execution. script = script.Clone()
		script.Add("request", reqData)
		script.Add("response", respData)

		c, _ := script.Compile()

		err := c.Run()
		if err != nil {
			// TODO report
			continue
		}

		if c.IsDefined("body") {
			bodyIsSet = true
			body = c.Get("body").String()
		}

		if c.IsDefined("headers") {
			headers := c.Get("headers").Map()
			for k, v := range headers {
				resp.Header.Set(k, v.(string))
			}
		}
	}
	if bodyIsSet {
		resp.Body = ioutil.NopCloser(bytes.NewReader([]byte(body)))
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
