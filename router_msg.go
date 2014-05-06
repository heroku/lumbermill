package main

type routerMsg struct {
	Bytes     int
	Status    int
	Service   string
	Connect   string
	Dyno      string
	Method    string
	Path      string
	Host      string
	RequestId string
	Fwd       string
}
