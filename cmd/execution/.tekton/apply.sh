#!/bin/bash
kubectl delete pipeline ham-deploy-pipeline
kubectl delete pipelineRun ham-deploy-run

kubectl apply -f task-build.yaml
kubectl apply -f task-update-gitops.yaml
kubectl apply -f pipeline.yaml
kubectl apply -f pipeline-run.yaml
