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
	logger.Debug("get gossiper name")
	writeJsonResponse(w, g.GetName())
}

func setGossiperName(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	name := string(body)
	logger.Debug("setGossiperName with name: " + name)
	if strings.TrimSpace(name) == "" {
		return // naive validation
	}
	g.SetName(name)
}

func addPeer(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	name := string(body)
	logger.Debug("add peer: " + name)
	udpAddress, err := ResolveUDPAddr("udp4", name)
	CheckError(err, logger)

	g.UpdatePeersIfNeeded(udpAddress)
}

func getGossiperID(w http.ResponseWriter, r *http.Request) {
	logger.Debug("get id")
	writeJsonResponse(w, g.GetID(g.GetName()))
}

func getPeers(w http.ResponseWriter, r *http.Request) {
	logger.Debug("get peers")
	peers := g.GetPeersCopy()
	writeJsonResponse(w, peers)
}

func getOrigins(w http.ResponseWriter, r *http.Request) {
	logger.Debug("get origins")
	origins := g.GetOriginsCopy()
	writeJsonResponse(w, origins)
}

func sendRumorMessage(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)
	msg := string(body)

	// let's now feed message to gossiper via network (haha)
	SendRumorMessageToLocalPort(msg, g.GetClientAddress().Port, logger)
}

func sendPrivateMessage(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	s := strings.Split(string(body), "|") // terrible, sorry
	dest := s[0]
	text := s[1]

	SendPrivateMessageToLocalPort(text, dest, g.GetClientAddress().Port, logger)
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	logger.Debug("get messages")
	rmsgs := g.GetRumorMessages()
	pmsgs := g.GetPrivateMessages()

	msgs := map[string]interface{}{"rumor-messages" : rmsgs, "private-messages" : pmsgs}
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

	logger.Debug("started web-server thread")


	r := mux.NewRouter()

	r.Methods("GET").Subrouter().HandleFunc("/getGossiperName", getGossiperName)
	r.Methods("POST").Subrouter().HandleFunc("/setGossiperName", setGossiperName)
	r.Methods("GET").Subrouter().HandleFunc("/getGossiperID", getGossiperID)
	r.Methods("GET").Subrouter().HandleFunc("/getPeers", getPeers)
	r.Methods("POST").Subrouter().HandleFunc("/addPeer", addPeer)
	r.Methods("GET").Subrouter().HandleFunc("/getOrigins", getOrigins)
	r.Methods("POST").Subrouter().HandleFunc("/sendRumorMessage", sendRumorMessage)
	r.Methods("POST").Subrouter().HandleFunc("/sendPrivateMessage", sendPrivateMessage)
	r.Methods("GET").Subrouter().HandleFunc("/getMessages", getMessages)

	r.Handle("/", http.FileServer(http.Dir("./webserver/static"))) // relative path for main.go

	log.Println(http.ListenAndServe(":"+strconv.Itoa(webserverPort), r))
}
