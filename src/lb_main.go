package main

import (
	"common"
	"encoding/json"
	"fmt"
	"golb"
	"net/http"

	pb "proto/greeter"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func AddHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if host, ok := r.Form["host"]; !ok || len(host) <= 0 {
		w.Write([]byte("invalid request."))
		return
	}
	host := r.Form["host"][0]

	golb.AddWorker(golb.Worker{Host: host})

	regResp := common.CommonResp{
		Code:    common.REG_WORKER_OK,
		Message: "ok",
	}
	respBody, err := json.Marshal(regResp)
	if err != nil {
		w.Write([]byte("internel server error."))
		return
	}
	fmt.Printf("Suc to register worker: %s\n", host)
	w.Write(respBody)
}

func RemoveHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if host, ok := r.Form["host"]; !ok || len(host) <= 0 {
		w.Write([]byte("invalid request."))
		return
	}
	host := r.Form["host"][0]

	golb.RemoveWorker(golb.Worker{Host: host})

	regResp := common.CommonResp{
		Code:    common.REG_WORKER_OK,
		Message: "ok",
	}
	respBody, err := json.Marshal(regResp)
	if err != nil {
		w.Write([]byte("internel server error."))
		return
	}
	w.Write(respBody)
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("To call SayHello.")
	workers := golb.Workers()
	if len(workers) <= 0 {
		w.Write([]byte("out of service."))
		return
	}
	conn, err := grpc.Dial(workers[0].Host, grpc.WithInsecure())
	if err != nil {
		fmt.Println("grpc call failed.")
		w.Write([]byte("internel server error."))
		return
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)
	reqbody := pb.HelloRequest{
		Name:    "xiaomo",
		Message: "just4fun",
	}
	resp, err := client.SayHello(context.Background(), &reqbody)
	if err != nil {
		fmt.Println("call sayhello failed.")
		w.Write([]byte("internel server error."))
		return
	}
	w.Write([]byte(resp.Message))
}

func main() {
	http.HandleFunc("/add", AddHandler)
	http.HandleFunc("/remove", RemoveHandler)
	http.HandleFunc("/hello", HelloHandler)
	fmt.Println("to start server on port: 8088")
	http.ListenAndServe(":8088", nil)
}
