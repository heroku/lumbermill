package main

type NullPoster struct {
	chanGroup *ChanGroup
	name      string
}

func NewNullPoster(chanGroup *ChanGroup) *NullPoster {
	return &NullPoster{
		chanGroup: chanGroup,
		name:      "null",
	}
}

func (p *NullPoster) Run() {
	for _ = range p.chanGroup.points {
	}
}
