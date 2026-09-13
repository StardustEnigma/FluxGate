package handler

import (
	"encoding/json"
	"net/http"
)

type Handler struct{

}

func (h *Handler)Request(w http.ResponseWriter,r *http.Request){

	var Clientid string
	json.NewDecoder(r.Body).Decode(&Clientid)

}