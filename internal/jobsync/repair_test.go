package jobsync

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRepairTriggerIsAllowlisted(t *testing.T) {
	if !TriggerRepair.Valid() {
		t.Fatal("repair trigger must be accepted by the collection run model")
	}
}

func TestRepairRejectsNonMidnightOrSameCalendarDatesWithoutDatabase(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	repository := &Repository{}
	scope := StoredScope{ID: "scope"}
	midnight := time.Date(2026, 11, 7, 0, 0, 0, 0, shanghai)
	tests := []struct {
		name   string
		source time.Time
		target time.Time
		want   error
	}{
		{name: "same_day", source: midnight, target: midnight, want: ErrRepairInvalidDates},
		{name: "source_not_midnight", source: midnight.Add(time.Hour), target: midnight.AddDate(0, 0, -1), want: ErrRepairInvalidDates},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := repository.RepairPublishedObservation(context.Background(), scope, test.source, test.target)
			if !errors.Is(err, test.want) {
				t.Fatalf("repair error = %v, want %v", err, test.want)
			}
		})
	}
}
