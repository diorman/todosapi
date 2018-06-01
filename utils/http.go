package utils

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, code int, obj interface{}) {
	if e, ok := obj.(error); ok {
		obj = struct {
			Error string `json:"error"`
		}{e.Error()}
	}

	jsonData, err := json.Marshal(obj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(jsonData)
}
