package data

type Population struct {
	Code       string
	Expression string
}

type Stratifier struct {
	Code        string
	Description string
	Expression  string
}

type Group struct {
	Type        string
	Code        string
	Description string
	Population  []Population
	Stratifier  []Stratifier
}

type Measure struct {
	Library string
	Group   []Group
}
