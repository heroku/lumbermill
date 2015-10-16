package main

type nullPoster struct {
	destination *destination
	name        string
}

func newNullPoster(destination *destination) *nullPoster {
	return &nullPoster{
		destination: destination,
		name:        "null",
	}
}

func (p *nullPoster) Run() {
	for _ = range p.destination.envelopes {
	}
}
