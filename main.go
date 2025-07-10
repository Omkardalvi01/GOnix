package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"gopkg.in/yaml.v3"
)

type routes struct{
	Route map[string][]string `yaml:"routes"`
}

func get_path(u string ,r routes) string{
	path := r.Route[u][0]
	
	//deques the first element and adds it to the end of the slice
	r.Route[u] = r.Route[u][1:]
	r.Route[u] = append(r.Route[u] , path)
	return path
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

func logging(r *http.Request , b string){
	fmt.Printf("time : %v , url : %v , forwarded_url : %v\n" , time.Now() , r.URL , b)
}

func set_header(src , dest http.Header){
	for k , src_val := range src{
		for _ , v := range src_val{
			dest.Add(k , v)
		}
	}
}
func main() {

	f , err := os.ReadFile("config.yaml")
	if err != nil {
		fmt.Println(err)
	}
	var rout routes
	if err := yaml.Unmarshal(f , &rout); err != nil {
		fmt.Print(err)
	}
	

	log.Fatal(http.ListenAndServe(":5000", http.HandlerFunc( func(w http.ResponseWriter, r *http.Request) {
		urls := r.URL
		b := get_path(urls.String() , rout)
		logging(r , b)
		forward_to_backend(w , r, b)
	})))
	
}
