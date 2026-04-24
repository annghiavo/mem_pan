package fsrs

import (
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"mem_pan/services/study-service/internal/db"
)

func DBStateToFSRS(state string) gofsrs.State {
	switch state {
	case "learning":
		return gofsrs.Learning
	case "review":
		return gofsrs.Review
	case "relearning":
		return gofsrs.Relearning
	default:
		return gofsrs.New
	}
}

func FSRSStateToString(state gofsrs.State) string {
	switch state {
	case gofsrs.Learning:
		return "learning"
	case gofsrs.Review:
		return "review"
	case gofsrs.Relearning:
		return "relearning"
	default:
		return "new"
	}
}

func UserCardToFSRS(uc db.UserCard) gofsrs.Card {
	c := gofsrs.Card{
		Stability:     uc.Stability,
		Difficulty:    uc.Difficulty,
		ElapsedDays:   uint64(uc.ScheduledDays),
		ScheduledDays: uint64(uc.ScheduledDays),
		Reps:          uint64(uc.Reps),
		Lapses:        uint64(uc.Lapses),
		State:         DBStateToFSRS(uc.State),
		Due:           uc.NextReviewDate,
	}
	if uc.LastReviewDate.Valid {
		c.LastReview = uc.LastReviewDate.Time
	}
	return c
}

type ScheduleResult struct {
	Card      gofsrs.Card
	ReviewLog gofsrs.ReviewLog
}

func Schedule(params gofsrs.Parameters, card gofsrs.Card, rating gofsrs.Rating, now time.Time) ScheduleResult {
	f := gofsrs.NewFSRS(params)
	info := f.Next(card, now, rating)
	return ScheduleResult{
		Card:      info.Card,
		ReviewLog: info.ReviewLog,
	}
}
