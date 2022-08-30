package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
)

type LoginReqBody struct {
	LoginInfo Login `json:"login"`
}
type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRespBody struct {
	Errorcode int    `json:"errorcode"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	Sessionid string `json:"sessionid"`
}

type apiResp struct {
	respBody   string
	statusCode int
}

func nshttp(fqdn, endpoint, action string, body []byte, sessionID string) apiResp {

	// Create HTTP client and disable TLS verification for self-signed certs
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("error creating cookie jar - %s\r\n", err)
	}
	client := &http.Client{Jar: jar, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	// Create the HTTP request with the content-type header
	req, err := http.NewRequest(action, fmt.Sprintf("https://%s/nitro/v1/config/%s", fqdn, endpoint), bytes.NewBuffer(body))
	if err != nil {
		log.Fatalf("error creating http request - %s\r\n", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.AddCookie(&http.Cookie{Name: "NITRO_AUTH_TOKEN", Value: sessionID})
	}

	// Make the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("error making http request - %s\r\n", err)
	}

	fmt.Printf("%s request to %s - %d\r\n", action, req.URL.String(), resp.StatusCode)

	// Process response
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading api response - %s\r\n", err)
	}

	return apiResp{statusCode: resp.StatusCode, respBody: string(data[:])}

}

func main() {

	// Get the argument
	var server, user, password string
	flag.StringVar(&server, "server", "", "netscaler in format of fqdn:port or ip:port")
	flag.StringVar(&user, "user", "", "netscaler username")
	flag.StringVar(&password, "password", "", "netscaler password")

	flag.Parse()

	if server == "" || user == "" || password == "" {
		log.Fatal("the -server, -user, and -password flags are required. see nsextract -h")
	}

	// Create the login request
	loginReq := LoginReqBody{LoginInfo: Login{Username: user, Password: password}}
	loginReqBody, err := json.Marshal(loginReq)
	if err != nil {
		log.Fatalf("error marshaling login request - %s\r\n", err)
	}

	// Login to the netscaler
	api := nshttp(server, "login", "POST", loginReqBody, "")
	if api.statusCode != 201 {
		log.Fatalf("login received %d. expected 201.\r\n", api.statusCode)
	}

	// Unmarshall the response
	var resp LoginRespBody
	json.Unmarshal([]byte(api.respBody), &resp)

	// Iterate over the endpoints, call API, save as JSON
	for _, endpoint := range []string{"nsip", "ipset_binding?bulkbindings=yes", "netprofile", "service", "lbvserver_service_binding?bulkbindings=yes", "servicegroup_servicegroupmember_binding?bulkbindings=yes", "lbvserver_servicegroup_binding?bulkbindings=yes", "lbvserver"} {
		api = nshttp(server, endpoint, "GET", nil, resp.Sessionid)
		if api.statusCode != 200 {
			log.Fatalf("get %s received %d. expected 200.", strings.Split(endpoint, "?")[0], api.statusCode)
		}

		// Create the file
		fileName := strings.Split(endpoint, "?")[0] + ".json"
		outputFile, err := os.Create(fileName)
		if err != nil {
			log.Fatalf("error creating output file - %s\r\n", err)
		}
		defer outputFile.Close()
		_, err = outputFile.WriteString((api.respBody))
		if err != nil {
			log.Fatalf("error writing output - %s\r\n", err)
		}
		fmt.Printf("created %s\r\n", fileName)
	}

	fmt.Println("extract complete")
}
