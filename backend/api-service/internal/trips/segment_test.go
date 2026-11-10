package trips

import (
	"math"
	"testing"
	"time"
)

func speed(v float64) *float64 { return &v }

func baseTime() time.Time {
	return time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
}

func TestSegmentStationaryProducesNoTrips(t *testing.T) {
	points := []Point{
		{ID: 1, Lat: 35.0, Lon: -6.0, Speed: speed(0), RecordedAt: baseTime()},
		{ID: 2, Lat: 35.0, Lon: -6.0, Speed: nil, RecordedAt: baseTime().Add(time.Minute)},
	}
	actions, st := Segment(points, State{}, 2, 5*time.Minute)
	if len(actions) != 0 {
		t.Fatalf("expected no actions, got %d", len(actions))
	}
	if st.Open {
		t.Fatal("state should not be open")
	}
	if st.Cursor != 2 {
		t.Fatalf("cursor = %d, want 2", st.Cursor)
	}
}

func TestSegmentSingleMovingFixIsNotATrip(t *testing.T) {
	points := []Point{
		{ID: 1, Lat: 35.0, Lon: -6.0, Speed: speed(40), RecordedAt: baseTime()},
	}
	actions, st := Segment(points, State{}, 2, 5*time.Minute)
	if len(actions) != 0 {
		t.Fatalf("expected no actions, got %+v", actions)
	}
	if st.Open {
		t.Fatal("a single moving fix must not open a trip")
	}
	if st.Cursor != 0 {
		t.Fatalf("cursor = %d, want 0 so the candidate is re-read", st.Cursor)
	}
	if !st.Pending {
		t.Fatal("candidate should stay pending")
	}
}

func TestSegmentStartsAndUpdatesTrip(t *testing.T) {
	t0 := baseTime()
	points := []Point{
		{ID: 10, Lat: 35.0000, Lon: -6.0000, Speed: speed(30), RecordedAt: t0},
		{ID: 11, Lat: 35.0010, Lon: -6.0000, Speed: speed(40), RecordedAt: t0.Add(30 * time.Second)},
		{ID: 12, Lat: 35.0020, Lon: -6.0000, Speed: speed(45), RecordedAt: t0.Add(60 * time.Second)},
	}
	actions, st := Segment(points, State{}, 2, 5*time.Minute)

	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d (%+v)", len(actions), actions)
	}
	if actions[0].Kind != ActionStart {
		t.Fatalf("first action = %s, want start", actions[0].Kind)
	}
	if !actions[0].At.Equal(t0) {
		t.Fatalf("trip start = %s, want %s", actions[0].At, t0)
	}
	if actions[1].Kind != ActionUpdate || actions[2].Kind != ActionUpdate {
		t.Fatalf("expected two updates, got %s and %s", actions[1].Kind, actions[2].Kind)
	}
	if !st.Open {
		t.Fatal("trip should still be open")
	}
	// Two ~111 m legs.
	if st.DistanceM < 200 || st.DistanceM > 240 {
		t.Fatalf("distance = %.1f m, want about 222 m", st.DistanceM)
	}
	if st.Cursor != 12 {
		t.Fatalf("cursor = %d, want 12", st.Cursor)
	}
}

func TestSegmentClosesAfterStationaryDuration(t *testing.T) {
	t0 := baseTime()
	points := []Point{
		{ID: 1, Lat: 35.0, Lon: -6.0, Speed: speed(30), RecordedAt: t0},
		{ID: 2, Lat: 35.001, Lon: -6.0, Speed: speed(35), RecordedAt: t0.Add(30 * time.Second)},
		{ID: 3, Lat: 35.001, Lon: -6.0, Speed: speed(0), RecordedAt: t0.Add(1 * time.Minute)},
		{ID: 4, Lat: 35.001, Lon: -6.0, Speed: speed(0), RecordedAt: t0.Add(7 * time.Minute)},
	}
	actions, st := Segment(points, State{}, 2, 5*time.Minute)

	var closed bool
	for _, a := range actions {
		if a.Kind == ActionClose {
			closed = true
			if !a.At.Equal(t0.Add(30 * time.Second)) {
				t.Fatalf("closed at %s, want last moving time %s", a.At, t0.Add(30*time.Second))
			}
		}
	}
	if !closed {
		t.Fatalf("expected a close action, got %+v", actions)
	}
	if st.Open {
		t.Fatal("state should be closed")
	}
	if st.Cursor != 4 {
		t.Fatalf("cursor = %d, want 4", st.Cursor)
	}
}

func TestSegmentSplitsAcrossLongGap(t *testing.T) {
	t0 := baseTime()
	points := []Point{
		{ID: 1, Lat: 35.0, Lon: -6.0, Speed: speed(30), RecordedAt: t0},
		{ID: 2, Lat: 35.01, Lon: -6.0, Speed: speed(50), RecordedAt: t0.Add(30 * time.Second)},
		{ID: 3, Lat: 35.05, Lon: -6.0, Speed: speed(50), RecordedAt: t0.Add(30 * time.Minute)},
		{ID: 4, Lat: 35.06, Lon: -6.0, Speed: speed(50), RecordedAt: t0.Add(30*time.Minute + 30*time.Second)},
	}
	actions, st := Segment(points, State{}, 2, 5*time.Minute)

	if len(actions) != 5 {
		t.Fatalf("expected start + update + close + start + update, got %+v", actions)
	}
	if actions[0].Kind != ActionStart {
		t.Fatalf("actions[0] = %s, want start", actions[0].Kind)
	}
	if actions[2].Kind != ActionClose {
		t.Fatalf("actions[2] = %s, want close", actions[2].Kind)
	}
	if actions[3].Kind != ActionStart {
		t.Fatalf("actions[3] = %s, want start", actions[3].Kind)
	}
	if !st.Open {
		t.Fatal("new trip should be open")
	}
}

func TestSegmentResumesOpenTrip(t *testing.T) {
	t0 := baseTime()
	st := State{
		Open:         true,
		TripStart:    t0,
		LastMovingAt: t0,
		LastLat:      35.0,
		LastLon:      -6.0,
		DistanceM:    100,
		Cursor:       5,
	}
	points := []Point{
		{ID: 6, Lat: 35.001, Lon: -6.0, Speed: speed(30), RecordedAt: t0.Add(30 * time.Second)},
	}
	actions, next := Segment(points, st, 2, 5*time.Minute)
	if len(actions) != 1 || actions[0].Kind != ActionUpdate {
		t.Fatalf("expected one update, got %+v", actions)
	}
	if next.DistanceM <= 100 {
		t.Fatalf("distance did not grow: %.1f", next.DistanceM)
	}
	if next.Cursor != 6 {
		t.Fatalf("cursor = %d, want 6", next.Cursor)
	}
}

func TestSegmentIgnoresFixesAtOrBeforeCursor(t *testing.T) {
	t0 := baseTime()
	points := []Point{
		{ID: 5, Lat: 35.0, Lon: -6.0, Speed: speed(40), RecordedAt: t0},
		{ID: 6, Lat: 35.001, Lon: -6.0, Speed: speed(40), RecordedAt: t0.Add(30 * time.Second)},
	}
	actions, st := Segment(points, State{Cursor: 5}, 2, 5*time.Minute)
	// The first point is already consumed, so only one new moving fix exists:
	// it becomes the pending candidate and no trip opens yet.
	if len(actions) != 0 {
		t.Fatalf("expected no actions, got %+v", actions)
	}
	if st.Cursor != 5 {
		t.Fatalf("cursor = %d, want 5", st.Cursor)
	}
	if !st.Pending {
		t.Fatal("fix after cursor should become the pending candidate")
	}
}

func TestHaversineM(t *testing.T) {
	// One degree of latitude is about 111.2 km.
	got := HaversineM(35.0, -6.0, 36.0, -6.0)
	if math.Abs(got-111195) > 500 {
		t.Fatalf("HaversineM = %.0f, want about 111195", got)
	}
}
