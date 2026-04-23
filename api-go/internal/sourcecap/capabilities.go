package sourcecap

import (
	"context"
	"errors"
)

var ErrUnsupportedCapability = errors.New("unsupported source capability")

type File struct {
	ID   string
	Name string
	Size int64
}

type Info struct {
	Files        []File
	StorageTotal int64
	StorageUsed  int64
	StorageFree  int64
}

type InfoProvider interface {
	SourceInfo(context.Context) (Info, error)
}

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
)

type Quota struct {
	TotalBytes *int64
	UsedBytes  *int64
	FreeBytes  *int64
}

type Health struct {
	Status     HealthStatus
	ReasonCode string
	Message    string
	Quota      Quota
}

type HealthProber interface {
	ProbeSourceHealth(context.Context) (Health, error)
}
