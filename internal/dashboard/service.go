package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kanban/internal/codeforces"
)

type codeforcesClient interface {
	GetContestList(ctx context.Context) ([]codeforces.Contest, error)
	GetContestStandings(ctx context.Context, contestID int) (codeforces.Standings, error)
	GetUserStatus(ctx context.Context, handle string) ([]codeforces.Submission, error)
	GetUsersInfo(ctx context.Context, handles []string) ([]codeforces.UserInfo, error)
}

type Options struct {
	Title     string
	Users     []string
	StartDate time.Time
	Location  *time.Location
}

type Service struct {
	client codeforcesClient
	opts   Options
}

type userBundle struct {
	status   []codeforces.Submission
	userInfo codeforces.UserInfo
}

type analyzedUser struct {
	handle    string
	avatarURL string
	current   RatingLabel
	max       RatingLabel
	attempts  map[int]*contestAttempt
}

type contestMeta struct {
	contest      codeforces.Contest
	problemOrder []string
	problemNames map[string]string
}

type contestMetadataCache struct {
	Contests map[int]contestMetadataEntry `json:"contests"`
}

type contestMetadataEntry struct {
	ProblemOrder []string          `json:"problemOrder"`
	ProblemNames map[string]string `json:"problemNames"`
}

const contestMetadataCachePath = "data/contest-metadata.json"

type contestAttempt struct {
	contestID int
	virtualAt time.Time
	problems  map[string]*problemState
}

type problemState struct {
	index              string
	name               string
	wrongBeforeAC      int
	hasAttempt         bool
	hasAccepted        bool
	acceptedInVirtual  bool
	acceptedInPractice bool
}

func NewService(client codeforcesClient, opts Options) *Service {
	return &Service{client: client, opts: opts}
}

func (s *Service) Build(ctx context.Context) (Payload, error) {
	now := time.Now().In(s.opts.Location)

	contests, err := s.client.GetContestList(ctx)
	if err != nil {
		return Payload{}, err
	}

	usersInfo, err := s.client.GetUsersInfo(ctx, s.opts.Users)
	if err != nil {
		return Payload{}, err
	}
	usersInfoByHandle := indexUsersInfo(usersInfo)

	contestMap := buildContestMap(contests)
	loadContestMetadataCache(contestMap)
	analyzedUsers := make([]analyzedUser, 0, len(s.opts.Users))
	neededContestIDs := make(map[int]struct{})

	for _, handle := range s.opts.Users {
		bundle, err := s.fetchUserBundle(ctx, handle, usersInfoByHandle)
		if err != nil {
			return Payload{}, fmt.Errorf("fetch user %s: %w", handle, err)
		}

		current, max := buildRatingLabels(bundle.userInfo)
		attempts := analyzeContests(bundle.status, contestMap, s.opts.StartDate)
		for contestID := range attempts {
			neededContestIDs[contestID] = struct{}{}
		}

		analyzedUsers = append(analyzedUsers, analyzedUser{
			handle:    handle,
			avatarURL: bundle.userInfo.TitlePhoto,
			current:   current,
			max:       max,
			attempts:  attempts,
		})
	}

	if err := s.fillProblemMetadata(ctx, contestMap, neededContestIDs); err != nil {
		return Payload{}, err
	}
	if err := saveContestMetadataCache(contestMap); err != nil {
		return Payload{}, err
	}

	users := make([]UserDashboard, 0, len(analyzedUsers))
	for _, user := range analyzedUsers {
		users = append(users, renderUserDashboard(user, contestMap))
	}

	return Payload{
		Title:          s.opts.Title,
		StartDate:      s.opts.StartDate.Format("2006-01-02"),
		GeneratedAt:    now.Format(time.RFC3339),
		UpdatedAtLabel: now.Format("2006-01-02 15:04 MST"),
		Users:          users,
	}, nil
}

func (s *Service) fetchUserBundle(ctx context.Context, handle string, usersInfoByHandle map[string]codeforces.UserInfo) (userBundle, error) {
	status, err := s.client.GetUserStatus(ctx, handle)
	if err != nil {
		return userBundle{}, err
	}

	userInfo, ok := usersInfoByHandle[strings.ToLower(handle)]
	if !ok {
		return userBundle{}, fmt.Errorf("missing user.info payload for %s", handle)
	}

	return userBundle{status: status, userInfo: userInfo}, nil
}

func buildContestMap(contests []codeforces.Contest) map[int]*contestMeta {
	contestMap := make(map[int]*contestMeta, len(contests))
	for _, contest := range contests {
		if !isTargetContest(contest.Name) {
			continue
		}
		contestMap[contest.ID] = &contestMeta{
			contest:      contest,
			problemNames: make(map[string]string),
		}
	}
	return contestMap
}

func isTargetContest(name string) bool {
	if strings.Contains(name, "Div. 3") || strings.Contains(name, "Div. 4") {
		return false
	}
	return strings.Contains(name, "Div. 2")
}

func buildRatingLabels(info codeforces.UserInfo) (RatingLabel, RatingLabel) {
	current := info.Rating
	maxValue := info.MaxRating
	if current <= 0 && maxValue <= 0 {
		return RatingLabel{Value: 0, Rank: "unrated"}, RatingLabel{Value: 0, Rank: "unrated"}
	}
	if maxValue < current {
		maxValue = current
	}
	return RatingLabel{Value: current, Rank: ratingRank(current)}, RatingLabel{Value: maxValue, Rank: ratingRank(maxValue)}
}

func analyzeContests(submissions []codeforces.Submission, contestMap map[int]*contestMeta, startDate time.Time) map[int]*contestAttempt {
	filtered := make([]codeforces.Submission, 0, len(submissions))
	for _, submission := range submissions {
		if submission.ContestID == 0 {
			continue
		}
		if _, ok := contestMap[submission.ContestID]; !ok {
			continue
		}
		filtered = append(filtered, submission)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreationTimeSeconds == filtered[j].CreationTimeSeconds {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].CreationTimeSeconds < filtered[j].CreationTimeSeconds
	})

	firstVirtualStart := make(map[int]time.Time)
	for _, submission := range filtered {
		if submission.Author.ParticipantType != "VIRTUAL" {
			continue
		}
		start := time.Unix(submission.Author.StartTimeSeconds, 0).In(startDate.Location())
		current, exists := firstVirtualStart[submission.ContestID]
		if !exists || start.Before(current) {
			firstVirtualStart[submission.ContestID] = start
		}
	}

	eligible := make(map[int]bool)
	for contestID, virtualStart := range firstVirtualStart {
		if virtualStart.Before(startDate) {
			continue
		}
		eligible[contestID] = true
	}

	for _, submission := range filtered {
		contestID := submission.ContestID
		virtualStart, exists := firstVirtualStart[contestID]
		if !exists || !eligible[contestID] {
			continue
		}
		createdAt := time.Unix(submission.CreationTimeSeconds, 0).In(startDate.Location())
		if createdAt.Before(virtualStart) {
			eligible[contestID] = false
		}
	}

	attempts := make(map[int]*contestAttempt)
	for contestID, allowed := range eligible {
		if !allowed {
			continue
		}
		attempts[contestID] = &contestAttempt{
			contestID: contestID,
			virtualAt: firstVirtualStart[contestID],
			problems:  make(map[string]*problemState),
		}
	}

	for _, submission := range filtered {
		attempt, ok := attempts[submission.ContestID]
		if !ok {
			continue
		}

		participantType := submission.Author.ParticipantType
		if participantType != "VIRTUAL" && participantType != "PRACTICE" {
			continue
		}

		if participantType == "VIRTUAL" {
			start := time.Unix(submission.Author.StartTimeSeconds, 0).In(startDate.Location())
			if !start.Equal(attempt.virtualAt) {
				continue
			}
		}

		state := ensureProblemState(attempt, submission)
		applySubmission(state, participantType, submission)
	}

	for contestID, attempt := range attempts {
		if len(attempt.problems) == 0 {
			delete(attempts, contestID)
		}
	}

	return attempts
}

func ensureProblemState(attempt *contestAttempt, submission codeforces.Submission) *problemState {
	state, ok := attempt.problems[submission.Problem.Index]
	if ok {
		return state
	}

	state = &problemState{
		index: submission.Problem.Index,
		name:  submission.Problem.Name,
	}
	attempt.problems[submission.Problem.Index] = state
	return state
}

func applySubmission(state *problemState, participantType string, submission codeforces.Submission) {
	state.hasAttempt = true
	if state.hasAccepted {
		return
	}

	verdict := submission.Verdict
	if verdict == "OK" {
		state.hasAccepted = true
		if participantType == "VIRTUAL" {
			state.acceptedInVirtual = true
		} else if participantType == "PRACTICE" {
			state.acceptedInPractice = true
		}
		return
	}

	if verdict == "COMPILATION_ERROR" || submission.PassedTestCount == 0 {
		return
	}

	state.wrongBeforeAC++
}

func (s *Service) fillProblemMetadata(ctx context.Context, contestMap map[int]*contestMeta, neededContestIDs map[int]struct{}) error {
	contestIDs := make([]int, 0, len(neededContestIDs))
	for contestID := range neededContestIDs {
		contestIDs = append(contestIDs, contestID)
	}
	sort.Ints(contestIDs)

	for _, contestID := range contestIDs {
		meta := contestMap[contestID]
		if meta == nil || len(meta.problemOrder) > 0 {
			continue
		}

		standings, err := s.client.GetContestStandings(ctx, contestID)
		if err != nil {
			return fmt.Errorf("fetch standings for contest %d: %w", contestID, err)
		}

		meta.problemOrder = make([]string, 0, len(standings.Problems))
		for _, problem := range standings.Problems {
			meta.problemOrder = append(meta.problemOrder, problem.Index)
			meta.problemNames[problem.Index] = problem.Name
		}
	}

	return nil
}

func indexUsersInfo(usersInfo []codeforces.UserInfo) map[string]codeforces.UserInfo {
	indexed := make(map[string]codeforces.UserInfo, len(usersInfo))
	for _, userInfo := range usersInfo {
		indexed[strings.ToLower(userInfo.Handle)] = userInfo
	}
	return indexed
}

func loadContestMetadataCache(contestMap map[int]*contestMeta) {
	file, err := os.Open(contestMetadataCachePath)
	if err != nil {
		return
	}
	defer file.Close()

	var cache contestMetadataCache
	if err := json.NewDecoder(file).Decode(&cache); err != nil {
		return
	}

	for contestID, entry := range cache.Contests {
		meta := contestMap[contestID]
		if meta == nil || len(meta.problemOrder) > 0 {
			continue
		}
		meta.problemOrder = append([]string(nil), entry.ProblemOrder...)
		meta.problemNames = cloneProblemNames(entry.ProblemNames)
	}
}

func saveContestMetadataCache(contestMap map[int]*contestMeta) error {
	cache := contestMetadataCache{Contests: make(map[int]contestMetadataEntry)}
	for contestID, meta := range contestMap {
		if meta == nil || len(meta.problemOrder) == 0 {
			continue
		}
		cache.Contests[contestID] = contestMetadataEntry{
			ProblemOrder: append([]string(nil), meta.problemOrder...),
			ProblemNames: cloneProblemNames(meta.problemNames),
		}
	}

	if err := os.MkdirAll(filepath.Dir(contestMetadataCachePath), 0o755); err != nil {
		return err
	}

	file, err := os.Create(contestMetadataCachePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cache)
}

func cloneProblemNames(input map[string]string) map[string]string {
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func renderUserDashboard(user analyzedUser, contestMap map[int]*contestMeta) UserDashboard {
	attempts := make([]*contestAttempt, 0, len(user.attempts))
	for _, attempt := range user.attempts {
		attempts = append(attempts, attempt)
	}

	sort.Slice(attempts, func(i, j int) bool {
		if attempts[i].virtualAt.Equal(attempts[j].virtualAt) {
			return attempts[i].contestID > attempts[j].contestID
		}
		return attempts[i].virtualAt.After(attempts[j].virtualAt)
	})

	rows := make([]ContestRow, 0, len(attempts))
	for _, attempt := range attempts {
		rows = append(rows, renderContestRow(attempt, contestMap[attempt.contestID]))
	}

	return UserDashboard{
		Handle:        user.handle,
		AvatarURL:     user.avatarURL,
		CurrentRating: user.current,
		MaxRating:     user.max,
		Contests:      rows,
	}
}

func renderContestRow(attempt *contestAttempt, meta *contestMeta) ContestRow {
	location := attempt.virtualAt.Location()
	order := mergedProblemOrder(meta, attempt)
	problems := make([]ProblemCell, 0, len(order))

	for _, index := range order {
		state, exists := attempt.problems[index]
		problemName := ""
		if meta != nil {
			problemName = meta.problemNames[index]
		}
		if problemName == "" && exists {
			problemName = state.name
		}

		cell := ProblemCell{
			Index:         index,
			Name:          problemName,
			URL:           problemURL(attempt.contestID, index),
			Status:        "pending",
			AttemptsLabel: "--",
		}

		if exists {
			cell.Status = "attempt"
			cell.Attempts = state.wrongBeforeAC
			if state.acceptedInVirtual {
				cell.Status = "virtual"
			} else if state.acceptedInPractice {
				cell.Status = "upsolved"
			}

			if state.hasAccepted {
				if state.wrongBeforeAC == 0 {
					cell.AttemptsLabel = "+"
				} else {
					cell.AttemptsLabel = fmt.Sprintf("+%d", state.wrongBeforeAC)
				}
			} else if state.hasAttempt {
				cell.AttemptsLabel = fmt.Sprintf("-%d", state.wrongBeforeAC)
			}
		}

		problems = append(problems, cell)
	}

	name := fmt.Sprintf("Contest %d", attempt.contestID)
	startedAt := ""
	if meta != nil {
		name = meta.contest.Name
		startedAt = time.Unix(meta.contest.StartTimeSeconds, 0).In(location).Format("2006-01-02 15:04")
	}

	return ContestRow{
		ContestID:           attempt.contestID,
		Name:                name,
		URL:                 contestURL(attempt.contestID),
		FriendsStandingsURL: friendsStandingsURL(attempt.contestID),
		Problems:            problems,
		StartedAt:           startedAt,
		VirtualAt:           attempt.virtualAt.Format("2006-01-02 15:04"),
	}
}

func mergedProblemOrder(meta *contestMeta, attempt *contestAttempt) []string {
	seen := make(map[string]struct{})
	order := make([]string, 0)
	if meta != nil {
		order = append(order, meta.problemOrder...)
		for _, index := range meta.problemOrder {
			seen[index] = struct{}{}
		}
	}

	extra := make([]string, 0)
	for index := range attempt.problems {
		if _, ok := seen[index]; ok {
			continue
		}
		extra = append(extra, index)
	}
	sort.Slice(extra, func(i, j int) bool {
		return problemOrderLess(extra[i], extra[j])
	})

	return append(order, extra...)
}

func problemOrderLess(left, right string) bool {
	if len(left) == 0 || len(right) == 0 {
		return left < right
	}
	if left[0] != right[0] {
		return left < right
	}
	return left < right
}

func contestURL(contestID int) string {
	return fmt.Sprintf("https://codeforces.com/contest/%d", contestID)
}

func problemURL(contestID int, index string) string {
	return fmt.Sprintf("https://codeforces.com/contest/%d/problem/%s", contestID, index)
}

func friendsStandingsURL(contestID int) string {
	return fmt.Sprintf("https://codeforces.com/contest/%d/standings/friends/true", contestID)
}

func ratingRank(rating int) string {
	switch {
	case rating >= 3000:
		return "legendary grandmaster"
	case rating >= 2600:
		return "international grandmaster"
	case rating >= 2400:
		return "grandmaster"
	case rating >= 2300:
		return "international master"
	case rating >= 2100:
		return "master"
	case rating >= 1900:
		return "candidate master"
	case rating >= 1600:
		return "expert"
	case rating >= 1400:
		return "specialist"
	case rating >= 1200:
		return "pupil"
	case rating > 0:
		return "newbie"
	default:
		return "unrated"
	}
}
