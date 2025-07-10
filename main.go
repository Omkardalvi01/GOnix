package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var routes = map[string]string{
	"/user" : "http://localhost:4000/user",
	"/admin" : "http://localhost:4000/admin" ,
}

func proxy_handler(w http.ResponseWriter , r *http.Request){
	
	urls := r.URL
	b := routes[urls.String()]
	logging(r)
	forward_to_backend(w , r, b)
	// tr.RoundTrip()

}

func forward_to_backend(w http.ResponseWriter , r *http.Request , backend string){
	tr := http.Transport{
		IdleConnTimeout: 10 * time.Second,
		MaxConnsPerHost: 10,
	}
	req , err := http.NewRequest(r.Method , backend , r.Body)
	if err != nil {
		fmt.Println(err)
	}

	resp , err := tr.RoundTrip(req)
	if err != nil{
		fmt.Println(err)
	}

	set_header(resp.Header , w.Header())
	w.WriteHeader(resp.StatusCode)
	io.Copy(w , resp.Body)
	
}

func logging(r *http.Request){
	fmt.Printf("time : %v , url : %v , ip_address : %v" , time.Now() , r.URL , r.RemoteAddr)
}

func set_header(src , dest http.Header){
	for k , src_val := range src{
		for _ , v := range src_val{
			dest.Add(k , v)
		}
	}
}
func main() {
	log.Fatal(http.ListenAndServe(":5000", http.HandlerFunc(proxy_handler)))
}
