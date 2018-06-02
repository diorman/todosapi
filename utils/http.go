package utils

import (
	"encoding/json"
	"errors"
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

func WriteStandardErrorJSON(w http.ResponseWriter, code int) {
	WriteJSON(w, code, errors.New(http.StatusText(code)))
}
