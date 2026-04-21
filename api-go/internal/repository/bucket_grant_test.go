package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role BucketRole
		want bool
	}{
		{RoleOwner, true},
		{RoleEditor, true},
		{RoleViewer, true},
		{"admin", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ValidRole(tt.role); got != tt.want {
			t.Errorf("ValidRole(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestRoleAtLeast(t *testing.T) {
	t.Parallel()
	tests := []struct {
		have     BucketRole
		required BucketRole
		want     bool
	}{
		// Owner has at least everything.
		{RoleOwner, RoleOwner, true},
		{RoleOwner, RoleEditor, true},
		{RoleOwner, RoleViewer, true},
		// Editor has at least editor and viewer.
		{RoleEditor, RoleOwner, false},
		{RoleEditor, RoleEditor, true},
		{RoleEditor, RoleViewer, true},
		// Viewer has at least viewer only.
		{RoleViewer, RoleOwner, false},
		{RoleViewer, RoleEditor, false},
		{RoleViewer, RoleViewer, true},
		// Invalid role has nothing.
		{"unknown", RoleViewer, false},
	}
	for _, tt := range tests {
		if got := RoleAtLeast(tt.have, tt.required); got != tt.want {
			t.Errorf("RoleAtLeast(%q, %q) = %v, want %v", tt.have, tt.required, got, tt.want)
		}
	}
}

func TestBucketGrantDocumentFilterScopesToBucketAndID(t *testing.T) {
	t.Parallel()
	bucketID := primitive.NewObjectID()
	grantID := primitive.NewObjectID()

	filter := bucketGrantDocumentFilter(bucketID, grantID)

	if len(filter) != 2 {
		t.Fatalf("expected two filter fields, got %d", len(filter))
	}
	if filter["bucket_id"] != bucketID {
		t.Fatalf("expected bucket_id %s, got %v", bucketID.Hex(), filter["bucket_id"])
	}
	if filter["_id"] != grantID {
		t.Fatalf("expected _id %s, got %v", grantID.Hex(), filter["_id"])
	}
}
