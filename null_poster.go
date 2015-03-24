package main

type nullPoster struct {
	destination *Destination
	name        string
}

func newNullPoster(destination *Destination) *nullPoster {
	return &nullPoster{
		destination: destination,
		name:        "null",
	}
}

func (p *nullPoster) Run() {
	for _ = range p.destination.points {
	}
}
