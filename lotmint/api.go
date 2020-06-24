package api


/*
The api.go defines the methods that can be called from the outside. Most
of the methods will take a roster so that the service knows which nodes
it should work with.
This part of the service runs on the client or the app.
*/

// ServiceName is used for registration on the onet.
const ServiceName = "LotmintService"
