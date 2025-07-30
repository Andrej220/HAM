// recives API requests and put in Kafka queue
//root@test-pod:/# curl -X GET https://10.42.0.160:8083/datacollectorProducer -d '{"hostid": 2,"scriptid": 2}'

package main

import(
	"net/http"
	"github.com/andrej220/HAM/pkg/lg"
	"github.com/andrej220/HAM/pkg/serverutil"
	"os"
	"github.com/caarlos0/env/v6"
	"context"
	"time"
	dm "github.com/andrej220/HAM/pkg/shared-models"
	"github.com/segmentio/kafka-go"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
)

const (
	serviceName    = "DATACOLLECTORPRODUCER"
	servicePort    = "8083"
	HTTPpath       = "/datacollectorProducer"
	MAXTIMEOUT     time.Duration = 2 * time.Minute
)

type Config struct {
    KafkaBrokers string `env:"KAFKA_BROKERS" envDefault:"kafka.kafka.svc.cluster.local:9092"`
    KafkaTopic   string `env:"KAFKA_TOPIC" envDefault:"remote-requests"`
}

type messageWriter interface {
    WriteMessages(context.Context, ...kafka.Message) error
    Close() error
}

type  Producer struct{
	writer  messageWriter
	lg 		lg.Logger
}


type Handler struct{
	producer 	*Producer
	lg 			lg.Logger
}

func newKafkaProducer(lg lg.Logger, cfg Config) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.KafkaBrokers),
			Topic:    cfg.KafkaTopic,
			Balancer: &kafka.LeastBytes{},
			Async:    false, 
			AllowAutoTopicCreation: true,
		},
		lg: lg,
	}
}

func newProducerHandler(cfg  Config, lg lg.Logger) http.Handler {
	producer := newKafkaProducer(lg, cfg)
	handler := &Handler{
		producer: producer,
		lg:       lg,
	}
	lg.Info("Created handler with Kafka producer")
	return handler
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	request, ok := r.Context().Value("request").(dm.Request)
	if !ok {
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(lg.Attach(context.Background(), h.lg), MAXTIMEOUT)

	defer cancel()
	// set new UUID to the request
	request.ExecutionUID = uuid.New()

	message, err := json.Marshal(request)
	if err != nil {
		h.lg.Error("Failed to marshal request", lg.Any("err", err))
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	//h.lg.Info("prepare to send message")
	err = h.producer.writer.WriteMessages(ctx,
		kafka.Message{
			Key:   request.ExecutionUID[:],  
			Value: message,
			Time:  time.Now(),
		},
	)
	//h.lg.Info("Message is sent")

	if err != nil {
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			h.lg.Error("Kafka topic does not exist", 
				//lg.String("topic", kafkaTopic),
				lg.String("action", "Create the topic manually or enable auto-creation"))
		}
		http.Error(rw, "Failed to process request", http.StatusInternalServerError)
		h.lg.Info("Failed to process request", lg.Any("ERROR",err))
		return
	}
	rw.WriteHeader(http.StatusAccepted)
	rw.Write([]byte("Request accepted and queued\n"))
}


func main(){
	cfg    := lg.NewConfigFromFlags(serviceName)
	logger := lg.New(cfg)

	var kafkaCfg Config
	if err := env.Parse(&kafkaCfg); err != nil {
        logger.Error("failed to parse config: %v", lg.Any("error", err))
    }

	logger.Info("starting service ", lg.String("str",serviceName), lg.String("port", servicePort))

	mux := http.NewServeMux()
	handler := newProducerHandler(kafkaCfg, logger)
	mux.Handle(HTTPpath, serverutil.NewValidationHandler[dm.Request](handler))

	config := serverutil.DefaultServerConfig()
	config.Logger = logger
	config.Port = servicePort 
	if err := serverutil.RunServer(mux, config); err != nil {
		logger.Error("Fatal error. Failed to run server: %v", lg.Any("err",err))
		os.Exit(1)
	}
}
