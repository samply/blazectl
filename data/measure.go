package data

type Population struct {
	Code       string
	Expression string
}

type Stratifier struct {
	Code       string
	Expression string
}

type Group struct {
	Type       string
	Population []Population
	Stratifier []Stratifier
}

type Measure struct {
	Library string
	Group   []Group
}
