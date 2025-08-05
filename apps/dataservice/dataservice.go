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

	// TODO: Debugging	
	//body, _ := io.ReadAll(r.Body)
    //fmt.Println("Raw received JSON:", string(body)) // Debug what's actually received
    //
    //var graph gp.Graph
    //if err := json.Unmarshal(body, &graph); err != nil {
    //    http.Error(rw, "Invalid JSON", http.StatusBadRequest)
    //    return
    //}
    //
    //fmt.Printf("Unmarshaled graph: %+v\n", graph) // Debug the parsed structure
    //fmt.Printf("Root children count: %d\n", len(graph.Root.Children))

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
	
	cfg, err := initConfig("./apps/datacollector/config.yaml")
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


//{
	//	"_id": "config_123",
	//	"ConfigUUID": "550e8400-e29b-41d4-a716-446655440000", // Уникальный UUID
	//	"CustomerId": "cust_001",
	//	"HostId": "host_001",
	//	"Configuration": {
		//	  "os": "Ubuntu 20.04",
		//	  "cpu": "Intel Xeon",
		//	  "memory": "16GB",
		//	  "settings": { "port": 8080, "enabled": true }
		//	},
		//	"UpdatedAt": "2025-04-24T10:00:00Z"
		//  }
		
//func SaveDataCollection(ctx context.Context, payload DataCollectionRequest) error
//func SendMetadataToMetadataService(ctx context.Context, meta DataCollectionResult) error
//func GetLatestData(ctx context.Context, customerId, deviceId string) (DataCollection, error)
//func GetHistory(ctx context.Context, customerId, deviceId string, limit, offset int) ([]Archive, error)
//func DeleteHistory(ctx context.Context, customerId, deviceId string) error
//func DeleteOneHistory(ctx context.Context, customerId, deviceId, configUUID string) error
//func GetDataByUUID(ctx context.Context, configUUID string) (DataCollection, error)

//// Full output, saved in MongoDB
//type DataCollection struct {
	//    CustomerID string
	//    DeviceID   string
//    ConfigUUID string
//    ScriptID   string
//    Output     map[string]interface{}
//    ExecutedAt time.Time
//}
//
//// Metadata (lightweight), sent to Metadata Service
//type DataCollectionResult struct {
//    CustomerID string
//    DeviceID   string
//    ConfigUUID string
//    ScriptID   string
//    Status     string  // pending, success, failed
//    Details    string  // optional: "timeout", "error parsing"
//    ExecutedAt time.Time
//}

//POST /dataservice-results/
//Body:
//{
//  "customerId": "cust123",
//  "deviceId": "dev123",
//  "configUUID": "cfg-uuid",
//  "scriptId": "script456",
//  "status": "success",
//  "executedAt": "2025-04-27T10:00:00Z"
//}

//type Customer struct {
//    CustomerID string
//    Name       string
//    Email      string
//}
//
//type Device struct {
//    DeviceID      string
//    CustomerID    string
//    Model         string
//    System        string
//    LinkedScripts []string
//}
//
//type Script struct {
//    ScriptID              string
//    Name                  string
//    Version               string
//    RemoteHost            string
//    Login                 string
//    Password              string
//    ApplicableDeviceTypes []string
//    ApplicableSystems     []string
//    CreatedBy             string
//    CreatedAt             time.Time
//    UpdatedAt             time.Time
//}




