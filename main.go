package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"flag"
	"github.com/gorilla/mux"
	"encoding/json"
	"bytes"
	"time"
	"strconv"
	"io/ioutil"
	"strings"
)

var host, port string

// PredictionResponse
type PredictionResponse struct {
	Rate        float64   `json:"rate,omitempty"`
	LastWeek float64   `json:"lastWeek,omitempty"`
	Prediction float64   `json:"prediction,omitempty"`
}

type ErrorResponse struct {
	Status        int   `json:"status,omitempty"`
	Error string   `json:"error,omitempty"`
}

type SensorThingsResponseBase struct {
	Count int   `json:"count,omitempty"`
	NextLink      string `json:"@iot.nextLink,omitempty"`
}

type SensorThingsResponseObservations struct {
	SensorThingsResponseBase
	Observations []Observation `json:"value,omitempty"`
}

type BaseEntity struct {
	ID      interface{} `json:"@iot.id,omitempty"`
	NavSelf string      `json:"@iot.selfLink,omitempty"`
}

type Observation struct {
	BaseEntity
	PhenomenonTime       string                 `json:"phenomenonTime,omitempty"`
	Result               interface{}            `json:"result,omitempty"`
	ResultTime           string                 `json:"resultTime,omitempty"`
	ResultQuality        string                 `json:"resultQuality,omitempty"`
	ValidTime            string                 `json:"validTime,omitempty"`
	Parameters           map[string]interface{} `json:"parameters,omitempty"`
	NavDatastream        string                 `json:"Datastream@iot.navigationLink,omitempty"`
	NavFeatureOfInterest string                 `json:"FeatureOfInterest@iot.navigationLink,omitempty"`
}

func main() {
	hostFlag := flag.String("host", "", "Define the host where you want to run the predict service on, can also set by environment variable gost_predict_host, default = localhost")
	hostPort := flag.String("port", "8080", "Define the port where you want to run the predict service on, can also set by environment variable gost_predict_port, default = 8080")
	flag.Parse()

	host = *hostFlag
	port = *hostPort

	getEnvironmentVariables()
	fmt.Printf("Starting gost-predict server: %v:%v\n", host, port)

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/Predict", predict)
	router.HandleFunc("/predict", predict)

	log.Fatal(http.ListenAndServe(fmt.Sprintf("%v:%v", host, port), router))
}

func predict(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	host, ok := params["host"]
	if !ok {
		sendError(w, "missing host param")
		return
	}

	datastream, ok := params["datastream"]
	if !ok {
		sendError(w, "missing datastream param")
		return
	}

	time, ok := params["time"]
	if !ok {
		sendError(w, "missing time param")
		return
	}

	span, ok := params["span"]
	if !ok {
		sendError(w, "missing span param")
		return
	}

	p, err := createPredict(host[0], datastream[0], time[0], span[0]);
	if err != nil {
		sendError(w, err.Error())
		return
	}

	sendPrediction(w, p)
}

func createPredict(host, datastream, predictTime, span string) (*PredictionResponse, error){
	layout := "2006-01-02T15:04:05.000Z"
	i, _ := strconv.ParseInt(span, 10, 64)
	toPredictTime, err := time.Parse(layout, predictTime)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse predictTime: %v", predictTime)
	}

	then := time.Now().UTC().Add(- (time.Minute*time.Duration(i)))
	var spanUrl =  fmt.Sprintf("%v/v1.0/Datastreams(%v)/Observations?$filter=phenomenonTime gt '%v'&$select=result,phenomenonTime", host, datastream, then.Format(time.RFC3339Nano))
	obs, err := getObservations(spanUrl, nil)
	if err != nil {
		return nil, err
	}

	// not enough data to make a prediction, get latest data
	if len(obs) < 2 {
		var spanUrl =  fmt.Sprintf("%v/v1.0/Datastreams(%v)/Observations?$top=2&$select=result,phenomenonTime", host, datastream)
		obs, err = getObservations(spanUrl, nil)
		if err != nil {
			return nil, err
		}
	}

	obNow := Observation{
		PhenomenonTime: time.Now().UTC().Format(time.RFC3339Nano),
		Result: obs[0].Result,
	}
	obs = append([]Observation{ obNow }, obs...)

	rates := make([]float64, 0)
	rateTotal := 0.0
	lastKnown := 0.0
	for i := 0; i < len(obs) -1; i++ {
		t1, _ := time.Parse(layout, obs[i].PhenomenonTime)
		t2, _ := time.Parse(layout, obs[i + 1].PhenomenonTime)

		delta := t1.Sub(t2)
		r1, _ := strconv.ParseFloat(fmt.Sprintf("%v", obs[i].Result), 64)
		r2, _ := strconv.ParseFloat(fmt.Sprintf("%v", obs[i + 1].Result), 64)
		if i == 0 {
			lastKnown = r1
		}

		diff := r1 - r2
		ratePerMin := diff / delta.Minutes()
		rateTotal += ratePerMin
		rates = append(rates, ratePerMin)
	}

	medianRate := rateTotal / float64(len(rates))
	predictAndNowMinutesDelta := toPredictTime.Sub(time.Now().UTC()).Minutes()

	pr := &PredictionResponse{
		Rate: medianRate,
		Prediction: lastKnown + (predictAndNowMinutesDelta * medianRate),
	}

	return pr, nil
}

func getObservations(url string, observations []Observation) ([]Observation, error) {
	if len(observations) == 0 {
		observations = make([]Observation, 0)
	}

	request, err := http.NewRequest("GET", encodeUrl(url), nil)
	request.Header.Add("Content-Type", "application/json")
	client := http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	var obs SensorThingsResponseObservations
	err = json.Unmarshal(b, &obs)
	if err != nil {
		return nil, err
	}

	for _, o := range obs.Observations {
		observations = append(observations, o)
	}

	if len(obs.NextLink) != 0 {
		observations, err = getObservations(obs.NextLink, observations)
		if err != nil {
			return nil, err
		}
	}

	return observations, nil
}

func encodeUrl(url string) string {
	n := url
	n = strings.Replace(n, " ", "%20", -1)
	n = strings.Replace(n, "'", "%27", -1)
	return n
}

func sendError(w http.ResponseWriter, error string){
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(400)

	er := &ErrorResponse{
		Status: 400,
		Error: error,
	}

	b, err := JSONMarshal(er, true)
	if err != nil {
		panic(err)
	}

	w.Write(b)
}

func sendPrediction(w http.ResponseWriter, prediction *PredictionResponse){
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(200)

	b, err := JSONMarshal(prediction, true)
	if err != nil {
		panic(err)
	}

	w.Write(b)
}


//JSONMarshal converts the data and converts special characters such as &
func JSONMarshal(data interface{}, safeEncoding bool) ([]byte, error) {
	var b []byte
	var err error
	b, err = json.MarshalIndent(data, "", "   ")

	if safeEncoding {
		b = bytes.Replace(b, []byte("\\u003c"), []byte("<"), -1)
		b = bytes.Replace(b, []byte("\\u003e"), []byte(">"), -1)
		b = bytes.Replace(b, []byte("\\u0026"), []byte("&"), -1)
	}
	return b, err
}

func getEnvironmentVariables(){
	predictHost := os.Getenv("gost_predict_host")
	if predictHost != "" {
		host = predictHost;
	}

	predictPort := os.Getenv("gost_predict_port")
	if predictPort != "" {
		port = "8080";
	}
}