package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/andrej220/HAM/pkg/config/configstore"
)
// Ensure MongoStore implements the ConfigStore interface
var _ configstore.ConfigStore = (*MongoStore)(nil)

type MongoStore struct {
	Client     *mongo.Client
	Collection *mongo.Collection
	ID         string // name of a microservice "data-collector"
}

func New(uri, dbName, collName, id string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	//  ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return &MongoStore{
		Client:     client,
		Collection: client.Database(dbName).Collection(collName),
		ID:         id,
	}, nil
}

func (m *MongoStore) Load(out any)  error {
    filter := bson.M{"_id": m.ID}
    res := m.Collection.FindOne(context.Background(), filter)
    
    if err := res.Err(); err != nil {
        if err == mongo.ErrNoDocuments {
            return fmt.Errorf("document with ID %q not found", m.ID)
        }
        return fmt.Errorf("MongoDB FindOne failed: %w", err)
    }
    if err := res.Decode(&out); err != nil {
        return fmt.Errorf("failed to decode document: %w", err)
    }
    return nil
}

func (m *MongoStore) Save(in any) error {
	if in == nil {
		return fmt.Errorf("Save: input parameter must not be nil")
	}
	_, err := m.Collection.ReplaceOne(
		context.Background(),
		bson.M{"_id": m.ID},
		in,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("Save: MongoDB ReplaceOne failed: %w", err)
	}
	return nil
}

func (m *MongoStore) Watch(onChange func()) error {
	return fmt.Errorf("Watch not implemented for MongoDB store")
}
