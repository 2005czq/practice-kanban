package dashboard

type Payload struct {
	Title          string          `json:"title"`
	StartDate      string          `json:"startDate"`
	GeneratedAt    string          `json:"generatedAt"`
	UpdatedAtLabel string          `json:"updatedAtLabel"`
	Users          []UserDashboard `json:"users"`
}

type UserDashboard struct {
	Handle        string       `json:"handle"`
	AvatarURL     string       `json:"avatarUrl"`
	CurrentRating RatingLabel  `json:"currentRating"`
	MaxRating     RatingLabel  `json:"maxRating"`
	Contests      []ContestRow `json:"contests"`
}

type RatingLabel struct {
	Value int    `json:"value"`
	Rank  string `json:"rank"`
}

type ContestRow struct {
	ContestID           int           `json:"contestId"`
	Name                string        `json:"name"`
	URL                 string        `json:"url"`
	FriendsStandingsURL string        `json:"friendsStandingsUrl"`
	Problems            []ProblemCell `json:"problems"`
	StartedAt           string        `json:"startedAt"`
	VirtualAt           string        `json:"virtualAt"`
}

type ProblemCell struct {
	Index         string `json:"index"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Status        string `json:"status"`
	AttemptsLabel string `json:"attemptsLabel"`
	Attempts      int    `json:"attempts"`
}
