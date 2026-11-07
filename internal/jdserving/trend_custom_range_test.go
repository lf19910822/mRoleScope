package jdserving

import (
	"testing"
	"time"
)

func TestResolveTrendCalendarRangeAcceptsInclusiveCustomDates(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, shanghai)
	end := time.Date(2026, time.October, 15, 0, 0, 0, 0, shanghai)
	got, err := resolveTrendCalendarRange(TrendQuery{Range: TrendRangeCustom, StartDate: start, EndDate: end})
	if err != nil {
		t.Fatalf("resolve custom range: %v", err)
	}
	if got.Start == nil || *got.Start != "2026-10-01" || got.End != "2026-10-15" {
		t.Fatalf("custom calendar range = %#v", got)
	}
}

func TestResolveTrendCalendarRangeRejectsReversedOrNonMidnightCustomDates(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, time.October, 15, 0, 0, 0, 0, shanghai)
	end := time.Date(2026, time.October, 1, 0, 0, 0, 0, shanghai)
	if _, err := resolveTrendCalendarRange(TrendQuery{Range: TrendRangeCustom, StartDate: start, EndDate: end}); err != ErrInvalidTrendQuery {
		t.Fatalf("reversed custom range error = %v", err)
	}
	if _, err := resolveTrendCalendarRange(TrendQuery{Range: TrendRangeCustom, StartDate: start.Add(time.Hour), EndDate: start}); err != ErrInvalidTrendQuery {
		t.Fatalf("non-midnight custom range error = %v", err)
	}
}

func TestResolveTrendCalendarRangeAcceptsDatesOutsidePublishedHistoryAndFuture(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(shanghai)
	future := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, shanghai).AddDate(0, 0, 1)
	got, err := resolveTrendCalendarRange(TrendQuery{Range: TrendRangeCustom, StartDate: future, EndDate: future})
	if err != nil {
		t.Fatalf("future custom range error = %v", err)
	}
	if got.Start == nil || *got.Start != future.Format("2006-01-02") || got.End != future.Format("2006-01-02") {
		t.Fatalf("future custom range = %#v", got)
	}
}
