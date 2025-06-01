#!/bin/bash
echo "Deleting pipeline..."
kubectl delete pipeline datacollector-deploy-pipeline
kubectl delete pipelineRun datacollector-deploy-run
# TODO: move it to cluster installation
kubectl delete secret git-ssh-key

echo "Creating/updating tasks and pipelines"
kubectl apply -f task-build.yaml
kubectl apply -f task-update-gitops.yaml
kubectl apply -f pipeline.yaml
kubectl apply -f pipeline-run.yaml

echo "Creating/updating secrets with ssh keys"
kubectl create secret generic git-ssh-key \
  --from-file=ssh-privatekey=~.ssh/ham-gitops \
  --from-file=ssh-publickey=.ssh/ham-gitpos.pub \
  --from-literal=known_hosts="$(ssh-keyscan github.com)"