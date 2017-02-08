#Quick 'n Dirty predict service.  
Quick 'n Dirty SensorThings predict service that requests the observations for a given Datastream 
for the last xx minutes and calculates the change per minute.

##Settings
Host and Port of this service can only be set trough environment variables:
- gost_predict_host
- gost_predict_port

##Endpoint /Predict
### Params
host = SensorThings host  
datastream = ID of the datastream to make a prediction for (must be an OM_Measurement stream)  
time = the date time in ISO8601 to make a prediction for  
span = amount of minutes in the past to base the prediction on  
```
http://localhost:8080/Predict?host=http://mysensorthingsserver.com&datastream=93&time=2017-02-07T15:40:17.000Z&span=15
```
###Response
rate = result change per minute  
prediction = prediction result for the given time 
```javascript
{
   "rate": -0.27592232516989745,
   "prediction": 503.55318070888063
}
```