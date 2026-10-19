// Package webhook HTTP回调处理
package webhook

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/interaction/signature"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/token"
)

type ack struct {
	Op   dto.OPCode `json:"op"`
	Data uint32     `json:"d"`
}

// GenHeartbeatACK 生成 http gateway 的心跳回包
func GenHeartbeatACK(seq uint32) string {
	s, _ := json.Marshal(ack{Op: dto.WSHeartbeatAck, Data: seq})
	return string(s)
}

// GenDispatchACK 生成事件包的回包，如果处理失败，则返回的 d 为 1，服务端会尝试重试
func GenDispatchACK(success bool) string {
	var r uint32
	if !success {
		r = 1
	}
	s, _ := json.Marshal(ack{Op: dto.HTTPCallbackAck, Data: r})
	return string(s)
}

// Deprecated: DefaultGetSecretFunc 默认的获取 secret 的函数，默认从环境变量读取
// 开发者如果需要从自己的配置文件，或者是其他地方获取 secret，可以重写这个函数
var DefaultGetSecretFunc = func() string {
	return os.Getenv("QQBotSecret")
}

// HTTPHandler 用户处理回调事件，该函数实现的是 https://pkg.go.dev/net/http#HandleFunc
//
// 保持旧签名不变。内部使用默认配置：
//   - 同步 ACK
//   - 不去重
//
// 如果需要去重、异步 ACK、请求大小限制等能力，请使用 NewHandler 或 webhook.Server。
func HTTPHandler(w http.ResponseWriter, r *http.Request, credentials *token.QQBotCredentials) {
	NewHandler(HandlerOptions{
		Credentials: credentials,
	}).ServeHTTP(w, r)
}

// GenValidationACK 生成回调校验回包
func GenValidationACK(req *dto.WHValidationReq, header http.Header, secret string) []byte {
	h := header.Clone()
	h.Set(signature.HeaderTimestamp, req.EventTs)
	sig, err := signature.Generate(secret, h, []byte(req.PlainToken))
	if err != nil {
		log.Errorf("generate signature failed:%+v", err)
		return nil
	}
	rsp, err := json.Marshal(
		&dto.WHValidationRsp{
			PlainToken: req.PlainToken,
			Signature:  sig,
		})
	if err != nil {
		log.Errorf("handle validation failed:", err)
		return nil
	}
	return rsp
}
