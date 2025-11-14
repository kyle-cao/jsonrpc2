package main

import (
	"log"

	"github.com/google/uuid"       // 引入 uuid 包
	"github.com/kyle-cao/jsonrpc2" // 确保这里的模块名正确
)

// ArithParamsWithToken 匹配服务端的参数结构
type ArithParamsWithToken struct {
	A     int    `json:"a"`
	B     int    `json:"b"`
	Token string `json:"token"`
}

func main() {
	client, err := jsonrpc2.Dial("localhost:8080")
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	addParams := &ArithParamsWithToken{
		A:     10,
		B:     5,
		Token: "secret-token",
	}
	var addReply int

	// --- 1. 使用默认的自增 ID (简单模式) ---
	err = client.Call("Arith.Add", addParams, &addReply, 5)
	if err != nil {
		log.Printf("FAILURE: Arith.Add failed: %v", err)
	} else {
		log.Printf("SUCCESS: Arith.Add reply: %d", addReply)
	}

	// --- 2. 使用自定义的字符串 ID (高级模式) ---

	// 生成一个 UUID 作为自定义 ID
	traceID := uuid.New().String()
	log.Printf("Using custom trace ID: %s", traceID)
	// 使用新的 CallWithID 方法
	err = client.CallWithID(traceID, "Arith.Add", addParams, &addReply, 5)
	if err != nil {
		log.Printf("FAILURE: Arith.Add with custom ID failed: %v", err)
	} else {
		log.Printf("SUCCESS: Arith.Add with custom ID reply: %d", addReply)
	}

	// --- 3. ping ---
	if client.Ping() {
		log.Println("Ping SUCCESS")
	} else {
		log.Println("Ping FAILURE")
	}
}
