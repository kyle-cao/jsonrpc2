package main

import (
	"github.com/kyle-cao/jsonrpc2"
	"log"
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

	var addReply int

	log.Println("\n--- Calling Arith.Add with correct token ---")
	addParamsSuccess := &ArithParamsWithToken{
		A:     10,
		B:     5,
		Token: "secret-token",
	}
	err = client.Call("Arith.Add", addParamsSuccess, &addReply, 5)
	if err != nil {
		log.Printf("FAILURE: Arith.Add failed unexpectedly: %v", err)
	} else {
		log.Printf("SUCCESS: Arith.Add reply: %d", addReply)
	}
}
