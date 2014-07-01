package main

type ChanGroup []chan []interface{}

func NewChanGroup(chanCap int) ChanGroup {
	group := make(ChanGroup, numSeries)
	for i := 0; i < numSeries; i++ {
		group[i] = make(chan []interface{}, chanCap)
	}

	return group
}
