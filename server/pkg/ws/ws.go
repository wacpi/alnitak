package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"interastral-peace.com/alnitak/utils"
)

type removeWsConn func(id, groupId interface{})

// 判断是否为正常的WebSocket连接断开错误
func isNormalCloseError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 检查常见的正常断开错误
	normalErrors := []string{
		"wsasend: An established connection was aborted by the software in your host machine",
		"connection reset by peer",
		"broken pipe",
		"use of closed network connection",
		"websocket: close sent",
	}

	for _, normalErr := range normalErrors {
		if contains(errStr, normalErr) {
			return true
		}
	}
	return false
}

// 简单的字符串包含检查
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin: func(r *http.Request) bool { // 取消ws跨域校验
		return true
	},
}

// 创建websocket连接
func CreateWsConn(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return wsupgrader.Upgrade(w, r, nil)
}

// 处理ws请求
func WsHandler(conn *websocket.Conn, id, groupId interface{}, m chan interface{}, removeConn removeWsConn) {
	// 设置pong处理器,当收到pong消息时更新最后活跃时间
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(time.Second * 60))
		return nil
	})

	// 设置初始读取超时时间
	conn.SetReadDeadline(time.Now().Add(time.Second * 60))

	// 创建一个定时器用于服务端心跳
	pingTicker := time.NewTicker(time.Second * 30)
	defer pingTicker.Stop()

	// 启动一个goroutine来读取客户端消息(主要是为了处理pong响应)
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				conn.Close()
				return
			}
		}
	}()

	for {
		select {
		case content, ok := <-m:
			// 从消息通道接收消息，然后推送给前端
			conn.SetWriteDeadline(time.Now().Add(time.Second * 10))
			if err := conn.WriteJSON(content); err != nil {
				// 只有非正常断开才记录错误日志
				if !isNormalCloseError(err) {
					utils.ErrorLog("发送消息错误", "ws", err.Error())
				}
				if ok {
					go func() {
						m <- content
					}()
				}

				conn.Close()
				removeConn(id, groupId)
				return
			}
		case <-pingTicker.C:
			// 服务端心跳:每30秒ping一次客户端，查看其是否在线
			conn.SetWriteDeadline(time.Now().Add(time.Second * 10))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				// 只有非正常断开才记录错误日志
				if !isNormalCloseError(err) {
					utils.ErrorLog("发送ping失败", "ws", err.Error())
				}
				conn.Close()
				removeConn(id, groupId)
				return
			}
		}
	}
}
