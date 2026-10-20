package openapi

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	"github.com/sealdice/botgo/log"
)

// 提供一组过滤器支持，开发者可以通过请求过滤器和返回过滤器，实现模调上报、耗时监控等能力。

// HTTPFilter 请求过滤器
type HTTPFilter func(req *http.Request, response *http.Response) error

var (
	filterLock         = sync.RWMutex{}
	reqFilterChainSet  = map[string]HTTPFilter{}
	reqFilterChains    []string
	respFilterChainSet = map[string]HTTPFilter{}
	respFilterChains   []string
)

// RegisterReqFilter 注册请求过滤器
func RegisterReqFilter(name string, filter HTTPFilter) {
	filterLock.Lock()
	defer filterLock.Unlock()
	if _, ok := reqFilterChainSet[name]; ok {
		return
	}
	reqFilterChainSet[name] = filter
	reqFilterChains = append(reqFilterChains, name)
}

// RegisterRespFilter 注册返回过滤器
func RegisterRespFilter(name string, filter HTTPFilter) {
	filterLock.Lock()
	defer filterLock.Unlock()
	if _, ok := respFilterChainSet[name]; ok {
		return
	}
	respFilterChainSet[name] = filter
	respFilterChains = append(respFilterChains, name)
}

// DoReqFilterChains 按照注册顺序执行请求过滤器
func DoReqFilterChains(req *http.Request, resp *http.Response) error {
	for _, entry := range snapshotFilters(&reqFilterChains, reqFilterChainSet) {
		if err := runHTTPFilter(entry.name, entry.fn, req, resp); err != nil {
			return err
		}
	}
	return nil
}

// DoRespFilterChains 按照注册顺序执行返回过滤器
func DoRespFilterChains(req *http.Request, resp *http.Response) error {
	for _, entry := range snapshotFilters(&respFilterChains, respFilterChainSet) {
		if err := runHTTPFilter(entry.name, entry.fn, req, resp); err != nil {
			return err
		}
	}
	return nil
}

type namedFilter struct {
	name string
	fn   HTTPFilter
}

func snapshotFilters(chain *[]string, set map[string]HTTPFilter) []namedFilter {
	filterLock.RLock()
	defer filterLock.RUnlock()
	names := *chain
	result := make([]namedFilter, 0, len(names))
	for _, name := range names {
		if fn, ok := set[name]; ok {
			result = append(result, namedFilter{name: name, fn: fn})
		}
	}
	return result
}

// runHTTPFilter 隔离用户过滤器 panic，避免一次过滤器故障导致消息处理协程崩溃。
func runHTTPFilter(name string, fn HTTPFilter, req *http.Request, resp *http.Response) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("filter %q panic: %v\n%s", name, r, debug.Stack())
			log.Errorf("[QQBot][openapi] %v", err)
		}
	}()
	return fn(req, resp)
}
