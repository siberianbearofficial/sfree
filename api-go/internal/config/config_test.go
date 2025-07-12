package config

import "testing"

func TestLoadLocal(t *testing.T) {
	t.Setenv("ENV", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mongo.Host != "localhost" {
		t.Fatalf("unexpected host %s", cfg.Mongo.Host)
	}
	if cfg.Mongo.Password != "example" {
		t.Fatalf("unexpected password %s", cfg.Mongo.Password)
	}
}

func TestLoadTestEnv(t *testing.T) {
	t.Setenv("ENV", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mongo.Host != "mongo" {
		t.Fatalf("unexpected host %s", cfg.Mongo.Host)
	}
	if cfg.Mongo.Database != "s3aas_test" {
		t.Fatalf("unexpected db %s", cfg.Mongo.Database)
	}
}
