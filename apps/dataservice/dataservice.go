package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	time "time"

	gp "github.com/andrej220/HAM/pkg/graphproc"
	"github.com/andrej220/HAM/pkg/serverutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/andrej220/HAM/pkg/config"
)

// TODO: implement api function to initialize MongoDB

const mongoDBCollection = "mycollection"
const mongoDBDatabase = "appdb"

type SaveOptions struct {
	Overwrite bool
	Prefix    string
	Id        string
	UUID 	  string
}

type dataserviceResponse struct {
}

type dataserviceHandler struct {
	mongodbClient *mongo.Client
	dbConf * DBConfig
}

func NewDataserviceHandler(mdbClient *mongo.Client, dbconf *DBConfig) *dataserviceHandler {
	return &dataserviceHandler{
		mongodbClient: mdbClient,
		dbConf: dbconf,
	}
}

type DataServiceRequest struct {
    CustomerID string           `json:"customerId"`
    DeviceID   string           `json:"deviceId"`
    ConfigUUID string           `json:"configUUID"`
    Output     *gp.Node  		`json:"output"`
    ExecutedAt time.Time        `json:"executedAt"`
}

// SaveToMongo saves data to a MongoDB collection.
func SaveToMongo(data any, collection *mongo.Collection, opts ...SaveOptions) error {
	opt := SaveOptions{
		Overwrite: true,
		Prefix:    "",
		Id:        "",
		UUID: 	   "",
	}

	if len(opts) > 0 {
		opt = opts[0]
	}
	fmt.Println("SaveToMongo: ", opt)
	docID := opt.Prefix +"_"+opt.Id+"_"+opt.UUID

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		if err != nil {
			fmt.Println("Error saving to MongoDB:", err)
			} else  {
		fmt.Println("Save data completed successfully")
		} 
		return err
	}

	return err
}


func (h *dataserviceHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {

	request, ok := r.Context().Value("request").(gp.Graph)
	if !ok {
		http.Error(rw, "Invalid request.", http.StatusBadRequest)
		return
	}
	
	collection := h.mongodbClient.Database(h.dbConf.MongoDBName).Collection(h.dbConf.MongoCollection)
	opt := SaveOptions{
		Overwrite: true,
		Prefix:    "",
		Id:        strconv.Itoa(request.HostCfg.HostID),
		UUID: 	   request.UUID.String(),
	}
	err := SaveToMongo(request, collection, opt)
	if err != nil {
		log.Printf("Failed saving to MongoDB %v:", err)
	}

}

func dbinitialize(MongoDBURI string) (*mongo.Client, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(MongoDBURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v, check DBURI", err)
		return nil, err
	}
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v, ping operation failed", err)
		return nil, err
	}

	log.Println("Connected to MongoDB")
	return client, nil
}

func dbCloseConnection(client *mongo.Client) {
	err := client.Disconnect(context.Background())
	if err != nil {
		log.Fatalf("Failed to disconnect from MongoDB: %v", err)
	}
	log.Println("Disconnected from MongoDB")	
}

func initConfig(path string)(*DataserviceConfig, error){
	store, err := config.NewStore(config.FileStore, &config.FileConfig{Path: path})
    if err != nil {
        return nil, err
    }
    var cfg DataserviceConfig
    if err := store.Load(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func main() {
	
	cfg, err := initConfig(config.GetConfigPath(PROJECTNAME, SERVICENAME, CONFIGFILENAME))
	if err != nil {
		log.Fatalf("Failed to setup configuration, %v", err)
	}

	// TODO: establish connection to PostgreSQL
	mdbClient, err := dbinitialize(cfg.DB.MongoURI)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
		return
	}
	defer dbCloseConnection(mdbClient)
	
	mux := http.NewServeMux()
	handler := NewDataserviceHandler(mdbClient, &cfg.DB.DBConf)
	mux.Handle(cfg.Server.Endpoint, serverutil.NewValidationHandler[gp.Graph](handler,gp.ValidateGraph))
	config:= serverutil.DefaultServerConfig()
	config.Port = cfg.Server.Port
	serverutil.RunServer(mux, config)

	// TODO: implement graceful DB shutdown

}
