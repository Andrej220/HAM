package dataservice 

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type Options struct {
    Overwrite bool
    Prefix    string
    Indent    string
}

func WriteFile(data any, filename string, opts ...Options) error{

	opt := Options{
        Overwrite: true,
        Prefix:    "",
        Indent:    "    ",
    }

    if len(opts) > 0 {
        opt = opts[0]
    }

	if filename == "" {
        return os.ErrInvalid
    }

	if _, err := os.Stat(filename); !os.IsNotExist(err) && !opt.Overwrite {
        return os.ErrExist
    }

    err := os.MkdirAll(filepath.Dir(filename), 0755)
    if err != nil {
        return err
    }

    jsonBytes, err := json.MarshalIndent(data, opt.Prefix, opt.Indent)
    if err != nil {
        return err
    }
    return  os.WriteFile(filename, jsonBytes, 0644)
}

func SaveData(data any) error {
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://root:BnJlZJ7DOy@localhost:27017/appdb?authSource=admin"))
    if err != nil {
		return err
	}
    collection := client.Database("appdb").Collection("mycollection")

	opt := Options{
		Overwrite: true,
		Prefix:    "",
		Indent:    "    ",
	}    
    SaveToMongo(data, collection, "mydata1",  opt)
    return nil
}

// SaveToMongo saves data to a MongoDB collection.
func SaveToMongo(data any, collection *mongo.Collection, id string, opts ...Options) error {
	opt := Options{
		Overwrite: true,
		Prefix:    "",
		Indent:    "    ",
	}

	if len(opts) > 0 {
		opt = opts[0]
	}

	if id == "" {
		return errors.New("invalid ID")
	}

	docID := opt.Prefix + id

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existing bson.M
	err := collection.FindOne(ctx, bson.M{"_id": docID}).Decode(&existing)
	if err == nil && !opt.Overwrite {
		return errors.New("document already exists")
	}

	if err == nil && opt.Overwrite || err == mongo.ErrNoDocuments {
		doc := bson.M{"_id": docID}
		dataBytes, err := bson.Marshal(data)
		if err != nil {
			return err
		}
		if err := bson.Unmarshal(dataBytes, &doc); err != nil {
			return err
		}

		upsert := true
		_, err = collection.ReplaceOne(
			ctx,
			bson.M{"_id": docID},
			doc,
			&options.ReplaceOptions{Upsert: &upsert},
		)
		return err
	}

	return err
}


//import (
//	"context"
//	"yourmodule/dataservice"
//	"go.mongodb.org/mongo-driver/mongo"
//	"go.mongodb.org/mongo-driver/mongo/options"
//)
//
//func main() {
//	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
//	if err != nil {
//		panic(err)
//	}
//	collection := client.Database("mydb").Collection("mycollection")
//
//	data := map[string]string{"name": "Alice", "role": "Admin"}
//	err = dataservice.SaveToMongo(data, collection, "user1", dataservice.Options{
//		Overwrite: true,
//		Prefix:    "users/",
//	})
//	if err != nil {
//		panic(err)
//	}
//}