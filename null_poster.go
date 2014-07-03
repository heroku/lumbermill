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
	for {
		select {
		case _, open := <-p.chanGroup.points[Router]:
			if !open {
				break
			}
		case _, open := <-p.chanGroup.points[EventsRouter]:
			if !open {
				break
			}
		case _, open := <-p.chanGroup.points[DynoMem]:
			if !open {
				break
			}
		case _, open := <-p.chanGroup.points[DynoLoad]:
			if !open {
				break
			}
		case _, open := <-p.chanGroup.points[EventsDyno]:
			if !open {
				break
			}
		}
	}
}
