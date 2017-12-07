package golb

import (
	"logger"
	"net/http"
	pb "proto/greeter"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func InitHandlers() {
	http.HandleFunc("/hello", HelloHandler)
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	logger.LogDebug("To call SayHello.")
	worker, err := GetWorker()
	if err != nil {
		w.Write([]byte("out of service."))
		return
	}
	conn, err := grpc.Dial(worker.Host, grpc.WithInsecure())
	if err != nil {
		logger.LogError("grpc call failed.")
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
		logger.LogError("call sayhello failed.")
		w.Write([]byte("internel server error."))
		return
	}
	w.Write([]byte(resp.Message))
}
