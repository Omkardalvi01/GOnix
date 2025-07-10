package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test_path(t *testing.T){
	f , err := os.ReadFile("config.yaml")
	if err != nil {
		t.Errorf("Error : %v",err)
	}
	var rout routes
	if err := yaml.Unmarshal(f , &rout); err != nil {
		t.Errorf("Error : %v",err)
	}
	res := get_path("/user", rout)
	fmt.Println(res)
	res = get_path("/user", rout)
	fmt.Println(res)
	res = get_path("/user", rout)
	fmt.Println(res)
	res = get_path("/user", rout)
	fmt.Println(res)
}

func Test_verify_token(t *testing.T){
	example := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.vgmErpVc7vMC8DXeL8s0WiVH8lg8Tw-78t6QzIEu4WU"
	if err := verify(example); err != nil{
		t.Errorf("Error : %v",err)
	}

}

func Test_authenticate(t *testing.T){
	var data = []byte("Hello, World") 
	r , err := http.NewRequest("GET" , "/user" , bytes.NewReader(data))
	if err != nil{
		t.Errorf("Error : %v",err)
	}
	r.Header.Add("Authorization" , "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.vgmErpVc7vMC8DXeL8s0WiVH8lg8Tw-78t6QzIEu4WU")
	err = authenticate(r)
	if err != nil{
		t.Errorf("Error : %v",err)
	}
}