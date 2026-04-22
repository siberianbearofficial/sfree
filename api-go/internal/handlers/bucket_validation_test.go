package handlers

import (
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/manager"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidateSourceWeightsRejectsInvalidKey(t *testing.T) {
	t.Parallel()
	sourceID := primitive.NewObjectID()

	err := validateSourceWeights(map[string]int{"not-an-object-id": 1}, []primitive.ObjectID{sourceID})
	if err == nil {
		t.Fatal("expected invalid source weight key error")
	}
	if !strings.Contains(err.Error(), "valid source id") {
		t.Fatalf("expected useful error, got %q", err.Error())
	}
}

func TestValidateSourceWeightsRejectsUnknownSource(t *testing.T) {
	t.Parallel()
	attachedSourceID := primitive.NewObjectID()
	unknownSourceID := primitive.NewObjectID()

	err := validateSourceWeights(map[string]int{unknownSourceID.Hex(): 1}, []primitive.ObjectID{attachedSourceID})
	if err == nil {
		t.Fatal("expected unknown source weight key error")
	}
	if !strings.Contains(err.Error(), "not attached") {
		t.Fatalf("expected useful error, got %q", err.Error())
	}
}

func TestValidateSourceWeightsRejectsNonPositiveWeight(t *testing.T) {
	t.Parallel()
	sourceID := primitive.NewObjectID()

	err := validateSourceWeights(map[string]int{sourceID.Hex(): 0}, []primitive.ObjectID{sourceID})
	if err == nil {
		t.Fatal("expected non-positive source weight error")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Fatalf("expected useful error, got %q", err.Error())
	}
}

func TestValidateSourceWeightsRejectsOverLimitWeight(t *testing.T) {
	t.Parallel()
	sourceID := primitive.NewObjectID()

	err := validateSourceWeights(map[string]int{sourceID.Hex(): manager.MaxWeightedSourceWeight + 1}, []primitive.ObjectID{sourceID})
	if err == nil {
		t.Fatal("expected over-limit source weight error")
	}
	if !strings.Contains(err.Error(), "must be <=") {
		t.Fatalf("expected useful error, got %q", err.Error())
	}
}

func TestValidateSourceWeightsAllowsMissingDefaultWeight(t *testing.T) {
	t.Parallel()
	firstSourceID := primitive.NewObjectID()
	secondSourceID := primitive.NewObjectID()

	err := validateSourceWeights(map[string]int{firstSourceID.Hex(): 2}, []primitive.ObjectID{firstSourceID, secondSourceID})
	if err != nil {
		t.Fatalf("expected missing weight to default later, got %v", err)
	}
}
