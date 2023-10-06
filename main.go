package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type Color int

const (
	Neutral Color = iota
	Black
	White
)

// 0 w +

const (
	Draw     = "="
	Win      = "1"
	Loss     = "0"
	Bye      = "+"
	Forefeit = "-"
)

var ScoreMap map[string]float64 = map[string]float64{
	Win:      1.0,
	Draw:     0.5,
	Loss:     0.0,
	Forefeit: -1.0,
	Bye:      1.0,
}
var BBpCodePointsMap map[string]float64 = map[string]float64{
	"W": ScoreMap[Win],
	"D": ScoreMap[Draw],
	"F": ScoreMap[Loss],
	"U": ScoreMap[Win],
}

// Logging purposes
// var ReverseScoreMap map[float64]string = map[float64]string{
// 	1.0: "1",
// 	0.5: "½",
// 	0.0: "0",
// }

type ID string

// if a player opponet has this ID, the player is considered to have been given a bye
const BYE_DUMMY_ID ID = "FFFFFFFFFFFF"

type Game struct {
	ID     uint   `json:"id"`
	White  string `json:"white"`
	Black  string `json:"black"`
	Result string `json:"result"`
	Moves  string `json:"moves"`
}

type Tournament struct {
	Name         string
	Participants map[ID]*Player
	Rounds       uint
	RatingsRank  []ID
	PointsRank   []ID
	RanktoIDs    map[int]ID
	Ongoing      bool
}

type Player struct {
	ID        ID
	Rating    int
	Points    float64
	RatingPos int
	PointsPos int
	Results   []Result
}

type Result struct {
	OpponentID ID
	Color      string // "w" or "b"
	Score      string // "1", "½", "0", or "-"
}

type Pair struct {
	White ID
	Black ID
}

type RoundInfo struct {
	Round int
	Pairs []Pair
}

// color is the color the adding player played
func (p *Player) addResult(color Color, score string, opponentID ID) {

	colorStr := "w"
	if color != White {
		colorStr = "b"
	}

	p.Results = append(p.Results, Result{
		OpponentID: opponentID,
		Color:      colorStr,
		Score:      score,
	})
	p.Points += ScoreMap[score]

}

func (t *Tournament) AddParticpant(playerId ID, rating int) *Player {

	_, exists := t.Participants[playerId]
	if exists {
		return nil
	}
	participant := &Player{
		ID:      playerId,
		Rating:  rating,
		Results: make([]Result, 0),
	}

	t.Participants[playerId] = participant
	t.RatingsRank = append(t.RatingsRank, ID(participant.ID))
	return participant
}

// find a way to handle bye
func (t *Tournament) AssignBye(playerId ID) {
	player := t.Participants[playerId]
	player.addResult(White, Bye, BYE_DUMMY_ID)
}

func (t *Tournament) RecordGameResult(whiteID ID, blackID ID, winner Color) {
	playerWhite := t.Participants[whiteID]
	playerBlack := t.Participants[blackID]

	var whiteScore, blackScore string
	switch winner {
	case White:
		whiteScore = Win
		blackScore = Loss
	case Black:
		whiteScore = Loss
		blackScore = Win
	default:
		// Handle draw or other cases here
		whiteScore = Draw
		blackScore = Draw
	}
	// fmt.Printf("%s played %s score: %s : %s \n", whiteID, blackID, ReverseScoreMap[whiteScore], ReverseScoreMap[blackScore])
	playerWhite.addResult(White, whiteScore, blackID)
	playerBlack.addResult(Black, blackScore, whiteID)

}

// Move this logic to ADDPARTICIPANT
func (t *Tournament) Start() {
	sort.Slice(t.RatingsRank, func(i, j int) bool {
		return t.Participants[t.RatingsRank[i]].Rating > t.Participants[t.RatingsRank[j]].Rating
	})

	for idx, ID := range t.RatingsRank {
		position := idx + 1
		t.Participants[ID].RatingPos = position
		t.RanktoIDs[position] = ID
	}
	InitialPointsRank := make([]ID, len(t.RatingsRank))
	copy(InitialPointsRank, t.RatingsRank)
	t.PointsRank = InitialPointsRank
	t.Ongoing = true

}

func havePlayedAgainstEachOther(playerA *Player, playerB *Player) (float64, float64) {
	var playerAScore, playerBScore float64
	for idx, result := range playerA.Results {
		if result.OpponentID == playerB.ID {
			// if result.Color == White {
			playerAScore = ScoreMap[result.Score]
			// this is because both of them will have the result of their games at same index
			playerBScore = ScoreMap[playerB.Results[idx].Score]
			// fmt.Println(playerB.Results[idx].OpponentID == playerA.ID)

		}
	}
	return playerAScore, playerBScore
}

func (t *Tournament) sortPointsRanking() {
	sort.Slice(t.PointsRank, func(i, j int) bool {
		playerA := t.Participants[t.PointsRank[i]]
		playerB := t.Participants[t.PointsRank[j]]

		// Compare total points first
		if playerA.Points != playerB.Points {
			return playerA.Points > playerB.Points
		}
		/*
			other tie breakers
		*/
		// H2H
		directA, directB := havePlayedAgainstEachOther(playerA, playerB)
		if directA != directB {
			return directA > directB
		}
		// Rating-Last resort
		if playerA.Rating != playerB.Rating {
			return playerA.Rating > playerB.Rating
		}
		return true
	})
	for position, ID := range t.PointsRank {
		t.Participants[ID].PointsPos = position + 1
	}

}

func (t *Tournament) toBBp() string {
	header := fmt.Sprintf("012 %s 1110065304\n", t.Name)
	lines := make([]string, len(t.Participants))

	for ratingRank, ID := range t.RatingsRank {
		p := t.Participants[ID]
		results := make([]string, len(p.Results))
		// result.OpponentID supposed to be t.Participants[result.OpponentID]
		for i, result := range p.Results {
			results[i] = fmt.Sprintf("%d %s %s", t.Participants[result.OpponentID].RatingPos, result.Color, result.Score)
		}

		// 001    1      Test0001 Player0001              10000                               1    1     4 w 1
		lines[ratingRank] = fmt.Sprintf("%s%5d%6s%s %s%19d%32.1f%5d%5s%s\n",
			"001", ratingRank+1, "", fmt.Sprintf("Test%04d", ratingRank+1), fmt.Sprintf("Player%04d", ratingRank+1), p.Rating, p.Points, p.PointsPos, "", strings.Join(results, strings.Repeat(" ", 5)))
	}

	footer := fmt.Sprintf("XXR %d\n", t.Rounds)
	for score, value := range BBpCodePointsMap {
		footer += fmt.Sprintf("BB%s%2s%.1f\n", score, "", value)
	}
	return header + strings.Join(lines, "") + footer
}

func readPairFromFile(filePath string, ranktoIDMap map[int]ID) (RoundInfo, error) {
	var roundInfo RoundInfo

	file, err := os.Open(filePath)
	if err != nil {
		return roundInfo, err
	}
	scanner := bufio.NewScanner(file)

	if scanner.Scan() {
		round, err := strconv.Atoi(scanner.Text())
		if err != nil {
			return roundInfo, fmt.Errorf("invalid round information: %s", scanner.Text())
		}
		roundInfo.Round = round
	}
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		whitePos, err := strconv.Atoi(parts[0])
		if err != nil {
			return roundInfo, fmt.Errorf("invalid white ID: %s", parts[0])
		}

		blackPos, err := strconv.Atoi(parts[1])
		if err != nil {
			return roundInfo, fmt.Errorf("invalid black ID: %s", parts[1])
		}

		pair := Pair{
			White: ranktoIDMap[whitePos],
			Black: ranktoIDMap[blackPos],
		}
		roundInfo.Pairs = append(roundInfo.Pairs, pair)
	}
	return roundInfo, nil
}
func (t *Tournament) FetchPairs() ([]Pair, error) {

	err := os.WriteFile("bbp/input.txt", []byte(t.toBBp()), 0644)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("./bbp/bbpPairings.exe", "--dutch", "./bbp/input.txt", "-p", "./bbp/output.txt")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// handle errors that could come from bbpairing
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error running bbpPairings.exe:", err)
		return nil, err
	}

	fmt.Println("bbpPairings.exe completed successfully.")

	RoundInfo, err := readPairFromFile("bbp/output.txt", t.RanktoIDs)

	if err != nil {
		return nil, err
	}

	return RoundInfo.Pairs, nil
}

func NewTournament(name string, rounds uint) *Tournament {
	var participantsMap, rankIDsMap = make(map[ID]*Player), make(map[int]ID)

	// Listed as a participant so its rankPosition ID can be checked and seen to be zero
	participantsMap[BYE_DUMMY_ID] = &Player{ID: BYE_DUMMY_ID}
	// so that 0 pos will always be dummy
	rankIDsMap[0] = BYE_DUMMY_ID

	return &Tournament{
		Name:         name,
		Participants: participantsMap,
		RatingsRank:  make([]ID, 0),
		RanktoIDs:    rankIDsMap,
		Rounds:       rounds,
	}
}

func main() {
	tournament := NewTournament("My Tournament", 5)
	users := []struct {
		Name   string
		Rating int
	}{{"Emmanuel", 3000}, {"Joy", 500}, {"Peniel", 1000}, {"Daniel", 4000}, {"Peace", 200}}

	players := []*Player{}
	for _, u := range users {
		player := tournament.AddParticpant(ID(u.Name), u.Rating)
		if player != nil {
			players = append(players, player)
		}
	}
	tournament.Start()
	fmt.Println("========Initial STANDINGS=======")
	for _, ID := range tournament.RatingsRank {
		p := tournament.Participants[ID]
		fmt.Printf("%-20s %d %d\n", tournament.RanktoIDs[p.RatingPos], p.Rating, p.RatingPos)
	}
	fmt.Println("")

	tournament.RecordGameResult(players[0].ID, players[1].ID, White)
	tournament.RecordGameResult(players[2].ID, players[3].ID, White)
	tournament.AssignBye(players[4].ID)
	tournament.sortPointsRanking()
	fmt.Println("")
	tournament.RecordGameResult(players[3].ID, players[4].ID, White)
	tournament.RecordGameResult(players[2].ID, players[0].ID, Neutral)
	tournament.AssignBye(players[1].ID)
	tournament.sortPointsRanking()

	fmt.Println("========STANDINGS=======")
	for _, ID := range tournament.RatingsRank {
		fmt.Printf("%-20s %d %.1f\n", ID, tournament.Participants[ID].PointsPos, tournament.Participants[ID].Points)

	}

	pairs, err := tournament.FetchPairs()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("=========PAIRING========")
	for _, pair := range pairs {
		if pair.Black == BYE_DUMMY_ID {
			fmt.Printf("%s will be assigned a bye\n", pair.White)
			continue
		}
		fmt.Printf("%s as White vs %s as Black\n", pair.Black, pair.White)
	}

	// fmt.Println("========BBP RANK=======")
	// for idx, ID := range tournament.RatingsRank {
	// 	p := tournament.Participants[ID]
	// 	fmt.Printf("%d %-20s %d %.1f\n", idx+1, ID, p.RatingPos, tournament.Participants[ID].Points)
	// 	// fmt.Printf("%d %-20s %d %d \n", idx, ID, p.Rating, p.RatingPos)
	// }
	// fmt.Println("")

}
