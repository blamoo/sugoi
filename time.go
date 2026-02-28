package main

import (
	"fmt"
	"math"
	"time"
)

func RelativeTime(t time.Time) string {
	now := float64(time.Now().Unix())
	dt := float64(t.Unix())

	diff := now - dt

	var num float64
	var sc string

	if diff < (60 * 60) {
		num = math.Floor(diff / (60))
		if num == 1 {
			sc = "minute"
		} else {
			sc = "minutes"
		}
	} else if diff < (60 * 60 * 24) {
		num = math.Floor(diff / (60 * 60))
		if num == 1 {
			sc = "hour"
		} else {
			sc = "hours"
		}
	} else if diff < (60 * 60 * 24 * 30) {
		num = math.Floor(diff / (60 * 60 * 24))
		if num == 1 {
			sc = "day"
		} else {
			sc = "days"
		}
	} else if diff < (60 * 60 * 24 * 30 * 12) {
		num = math.Floor(diff / (60 * 60 * 24 * 30))
		if num == 1 {
			sc = "month"
		} else {
			sc = "months"
		}
	} else {
		num = math.Floor(diff / (60 * 60 * 24 * 30 * 12))
		if num == 1 {
			sc = "year"
		} else {
			sc = "years"
		}
	}

	return fmt.Sprintf("%d %s ago", int64(num), sc)
}
