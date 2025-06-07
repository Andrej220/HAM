// recives API requests and put in Kafka queue

package main

import(
	"net/http"
	"github.com/andrej220/HAM/pkg/lg"
	"github.com/andrej220/HAM/pkg/serverutil"
	"os"
	"context"

)

const serviceName = "DATACOLLECTORPRODUCER"
const servicePort = "8083"
const HTTPpath = "/datacollectorProducer"
const MAXTIMEOUT time.Duration = 1 * time.Minute

type  Producer struct{

}

type httpRequest struct {

}

type Handler struct{
	lg lg.Logger

}

func newProducerHandler(lg lg.Logger) http.Handler {
	handler := &Handler{lg:lg}
	lg.Info("Created handler")
	return handler
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	request, ok := r.Context().Value("request").(httpRequest)
	if !ok {
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(lg.Attach(context.Background(), h.lg), MAXTIMEOUT)
	
}


func main(){
    cfg    := lg.NewConfigFromFlags(serviceName)
    logger := lg.New(cfg)

	logger.Info("starting service ", lg.String("str",serviceName), lg.String("port", servicePort))

	mux := http.NewServeMux()
	handler := newProducerHandler(logger)
	mux.Handle(HTTPpath, serverutil.NewValidationHandler[httpRequest](handler))

	config := serverutil.DefaultServerConfig()
	config.Logger = logger
	config.Port = servicePort 
	if err := serverutil.RunServer(mux, config); err != nil {
		logger.Error("Fatal error. Failed to run server: %v", lg.Any("err",err))
		os.Exit(1)
	}
}