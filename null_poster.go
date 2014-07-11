package main

type NullPoster struct {
	destination *Destination
	name        string
}

func NewNullPoster(destination *Destination) *NullPoster {
	return &NullPoster{
		destination: destination,
		name:        "null",
	}
}

func (p *NullPoster) Run() {
	for _ = range p.destination.points {
	}
}
