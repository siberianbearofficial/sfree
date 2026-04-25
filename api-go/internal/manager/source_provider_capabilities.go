package manager

import (
	"context"
	"io"

	"github.com/example/sfree/api-go/internal/sourcecap"
)

type sourceStreamDownloader interface {
	DownloadStream(context.Context, string) (io.ReadCloser, error)
}

func inspectSourceProvider(ctx context.Context, cli SourceClient) (SourceInfo, error) {
	infoClient, ok := cli.(sourcecap.InfoProvider)
	if !ok {
		return SourceInfo{}, ErrUnsupportedSourceType
	}
	info, err := infoClient.SourceInfo(ctx)
	if err != nil {
		return SourceInfo{}, err
	}
	return sourceInfoFromCapability(info), nil
}

func probeSourceHealthProvider(ctx context.Context, cli SourceClient) (sourcecap.Health, error) {
	healthClient, ok := cli.(sourcecap.HealthProber)
	if !ok {
		return sourcecap.Health{}, sourcecap.ErrUnsupportedCapability
	}
	return healthClient.ProbeSourceHealth(ctx)
}

func downloadSourceProviderFile(ctx context.Context, cli SourceClient, fileID string) (io.ReadCloser, error) {
	if streamClient, ok := cli.(sourceStreamDownloader); ok {
		return streamClient.DownloadStream(ctx, fileID)
	}
	return cli.Download(ctx, fileID)
}

func sourceInfoFromCapability(info sourcecap.Info) SourceInfo {
	respFiles := make([]SourceInfoFile, 0, len(info.Files))
	for _, f := range info.Files {
		respFiles = append(respFiles, SourceInfoFile{ID: f.ID, Name: f.Name, Size: f.Size})
	}
	return SourceInfo{
		Files:        respFiles,
		StorageTotal: info.StorageTotal,
		StorageUsed:  info.StorageUsed,
		StorageFree:  info.StorageFree,
	}
}
