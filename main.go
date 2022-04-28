package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/load"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var LoadAverageChannel chan map[string]float64

var NumCPUCores int

// define a reader which will listen for
// new messages being sent to our WebSocket
// endpoint
func readWrite(conn *websocket.Conn) {

	go func() {
		for {
			// read in a message
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}
			// print out that message for clarity
			fmt.Println(string(p))
		}
	}()

	go func() {
		for {
			jsonBytes, _ := json.Marshal(<-LoadAverageChannel)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(jsonBytes)); err != nil {
				log.Println(err)
				return
			}
		}
	}()
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Client Connected")
	err = ws.WriteMessage(1, []byte("Hi Client!"))
	if err != nil {
		log.Println(err)
	}
	// listen indefinitely for new messages coming
	// through on our WebSocket connection
	readWrite(ws)
}

//func homePage(w http.ResponseWriter, r *http.Request) {
//	fmt.Fprintf(w, "Home Page")
//}

func getCpuLoad() map[string]float64 {
	info, _ := load.Avg()
	loadAverage := make(map[string]float64)
	loadAverage["min1"] = info.Load1
	loadAverage["min5"] = info.Load5
	loadAverage["min15"] = info.Load15
	return loadAverage
}

func setupRoutes() {
	//http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {

	var wg sync.WaitGroup

	staticDirectoryPtr := flag.String("static", "static", "static directory which has index.html")

	flag.Parse()

	if *staticDirectoryPtr == "" {
		fmt.Println("please provide -static <static_directory>")
		return
	}

	staticDir := *staticDirectoryPtr

	router := mux.NewRouter().StrictSlash(false)

	httpDir := http.Dir(staticDir)
	httpFileServer := http.FileServer(httpDir)
	httpFileHandler := http.StripPrefix("/load", httpFileServer)
	router.PathPrefix("/load").Handler(httpFileHandler)

	fmt.Println("Socket Server Starting...")

	setupRoutes()

	NumCPUCores = runtime.NumCPU()

	log.Printf("NumCPUCores : %v", NumCPUCores)

	LoadAverageChannel = make(chan map[string]float64)

	go func() {
		for {
			loadAverage := getCpuLoad()
			loadAverage["max_cores"] = float64(NumCPUCores)

			loadAverage["min1_percent"] = loadAverage["min1"] / float64(NumCPUCores) * 100.0
			loadAverage["min5_percent"] = loadAverage["min5"] / float64(NumCPUCores) * 100.0
			loadAverage["min15_percent"] = loadAverage["min15"] / float64(NumCPUCores) * 100.0
			log.Printf("loadAverage : %v", loadAverage)
			LoadAverageChannel <- loadAverage
			time.Sleep(1 * time.Second)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("http server : listening on port 9000 ...")
		log.Fatal(http.ListenAndServe(":9000", nil))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("http server : listening on port 9009 ...")
		log.Fatal(http.ListenAndServe(":9009", router))
	}()
	wg.Wait()
}
