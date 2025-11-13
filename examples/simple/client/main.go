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

	// --- 1. 使用默认的自增 ID (简单模式) ---
	log.Println("\n--- 1. Calling Arith.Ping with auto-incrementing ID ---")
	var pingReply string
	// 使用简单的 Call 方法，ID 会自动生成
	err = client.Call("Arith.Ping", nil, &pingReply)
	if err != nil {
		log.Printf("Arith.Ping error: %v", err)
	} else {
		log.Printf("Arith.Ping reply: %s", pingReply)
	}

	// --- 2. 使用自定义的字符串 ID (高级模式) ---
	log.Println("\n--- 2. Calling Arith.Add with custom UUID ID ---")
	addParams := &ArithParamsWithToken{
		A:     10,
		B:     5,
		Token: "secret-token",
	}
	var addReply int
	// 生成一个 UUID 作为自定义 ID
	traceID := uuid.New().String()
	log.Printf("Using custom trace ID: %s", traceID)
	// 使用新的 CallWithID 方法
	err = client.CallWithID(traceID, "Arith.Add", addParams, &addReply)
	if err != nil {
		log.Printf("FAILURE: Arith.Add with custom ID failed: %v", err)
	} else {
		log.Printf("SUCCESS: Arith.Add with custom ID reply: %d", addReply)
	}

	// --- 3. 再次使用默认的自增 ID，验证序列号继续增长 ---
	log.Println("\n--- 3. Calling Arith.Ping again to check sequence ---")
	err = client.Call("Arith.Ping", nil, &pingReply)
	if err != nil {
		log.Printf("Arith.Ping error: %v", err)
	} else {
		log.Printf("Arith.Ping reply: %s", pingReply)
	}
}
