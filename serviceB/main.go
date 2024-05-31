package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"unicode"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"go.opentelemetry.io/otel"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type ViaCep struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro"`
}

type WeatherApi struct {
	Current struct {
		TempC float64 `json:"temp_c"`
		TempF float64 `json:"temp_f"`
		TempK float64 `json:"temp_k"`
	} `json:"current"`
}

type Response struct {
	TempC float64 `json:"temp_c"`
	TempF float64 `json:"temp_f"`
	TempK float64 `json:"temp_k"`
	City  string  `json:"city"`
}

func main() {
	startZipkin()

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/weather", SearchCepHandler)

	http.ListenAndServe(":8081", r)
}

func SearchCepHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("service-b").Start(r.Context(), "service-b-full-request")
	defer span.End()

	cepParam := r.URL.Query().Get("cep")

	validate := regexp.MustCompile(`^[0-9]{8}$`)
	if !validate.MatchString(cepParam) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	cep, err := SearchCep(cepParam, ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorStr := err.Error()
		w.Write([]byte("error while searching for cep: " + errorStr))
		return
	}

	if cep.Erro {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("can not find zipcode"))
		return
	}

	weather, err := SearchTemperature(cep.Localidade, ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorStr := err.Error()
		w.Write([]byte("error while searching for temperature: " + errorStr))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(weather)
}

func SearchCep(cep string, ctx context.Context) (*ViaCep, error) {
	_, span := otel.Tracer("service-b").Start(ctx, "call-to-viacep")
	defer span.End()

	req, err := http.Get("http://viacep.com.br/ws/" + cep + "/json/")
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	res, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var data ViaCep
	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func SearchTemperature(city string, ctx context.Context) (*Response, error) {
	_, span := otel.Tracer("service-b").Start(ctx, "call-to-weatherapi")
	defer span.End()

	urlWeatherApi := "http://api.weatherapi.com/v1/current.json?key=12969ce544064451ab2103040240905&aqi=no&q=" + removeDiacriticsAndEncodeCityName(city)
	req, err := http.Get(urlWeatherApi)

	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	res, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var data WeatherApi
	err = json.Unmarshal(res, &data)
	if err != nil {
		return nil, err
	}

	data.Current.TempF = data.Current.TempC*1.8 + 32
	data.Current.TempK = data.Current.TempC + 273.15

	return &Response{City: city, TempC: data.Current.TempC, TempF: data.Current.TempF, TempK: data.Current.TempK}, nil
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r)
}

func removeDiacriticsAndEncodeCityName(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	result, _, _ := transform.String(t, s)
	result = url.QueryEscape(result)
	return result
}
