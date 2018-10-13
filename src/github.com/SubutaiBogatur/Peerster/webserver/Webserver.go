package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	. "github.com/SubutaiBogatur/Peerster/gossiper"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	log "github.com/sirupsen/logrus"
)

// * webserver is a thread, that listens on a given port for http-requests from frontend (ie browser). Frontend
//    regularly asks for new information about peers and sometimes makes new orders for a gossiper

var (
	g *Gossiper = nil

	logger = log.WithField("bin", "webserv")
)

func getGossiperName(w http.ResponseWriter, r *http.Request) {
	logger.Info("get gossiper name")

	data, err := json.Marshal(g.Name.Load().(string))
	if err != nil {
		log.Warn("error when making json response")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func setGossiperName(w http.ResponseWriter, r *http.Request) {
	logger.Info("set gossiper name")

	var body, err = ioutil.ReadAll(r.Body)
	if err != nil {
		log.Warn("error json")
	}

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var name = ""
	err = json.Unmarshal(body, &name)

	g.Name.Store(name)
}

func StartWebserver(gossiper *Gossiper) {
	g = gossiper

	fmt.Println("server started")

	r := mux.NewRouter()

	r.Methods("GET").Subrouter().HandleFunc("/getGossiperName", getGossiperName)
	r.Methods("POST").Subrouter().HandleFunc("/setGossiperName", setGossiperName)


	r.Handle("/", http.FileServer(http.Dir("./static")))

	log.Println(http.ListenAndServe(":8080", r))
}
