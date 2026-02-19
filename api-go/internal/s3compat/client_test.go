package s3compat

import "testing"

func TestParseConfigRequiresFields(t *testing.T) {
	t.Parallel()
	_, err := ParseConfig(`{"endpoint":"http://localhost:9000"}`)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseConfigSetsDefaultRegion(t *testing.T) {
	t.Parallel()
	cfg, err := ParseConfig(`{"endpoint":"http://localhost:9000","bucket":"b","access_key_id":"a","secret_access_key":"s"}`)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("unexpected region: %s", cfg.Region)
	}
}
