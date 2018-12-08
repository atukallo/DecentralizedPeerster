package webserver

import (
	"encoding/json"
	. "github.com/SubutaiBogatur/Peerster/config"
	. "github.com/SubutaiBogatur/Peerster/gossiper"
	. "github.com/SubutaiBogatur/Peerster/utils"
	. "github.com/SubutaiBogatur/Peerster/utils/send-utils"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	. "net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// * webserver is a thread, that listens on a given port for http-requests from frontend (ie browser). Frontend
//    regularly asks for new information about peers and sometimes makes new orders for a gossiper

var (
	g *Gossiper = nil // careful, it's shared by many threads

	webserverPort = 8080

	logger = log.WithField("bin", "webs")
)

func getGossiperName(w http.ResponseWriter, r *http.Request) {
	//logger.Debug("get gossiper name")
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
	//logger.Debug("get id")
	writeJsonResponse(w, g.GetID(g.GetName()))
}

func getPeers(w http.ResponseWriter, r *http.Request) {
	//logger.Debug("get peers")
	peers := g.GetPeersCopy()
	writeJsonResponse(w, peers)
}

func getOrigins(w http.ResponseWriter, r *http.Request) {
	//logger.Debug("get origins")
	origins := g.GetOriginsCopy()
	writeJsonResponse(w, origins)
}

func sendRumorMessage(w http.ResponseWriter, r *http.Request) {
	logger.Debug("post: send rumor message")
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)
	msg := string(body)

	// let's now feed message to gossiper via network (haha)
	SendRumorMessageToLocalPort(msg, g.GetClientAddress().Port, logger)
}

func sendPrivateMessage(w http.ResponseWriter, r *http.Request) {
	logger.Debug("post: send private message")
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	s := strings.Split(string(body), "|") // terrible, sorry
	dest := s[0]
	text := s[1]

	SendPrivateMessageToLocalPort(text, dest, g.GetClientAddress().Port, logger)
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	//logger.Debug("get messages")
	rmsgs := g.GetRumorMessages()
	pmsgs := g.GetPrivateMessages()

	msgs := map[string]interface{}{"rumor-messages": rmsgs, "private-messages": pmsgs}
	writeJsonResponse(w, msgs)
}

func shareFile(w http.ResponseWriter, r *http.Request) {
	logger.Debug("post: share file")
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	path := string(body)
	SendToShareMessageToLocalPort(path, g.GetClientAddress().Port, logger)
}

func getSharedFiles(w http.ResponseWriter, r *http.Request) {
	writeJsonResponse(w, g.GetSharedFiles())
}

func requestFile(w http.ResponseWriter, r *http.Request) {
	logger.Debug("post: request file")
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	s := strings.Split(string(body), "|") // terrible, sorry
	dest := s[0]
	hash := s[1]

	// get name
	filenumber := 0
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(DownloadsPath, dest+"-"+strconv.Itoa(i))); os.IsNotExist(err) {
			filenumber = i
			break
		}
	}

	SendToDownloadMessageToLocalPort(dest+"-"+strconv.Itoa(filenumber), hash, dest, g.GetClientAddress().Port, logger)
}

func search(w http.ResponseWriter, r *http.Request) {
	logger.Debug("post: search")
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	keywords := strings.Split(string(body), ",") // terrible, sorry
	SendToSearchMessaageToLocalPort(keywords, 0, g.GetClientAddress().Port, logger)
}

func downloadFound(w http.ResponseWriter, r *http.Request) {
	logger.Debug("post: download found")
	body, err := ioutil.ReadAll(r.Body)
	CheckError(err, logger)

	name := string(body)
	g.DownloadFoundByName(name)
}

func getSearchMatches(w http.ResponseWriter, r *http.Request) {
	writeJsonResponse(w, g.GetFullSearchMatches())
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
	r.Methods("POST").Subrouter().HandleFunc("/shareFile", shareFile)
	r.Methods("GET").Subrouter().HandleFunc("/getSharedFiles", getSharedFiles)
	r.Methods("POST").Subrouter().HandleFunc("/requestFile", requestFile)
	r.Methods("POST").Subrouter().HandleFunc("/search", search)
	r.Methods("GET").Subrouter().HandleFunc("/getSearchMatches", getSearchMatches)
	r.Methods("POST").Subrouter().HandleFunc("/downloadFound", downloadFound)

	r.Handle("/", http.FileServer(http.Dir("./webserver/static"))) // relative path for main.go

	log.Println(http.ListenAndServe(":"+strconv.Itoa(webserverPort), r))
}
