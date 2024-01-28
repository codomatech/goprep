package main

import (
	"errors"
	"fmt"
	"go.starlark.net/starlark"
	"net/http"
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
	thread := &starlark.Thread{
		Name:  fmt.Sprintf("thead: %s", filePath),
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}
	script, err := starlark.ExecFile(thread, filePath, nil, nil)
	if err != nil {
		return nil, err
	}

	if !script.Has("modify") {
		return nil, errors.New("no `modify` function defined")
	}

	return &ProxyScript{filePath, script, thread}, nil
}

func httpRequestToMap(request *http.Request, reqData map[string]string) {

	reqData["Method"] = request.Method
	reqData["URL"] = request.URL.String()
	reqData["Proto"] = request.Proto
	reqData["ProtoMajor"] = fmt.Sprintf("%d", request.ProtoMajor)
	reqData["ProtoMinor"] = fmt.Sprintf("%d", request.ProtoMinor)

	for key, values := range request.Header {
		reqData[fmt.Sprintf("Header_%s", key)] = values[0]
	}

	reqData["ContentLength"] = fmt.Sprintf("%d", request.ContentLength)
	reqData["TransferEncoding"] = fmt.Sprintf("%v", request.TransferEncoding)
	reqData["Form"] = fmt.Sprintf("%v", request.Form)
	reqData["PostForm"] = fmt.Sprintf("%v", request.PostForm)
	reqData["MultipartForm"] = fmt.Sprintf("%v", request.MultipartForm)
	reqData["Trailer"] = fmt.Sprintf("%v", request.Trailer)
	reqData["RemoteAddr"] = request.RemoteAddr
	reqData["RequestURI"] = request.RequestURI
	// Host carries the proxied server address, scripts should use the Host header
	reqData["Host"] = request.Host
	reqData["TLSEnabled"] = "false"
	if request.TLS != nil {
		reqData["TLSEnabled"] = "true"
	}
}

func httpResponseToMap(resp *http.Response, respData map[string]string) {

	respData["Status"] = resp.Status
	respData["StatusCode"] = fmt.Sprintf("%d", resp.StatusCode)
	respData["Proto"] = resp.Proto
	respData["ProtoMajor"] = fmt.Sprintf("%d", resp.ProtoMajor)
	respData["ProtoMinor"] = fmt.Sprintf("%d", resp.ProtoMinor)

	for key, values := range resp.Header {
		respData[fmt.Sprintf("Header_%s", key)] = values[0]
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
			logger.Info("error making dict: %+v\n", err)
			panic("starlak VM is unusable, aborting")
		}
	}
	return dict
}
