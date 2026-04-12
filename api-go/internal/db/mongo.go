package db

import (
	"context"
	"fmt"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	connGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mongo_connected",
		Help: "MongoDB connection status",
	})
)

func init() {
	prometheus.MustRegister(connGauge)
}

type Mongo struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func Connect(ctx context.Context, cfg config.MongoConfig) (*Mongo, error) {
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		connGauge.Set(0)
		return nil, err
	}
	connGauge.Set(1)
	return &Mongo{Client: client, DB: client.Database(cfg.Database)}, nil
}

func (m *Mongo) Close(ctx context.Context) error {
	connGauge.Set(0)
	return m.Client.Disconnect(ctx)
}
