#!/bin/bash

echo "Deleting old Tasks, Pipelines, PipelineRuns, and Secrets…"
kubectl delete task build-push      || true
kubectl delete task update-gitops   || true
kubectl delete pipeline microservice-deploy-pipeline    || true
kubectl delete pipelinerun datacollector-deploy-to-test  || true
kubectl delete pipelinerun datacollector-deploy-to-production || true


kubectl delete secret git-ssh-key   || true

echo "Creating/updating Tasks and Pipelines…"
kubectl apply -f tasks/task-build.yaml
kubectl apply -f tasks/task-update-gitops.yaml
kubectl apply -f pipelines/pipeline.yaml
kubectl apply -f pipelines/datacollector-pipeline-run.yaml

echo "Creating/updating SSH secret for GitOps…"
kubectl create secret generic git-ssh-key \
  --from-file=ssh-privatekey=~/.ssh/ham-gitops \
  --from-file=ssh-publickey=~/.ssh/ham-gitops.pub \
  --from-literal=known_hosts="$(ssh-keyscan github.com)"
