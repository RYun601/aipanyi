package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

// asyncMethods 调用后不立即返回、结果经事件通道推送的方法白名单。
//
// 这些方法在 Wails 下同样是"发起即返回"（内部 goroutine 流式推送），
// 此处必须保持一致，否则 HTTP 请求会一直悬挂到任务结束。
var asyncMethods = map[string]bool{
	"ChatWithAgent":     true,
	"ChatWithAgentKBQA": true,
	"CheckUpdate":       true,
	"DownloadUpdate":    true,
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

type callRequest struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

type callResponse struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// handleCall 通用绑定派发器：POST /api/call {"method":"GetAiConfigs","args":[]}
//
// 参数按方法声明的类型反射构造后 json.Unmarshal，支持字符串/数值/结构体/指针/*int；
// 返回值约定与 Wails 对齐：最后一个返回值为 error 且非 nil 时返回 {"error": "..."}，
// 前端 shim 据此抛出异常，保持既有 try/catch 语义。
func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	var req callRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body: "+err.Error())
		return
	}
	if req.Method == "" {
		writeError(w, http.StatusBadRequest, "missing method")
		return
	}

	mv := reflect.ValueOf(s.app).MethodByName(req.Method)
	if !mv.IsValid() {
		writeError(w, http.StatusNotFound, "method not found: "+req.Method)
		return
	}
	mt := mv.Type()
	if len(req.Args) != mt.NumIn() {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("method %s expects %d args, got %d", req.Method, mt.NumIn(), len(req.Args)))
		return
	}

	in := make([]reflect.Value, 0, len(req.Args))
	for i, raw := range req.Args {
		pv := reflect.New(mt.In(i))
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, pv.Interface()); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("arg %d decode failed: %v", i, err))
				return
			}
		}
		in = append(in, pv.Elem())
	}

	if asyncMethods[req.Method] {
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					s.logger.Printf("async call %s panic: %v", req.Method, rec)
				}
			}()
			mv.Call(in)
		}()
		writeJSON(w, http.StatusOK, callResponse{Data: "accepted"})
		return
	}

	writeJSON(w, http.StatusOK, s.invoke(mv, in))
}

// invoke 同步调用并把结果/错误转为 callResponse。
func (s *Server) invoke(mv reflect.Value, in []reflect.Value) (resp callResponse) {
	defer func() {
		if rec := recover(); rec != nil {
			s.logger.Printf("call panic: %v", rec)
			resp = callResponse{Error: fmt.Sprintf("panic: %v", rec)}
		}
	}()

	for _, rv := range mv.Call(in) {
		if rv.Type().Implements(errorType) {
			if !rv.IsNil() {
				resp.Error = rv.Interface().(error).Error()
				return resp
			}
			continue
		}
		resp.Data = rv.Interface()
	}
	return resp
}
