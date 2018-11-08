package webserver

import (
	"encoding/json"
	. "github.com/SubutaiBogatur/Peerster/gossiper"
	. "github.com/SubutaiBogatur/Peerster/utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	. "net"
	"net/http"
	"strconv"
	"strings"
)

// * webserver is a thread, that listens on a given port for http-requests from frontend (ie browser). Frontend
//    regularly asks for new information about peers and sometimes makes new orders for a gossiper

var (
	g *Gossiper = nil // careful, it's shared by many threads

	webserverPort         = 8080

	logger = log.WithField("bin", "webs")
)

func getGossiperName(w http.ResponseWriter, r *http.Request) {
	logger.Info("get gossiper name")
	writeJsonResponse(w, g.GetName())
}

func setGossiperName(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Warn("error json, when calling setGossiperName")
	}
	name := string(body)
	log.Info("setGossiperName with name: " + name)
	if strings.TrimSpace(name) == "" {
		return // naive validation
	}
	g.SetName(name)
}

func addPeer(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Warn("error json, when calling setGossiperName")
	}
	name := string(body)
	logger.Info("add peer: " + name)
	udpAddress, err := ResolveUDPAddr("udp4", name)
	if err != nil {
		log.Warn("unable to decode addr")
		return
	}

	g.UpdatePeersIfNeeded(udpAddress)
}

func getGossiperID(w http.ResponseWriter, r *http.Request) {
	logger.Info("get id")
	writeJsonResponse(w, g.GetID(g.GetName()))
}

func getPeers(w http.ResponseWriter, r *http.Request) {
	logger.Info("get peers")
	peers := g.GetPeersCopy()
	writeJsonResponse(w, peers)
}

func sendRumorMessage(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Warn("error json, when calling setGossiperName")
	}
	msg := string(body)

	// let's now feed message to gossiper via network (haha)
	SendRumorMessageToLocalPort(msg, g.GetClientAddress().Port, logger)
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	logger.Info("get messages")
	msgs := *g.GetMessages()
	writeJsonResponse(w, msgs)
}

func writeJsonResponse(w http.ResponseWriter, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Warn("error when making json response")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func StartWebserver(gossiper *Gossiper) {
	g = gossiper
	logger = logger.WithField("a", g.GetPeerAddress().String())

	logger.Info("started web-server thread")


	r := mux.NewRouter()

	r.Methods("GET").Subrouter().HandleFunc("/getGossiperName", getGossiperName)
	r.Methods("POST").Subrouter().HandleFunc("/setGossiperName", setGossiperName)
	r.Methods("GET").Subrouter().HandleFunc("/getGossiperID", getGossiperID)
	r.Methods("GET").Subrouter().HandleFunc("/getPeers", getPeers)
	r.Methods("POST").Subrouter().HandleFunc("/addPeer", addPeer)
	r.Methods("POST").Subrouter().HandleFunc("/sendRumorMessage", sendRumorMessage)
	r.Methods("GET").Subrouter().HandleFunc("/getMessages", getMessages)

	r.Handle("/", http.FileServer(http.Dir("./webserver/static"))) // relative path for main.go

	log.Println(http.ListenAndServe(":"+strconv.Itoa(webserverPort), r))
}
