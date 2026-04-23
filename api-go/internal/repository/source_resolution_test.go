package repository

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestOrderSourcesByIDsPreservesRequestedOrder(t *testing.T) {
	firstID := primitive.NewObjectID()
	secondID := primitive.NewObjectID()
	thirdID := primitive.NewObjectID()
	byID := map[primitive.ObjectID]Source{
		firstID:  {ID: firstID, Name: "first"},
		secondID: {ID: secondID, Name: "second"},
		thirdID:  {ID: thirdID, Name: "third"},
	}

	sources, err := orderSourcesByIDs([]primitive.ObjectID{thirdID, firstID, secondID}, byID)
	if err != nil {
		t.Fatalf("orderSourcesByIDs returned error: %v", err)
	}
	got := []string{sources[0].Name, sources[1].Name, sources[2].Name}
	want := []string{"third", "first", "second"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("source %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOrderSourcesByIDsReturnsMissingSourceError(t *testing.T) {
	presentID := primitive.NewObjectID()
	missingID := primitive.NewObjectID()
	byID := map[primitive.ObjectID]Source{
		presentID: {ID: presentID, Name: "present"},
	}

	_, err := orderSourcesByIDs([]primitive.ObjectID{presentID, missingID}, byID)
	if !errors.Is(err, ErrSourcesNotFound) {
		t.Fatalf("expected ErrSourcesNotFound, got %v", err)
	}
	var missing SourcesNotFoundError
	if !errors.As(err, &missing) {
		t.Fatalf("expected SourcesNotFoundError, got %T", err)
	}
	if len(missing.IDs) != 1 || missing.IDs[0] != missingID {
		t.Fatalf("unexpected missing ids: %v", missing.IDs)
	}
}

func TestOrderSourcesByIDsAllowsEmptySourceList(t *testing.T) {
	sources, err := orderSourcesByIDs(nil, map[primitive.ObjectID]Source{})
	if err != nil {
		t.Fatalf("orderSourcesByIDs returned error: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected empty source list, got %v", sources)
	}
}
