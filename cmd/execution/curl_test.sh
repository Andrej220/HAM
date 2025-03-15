echo "Start testing API service."
curl localhost:8081/executor -d '{"hostid":0,"scriptid":1}'
curl localhost:8081/executor -d '{"hostid":1,"scriptid":1}'
curl localhost:8081/executor -d '{"hostid":2,"scriptid":1}'

echo "Stop testing"
