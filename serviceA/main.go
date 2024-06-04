package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"go.opentelemetry.io/otel"
)

type Response struct {
	TempC float64 `json:"temp_c"`
	TempF float64 `json:"temp_f"`
	TempK float64 `json:"temp_k"`
	City  string  `json:"city"`
}

func main() {
	startZipkin()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/weather", SearchCepHandler)

	log.Fatal(http.ListenAndServe(":8080", r))
}

func SearchCepHandler(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")

	if cep == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("cep parameter is required"))
		return
	}

	validate := regexp.MustCompile(`^[0-9]{8}$`)
	if !validate.MatchString(cep) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	temperature, err := CallServiceB(cep, r.Context())

	if err != nil {
		errorStr := err.Error()
		if errorStr == "can not find zipcode" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(errorStr))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error while searching for cep: " + errorStr))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if temperature != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(temperature)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("can not find temperature"))
	}
}

func CallServiceB(cep string, ctx context.Context) (*Response, error) {
	_, span := otel.Tracer("service-a").Start(ctx, "call-to-service-b")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://serviceb:8081/weather?cep="+cep, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("can not find zipcode")
	}

	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var data Response
	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}
