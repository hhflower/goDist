package main

import (
	"github.com/moxiaomomo/goDist/api/handler"
)

// func Test_apiServer(t *testing.T) {
// 	handler.StartAPIServer("127.0.0.1:6000")
// }

func main() {
	handler.StartAPIServer("127.0.0.1:6000")
}
